// Copyright Contributors to the Open Cluster Management project
package database

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/model"
)

func Test_SyncData(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	dao.batchSize = 1

	// Mock PosgreSQL calls
	br := BatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(4)

	// Prepare Request data
	data, _ := os.Open("./mocks/simple.json")
	var syncEvent model.SyncEvent
	json.NewDecoder(data).Decode(&syncEvent)

	// Execute test
	dao.SyncData(syncEvent, "test-cluster")
}
