// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"k8s.io/klog/v2"
)

// Path mounted by search-v2-operator for external PostgreSQL.
// Variable to allow overriding in tests.
var postgresInitScriptPath = "/etc/search/postgres-init/postgresql.sql"

// Bound startup delays caused by advisory and DDL lock contention.
const postgresInitTimeout = 2 * time.Minute

// Serialize initialization across replicas to prevent concurrent DDL failures.
// Keep the lock ID stable across versions.
const postgresInitLockID int64 = 0x5345415243484901

// RunPostgresInitScript applies the operator-mounted SQL atomically.
// The script must be idempotent because it runs on every startup.
// With internal PostgreSQL, no script is mounted and this is a no-op.
func (dao *DAO) RunPostgresInitScript(ctx context.Context) error {
	script, err := os.ReadFile(postgresInitScriptPath) // #nosec G304 - operator-mounted path, not user input
	if errors.Is(err, fs.ErrNotExist) {
		klog.V(3).Infof("No PostgreSQL init script at %s. Skipping.", postgresInitScriptPath)
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to read PostgreSQL init script %s: %w", postgresInitScriptPath, err)
	}
	if strings.TrimSpace(string(script)) == "" {
		klog.Warningf("PostgreSQL init script %s is empty. Skipping.", postgresInitScriptPath)
		return nil
	}

	klog.Infof("Applying PostgreSQL init script %s", postgresInitScriptPath)

	ctx, cancel := context.WithTimeout(ctx, postgresInitTimeout)
	defer cancel()

	tx, err := dao.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("unable to begin transaction for PostgreSQL init script: %w", err)
	}

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", postgresInitLockID); err != nil {
		rollbackPostgresInit(tx, "Error acquiring advisory lock for PostgreSQL init script.", err)
		return fmt.Errorf("unable to lock for PostgreSQL init script: %w", err)
	}

	// pgx uses the simple query protocol for Exec without bind parameters,
	// allowing multiple SQL statements in a single call.
	if _, err := tx.Exec(ctx, string(script)); err != nil {
		rollbackPostgresInit(tx, "Error executing PostgreSQL init script.", err)
		return fmt.Errorf("unable to execute PostgreSQL init script %s: %w", postgresInitScriptPath, err)
	}

	if err := tx.Commit(ctx); err != nil {
		rollbackPostgresInit(tx, "Error committing PostgreSQL init script.", err)
		return fmt.Errorf("unable to commit PostgreSQL init script %s: %w", postgresInitScriptPath, err)
	}

	klog.Infof("Applied PostgreSQL init script %s", postgresInitScriptPath)
	return nil
}

// rollbackPostgresInit uses a separate context to allow rollback
// even if the initialization context has expired.
func rollbackPostgresInit(tx pgx.Tx, logMessage string, cause error) {
	checkError(cause, logMessage)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Ignore ErrTxClosed because the transaction has already ended.
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		checkError(err, "Error rolling back the PostgreSQL init script transaction.")
	}
}
