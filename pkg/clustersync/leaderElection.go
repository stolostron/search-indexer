// Copyright Contributors to the Open Cluster Management project

package clustersync

import (
	"context"
	"time"

	"github.com/stolostron/search-indexer/pkg/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	klog "k8s.io/klog/v2"
)

func getNewLock(lockname, podName, podNamespace string) *resourcelock.LeaseLock {

	client := config.Cfg.KubeClient

	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockname,
			Namespace: podNamespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podName,
		},
	}
}

func runLeaderElection(ctx context.Context, lock *resourcelock.LeaseLock) {
	for {
		select {
		case <-ctx.Done():
			klog.Info("Exit runLeaderElection().")
			return
		default:
			klog.Info("Attempting to become leader.")
			leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
				Lock:            lock,
				ReleaseOnCancel: true, // Releases the lock on context cancel.
				LeaseDuration:   15 * time.Second,
				RenewDeadline:   10 * time.Second,
				RetryPeriod:     2 * time.Second,
				Callbacks: leaderelection.LeaderCallbacks{
					OnStartedLeading: func(c context.Context) {
						klog.Info("I'm the leader! Starting syncClusters()")
						syncClusters(c)
					},
					OnStoppedLeading: func() {
						klog.Info("I'm no longer the leader.")

					},
					OnNewLeader: func(current_id string) {
						if current_id != config.Cfg.PodName {
							klog.Infof("Leader is %s", current_id)
						}
					},
				},
			})
		}
	}
}
