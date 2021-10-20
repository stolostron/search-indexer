// Copyright Contributors to the Open Cluster Management project

package database

import (
	// "context"
	// "encoding/json"
	// "strings"
	// "sync"

	// pgx "github.com/jackc/pgx/v4"
	"github.com/open-cluster-management/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func ResyncData(event model.SyncEvent, clusterName string) {
	klog.Info("Resync full state for cluster ", clusterName)

}
