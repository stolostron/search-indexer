// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/model"
	"github.com/stretchr/testify/assert"
)

func Test_ResyncData(t *testing.T) {
	// Prepare a mock DAO instance.
	dao, mockPool := buildMockDAO(t)
	columns := []string{"SourceUID"}

	pgxRows := pgxpoolmock.NewRows(columns).AddRow([]string{"srcuid1"}).ToPgxRows()

	// Mock PostgreSQL apis
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(`SELECT "uid", "data" FROM "search"."resources" WHERE ("cluster" = 'test-cluster')`), []interface{}{}).Return(pgxRows, nil)
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(`SELECT "sourceid", "edgetype", "destid" FROM "search"."edges" WHERE (("edgetype" != 'interCluster') AND ("cluster" = 'test-cluster'))`), []interface{}{}).Return(pgxRows, nil)

	br := BatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(2)

	// Prepare Request data.
	data, _ := os.Open("./mocks/simple.json")
	var syncEvent model.SyncEvent
	json.NewDecoder(data).Decode(&syncEvent) //nolint: errcheck

	// Supress console output to prevent log messages from polluting test output.
	defer SupressConsoleOutput()()

	// Execute function test.
	response := &model.SyncResponse{}
	err := dao.ResyncData(context.Background(), syncEvent, "test-cluster", response)

	assert.Nil(t, err)
}

// TODO: Re-enable after errors are captured

// func Test_ResyncData_errors(t *testing.T) {
// 	// Prepare a mock DAO instance.
// 	dao, mockPool := buildMockDAO(t)

// 	// Mock PostgreSQL apis
// 	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Eq([]interface{}{})).Return(nil, errors.New("Delete error")).Times(2)
// 	br := BatchResults{}
// 	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br)

// 	// Prepare Request data.
// 	data, _ := os.Open("./mocks/simple.json")
// 	var syncEvent model.SyncEvent
// 	json.NewDecoder(data).Decode(&syncEvent) //nolint: errcheck

// 	// Supress console output to prevent log messages from polluting test output.
// 	defer SupressConsoleOutput()()

// 	// Execute function test.
// 	response := &model.SyncResponse{}
// 	err := dao.ResyncData(context.Background(), syncEvent, "test-cluster", response)

// 	assert.NotNil(t, err)
// }
