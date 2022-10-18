// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"fmt"

	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

// Overrides the existing state of a cluster with the new data.
// NOTE: This logic is not optimized. We use the simplest approach because this is a failsafe to
//       recover from rare sync problems. At the moment this is good enough without adding complexity.
func (dao *DAO) ResyncData(event model.SyncEvent, clusterName string, syncResponse *model.SyncResponse) {
	klog.Infof(
		"Resync full state for cluster %s. This is normal, but it could be a problem if it happens often.",
		clusterName)

	// DELETE from search.resources WHERE cluster=$1
	delResourcesSql, args, delResourcesSqlErr := goquDelete("resources", "cluster", clusterName)
	checkError(delResourcesSqlErr, fmt.Sprintf("Error creating query to delete cluster resources for %s.",
		clusterName))

	_, err := dao.pool.Exec(context.TODO(), delResourcesSql, args)
	if err != nil {
		klog.Warningf("Error deleting resources during resync of cluster %s. Error: %+v", clusterName, err)
	}
	// DELETE from search.edges WHERE cluster=$1
	delEdgesSql, args, delEdgesSqlErr := goquDelete("edges", "cluster", clusterName)
	checkError(delEdgesSqlErr, fmt.Sprintf("Error creating query to delete edges for %s.", clusterName))

	_, err = dao.pool.Exec(context.TODO(), delEdgesSql, args)
	if err != nil {
		klog.Warningf("Error deleting edges during resync of cluster %s. Error: %+v", clusterName, err)
	}
	dao.SyncData(event, clusterName, syncResponse)
}
