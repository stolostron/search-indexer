// Copyright Contributors to the Open Cluster Management project

package clustersync

import (
	"context"

	"github.com/stolostron/search-indexer/pkg/config"
	klog "k8s.io/klog/v2"
)

func ElectLeaderAndStart(ctx context.Context) {
	client := config.Cfg.KubeClient
	lockName := "search-indexer.open-cluster-management.io"
	podName := config.Cfg.PodName
	podNamespace := config.Cfg.PodNamespace

	lock := getNewLock(client, lockName, podName, podNamespace)
	runLeaderElection(ctx, lock)
}

func syncClusters(ctx context.Context) {
	klog.Info("TODO: Start Sync clusters here.")

	<-ctx.Done() // Wait for exit signal.
	klog.Info("Exit syncClusters().")
}
