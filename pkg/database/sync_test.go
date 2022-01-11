// Copyright Contributors to the Open Cluster Management project
package database

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	// "github.com/driftprogramming/pgxpoolmock/testdata"
	"github.com/golang/mock/gomock"
	// "github.com/stretchr/testify/assert"
	"github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"
	"github.com/open-cluster-management/search-indexer/pkg/model"
)

type BatchResults struct{}

// Exec() (pgconn.CommandTag, error)
// Query() (Rows, error)
// QueryRow() Row
// QueryFunc(scans []interface{}, f func(QueryFuncRow) error) (pgconn.CommandTag, error)
// Close() error
func (s BatchResults) Exec() (pgconn.CommandTag, error) {
	fmt.Println("MOCKING Exec()!!!")
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

func Test_SyncData(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// given
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	br := BatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br)
	dao := DAO{
		pool: mockPool,
	}

	dataFile, _ := os.Open("../server/mocks/simple.json")
	defer dataFile.Close()
	data, _ := ioutil.ReadAll(dataFile)
	var syncEvent model.SyncEvent
	err := json.Unmarshal([]byte(data), &syncEvent)

	dao.SyncData(syncEvent, "test-cluster")
}
