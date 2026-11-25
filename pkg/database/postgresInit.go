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

// postgresInitScriptPath is where search-v2-operator mounts the postgresql.sql key of the
// search-postgres ConfigMap into the Indexer container. The two repositories agree on this
// path by convention; see IndexerDeployment in search-v2-operator before changing it.
//
// It is a var rather than a const only so tests can point it at a fixture.
var postgresInitScriptPath = "/etc/search/postgres-init/postgresql.sql"

// postgresInitTimeout bounds the whole init step.
//
// The script replaces triggers on search.resources, which needs an ACCESS EXCLUSIVE lock on
// that table. During a rolling update the outgoing Indexer is still writing, so the incoming
// one can wait behind those writes, on top of any wait for the advisory lock. Without a bound
// it would wait forever: the HTTP server has not started yet, so there is no readiness probe
// to fail and the rollout would stall with no signal. Failing instead turns that into a
// CrashLoopBackOff, which is visible and retries on its own.
const postgresInitTimeout = 2 * time.Minute

// postgresInitLockID is the key for the advisory lock that serializes the init script.
//
// The Indexer is scalable (spec.deployments.indexer.replicaCount), so several replicas can
// reach this code at once on a fresh database. The triggers are safe on their own — DROP and
// CREATE TRIGGER take an ACCESS EXCLUSIVE lock on search.resources, so PostgreSQL serializes
// them — but CREATE OR REPLACE FUNCTION is not covered by that table lock and concurrent
// replacements of the same function can fail with "tuple concurrently updated".
//
// The value is arbitrary but must never change: PostgreSQL only guarantees mutual exclusion
// between callers using the same key.
const postgresInitLockID int64 = 0x5345415243484901

// RunPostgresInitScript applies the SQL script that search-v2-operator mounts when Search is
// backed by an external PostgreSQL instance. The script holds the trigger functions and
// triggers on search.resources that generate interCluster edges and the LISTEN/NOTIFY events
// search-v2-api subscribes to.
//
// With the built-in database the postgres pod applies the same SQL from its start script, so
// nothing is mounted and this is a no-op. An external database has no such pod, which is why
// the Indexer applies it here, using the writer credentials it already holds.
//
// The whole script runs in one transaction. That matters because the script drops each
// trigger before recreating it: without the transaction, a failure in between would leave
// search.resources with no trigger at all, silently losing edges and WebSocket updates.
//
// Every replica runs the script, and so does every restart, so it must stay idempotent.
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

	// pg_advisory_xact_lock blocks until the lock is free and releases it when this
	// transaction ends, including on rollback, so a crashed replica cannot strand it.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", postgresInitLockID); err != nil {
		rollbackPostgresInit(tx, "Error acquiring advisory lock for PostgreSQL init script.", err)
		return fmt.Errorf("unable to lock for PostgreSQL init script: %w", err)
	}

	// pgx routes an Exec with no bind parameters through the simple query protocol, which is
	// what allows a multi-statement script to run in a single call. The extended protocol used
	// for parameterized queries permits only one statement, and would reject this script.
	// Dollar-quoted PL/pgSQL bodies are parsed by the server, so they pass through unharmed —
	// but for the same reason the script must not contain bind parameter placeholders.
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

// rollbackPostgresInit aborts the transaction, releasing the advisory lock and the ACCESS
// EXCLUSIVE lock the script may hold on search.resources.
//
// It deliberately does not reuse the caller's context. The most likely reason to get here is
// that context expiring, and a rollback issued on an expired context never reaches the server,
// which would leave the locks held until the connection is reaped.
func rollbackPostgresInit(tx pgx.Tx, logMessage string, cause error) {
	checkError(cause, logMessage)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ErrTxClosed means pgx already tore the connection down, which aborts the transaction
	// server-side. Reporting it would be noise on top of the error that actually matters.
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		checkError(err, "Error rolling back the PostgreSQL init script transaction.")
	}
}
