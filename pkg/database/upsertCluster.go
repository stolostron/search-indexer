package database

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"time"

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
		if res, err := tx.Exec(ctx, "DELETE FROM search.resources WHERE cluster=$1", clusterName); err != nil {
			checkErrorAndRollback(err,
				fmt.Sprintf("Error deleting resources from search.resources for clusterName %s.", clusterName), tx, ctx)
			return err
		} else {
			resourcesDeleted = res.RowsAffected()
			rowsDeleted = rowsDeleted + resourcesDeleted
		}

		// Delete edges for cluster from DB
		if res, err := tx.Exec(ctx, "DELETE FROM search.edges WHERE cluster=$1", clusterName); err != nil {
			checkErrorAndRollback(err,
				fmt.Sprintf("Error deleting resources from search.edges for clusterName %s.", clusterName), tx, ctx)
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
	if res, err := dao.pool.Exec(ctx, "DELETE FROM search.resources WHERE uid=$1", clusterUID); err != nil {
		checkError(err, fmt.Sprintf("Error deleting cluster %s from search.resources.", clusterUID))
		return err
	} else {
		rowsDeleted = res.RowsAffected()
	}
	return nil
}

func (dao *DAO) UpsertCluster(resource model.Resource) {
	data, _ := json.Marshal(resource.Properties)
	clusterName := resource.Properties["name"].(string)
	query := "INSERT INTO search.resources as r (uid, cluster, data) values($1,'',$2) ON CONFLICT (uid) DO UPDATE SET data=$2 WHERE r.uid=$1"
	args := []interface{}{resource.UID, string(data)}

	// Insert cluster node if cluster does not exist in the DB
	if !dao.clusterInDB(resource.UID) || !dao.clusterPropsUpToDate(resource.UID, resource) {
		_, err := dao.pool.Exec(context.TODO(), query, args...)
		if err != nil {
			klog.Warningf("Error inserting/updating cluster with query %s, %s: %s ", query, clusterName, err.Error())
		} else {
			UpdateClustersCache(resource.UID, resource.Properties)
		}
	} else {
		klog.V(4).Infof("Cluster %s already exists in DB and properties are up to date.", clusterName)
		return
	}

}

func (dao *DAO) clusterInDB(clusterUID string) bool {
	_, ok := ReadClustersCache(clusterUID)
	if !ok {
		klog.V(3).Infof("Cluster [%s] is not in existingClustersCache. Updating cache with latest state from database.",
			clusterUID)
		query := "SELECT uid, data from search.resources where uid=$1"
		rows, err := dao.pool.Query(context.TODO(), query, clusterUID)
		if err != nil {
			klog.Errorf("Error while fetching cluster %s from database: %s", clusterUID, err.Error())
		}
		if rows != nil {
			for rows.Next() {
				var uid string
				var data interface{}
				err := rows.Scan(&uid, &data)
				if err != nil {
					klog.Errorf("Error %s retrieving rows for query:%s", err.Error(), query)
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
