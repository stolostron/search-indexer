package database

import (
	"context"
	"fmt"

	pgxpool "github.com/jackc/pgx/v4/pgxpool"
	"github.com/jlpadilla/search-indexer/pkg/config"
	"k8s.io/klog/v2"
)

// type dbconn interface {
// 	Begin(ctx context.Context) (pgxpool.Tx, error)
// 	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
// 	Query(ctx context.Context, sql string, optionsAndArgs ...interface{}) (pgxpool.Rows, error)
// 	QueryRow(ctx context.Context, sql string, optionsAndArgs ...interface{}) pgxpool.Row
// }

var pool *pgxpool.Pool

func init() {
	klog.Info("Initializing database connection.")
	initializePool()
}

func initializePool() {
	cfg := config.New()

	// TODO: Validate the configuration.

	database_url := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)
	klog.Info("Connecting to PostgreSQL at: ", fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", cfg.DBUser, "*****", cfg.DBHost, cfg.DBPort, cfg.DBName))

	config, configErr := pgxpool.ParseConfig(database_url)
	if configErr != nil {
		klog.Error("Error parsing database connection configuration.", configErr)
	}

	// config.MaxConns = maxConnections
	conn, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		klog.Error("Unable to connect to database: %+v\n", err)
	}

	pool = conn
}

func GetConnection() *pgxpool.Pool {
	err := pool.Ping(context.Background())
	if err != nil {
		panic(err)
	}
	klog.Info("Successfully connected to database!")
	return pool
}
