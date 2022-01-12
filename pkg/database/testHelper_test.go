// Copyright Contributors to the Open Cluster Management project

package database

import (
	"github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
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

func buildMockDAO(t *testing.T) (DAO, *pgxpoolmock.MockPgxPool) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	dao := NewDAO(mockPool)

	return dao, mockPool
}
