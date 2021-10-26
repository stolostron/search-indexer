// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"fmt"

	pgxpool "github.com/jackc/pgx/v4/pgxpool"
	"github.com/open-cluster-management/search-indexer/pkg/config"
	"k8s.io/klog/v2"
)

var pool *pgxpool.Pool

func init() {
	klog.Info("Initializing database connection.")
	initializePool()
}

func initializePool() {
	cfg := config.New()

	database_url := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)
	klog.Info("Connecting to PostgreSQL at: ", fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", cfg.DBUser, "*****", cfg.DBHost, cfg.DBPort, cfg.DBName))

	config, configErr := pgxpool.ParseConfig(database_url)
	if configErr != nil {
		klog.Error("Error parsing database connection configuration.", configErr)
	}

	conn, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		klog.Error("Unable to connect to database: %+v\n", err)
	}

	pool = conn
}

func GetConnection() *pgxpool.Pool {
	if pool == nil {
		initializePool()
	}

	if pool != nil {
		err := pool.Ping(context.Background())
		if err != nil {
			klog.Error("Unable to get a database connection. ", err)
			// Here we may need to add retry.
			return nil
		}
		klog.Info("Successfully connected to database!")
		return pool
	}
	return nil
}
