// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"testing"

	"github.com/doug-martin/goqu/v9"
	pgx "github.com/jackc/pgx/v4"
)

func Test_ClusterTotals(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	clusterName := "cluster_foo"
	batch := &pgx.Batch{}
	// mock Result
	br := BatchResults{
		MockRows: MockRows{
			mockData: []map[string]interface{}{{"count": 10}},
			index:    1,
		},
	}
	// mock queries
	resourceCountSql, params, _ := goqu.From(goqu.S("search").Table("resources")).
		Select(goqu.COUNT("*")).
		Where(goqu.C("cluster").Eq(clusterName)).ToSQL()
	batch.Queue(resourceCountSql, params)
	edgeCountSql, params, _ := goqu.From(goqu.S("search").Table("edges")).
		Select(goqu.COUNT("*")).
		Where(goqu.C("cluster").Eq(clusterName),
			goqu.C("edgetype").Neq("interCluster")).ToSQL()
	batch.Queue(edgeCountSql, params)

	mockPool.EXPECT().SendBatch(context.Background(), batch).Return(br)
	// Execute function test.
	resourceCount, edgeCount := dao.ClusterTotals("cluster_foo")

	AssertEqual(t, resourceCount, 10, "resource count should be 10")
	AssertEqual(t, edgeCount, 10, "edge count should be 10")
}
