// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"

	pgx "github.com/jackc/pgx/v4"
	"k8s.io/klog/v2"
)

// Query resource and edge count for a cluster. Used for data validation.
func (dao *DAO) ClusterTotals(clusterName string) (resources int, edges int) {
	batch := &pgx.Batch{}
	batch.Queue("SELECT count(*) FROM search.resources WHERE cluster=$1", clusterName)
	batch.Queue("SELECT count(*) FROM search.edges WHERE cluster=$1", clusterName)

	br := dao.pool.SendBatch(context.Background(), batch)
	defer br.Close()

	resourcesRow := br.QueryRow()
	resourcesErr := resourcesRow.Scan(&resources)
	if resourcesErr != nil {
		klog.Error("Error reading total resources for cluster ", clusterName)
	}
	edgesRow := br.QueryRow()
	edgesErr := edgesRow.Scan(&edges)
	if edgesErr != nil {
		klog.Error("Error reading total edges for cluster ", clusterName)
	}

	return resources, edges
}
