package database

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func (dao *DAO) DeleteCluster(clusterName string) {
	clusterUID := string("cluster__" + clusterName)
	// Delete resources for cluster from resources table from DB
	_, err := dao.pool.Exec(context.Background(), "DELETE FROM search.resources WHERE cluster=$1", clusterName)
	checkError(err, fmt.Sprintf("Error deleting resources from search.resources for clusterName %s.", clusterName))

	// Delete edges for cluster from DB
	_, err = dao.pool.Exec(context.Background(), "DELETE FROM search.edges WHERE cluster=$1", clusterName)
	checkError(err, fmt.Sprintf("Error deleting resources from search.edges for clusterName %s.", clusterName))

	// Delete cluster node from DB
	_, err = dao.pool.Exec(context.Background(), "DELETE FROM search.resources WHERE uid=$1", clusterUID)
	checkError(err, fmt.Sprintf("Error deleting cluster %s from search.resources.", clusterName))

	DeleteClustersCache(clusterUID)
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
