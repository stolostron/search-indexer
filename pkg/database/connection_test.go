// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pashagolub/pgxmock"
)

func Test_initializeTables(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE SCHEMA IF NOT EXISTS search")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE TABLE IF NOT EXISTS search.resources (uid TEXT PRIMARY KEY, cluster TEXT, data JSONB)")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE TABLE IF NOT EXISTS search.edges (sourceId TEXT, sourceKind TEXT,destId TEXT,destKind TEXT,edgeType TEXT,cluster TEXT, PRIMARY KEY(sourceId, destId, edgeType))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS data_kind_idx ON search.resources USING GIN ((data -> 'kind'))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS data_namespace_idx ON search.resources USING GIN ((data -> 'namespace'))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS data_name_idx ON search.resources USING GIN ((data ->  'name'))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS data_cluster_idx ON search.resources USING btree (cluster)")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS data_composite_idx ON search.resources USING GIN ((data -> '_hubClusterResource'::text), (data -> 'namespace'::text), (data -> 'apigroup'::text), (data -> 'kind_plural'::text))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS data_hubCluster_idx ON search.resources USING GIN ((data ->  '_hubClusterResource')) WHERE data ? '_hubClusterResource'")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS edges_sourceid_idx ON search.edges USING btree (sourceid)")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS edges_destid_idx ON search.edges USING btree (destid)")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS edges_cluster_idx ON search.edges USING btree (cluster)")).Return(nil, nil)

	// Execute function test.
	dao.InitializeTables(context.Background())

}

func Test_checkErrorAndRollback(t *testing.T) {
	mockConn, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockConn.Close(context.Background())
	mockConn.ExpectRollback()
	e := errors.New("table resources not found")
	logMessage := "Error commiting delete cluster transaction for cluster: cluster_foo"
	// Execute function test.
	checkErrorAndRollback(e, logMessage, mockConn, context.Background())

}
func Test_checkErrorAndRollbackError(t *testing.T) {
	mockConn, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockConn.Close(context.Background())
	e := errors.New("table resources not found")

	mockConn.ExpectRollback().WillReturnError(e) // Rollback returns error
	logMessage := "Error commiting delete cluster transaction for cluster: cluster_foo"
	// Execute function test.
	checkErrorAndRollback(e, logMessage, mockConn, context.Background())

}
