// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgconn/stmtcache"
	"github.com/jackc/pgx/v4"
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

// preparedStatementCache wraps pgx's statement cache so dynamic SQL (especially DELETE ... IN (...))
// is not retained as named prepared statements. Caching those blows memory because each
// distinct IN-list length is a unique statement.
//
// For skipped statements we still Prepare with an empty name (unnamed). That is the pgx v4
// equivalent of pgx v5's QueryExecModeExec: extended protocol without filling the cache.
type preparedStatementCache struct {
	stmtcache.Cache
	conn *pgconn.PgConn
}

func (c *preparedStatementCache) Get(ctx context.Context, sql string) (*pgconn.StatementDescription, error) {
	// Match both "DELETE FROM" and goqu/sync "DELETE from".
	// Only bypass cache for variable-shape IN-list deletes.
	normalized := strings.ToUpper(sql)
	if strings.Contains(normalized, "DELETE FROM") && strings.Contains(normalized, " IN (") {
		return c.conn.Prepare(ctx, "", sql, nil)
	}
	return c.Cache.Get(ctx, sql)
}

// Creates new DAO instance.
func NewDAO(p pgxpoolmock.PgxPool) DAO {
	// Crete DAO with default values.
	dao := DAO{
		batchSize: config.Cfg.DBBatchSize,
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

// Checks new connection is healthy before using it.
func afterConnect(ctx context.Context, c *pgx.Conn) error {
	if err := c.Ping(ctx); err != nil {
		klog.V(7).Info("New DB connection from pool was unhealthy. ", err)
		return err
	}
	return nil
}

// Checks idle connection is healthy before using it.
func beforeAcquire(ctx context.Context, c *pgx.Conn) bool {
	if err := c.Ping(ctx); err != nil {
		klog.V(7).Info("Idle DB connection from pool is unhealthy, destroying it. ", err)
		return false
	}
	return true
}

func initializePool() pgxpoolmock.PgxPool {
	cfg := config.Cfg

	dbConnString := fmt.Sprint(
		"host=", cfg.DBHost,
		" port=", cfg.DBPort,
		" user=", cfg.DBUser,
		" password=", cfg.DBPass,
		" dbname=", cfg.DBName,
		" application_name=search-indexer",
		" sslmode=require", // https://www.postgresql.org/docs/current/libpq-connect.html
	)

	// Remove password from connection log.
	redactedDbConn := strings.ReplaceAll(dbConnString, cfg.DBPass, "[REDACTED]")
	klog.Infof("Connecting to PostgreSQL using: %s", redactedDbConn)

	config, configErr := pgxpool.ParseConfig(dbConnString)
	if configErr != nil {
		klog.Fatal("Error parsing database connection configuration. ", configErr)
	}

	// Keep a small LRU for stable statements (upserts). Skip caching dynamic DELETE/oversized SQL.
	// This is a workaround for pgx v4. We should migrate to pgx v5 and use the default behavior.
	config.ConnConfig.BuildStatementCache = func(conn *pgconn.PgConn) stmtcache.Cache {
		return &preparedStatementCache{
			Cache: stmtcache.New(conn, stmtcache.ModePrepare, 8),
			conn:  conn,
		}
	}

	config.AfterConnect = afterConnect   // Checks new connection health before using it.
	config.BeforeAcquire = beforeAcquire // Checks idle connection health before using it.
	// Add jitter to prevent all connections from being closed at same time.
	config.MaxConnLifetimeJitter = time.Duration(cfg.DBMaxConnLifeJitter) * time.Millisecond
	config.MaxConns = cfg.DBMaxConns
	config.MaxConnIdleTime = time.Duration(cfg.DBMaxConnIdleTime) * time.Millisecond
	config.MaxConnLifetime = time.Duration(cfg.DBMaxConnLifeTime) * time.Millisecond
	config.MinConns = cfg.DBMinConns

	klog.Infof("Using pgxpool.Config %+v", config)

	retry := 0
	var conn *pgxpool.Pool
	var err error
	for {
		conn, err = pgxpool.ConnectConfig(context.TODO(), config)
		if err != nil {
			// Max wait time is 30 sec
			waitMS := int(math.Min(float64(retry*500), float64(cfg.MaxBackoffMS/10)))
			timeToSleep := time.Duration(waitMS) * time.Millisecond
			retry++
			klog.Errorf("Unable to connect to database: %+v. Will retry in %s\n", err, timeToSleep)
			time.Sleep(timeToSleep)
		} else {
			klog.Info("Successfully connected to database!")
			break
		}
	}

	return conn
}

func (dao *DAO) InitializeTables(ctx context.Context) {
	if config.Cfg.DevelopmentMode {
		klog.Warning("Dropping search schema for development only. We must not see this message in production.")
		_, err := dao.pool.Exec(ctx, "DROP SCHEMA IF EXISTS search CASCADE")
		checkError(err, "Error dropping schema search.")
	}

	_, err := dao.pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS search")
	checkError(err, "Error creating schema.")
	_, err = dao.pool.Exec(ctx,
		"CREATE TABLE IF NOT EXISTS search.resources (uid TEXT PRIMARY KEY, cluster TEXT, data JSONB)")
	checkError(err, "Error creating table search.resources.")
	_, err = dao.pool.Exec(ctx,
		"CREATE TABLE IF NOT EXISTS search.edges (sourceId TEXT, sourceKind TEXT,destId TEXT,destKind TEXT,edgeType TEXT,cluster TEXT, PRIMARY KEY(sourceId, destId, edgeType))")
	checkError(err, "Error creating table search.edges.")

	// Jsonb indexing data keys:
	_, err = dao.pool.Exec(ctx,
		"CREATE INDEX IF NOT EXISTS data_kind_idx ON search.resources USING GIN ((data -> 'kind'))")
	checkError(err, "Error creating index on search.resources data key kind.")

	_, err = dao.pool.Exec(ctx,
		"CREATE INDEX IF NOT EXISTS data_namespace_idx ON search.resources USING GIN ((data -> 'namespace'))")
	checkError(err, "Error creating index on search.resources data key namespace.")

	_, err = dao.pool.Exec(ctx,
		"CREATE INDEX IF NOT EXISTS data_name_idx ON search.resources USING GIN ((data ->  'name'))")
	checkError(err, "Error creating index on search.resources data key name.")

	_, err = dao.pool.Exec(ctx,
		"CREATE INDEX IF NOT EXISTS data_cluster_idx ON search.resources USING btree (cluster)")
	checkError(err, "Error creating index on search.resources cluster.")

	_, err = dao.pool.Exec(ctx,
		"CREATE INDEX IF NOT EXISTS data_composite_idx ON search.resources USING GIN "+
			"((data -> '_hubClusterResource'::text), (data -> 'namespace'::text), "+
			"(data -> 'apigroup'::text), (data -> 'kind_plural'::text))")
	checkError(err, "Error creating index on search.resources data composite.")

	_, err = dao.pool.Exec(ctx,
		"CREATE INDEX IF NOT EXISTS data_hubCluster_idx ON search.resources USING GIN "+
			"((data ->  '_hubClusterResource')) WHERE data ? '_hubClusterResource'")
	checkError(err, "Error creating index on search.resources data key _hubClusterResource.")

	_, err = dao.pool.Exec(ctx,
		"CREATE INDEX IF NOT EXISTS edges_sourceid_idx ON search.edges USING btree (sourceid)")
	checkError(err, "Error creating index on search.edges key sourceid.")

	_, err = dao.pool.Exec(ctx,
		"CREATE INDEX IF NOT EXISTS edges_destid_idx ON search.edges USING btree (destid)")
	checkError(err, "Error creating index on search.edges key destid.")

	_, err = dao.pool.Exec(ctx,
		"CREATE INDEX IF NOT EXISTS edges_cluster_idx ON search.edges USING btree (cluster)")
	checkError(err, "Error creating index on search.edges key cluster.")

	//GRANT USAGE ON SCHEMA search TO search_api_ro, search_mcp_ro;
	_, err = dao.pool.Exec(ctx,
		"GRANT USAGE ON SCHEMA search TO search_api_ro, search_mcp_ro")
	checkError(err, "Error granting usage on schema search to search_api_ro, search_mcp_ro.")

	// GRANT SELECT ON search.resources TO search_api_ro, search_mcp_ro;
	_, err = dao.pool.Exec(ctx,
		"GRANT SELECT ON search.resources TO search_api_ro, search_mcp_ro")
	checkError(err, "Error granting select on search.resources to search_api_ro, search_mcp_ro.")
	_, err = dao.pool.Exec(ctx,
		"GRANT SELECT ON search.edges TO search_api_ro, search_mcp_ro")
	checkError(err, "Error granting select on search.edges to search_api_ro, search_mcp_ro.")

	// ALTER DEFAULT PRIVILEGES IN SCHEMA search GRANT SELECT ON TABLES TO search_api_ro, search_mcp_ro;
	_, err = dao.pool.Exec(ctx,
		"ALTER DEFAULT PRIVILEGES IN SCHEMA search GRANT SELECT ON TABLES TO search_api_ro, search_mcp_ro")
	checkError(err, "Error granting select on tables to search_api_ro, search_mcp_ro.")
}

func checkError(err error, logMessage string) {
	if err != nil {
		klog.Error(logMessage, " ", err)
	}
}

func checkErrorAndRollback(err error, logMessage string, tx pgx.Tx, ctx context.Context) {
	checkError(err, logMessage)
	if err := tx.Rollback(ctx); err != nil {
		checkError(err, "Encountered error while rolling back cluster delete transaction command")
	}
}
