package database

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/jackc/pgx/v4"
	"github.com/stolostron/search-indexer/pkg/config"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func (dao *DAO) DeleteClusterAndResources(ctx context.Context, clusterName string, deleteClusterNode bool) {
	clusterUID := string("cluster__" + clusterName)
	if err := dao.deleteWithRetry(dao.DeleteClusterResourcesTxn, ctx, clusterName); err == nil {
		klog.V(2).Infof("Successfully deleted resources and edges for cluster %s from database!", clusterName)

	}

	if deleteClusterNode {
		if err := dao.deleteWithRetry(dao.DeleteClusterTxn, ctx, clusterUID); err == nil {
			klog.V(2).Infof("Successfully deleted cluster node %s from database!", clusterName)
			// Delete cluster from existing clusters cache
			DeleteClustersCache(clusterUID)
		}
	}
}

func (dao *DAO) deleteWithRetry(deleteFunction func(context.Context, string) error,
	ctx context.Context, clusterName string) error {

	retry := 0
	cfg := config.Cfg

	// Retry cluster deletion till it succeeds
	for {
		// If a statement within a transaction fails, the transaction can get aborted and rest of the statements
		// can get skipped. So if any statements fail, we retry the entire transaction
		err := deleteFunction(ctx, clusterName)
		if err != nil {
			waitMS := int(math.Min(float64(retry*500), float64(cfg.MaxBackoffMS)))
			timetoSleep := time.Duration(waitMS) * time.Millisecond
			retry++
			klog.Errorf("Unable to process cluster delete transaction: %+v. Retry in %s\n", err, timetoSleep)
			time.Sleep(timetoSleep)
		} else {
			break
		}
	}
	return nil
}

func (dao *DAO) DeleteClusterResourcesTxn(ctx context.Context, clusterName string) error {
	start := time.Now()
	var rowsDeleted, resourcesDeleted, edgesDeleted int64

	defer func() {
		// Log a warning if delete is too slow.
		// Note the 100ms is just an initial guess, we should adjust based on normal execution time.
		if time.Since(start) > 100*time.Millisecond {
			klog.Warningf("Delete of %s took %s. Resources Deleted: %d, Edges Deleted: %d, Total RowsDeleted: %d",
				clusterName, time.Since(start), resourcesDeleted, edgesDeleted, rowsDeleted)
			return
		}
		klog.V(4).Infof("Delete of %s took %s. Resources Deleted: %d, Edges Deleted: %d, Total RowsDeleted: %d",
			clusterName, time.Since(start), resourcesDeleted, edgesDeleted, rowsDeleted)
	}()
	tx, txErr := dao.pool.BeginTx(ctx, pgx.TxOptions{})
	if txErr != nil {
		klog.Error("Error while beginning transaction block for deleting cluster ", clusterName)
		return txErr
	} else {
		// Delete resources for cluster from resources table from DB

		// Create the query
		sql, args, err := goquDelete("resources", "cluster", clusterName)
		checkError(err, fmt.Sprintf("Error creating query to delete cluster resources for %s.", clusterName))
		if err != nil {
			return err
		}
		klog.V(4).Infof("Query to delete cluster resources for %s - sql: %s args: %+v", clusterName, sql, args)

		if res, err := tx.Exec(ctx, sql, args...); err != nil {
			checkErrorAndRollback(err,
				fmt.Sprintf("Error deleting resources from search.resources for clusterName %s.", clusterName), tx, ctx)
			return err
		} else {
			resourcesDeleted = res.RowsAffected()
			rowsDeleted = rowsDeleted + resourcesDeleted
		}

		// Create the query
		sql, args, err = goquDelete("edges", "cluster", clusterName)
		checkError(err, fmt.Sprintf("Error creating query to delete edges for %s.", clusterName))
		if err != nil {
			return err
		}
		// Delete edges for cluster from DB
		if res, err := tx.Exec(ctx, sql, args...); err != nil {
			checkErrorAndRollback(err,
				fmt.Sprintf("Error deleting edges from search.edges for clusterName %s.", clusterName), tx, ctx)
			return err
		} else {
			edgesDeleted = res.RowsAffected()
			rowsDeleted = rowsDeleted + edgesDeleted
		}

		if err := tx.Commit(ctx); err != nil {
			checkErrorAndRollback(err,
				fmt.Sprintf("Error committing delete cluster transaction for cluster: %s.", clusterName), tx, ctx)
			return err
		}
	}
	return nil
}

