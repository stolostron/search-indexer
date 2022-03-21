package database

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

var ExistingClustersMap map[string]interface{} // a map to hold Current clusters and properties

func (dao *DAO) UpsertCluster(resource model.Resource) {
	data, _ := json.Marshal(resource.Properties)
	clusterName := resource.Properties["name"].(string)
	query := "INSERT INTO search.resources as r (uid, cluster, data) values($1,'',$2) ON CONFLICT (uid) DO UPDATE SET data=$2 WHERE r.uid=$1"
	args := []interface{}{resource.UID, string(data)}

	// Insert cluster node if cluster does not exist in the DB
	if !dao.ClusterInDB(clusterName) || !dao.ClusterPropsUpToDate(clusterName, resource) {
		_, err := dao.pool.Exec(context.TODO(), query, args...)
		if err != nil {
			klog.Warningf("Error inserting/updating cluster with query %s, %s: %s ", query, clusterName, err.Error())
		} else {
			ExistingClustersMap[resource.UID] = resource.Properties
		}
	} else {
		klog.V(4).Infof("Cluster %s already exists in DB and properties are up to date.", clusterName)
		return
	}

}

func (dao *DAO) ClusterInDB(clusterName string) bool {
	clusterUID := string("cluster__" + clusterName)
	_, ok := ExistingClustersMap[clusterUID]

	if !ok {
		klog.V(3).Infof("cluster %s not in ExistingClustersMap. Checking in db", clusterName)
		query := "SELECT uid, data from search.resources where uid=$1"
		rows, err := dao.pool.Query(context.TODO(), query, clusterUID)
		if err != nil {
			klog.Errorf("Error while checking if cluster already exists in DB: %s", err.Error())
		}
		if rows != nil {
			for rows.Next() {
				var uid string
				var data interface{}
				err := rows.Scan(&uid, &data)
				if err != nil {
					klog.Errorf("Error %s retrieving rows for query:%s", err.Error(), query)
				} else {
					ExistingClustersMap[uid] = data
				}
			}
		}
		_, ok = ExistingClustersMap[clusterUID]
	}
	return ok
}

func (dao *DAO) ClusterPropsUpToDate(clusterName string, resource model.Resource) bool {
	clusterUID := string("cluster__" + clusterName)
	currProps := resource.Properties
	existingProps, ok := ExistingClustersMap[clusterUID].(map[string]interface{})
	if ok && len(existingProps) == len(currProps) {
		for key, currVal := range currProps {
			existingVal, ok := existingProps[key]

			if !ok || !reflect.DeepEqual(currVal, existingVal) {
				klog.V(4).Infof("Values doesn't match for key:%s, existing value:%s, new value:%s \n", key, existingVal, currVal)
				return false
			}
		}
		return true
	} else {
		klog.V(3).Infof("For cluster %s, properties needs to be updated.", clusterName)
		klog.V(5).Info("existingProps: ", existingProps)
		klog.V(5).Info("currProps: ", currProps)
		return false
	}
}
