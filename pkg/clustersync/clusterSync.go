// Copyright Contributors to the Open Cluster Management project

package clustersync

import (
	"context"

	klog "k8s.io/klog/v2"
)

func syncClusters(ctx context.Context) {
	klog.Info("TODO: Start Sync clusters here.")

	<-ctx.Done() // Wait for exit signal.
	klog.Info("Exit syncClusters().")
}
