// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/pashagolub/pgxmock"
	"github.com/stretchr/testify/assert"
)

// A script shaped like the one the operator mounts: several statements and a dollar-quoted
// PL/pgSQL body. It is sent verbatim in a single Exec, so the test asserts on it verbatim.
const testInitScript = `CREATE SCHEMA IF NOT EXISTS search;
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'search_reader_ro') THEN
        CREATE ROLE search_reader_ro NOLOGIN;
    END IF;
END
$$;
`

// writeInitScript writes a script to a temporary file and points the package at it.
func writeInitScript(t *testing.T, contents string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "postgresql.sql")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatalf("unable to write test init script: %v", err)
	}
	setInitScriptPath(t, path)
}

// setInitScriptPath overrides the mounted script path and restores it when the test ends.
func setInitScriptPath(t *testing.T, path string) {
	t.Helper()
	original := postgresInitScriptPath
	postgresInitScriptPath = path
	t.Cleanup(func() { postgresInitScriptPath = original })
}

func newMockTx(t *testing.T) pgxmock.PgxConnIface {
	t.Helper()
	mockConn, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	t.Cleanup(func() { _ = mockConn.Close(context.Background()) })
	return mockConn
}

// boundedContext matches a context carrying a deadline, i.e. one that postgresInitTimeout
// has been applied to.
type boundedContext struct{}

func (boundedContext) Matches(x interface{}) bool {
	ctx, ok := x.(context.Context)
	if !ok {
		return false
	}
	_, hasDeadline := ctx.Deadline()
	return hasDeadline
}

func (boundedContext) String() string { return "a context with a deadline" }

// The built-in PostgreSQL case: no script is mounted, so startup must not touch the database.
// buildMockDAO fails the test on any unexpected call, which is the assertion here.
func Test_RunPostgresInitScript_absentFileIsSkipped(t *testing.T) {
	setInitScriptPath(t, filepath.Join(t.TempDir(), "does-not-exist.sql"))

	dao, _ := buildMockDAO(t)

	assert.NoError(t, dao.RunPostgresInitScript(context.Background()))
}

func Test_RunPostgresInitScript_emptyFileIsSkipped(t *testing.T) {
	writeInitScript(t, "\n\t \n")

	dao, _ := buildMockDAO(t)

	assert.NoError(t, dao.RunPostgresInitScript(context.Background()))
}

// The script is taken under the advisory lock and committed as one transaction.
func Test_RunPostgresInitScript_appliesScriptInOneTransaction(t *testing.T) {
	writeInitScript(t, testInitScript)

	dao, mockPool := buildMockDAO(t)
	mockConn := newMockTx(t)
	// boundedContext asserts the timeout is applied: the transaction must not inherit the
	// caller's unbounded context, or a blocked DDL lock would stall startup forever.
	mockPool.EXPECT().BeginTx(boundedContext{}, pgx.TxOptions{}).Return(mockConn, nil)

	mockConn.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock($1)")).
		WithArgs(postgresInitLockID).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))
	// Sent as one statement with no bind parameters, which is what routes it through the
	// simple query protocol and lets the multi-statement script run.
	mockConn.ExpectExec(regexp.QuoteMeta(testInitScript)).
		WillReturnResult(pgxmock.NewResult("CREATE", 0))
	mockConn.ExpectCommit()

	assert.NoError(t, dao.RunPostgresInitScript(context.Background()))
	assert.NoError(t, mockConn.ExpectationsWereMet())
}

// A failing script must roll back and return, so main.go exits before sync starts rather
// than serving traffic against a half-initialized database.
func Test_RunPostgresInitScript_sqlErrorRollsBackAndSurfaces(t *testing.T) {
	writeInitScript(t, testInitScript)

	sqlErr := errors.New(`ERROR: role "search_reader_ro" already exists (SQLSTATE 42710)`)

	dao, mockPool := buildMockDAO(t)
	mockConn := newMockTx(t)
	mockPool.EXPECT().BeginTx(boundedContext{}, pgx.TxOptions{}).Return(mockConn, nil)

	mockConn.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock($1)")).
		WithArgs(postgresInitLockID).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))
	mockConn.ExpectExec(regexp.QuoteMeta(testInitScript)).WillReturnError(sqlErr)
	mockConn.ExpectRollback()

	err := dao.RunPostgresInitScript(context.Background())

	assert.ErrorIs(t, err, sqlErr)
	assert.NoError(t, mockConn.ExpectationsWereMet(), "the transaction must be rolled back")
}

// postgresInitTimeout firing mid-script must still roll back, so the advisory lock and the
// ACCESS EXCLUSIVE lock on search.resources are released instead of being held until the
// connection is reaped. rollbackPostgresInit issues the rollback on its own context for that
// reason — a rollback sent on the expired context would never reach the server.
//
// This covers the control flow only: pgxmock ignores the context passed to Rollback, so it
// cannot tell the two apart. The lock actually being released was verified against
// PostgreSQL 16 by calling rollbackPostgresInit and polling pg_locks.
func Test_RunPostgresInitScript_rollsBackWhenContextExpires(t *testing.T) {
	writeInitScript(t, testInitScript)

	dao, mockPool := buildMockDAO(t)
	mockConn := newMockTx(t)
	mockPool.EXPECT().BeginTx(boundedContext{}, pgx.TxOptions{}).Return(mockConn, nil)

	mockConn.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock($1)")).
		WithArgs(postgresInitLockID).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))
	// What pgx returns when the deadline lands while the script is running. The caller's
	// context is left live: cancelling it here would race the lock Exec above instead.
	mockConn.ExpectExec(regexp.QuoteMeta(testInitScript)).WillReturnError(context.DeadlineExceeded)
	mockConn.ExpectRollback()

	err := dao.RunPostgresInitScript(context.Background())

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NoError(t, mockConn.ExpectationsWereMet(), "a timed-out script must still roll back")
}

// An unreadable file is a misconfiguration, not a reason to start: report it without
// opening a transaction.
func Test_RunPostgresInitScript_readErrorSurfaces(t *testing.T) {
	setInitScriptPath(t, t.TempDir()) // a directory, so ReadFile fails with something other than ErrNotExist

	dao, _ := buildMockDAO(t)

	err := dao.RunPostgresInitScript(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to read PostgreSQL init script")
}
