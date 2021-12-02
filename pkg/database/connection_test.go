// Copyright Contributors to the Open Cluster Management project

package database

import (
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	// "github.com/driftprogramming/pgxpoolmock/testdata"
	"github.com/golang/mock/gomock"
	// "github.com/stretchr/testify/assert"
)

func Test_initializeTables(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// given
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	// columns := []string{"id", "price"}
	// pgxRows := pgxpoolmock.NewRows(columns).AddRow(100, 100000.9).ToPgxRows()

	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any()).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any()).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any()).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any()).Return(nil, nil)
	dao := DAO{
		pool: mockPool,
	}

	// when
	dao.InitializeTables()

	// then

	// TODO: Jorge, need to update these asserts.

	// assert.NotNil(t, actualOrder)
	// assert.Equal(t, 100, actualOrder.ID)
	// assert.Equal(t, 100000.9, actualOrder.Price)
}
