// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"testing"

	pgx "github.com/jackc/pgx/v4"
)

func Test_ClusterTotals(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	batch := &pgx.Batch{}
	// mock Result
	br := BatchResults{
		MockRows: MockRows{
			mockData: []map[string]interface{}{{"count": 10}},
			index:    1,
		},
	}
	// mock queries
	batch.Queue(`SELECT COUNT(*) FROM "search"."resources" WHERE (("cluster" = 'cluster_foo') AND ("uid" != 'cluster__cluster_foo'))`, []interface{}{}...)
	batch.Queue(`SELECT COUNT(*) FROM "search"."edges" WHERE (("cluster" = 'cluster_foo') AND ("edgetype" != 'interCluster'))`, []interface{}{}...)

	mockPool.EXPECT().SendBatch(context.Background(), batch).Return(br)
	// Execute function test.
	resourceCount, edgeCount := dao.ClusterTotals(context.Background(), "cluster_foo")

	AssertEqual(t, resourceCount, 10, "resource count should be 10")
	AssertEqual(t, edgeCount, 10, "edge count should be 10")
}
