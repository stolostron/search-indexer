// Copyright Contributors to the Open Cluster Management project

package database

import (
	"testing"

	"github.com/golang/mock/gomock"
)

func Test_initializeTables(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)

	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("DROP TABLE resources")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("DROP TABLE edges")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE TABLE IF NOT EXISTS edges (sourceId TEXT, sourceKind TEXT,destId TEXT,destKind TEXT,edgeType TEXT, cluster TEXT, PRIMARY KEY(sourceId, destId, edgeType))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE TABLE IF NOT EXISTS resources (uid TEXT PRIMARY KEY, cluster TEXT, data JSONB)")).Return(nil, nil)

	// Execute function test.
	dao.InitializeTables()

}
