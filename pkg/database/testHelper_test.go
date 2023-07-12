// Copyright Contributors to the Open Cluster Management project

package database

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/testutils"
	"k8s.io/klog/v2"
)

// AssertEqual checks if values are equal
func AssertEqual(t *testing.T, a interface{}, b interface{}, message string) {
	if a == b {
		return
	}
	t.Errorf("%s Received %v (type %v), expected %v (type %v)", message, a, reflect.TypeOf(a), b, reflect.TypeOf(b))
}

// Builds a DAO instance with a mock database connection.
func buildMockDAO(t *testing.T) (DAO, *pgxpoolmock.MockPgxPool) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	dao := NewDAO(mockPool)

	return dao, mockPool
}

func newMockRows() *testutils.MockRows {
	clusterResource := `{"uid":"cluster__name-foo", "data":{"apigroup":"internal.open-cluster-management.io", "consoleURL":"", "cpu":0, "created":"0001-01-01T00:00:00Z", "kind":"Cluster", "kubernetesVersion":"", "memory":0, "name":"name-foo", "nodes":0}}`
	var columnHeaders []string
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(clusterResource), &data); err != nil {
		klog.Error("Error unmarshaling mockrows")
		panic(err)
	}
	for k := range data {
		columnHeaders = append(columnHeaders, k)

	}
	mockData := make([]map[string]interface{}, 0)
	mockData = append(mockData, data)

	return &testutils.MockRows{
		MockData:      mockData,
		Index:         1,
		ColumnHeaders: columnHeaders,
	}
}

func (dao *DAO) Query() (*testutils.MockRows, error) {
	var e error
	return newMockRows(), e
}
