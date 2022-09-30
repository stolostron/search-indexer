// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"

	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

// Overrides the existing state of a cluster with the new data.
// NOTE: This logic is not optimized. We use the simplest approach because this is a failsafe to
//       recover from rare sync problems. At the moment this is good enough without adding complexity.
func (dao *DAO) ResyncData(event model.SyncEvent, clusterName string, syncResponse *model.SyncResponse) {
	klog.Infof(
		"Resync full state for cluster %s. This is normal, but if this happens often it may signal a sync problem.",
		clusterName)

	_, err := dao.pool.Exec(context.TODO(), "DELETE from search.resources WHERE cluster=$1", clusterName)
	if err != nil {
		klog.Warningf("Error deleting resources during resync of cluster %s. Error: %+v", clusterName, err)
	}
	_, err = dao.pool.Exec(context.TODO(), "DELETE from search.edges WHERE cluster=$1", clusterName)
	if err != nil {
		klog.Warningf("Error deleting edges during resync of cluster %s. Error: %+v", clusterName, err)
	}
	dao.SyncData(event, clusterName, syncResponse)
}
