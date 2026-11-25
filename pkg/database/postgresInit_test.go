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

// Multi-statement SQL fixture with a dollar-quoted PL/pgSQL body.
const testInitScript = `CREATE SCHEMA IF NOT EXISTS search;
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'search_reader_ro') THEN
        CREATE ROLE search_reader_ro NOLOGIN;
    END IF;
END
$$;
`

func writeInitScript(t *testing.T, contents string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "postgresql.sql")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatalf("unable to write test init script: %v", err)
	}
	setInitScriptPath(t, path)
}

// Override the script path for the duration of the test.
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

// boundedContext matches a context with a deadline.
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

func Test_RunPostgresInitScript_appliesScriptInOneTransaction(t *testing.T) {
	writeInitScript(t, testInitScript)

	dao, mockPool := buildMockDAO(t)
	mockConn := newMockTx(t)

	mockPool.EXPECT().BeginTx(boundedContext{}, pgx.TxOptions{}).Return(mockConn, nil)

	mockConn.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock($1)")).
		WithArgs(postgresInitLockID).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))
	mockConn.ExpectExec(regexp.QuoteMeta(testInitScript)).
		WillReturnResult(pgxmock.NewResult("CREATE", 0))
	mockConn.ExpectCommit()

	assert.NoError(t, dao.RunPostgresInitScript(context.Background()))
	assert.NoError(t, mockConn.ExpectationsWereMet())
}

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

func Test_RunPostgresInitScript_rollsBackWhenContextExpires(t *testing.T) {
	writeInitScript(t, testInitScript)

	dao, mockPool := buildMockDAO(t)
	mockConn := newMockTx(t)
	mockPool.EXPECT().BeginTx(boundedContext{}, pgx.TxOptions{}).Return(mockConn, nil)

	mockConn.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock($1)")).
		WithArgs(postgresInitLockID).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))
	// Simulate a timeout without cancelling the caller's context.
	mockConn.ExpectExec(regexp.QuoteMeta(testInitScript)).WillReturnError(context.DeadlineExceeded)
	mockConn.ExpectRollback()

	err := dao.RunPostgresInitScript(context.Background())

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NoError(t, mockConn.ExpectationsWereMet(), "a timed-out script must still roll back")
}

func Test_RunPostgresInitScript_readErrorSurfaces(t *testing.T) {
	setInitScriptPath(t, t.TempDir()) // a directory, so ReadFile fails with something other than ErrNotExist

	dao, _ := buildMockDAO(t)

	err := dao.RunPostgresInitScript(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to read PostgreSQL init script")
}
