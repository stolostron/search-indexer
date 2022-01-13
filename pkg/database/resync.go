// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"

	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func (dao *DAO) ResyncData(event model.SyncEvent, clusterName string) {
	klog.Info("Resync full state for cluster ", clusterName)

	// FIXME: REMOVE THIS WORKAROUND. Deleting data for cluster instead of reconcilimg with existing state.
	// klog.Warningf("FIXME: REMOVE THIS WORKAROUND. Deleting data for cluster [%s] instead of reconcilimg with existing state.", clusterName)
	r, e := dao.pool.Exec(context.Background(), "DELETE from resources WHERE cluster=$1", clusterName)
	klog.V(9).Infof("WORKAROUND. Deleting all resources for cluster [%s]. Result: %+v  Errors: %+v", clusterName, r, e)

	dao.SyncData(event, clusterName)

	// Get all the existing resources.
	// r, e := pool.Exec(context.Background(), "SELECT uid FROM resources where cluster=$1", clusterName)
	// klog.Infof("Got resuls %v %v", r, e)
}
