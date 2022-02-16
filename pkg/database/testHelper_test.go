// Copyright Contributors to the Open Cluster Management project

package database

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"

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

var disableConsoleWarning bool

// Supress console output to prevent log messages from polluting test output.
func SupressConsoleOutput() func() {
	if !disableConsoleWarning {
		fmt.Println("!!!!! Tests are supressing log output to stderr. !!!!!")
		disableConsoleWarning = true
	}
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
