package testutils

import (
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

// ====================================================
// Mock the Row interface defined in the pgx library.
// https://github.com/jackc/pgx/blob/master/rows.go#L24
// ====================================================
type MockRow struct {
	MockValue int
	MockError error
}

func (r *MockRow) Scan(dest ...interface{}) error {
	if r.MockError != nil {
		return r.MockError
	}
	*dest[0].(*int) = r.MockValue
	return nil
}

// ===========================================================
// Mock the BatchResults interface defined in the pgx library.
// https://github.com/jackc/pgx/blob/master/batch.go#L34
// ===========================================================
type MockBatchResults struct {
	Rows             []int
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
	row := &MockRow{MockValue: br.Rows[br.Index], MockError: br.MockErrorOnQuery}
	br.Index = br.Index + 1
	return row
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
