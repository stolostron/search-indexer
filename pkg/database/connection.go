// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"fmt"

	"github.com/driftprogramming/pgxpoolmock"
	pgxpool "github.com/jackc/pgx/v4/pgxpool"
	"github.com/stolostron/search-indexer/pkg/config"
	"k8s.io/klog/v2"
)

// Database Access Object. Use a DAO instance so we can replace the pool object in the unit tests.
type DAO struct {
	pool      pgxpoolmock.PgxPool
	batchSize int
}

var poolSingleton pgxpoolmock.PgxPool

// Creates new DAO instance.
func NewDAO(p pgxpoolmock.PgxPool) DAO {
	// Crete DAO with default values.
	dao := DAO{
		batchSize: 500,
	}
	if p != nil {
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

	databaseUrl := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)
	klog.Infof("Connecting to PostgreSQL at: postgresql://%s:%s@%s:%d/%s",
		cfg.DBUser, "*****", cfg.DBHost, cfg.DBPort, cfg.DBName)

	config, configErr := pgxpool.ParseConfig(databaseUrl)
	if configErr != nil {
		klog.Fatal("Error parsing database connection configuration. ", configErr)
	}

	conn, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		klog.Error("Unable to connect to database: %+v\n", err)
		// TODO: We need to retry the connection until successful.
	} else {
		klog.Info("Successfully connected to database!")
	}

	return conn
}

func (dao *DAO) InitializeTables() {
	if config.Cfg.DevelopmentMode {
		klog.Warning("Dropping search schema for development only. We must not see this message in production.")
		_, err := dao.pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS search CASCADE")
		checkError(err, "Error dropping schema search.")
	}

	_, err := dao.pool.Exec(context.Background(), "CREATE SCHEMA IF NOT EXISTS search")
	checkError(err, "Error creating schema.")
	_, err = dao.pool.Exec(context.Background(),
		"CREATE TABLE IF NOT EXISTS search.resources (uid TEXT PRIMARY KEY, cluster TEXT, data JSONB)")
	checkError(err, "Error creating table search.resources.")
	_, err = dao.pool.Exec(context.Background(),
		"CREATE TABLE IF NOT EXISTS search.edges (sourceId TEXT, sourceKind TEXT,destId TEXT,destKind TEXT,edgeType TEXT,cluster TEXT, PRIMARY KEY(sourceId, destId, edgeType))")
	checkError(err, "Error creating table search.edges.")

	// Jsonb indexing data keys:
	_, err = dao.pool.Exec(context.Background(),
		"CREATE INDEX IF NOT EXISTS data_kind_idx ON search.resources USING GIN ((data -> 'kind'))")
	checkError(err, "Error creating index on search.resources data key kind.")

	_, err = dao.pool.Exec(context.Background(),
		"CREATE INDEX IF NOT EXISTS data_namespace_idx ON search.resources USING GIN ((data -> 'namespace'))")
	checkError(err, "Error creating index on search.resources data key namespace.")

	_, err = dao.pool.Exec(context.Background(),
		"CREATE INDEX IF NOT EXISTS data_name_idx ON search.resources USING GIN ((data ->  'name'))")
	checkError(err, "Error creating index on search.resources data key name.")

}

func checkError(err error, logMessage string) {
	if err != nil {
		klog.Error(logMessage, " ", err)
	}
}
