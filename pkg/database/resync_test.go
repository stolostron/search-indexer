// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"errors"
	"github.com/driftprogramming/pgxpoolmock"
	"github.com/jackc/pgconn"
	"github.com/pashagolub/pgxmock"
	"io"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/model"
	"github.com/stolostron/search-indexer/pkg/testutils"
	"github.com/stretchr/testify/assert"
)

func Test_ResyncData(t *testing.T) {
	// Prepare a mock DAO instance.
	dao, mockPool := buildMockDAO(t)

	testutils.MockDatabaseState(mockPool) // Mock Postgres state and SELECT queries.

	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(4)

	// Prepare Request data.
	data, _ := os.Open("./mocks/simple.json")
	dataBytes, _ := io.ReadAll(data)

	// Supress console output to prevent log messages from polluting test output.
	defer testutils.SupressConsoleOutput()()

	// Execute function test.
	response := &model.SyncResponse{}
	err := dao.ResyncData(context.Background(), "test-cluster", response, dataBytes)

	assert.Nil(t, err)
}

func Test_ResyncData_errors(t *testing.T) {
	// Prepare a mock DAO instance.
	dao, mockPool := buildMockDAO(t)
	// Mock Postgres state and SELECT queries.
	testutils.MockDatabaseState(mockPool)

	// Mock error on INSERT.
	br := &testutils.MockBatchResults{MockErrorOnClose: errors.New("unexpected EOF")}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(2)

	// Prepare Request data.
	data, _ := os.Open("./mocks/simple.json")
	dataBytes, _ := io.ReadAll(data)

	// Supress console output to prevent log messages from polluting test output.
	defer testutils.SupressConsoleOutput()()

	// Execute function test.
	response := &model.SyncResponse{}
	err := dao.ResyncData(context.Background(), "test-cluster", response, dataBytes)

	assert.NotNil(t, err)
}

func Test_CheckHubClusterRenameWithoutChange(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)

	// Mock Postgres state with existing hub cluster test-cluster
	testutils.MockDatabaseState(mockPool)

	// test-cluster is the only cluster to exist in the database, no cleanup should happen
	err := dao.checkHubClusterRename(context.Background(), "test-cluster")

	// no further mock queries and old hub cluster cleanup was required
	assert.Nil(t, err)
}

func Test_CheckHubClusterRenameWithChange(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)

	// Mock Postgres state with existing hub cluster test-cluster
	testutils.MockDatabaseState(mockPool)

	// test-cluster gets cleaned up from search.resources
	mockPool.EXPECT().Exec(gomock.Any(),
		`DELETE FROM "search"."resources" WHERE ("cluster" = 'test-cluster')`,
		[]interface{}{}).Return(pgxmock.NewResult("DELETE", 1), nil)

	// test-cluster gets cleaned up from search.edges
	mockPool.EXPECT().Exec(gomock.Any(),
		`DELETE FROM "search"."edges" WHERE ("cluster" = 'test-cluster')`,
		[]interface{}{}).Return(pgxmock.NewResult("DELETE", 1), nil)

	// test-cluster should get cleaned up when we call this with the new hub cluster new-cluster
	err := dao.checkHubClusterRename(context.Background(), "new-cluster")

	assert.Nil(t, err)
}

func Test_HubClusterCleanupWithoutChangeWithRetry(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)

	// Mock Postgres state with existing hub cluster test-cluster
	testutils.MockDatabaseState(mockPool)

	// Mock a failed deletion of search.resources where it succeeds on second attempt
	retry := 0
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq(
		`DELETE FROM "search"."resources" WHERE ("cluster" = 'test-cluster')`),
		[]interface{}{}).Times(2).DoAndReturn(func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
		if retry == 0 {
			retry++
			return nil, errors.New("error deleting old hub cluster")
		} else {
			retry = 0
			return pgconn.CommandTag("DELETE 1"), nil
		}
	})

	// Mock a failed deletion of search.edges where it succeeds on second attempt
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq(
		`DELETE FROM "search"."edges" WHERE ("cluster" = 'test-cluster')`),
		[]interface{}{}).Times(2).DoAndReturn(func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
		if retry == 0 {
			retry++
			return nil, errors.New("error deleting old hub cluster")
		} else {
			return pgconn.CommandTag("DELETE 1"), nil
		}
	})

	// Mock selecting the distinct hub cluster names 4 times; once per attempt
	cluster := []string{"cluster"}
	clusterRows := pgxpoolmock.NewRows(cluster).AddRow("test-cluster").ToPgxRows()
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(
		`SELECT DISTINCT "cluster" FROM "search"."resources" WHERE "data"?'_hubClusterResource'`),
		[]interface{}{}).Return(clusterRows, nil).Times(4)

	// test-cluster should get cleaned up when we call this with the new hub cluster new-cluster
	dao.hubClusterCleanUpWithRetry(context.Background(), "new-cluster")

}
