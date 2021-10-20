// Copyright Contributors to the Open Cluster Management project

package database

import (
	// "context"
	// "encoding/json"
	// "strings"
	// "sync"

	// pgx "github.com/jackc/pgx/v4"
	"context"

	"github.com/open-cluster-management/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func ResyncData(event model.SyncEvent, clusterName string) {
	klog.Info("Resync full state for cluster ", clusterName)

	// Get all the existing resources.
	r, e := pool.Exec(context.Background(), "SELECT uid FROM resources where cluster=$1", clusterName)
	klog.Infof("Got resuls %v %v", r, e)
}
