// Copyright Contributors to the Open Cluster Management project

package clustersync

import (
	"context"
	"time"

	klog "k8s.io/klog/v2"
)

func syncClusters(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			klog.Info("Exit syncClusters(). Received context.Done().")
			return
		default:
			klog.Info("TODO: Sync clusters here.")
		}
		time.Sleep(5 * time.Second)
	}
}
