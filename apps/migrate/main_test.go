package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunMigrationFromEnvRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	err := runMigrationFromEnv()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "DATABASE_URL is required")
}

func TestRunMigrationReturnsReadOpenPingAndExecErrors(t *testing.T) {
	err := runMigration("dsn", "/missing.sql", sql.Open)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read migration failed")

	migrationPath := writeMigration(t, "SELECT 1;")
	err = runMigration("dsn", migrationPath, func(_, _ string) (*sql.DB, error) {
		return nil, assert.AnError
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postgres open failed")

	pingDB, pingMock, pingCleanup := newMigrationMockDB(t)
	defer pingCleanup()
	pingMock.ExpectPing().WillReturnError(assert.AnError)
	err = runMigration("dsn", migrationPath, func(_, _ string) (*sql.DB, error) {
		return pingDB, nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postgres ping failed")

	execDB, execMock, execCleanup := newMigrationMockDB(t)
	defer execCleanup()
	execMock.ExpectPing()
	execMock.ExpectExec("SELECT 1").WillReturnError(assert.AnError)
	err = runMigration("dsn", migrationPath, func(_, _ string) (*sql.DB, error) {
		return execDB, nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "migration failed")
}

func TestRunMigrationExecutesMigration(t *testing.T) {
	migrationPath := writeMigration(t, "CREATE TABLE test_table (id text);")
	db, mock, cleanup := newMigrationMockDB(t)
	defer cleanup()
	mock.ExpectPing()
	mock.ExpectExec("CREATE TABLE test_table").WillReturnResult(sqlmock.NewResult(0, 1))

	err := runMigration("dsn", migrationPath, func(driverName, dsn string) (*sql.DB, error) {
		assert.Equal(t, "postgres", driverName)
		assert.Equal(t, "dsn", dsn)
		return db, nil
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func writeMigration(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "migration.sql")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func newMigrationMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}
