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

type BatchResults struct{}

func (s BatchResults) Exec() (pgconn.CommandTag, error) {
	return nil, nil
}
func (s BatchResults) Query() (pgx.Rows, error) {
	return nil, nil
}
func (s BatchResults) QueryRow() pgx.Row {
	return nil
}
func (s BatchResults) QueryFunc(scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return nil, nil
}
func (s BatchResults) Close() error {
	return nil
}

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
