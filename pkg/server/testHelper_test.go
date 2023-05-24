// Copyright Contributors to the Open Cluster Management project
package server

import (
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
	"github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"
	"github.com/stolostron/search-indexer/pkg/database"
)

// ====================================================
// Mock the Row interface defined in the pgx library.
// https://github.com/jackc/pgx/blob/master/rows.go#L24
// ====================================================
type Row struct {
	mockValue int
	mockError error
}

func (r *Row) Scan(dest ...interface{}) error {
	if r.mockError != nil {
		return r.mockError
	}
	*dest[0].(*int) = r.mockValue
	return nil
}

// ===========================================================
// Mock the BatchResults interface defined in the pgx library.
// https://github.com/jackc/pgx/blob/master/batch.go#L34
// ===========================================================
type batchResults struct {
	rows             []int
	index            int
	mockErrorOnClose error // Return an error on Close()
	mockErrorOnExec  error // Return an error on Exec()
	mockErrorOnQuery error // Return an error on Query()
}

func (br *batchResults) Exec() (pgconn.CommandTag, error) {
	if br.mockErrorOnExec != nil {
		return nil, br.mockErrorOnExec
	}
	return nil, nil
}
func (br *batchResults) Query() (pgx.Rows, error) {
	if br.mockErrorOnQuery != nil {
		return nil, br.mockErrorOnQuery
	}
	return nil, nil
}
func (br *batchResults) QueryRow() pgx.Row {
	row := &Row{mockValue: br.rows[br.index], mockError: br.mockErrorOnQuery}
	br.index = br.index + 1
	return row
}
func (br *batchResults) QueryFunc(scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return nil, nil
}
func (br *batchResults) Close() error {
	if br.mockErrorOnClose != nil {
		return br.mockErrorOnClose
	}
	return nil
}

// Builds a ServerConfig instance with a mock database connection.
func buildMockServer(t *testing.T) (ServerConfig, *pgxpoolmock.MockPgxPool) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)

	dao := database.NewDAO(mockPool)
	server := ServerConfig{
		Dao: &dao,
	}
	return server, mockPool
}
