// Copyright Contributors to the Open Cluster Management project

package clustersync

import (
	"context"

	klog "k8s.io/klog/v2"
)

func SyncClusters() {
	klog.Info("Electing leader...")
	l := getNewLock("search-indexer.open-cluster-management.io", "open-cluster-management")

	runLeaderElection(l, context.TODO())
}
