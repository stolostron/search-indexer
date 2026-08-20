// Copyright Contributors to the Open Cluster Management project
package testutils

import (
	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
)

// MockDatabaseState mocks the existing database state for the local-cluster.
// All mock JSON payloads in pkg/server/mocks/ and pkg/database/mocks/ use "local-cluster/"
// UIDs, so the mock expectations must match that cluster name.
func MockDatabaseState(mockPool *pgxpoolmock.MockPgxPool) {
	columns := []string{"uid", "data"}
	resourceRows := pgxpoolmock.NewRows(columns).AddRow("uid-123", `{"kind: "mock"}`).ToPgxRows()
	edgeColumns := []string{"sourceId", "edgeType", "destId"}
	edgeRows := pgxpoolmock.NewRows(edgeColumns).AddRow("sourceId1", "edgeType1", "destId1").ToPgxRows()

	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(
		`SELECT "uid", "data" FROM "search"."resources" WHERE (("cluster" = $1) AND ("uid" != $2))`),
		[]interface{}{"local-cluster", "cluster__local-cluster"}).Return(resourceRows, nil)
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(
		`SELECT "sourceid", "edgetype", "destid" FROM "search"."edges" WHERE (("edgetype" != $1) AND ("cluster" = $2))`),
		[]interface{}{"interCluster", "local-cluster"}).Return(edgeRows, nil)
}
