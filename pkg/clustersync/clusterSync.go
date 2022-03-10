// Copyright Contributors to the Open Cluster Management project

package clustersync

import (
	"context"
	"time"

	klog "k8s.io/klog/v2"
)

func syncClusters(context context.Context) {
	for {
		klog.Info("TODO: Sync clusters here. Need to watch for context cancel().")
		time.Sleep(30 * time.Second)
	}
}
