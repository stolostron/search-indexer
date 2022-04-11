// Copyright Contributors to the Open Cluster Management project

package database

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgproto3/v2"
	pgx "github.com/jackc/pgx/v4"
	"k8s.io/klog/v2"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
)

// AssertEqual checks if values are equal
func AssertEqual(t *testing.T, a interface{}, b interface{}, message string) {
	if a == b {
		return
	}
	t.Errorf("%s Received %v (type %v), expected %v (type %v)", message, a, reflect.TypeOf(a), b, reflect.TypeOf(b))
}

// Supress console output to prevent log messages from polluting test output.
func SupressConsoleOutput() func() {
	fmt.Println("\t  !!!!! Test is supressing log output to stderr. !!!!!")

	nullFile, _ := os.Open(os.DevNull)
	stdErr := os.Stderr
	os.Stderr = nullFile

	return func() {
		defer nullFile.Close()
		os.Stderr = stdErr
	}
}

type BatchResults struct {
	mockErrorOnClose bool // Return an error on Close()
	mockErrorOnExec  bool // Return an error on Exec()
	mockErrorOnQuery bool // Return an error on Query()
}

func (s BatchResults) Exec() (pgconn.CommandTag, error) {
	var e error
	if s.mockErrorOnExec {
		e = fmt.Errorf("MockError")
	}
	return nil, e
}
func (s BatchResults) Query() (pgx.Rows, error) {
	var e error
	if s.mockErrorOnQuery {
		e = fmt.Errorf("MockError")
	}
	return nil, e
}
func (s BatchResults) QueryRow() pgx.Row {
	return nil
}
func (s BatchResults) QueryFunc(scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return nil, nil
}
func (s BatchResults) Close() error {
	if s.mockErrorOnClose {
		return fmt.Errorf("MockError")
	}
	return nil
}

// Builds a DAO instance with a mock database connection.
func buildMockDAO(t *testing.T) (DAO, *pgxpoolmock.MockPgxPool) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	dao := NewDAO(mockPool)

	return dao, mockPool
}

type MockRows struct {
	mockData []map[string]interface{}
	index    int
}

func (r *MockRows) Close() {}

func (r *MockRows) Err() error { return nil }

func (r *MockRows) CommandTag() pgconn.CommandTag { return nil }

func (r *MockRows) FieldDescriptions() []pgproto3.FieldDescription { return nil }

func (r *MockRows) Next() bool {
	r.index = r.index + 1
	return r.index <= len(r.mockData)
}

func (r *MockRows) Scan(dest ...interface{}) error {
	*dest[0].(*string) = r.mockData[r.index-1]["uid"].(string)
	props, _ := r.mockData[r.index-1]["data"].(map[string]interface{})
	dest[1] = props
	return nil
}

func (r *MockRows) Values() ([]interface{}, error) { return nil, nil }

func (r *MockRows) RawValues() [][]byte { return nil }

func newMockRows() *MockRows {
	clusterResource := `{"uid":"cluster__name-foo", "data":{"apigroup":"internal.open-cluster-management.io", "consoleURL":"", "cpu":0, "created":"0001-01-01T00:00:00Z", "kind":"Cluster", "kubernetesVersion":"", "memory":0, "name":"name-foo", "nodes":0}}`

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(clusterResource), &data); err != nil {
		klog.Error("Error unmarhsaling mockrows")
		panic(err)
	}

	mockData := make([]map[string]interface{}, 0)
	mockData = append(mockData, data)

	return &MockRows{
		mockData: mockData,
		index:    0,
	}
}

func (dao *DAO) Query() (*MockRows, error) {
	var e error
	return newMockRows(), e
}
