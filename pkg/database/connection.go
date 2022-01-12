// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"fmt"

	"github.com/driftprogramming/pgxpoolmock"
	pgxpool "github.com/jackc/pgx/v4/pgxpool"
	"github.com/open-cluster-management/search-indexer/pkg/config"
	"k8s.io/klog/v2"
)

// We need to create an instance of the DAO so we can replace and mock the connection in the unit tests.
type DAO struct {
	pool      pgxpoolmock.PgxPool
	batchSize int
}

var poolSingleton pgxpoolmock.PgxPool

// Creates new DAO instance.
func NewDAO(p pgxpoolmock.PgxPool) DAO {
	// Define default values.
	dao := DAO{
		batchSize: 500,
	}
	if p != nil {
		klog.Error("Using provided database connection. This path should only get executed during unit tests.")
		dao.pool = p
		return dao
	}

	if poolSingleton == nil {
		poolSingleton = initializePool()
	}
	dao.pool = poolSingleton
	return dao
}

func initializePool() pgxpoolmock.PgxPool {
	cfg := config.Cfg

	database_url := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)
	klog.Info("Connecting to PostgreSQL at: ", fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", cfg.DBUser, "*****", cfg.DBHost, cfg.DBPort, cfg.DBName))

	config, configErr := pgxpool.ParseConfig(database_url)
	if configErr != nil {
		klog.Fatal("Error parsing database connection configuration. ", configErr)
	}

	conn, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		klog.Error("Unable to connect to database: %+v\n", err)
		// TODO: We need to retry the connection until successful.
	}

	return conn
}

func (dao *DAO) InitializeTables() {
	// FIXME: REMOVE THIS WORKAROUND! Dropping tables to simplify development, we can't keep this for production.
	klog.Warning("FIXME: REMOVE THIS WORKAROUND! I'm dropping tables to simplify development, we can't keep this for production.")

	dao.pool.Exec(context.Background(), "DROP TABLE resources")
	dao.pool.Exec(context.Background(), "DROP TABLE edges")
	dao.pool.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS resources (uid TEXT PRIMARY KEY, cluster TEXT, data JSONB)")
	dao.pool.Exec(context.Background(), "CREATE TABLE IF NOT EXISTS edges (sourceId TEXT, destId TEXT)")
}
