// Copyright Contributors to the Open Cluster Management project
package server

import (
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/database"
)

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
