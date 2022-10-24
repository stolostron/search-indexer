// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"fmt"

	"github.com/doug-martin/goqu/v9"
	pgx "github.com/jackc/pgx/v4"
	"k8s.io/klog/v2"
)

// Query resource and edge count for a cluster. Used for data validation.
func (dao *DAO) ClusterTotals(clusterName string) (resources int, edges int) {
	batch := &pgx.Batch{}

	// Sample query: SELECT count(*) FROM search.resources WHERE cluster=$1
	resourceCountSql, params, err := goqu.From(goqu.S("search").Table("resources")).
		Select(goqu.COUNT("*")).
		Where(goqu.C("cluster").Eq(clusterName)).ToSQL()

	checkError(err, fmt.Sprintf("Error creating query to count resources in cluster %s:%s ",
		clusterName, err))
	klog.V(4).Infof("Data validation query for resource count in cluster %s - sql: %s args: %+v",
		clusterName, resourceCountSql, params)
	batch.Queue(resourceCountSql, params)

	// Sample query: SELECT count(*) FROM search.edges WHERE cluster=$1 and edgetype<>'interCluster'
	edgeCountSql, params, err := goqu.From(goqu.S("search").Table("edges")).
		Select(goqu.COUNT("*")).
		Where(goqu.C("cluster").Eq(clusterName),
			goqu.C("edgetype").Neq("interCluster")).ToSQL()
	klog.V(4).Infof("Data validation query for edge count in cluster %s - sql: %s args: %+v",
		clusterName, edgeCountSql, params)
	checkError(err, fmt.Sprintf("Error creating query to count edges in cluster %s:%s ",
		clusterName, err))
	batch.Queue(edgeCountSql, params)

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
