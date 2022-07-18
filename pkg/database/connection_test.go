// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pashagolub/pgxmock"
)

func Test_initializeTables(t *testing.T) {
	createViewScript := strings.TrimSpace(`CREATE or REPLACE VIEW search.all_edges AS 
	SELECT * from search.edges 
	UNION
	SELECT a.uid as sourceid , a.data->>'kind' as sourcekind, b.uid as destid, b.data->>'kind' as destkind, 
	'deployedBy' as edgetype, a.cluster as cluster  
	FROM search.resources a
	INNER JOIN search.resources b
	ON split_part(a.data->>'_hostingSubscription', '/', 1) = b.data->>'namespace'
	AND split_part(a.data->>'_hostingSubscription', '/', 2) = b.data->>'name'
	WHERE a.data->>'kind' = 'Subscription'
	AND b.data->>'kind' = 'Subscription'
	AND a.uid <> b.uid`)

	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE SCHEMA IF NOT EXISTS search")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE TABLE IF NOT EXISTS search.resources (uid TEXT PRIMARY KEY, cluster TEXT, data JSONB, type TEXT)")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE TABLE IF NOT EXISTS search.edges (sourceId TEXT, sourceKind TEXT,destId TEXT,destKind TEXT,edgeType TEXT,cluster TEXT, PRIMARY KEY(sourceId, destId, edgeType))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS data_kind_idx ON search.resources USING GIN ((data -> 'kind'))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS data_namespace_idx ON search.resources USING GIN ((data -> 'namespace'))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq("CREATE INDEX IF NOT EXISTS data_name_idx ON search.resources USING GIN ((data ->  'name'))")).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Eq(createViewScript)).Return(nil, nil)

	// Execute function test.
	dao.InitializeTables()

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
	checkErrorAndRollback(e, logMessage, mockConn, context.TODO())

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
	checkErrorAndRollback(e, logMessage, mockConn, context.TODO())

}
