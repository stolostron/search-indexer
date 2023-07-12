// Copyright Contributors to the Open Cluster Management project
package testutils

import (
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

// ===========================================================
// Mock the BatchResults interface defined in the pgx library.
// https://github.com/jackc/pgx/blob/master/batch.go#L34
// ===========================================================
type MockBatchResults struct {
	MockRows
	Index            int
	MockErrorOnClose error // Return an error on Close()
	MockErrorOnExec  error // Return an error on Exec()
	MockErrorOnQuery error // Return an error on Query()
}

func (br *MockBatchResults) Exec() (pgconn.CommandTag, error) {
	if br.MockErrorOnExec != nil {
		return nil, br.MockErrorOnExec
	}
	return nil, nil
}
func (br *MockBatchResults) Query() (pgx.Rows, error) {
	if br.MockErrorOnQuery != nil {
		return nil, br.MockErrorOnQuery
	}
	return nil, nil
}
func (br *MockBatchResults) QueryRow() pgx.Row {
	return &br.MockRows
}
func (br *MockBatchResults) QueryFunc(scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return nil, nil
}
func (br *MockBatchResults) Close() error {
	if br.MockErrorOnClose != nil {
		return br.MockErrorOnClose
	}
	return nil
}
