package database

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"

	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

var ExistingClustersCache map[string]interface{} // a map to hold Current clusters and properties

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

var mux sync.RWMutex

func ReadClustersCache(uid string) (interface{}, bool) {
	mux.RLock()
	defer mux.RUnlock()
	data, ok := ExistingClustersCache[uid]
	return data, ok
}

func UpdateClustersCache(uid string, data interface{}) {
	mux.Lock()
	defer mux.Unlock()
	ExistingClustersCache[uid] = data
}
func DeleteClustersCache(uid string) {
	mux.Lock()
	defer mux.Unlock()
	delete(ExistingClustersCache, uid)
}

func (dao *DAO) clusterInDB(clusterUID string) bool {
	_, ok := ReadClustersCache(clusterUID)
	if !ok {
		klog.V(3).Infof("Cluster [%s] is not in ExistingClustersMap. Updating cache with latest state from database.", clusterUID)
		query := "SELECT uid, data from search.resources where uid=$1"
		rows, err := dao.pool.Query(context.TODO(), query, clusterUID)
		if err != nil {
			klog.Errorf("Error while updating ExistingClusterCache from database: %s", err.Error())
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
					klog.V(4).Infof("Values doesn't match for key:%s, existing value:%s, new value:%s \n", key, existingVal, currVal)
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
		klog.V(3).Infof("Cluster [%s] is not in ExistingClustersMap.", clusterUID)
		return false
	}
}
