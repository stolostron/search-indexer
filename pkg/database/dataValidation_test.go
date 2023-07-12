// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	pgx "github.com/jackc/pgx/v4"
	"github.com/stolostron/search-indexer/pkg/testutils"

	"github.com/stretchr/testify/assert"
)

func Test_ClusterTotals(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	batch := &pgx.Batch{}
	// mock Result
	br := &testutils.MockBatchResults{
		MockRows: testutils.MockRows{
			MockData: []map[string]interface{}{{"count": 10}, {"count": 5}},
		},
	}
	// mock queries
	batch.Queue(`SELECT COUNT(*) FROM "search"."resources" WHERE (("cluster" = 'cluster_foo') AND ("uid" != 'cluster__cluster_foo'))`, []interface{}{}...)
	batch.Queue(`SELECT COUNT(*) FROM "search"."edges" WHERE (("cluster" = 'cluster_foo') AND ("edgetype" != 'interCluster'))`, []interface{}{}...)

	mockPool.EXPECT().SendBatch(context.Background(), batch).Return(br)
	// Execute function test.
	resourceCount, edgeCount, err := dao.ClusterTotals(context.Background(), "cluster_foo")

	AssertEqual(t, resourceCount, 10, "resource count should be 10")
	AssertEqual(t, edgeCount, 5, "edge count should be 5")
	assert.Nil(t, err)
}

func Test_ClusterTotals_withErrorQueryingResources(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)

	// mock Result
	br := &testutils.MockBatchResults{
		MockRows: testutils.MockRows{
			MockData:        []map[string]interface{}{{"count": 10}},
			MockErrorOnScan: errors.New("unexpected EOF"),
		},
	}
	// mock queries
	mockPool.EXPECT().SendBatch(context.Background(), gomock.Any()).Return(br)

	// Execute function test.
	resourceCount, edgeCount, err := dao.ClusterTotals(context.Background(), "cluster_foo")

	// Validate
	assert.Equal(t, resourceCount, 0)
	assert.Equal(t, edgeCount, 0)
	assert.NotNil(t, err)
}