func (dao *DAO) DeleteClusterTxn(ctx context.Context, clusterUID string) error {
	start := time.Now()
	var rowsDeleted int64

	defer func() {
		klog.V(4).Infof("Delete of %s took %s. Cluster Nodes Deleted: %d", clusterUID, time.Since(start), rowsDeleted)
	}()
	// Delete cluster node from DB.

	// Create the query
	sql, args, err := goquDelete("resources", "uid", clusterUID)
	checkError(err, fmt.Sprintf("Error creating query to delete clusterNode for %s.", clusterUID))
	if err != nil {
		return err
	}
	klog.V(4).Infof("Query to delete clusterNode for %s - sql: %s args: %+v", clusterUID, sql, args)

	if res, err := dao.pool.Exec(ctx, sql, args...); err != nil {
		checkError(err, fmt.Sprintf("Error deleting cluster %s from search.resources.", clusterUID))
		return err
	} else {
		rowsDeleted = res.RowsAffected()
	}
	return nil
}

func (dao *DAO) UpsertCluster(ctx context.Context, resource model.Resource) {
	data, _ := json.Marshal(resource.Properties)
	clusterName := resource.Properties["name"].(string)
	sql, args, err := goquInsertUpdate("resources", []interface{}{resource.UID, clusterName, string(data)})
	checkError(err, fmt.Sprintf("Error creating insert/update cluster query for %s", clusterName))
	if err != nil {
		//TO DO: store the pending cluster resource in cache and retry in case of error
		return
	}
	klog.V(4).Infof("Query to insert/update cluster for %s - sql: %s args: %+v", clusterName, sql, args)
	// Insert cluster node if cluster does not exist in the DB
	if !dao.clusterInDB(ctx, resource.UID) || !dao.clusterPropsUpToDate(resource.UID, resource) {
		_, err := dao.pool.Exec(ctx, sql, args...)
		if err != nil {
			klog.Warningf("Error inserting/updating cluster with query %s, %s: %s ", sql, clusterName, err.Error())
		} else {
			UpdateClustersCache(resource.UID, resource.Properties)
		}
	} else {
		klog.V(4).Infof("Cluster %s already exists in DB and properties are up to date.", clusterName)
		return
	}

}

func (dao *DAO) clusterInDB(ctx context.Context, clusterUID string) bool {
	_, ok := ReadClustersCache(clusterUID)
	if !ok {
		klog.V(3).Infof("Cluster [%s] is not in existingClustersCache. Updating cache with latest state from database.",
			clusterUID)

		// Create the query
		sql, args, err := goqu.From(goqu.S("search").Table("resources")).
			Select(goqu.C("uid"), goqu.C("data")).
			Where(goqu.C("uid").Eq(clusterUID)).ToSQL()
		if err != nil {
			klog.Errorf("Error creating query to check if the cluster node %s is in the database", clusterUID)
			return false // insert/update the cluster node in db
		}
		klog.V(4).Infof("Query to check if the cluster node %s is in the database - sql: %s args: %+v",
			clusterUID, sql, args)
		rows, err := dao.pool.Query(ctx, sql, args...)
		if err != nil {
			klog.Errorf("Error while fetching cluster %s from database: %s", clusterUID, err.Error())
			return false // insert/update the cluster node in db
		}

		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var uid string
				var data interface{}
				err := rows.Scan(&uid, &data)
				if err != nil {
					klog.Errorf("Error %s retrieving rows for clusterInDB query:%s", err.Error(), sql)
				} else {
					UpdateClustersCache(uid, data)
				}
			}
		}
		_, ok = ReadClustersCache(clusterUID)
	}
	return ok
}

