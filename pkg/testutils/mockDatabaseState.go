// Copyright Contributors to the Open Cluster Management project
package testutils

import (
	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
)

// Mocks the existing state of the database for the test-cluster.
func MockDatabaseState(mockPool *pgxpoolmock.MockPgxPool) {
	columns := []string{"uid", "data"}
	resourceRows := pgxpoolmock.NewRows(columns).AddRow("uid-123", `{"kind: "mock"}`).ToPgxRows()
	edgeColumns := []string{"sourceId", "edgeType", "destId"}
	edgeRows := pgxpoolmock.NewRows(edgeColumns).AddRow("sourceId1", "edgeType1", "destId1").ToPgxRows()
	cluster := []string{"cluster"}
	clusterRows := pgxpoolmock.NewRows(cluster).AddRow("test-cluster").ToPgxRows()

	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(
		`SELECT "uid", "data" FROM "search"."resources" WHERE (("cluster" = $1) AND ("uid" != $2))`),
		[]interface{}{"test-cluster", "cluster__test-cluster"}).Return(resourceRows, nil)
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(
		`SELECT "sourceid", "edgetype", "destid" FROM "search"."edges" WHERE (("edgetype" != $1) AND ("cluster" = $2))`),
		[]interface{}{"interCluster", "test-cluster"}).Return(edgeRows, nil)
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(
		`SELECT DISTINCT "cluster" FROM "search"."resources" WHERE ("data"?'_hubClusterResource' AND "data"->>'kind' <> 'Cluster')`),
		[]interface{}{}).Return(clusterRows, nil)
}
