// Copyright Contributors to the Open Cluster Management project

package database

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/model"
)

func Test_ResyncData(t *testing.T) {
	// Prepare a mock DAO instance.
	dao, mockPool := buildMockDAO(t)

	// Mock PosgreSQL api.
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("DELETE from resources WHERE cluster=$1"), gomock.Eq("test-cluster")).Return(nil, nil)
	br := BatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br)

	// Prepare Request data.
	data, _ := os.Open("./mocks/simple.json")
	var syncEvent model.SyncEvent
	json.NewDecoder(data).Decode(&syncEvent) //nolint: errcheck

	// Execute function test.
	dao.ResyncData(syncEvent, "test-cluster")
}
