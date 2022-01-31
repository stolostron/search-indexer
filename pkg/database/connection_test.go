// Copyright Contributors to the Open Cluster Management project

package database

import (
	"testing"

	"github.com/golang/mock/gomock"
)

func Test_initializeTables(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE SCHEMA IF NOT EXISTS search")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE TABLE IF NOT EXISTS search.resources (uid TEXT PRIMARY KEY, cluster TEXT, data JSONB)")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE TABLE IF NOT EXISTS search.edges (sourceId TEXT, sourceKind TEXT,destId TEXT,destKind TEXT,edgeType TEXT,cluster TEXT, PRIMARY KEY(sourceId, destId, edgeType))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX data_kind_idx ON search.resources USING GIN ((data -> 'kind'))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX data_namespace_idx ON search.resources USING GIN ((data -> 'namespace'))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX data_name_idx ON search.resources USING GIN ((data ->  'name'))")).Return(nil, nil)

	// Execute function test.
	dao.InitializeTables()

}
