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
}

func (r *Row) Scan(dest ...interface{}) error {
	*dest[0].(*int) = r.mockValue
	return nil
}

// ===========================================================
// Mock the BatchResults interface defined in the pgx library.
// https://github.com/jackc/pgx/blob/master/batch.go#L34
// ===========================================================
type BatchResults struct {
	rows  []int
	index *int
}

func (br BatchResults) Exec() (pgconn.CommandTag, error) {
	return nil, nil
}
func (br BatchResults) Query() (pgx.Rows, error) {
	return nil, nil
}
func (br BatchResults) QueryRow() pgx.Row {
	row := &Row{mockValue: br.rows[*br.index]}
	*br.index = *br.index + 1
	return row
}
func (br BatchResults) QueryFunc(scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return nil, nil
}
func (br BatchResults) Close() error {
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