func (dao *DAO) clusterPropsUpToDate(clusterUID string, resource model.Resource) bool {
	currProps := resource.Properties
	data, clusterPresent := ReadClustersCache(clusterUID)
	if clusterPresent {
		existingProps, ok := data.(map[string]interface{})
		if ok && len(existingProps) == len(currProps) {
			for key, currVal := range currProps {
				existingVal, ok := existingProps[key]

				if !ok || !reflect.DeepEqual(currVal, existingVal) {
					klog.V(4).Infof("cluster property values doesn't match for key:%s, existing value:%s, new value:%s \n",
						key, existingVal, currVal)
					return false
				}
			}
			return true
		} else {
			klog.V(3).Infof("For cluster %s, properties needs to be updated.", clusterUID)
			klog.V(5).Info("existingProps: ", existingProps)
			klog.V(5).Info("currProps: ", currProps)
			return false
		}
	} else {
		klog.V(3).Infof("Cluster [%s] is not in existingClustersCache.", clusterUID)
		return false
	}
}

// Create a goqu query to delete a row.
// Sample query:
//
//	DELETE from <tableName> WHERE <columnName> = '<arg>' AND uid != 'cluster__<arg>'
func goquDelete(tableName, columnName, arg string) (string, []interface{}, error) {
	var whereDs []exp.Expression
	whereDs = append(whereDs, goqu.C(columnName).Eq(arg))
	if columnName == "cluster" && tableName == "resources" {
		whereDs = append(whereDs, goqu.C("uid").Neq(string("cluster__"+arg))) // do not delete the cluster node
	}

	// Create the query
	sql, args, err := goqu.From(
		goqu.S("search").Table(tableName)).
		Delete().
		Where(whereDs...).ToSQL()
	return sql, args, err
}

// Create the upsert query
// query := "INSERT INTO search.resources as r (uid, cluster, data) values($1,”,$2)
// ON CONFLICT (uid) DO UPDATE SET data=$2 WHERE r.uid=$1"
func goquInsertUpdate(tableName string, args []interface{}) (string, []interface{}, error) {
	sql, args, err := goqu.From(
		goqu.S("search").Table(tableName).As("r")).
		Insert().
		Rows(goqu.Record{"uid": args[0], "cluster": args[1], "data": args[2]}).
		OnConflict(goqu.DoUpdate("uid",
			goqu.C("data").Set(args[2])).
			Where(goqu.L(`"r".uid`).Eq(args[0]))).ToSQL()

	return sql, args, err
}

// Query database for managed clusters:
func (dao *DAO) GetManagedClusters(ctx context.Context, hubClusterName string) ([]string, error) {

	schemaTable := goqu.S("search").Table("resources")
	ds := goqu.From(schemaTable)
	var managedClusters []string

	//select distinct cluster from search.resources;
	query, params, err := ds.Select("cluster").Distinct().ToSQL()
	if err != nil {
		klog.Errorf("Error building select distinct cluster query: %s", err.Error())
		return nil, err
	}
	klog.V(4).Infof("Query database for managed clusters: [%s] ", query)

	rows, err := dao.pool.Query(ctx, query, params...)
	if err != nil {
		klog.Errorf("Error resolving managed clusters query [%s] with params [%+v]. Error: [%+v]", query, params, err)
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		var mc string
		err = rows.Scan(&mc)
		if err != nil {
			klog.Errorf("Error reading cluster names. Error: %s Query: %s", err.Error(), query)
			continue
		}
		if mc != "" && mc != hubClusterName { //exclude the hub cluster and cluster node
			managedClusters = append(managedClusters, mc)
		}

	}
	return managedClusters, nil
}
