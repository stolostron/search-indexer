// Copyright Contributors to the Open Cluster Management project

package clustersync

import (
	"context"
	"time"

	"github.com/stolostron/search-indexer/pkg/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	klog "k8s.io/klog/v2"
)

func getNewLock(client *kubernetes.Clientset, lockname, podName, podNamespace string) *resourcelock.LeaseLock {
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
			klog.Info("Exit leader election.")
			return
		default:
			klog.V(1).Info("Attempting to become leader.")
			leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
				Lock:            lock,
				ReleaseOnCancel: true, // Releases the lock on context cancel.
				LeaseDuration:   15 * time.Second,
				RenewDeadline:   10 * time.Second,
				RetryPeriod:     2 * time.Second,
				Callbacks: leaderelection.LeaderCallbacks{
					OnStartedLeading: func(c context.Context) {
						klog.Info("I'm the leader! Starting leader activities.")
						syncClusters(c)
					},
					OnStoppedLeading: func() {
						klog.Info("I'm no longer the leader.")
					},
					OnNewLeader: func(currentId string) {
						if currentId != config.Cfg.PodName {
							klog.Infof("Leader is %s", currentId)
						}
					},
				},
			})
		}
	}
}
