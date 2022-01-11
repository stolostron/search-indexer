// Copyright Contributors to the Open Cluster Management project

package database

import (
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	// "github.com/driftprogramming/pgxpoolmock/testdata"
	"github.com/golang/mock/gomock"
	// "github.com/stretchr/testify/assert"
	"github.com/open-cluster-management/search-indexer/pkg/model"
)

func Test_ResyncData(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// given
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)

	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
	dao := DAO{
		pool: mockPool,
	}

	event := model.SyncEvent{} // TODO: Load from mock data.
	dao.ResyncData(event, "test-cluster")

}
