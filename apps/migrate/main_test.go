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

func TestIntegrationMigrationSchemaSeedAndReset(t *testing.T) {
	if os.Getenv("ASHN_INTEGRATION") != "1" {
		t.Skip("set ASHN_INTEGRATION=1 to run Postgres-backed migration tests")
	}

	dsn := os.Getenv("DATABASE_URL")
	require.NotEmpty(t, dsn, "DATABASE_URL is required for integration tests")

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	upPath := migrationPath(t, "000001_init.up.sql")
	resetMigrationSchema(t, db)
	require.NoError(t, runMigration(dsn, upPath, sql.Open))
	assertCoreSchema(t, db)
	assertSeedData(t, db)

	require.NoError(t, runMigration(dsn, upPath, sql.Open), "migration should be idempotent")
	assertTableCount(t, db, "providers", 6)
	assertTableCount(t, db, "trading_partners", 3)

	_, err = db.Exec(`INSERT INTO adventurers (id, name, rank, guild, region, coverage_status) VALUES ('adv-reset-proof', 'Reset Proof', 'Iron', 'Test Guild', 'Greenstone', 'Active')`)
	require.NoError(t, err)
	assertTableCount(t, db, "adventurers", 1)

	resetMigrationSchema(t, db)
	require.NoError(t, runMigration(dsn, upPath, sql.Open))
	assertTableCount(t, db, "adventurers", 0)
	assertSeedData(t, db)
}

func assertCoreSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{
		"adventurers",
		"providers",
		"trading_partners",
		"transactions",
		"claims",
		"enrollments",
		"premium_payments",
		"auth_requests",
		"inbound_messages",
		"transaction_jobs",
	} {
		assertTableExists(t, db, table)
	}

	assertColumns(t, db, "transactions", "raw_x12", "related_id")
	assertColumns(t, db, "claims", "authorization_transaction_id", "authorization_status", "authorization_reason", "allowed_amount_cents", "paid_amount_cents", "patient_responsibility_cents", "adjustment_amount_cents", "adjustment_reason", "denial_reason")
	assertColumns(t, db, "inbound_messages", "partner_id", "downstream_status")
	assertColumns(t, db, "trading_partners", "validation_profile", "route_target", "status")
	assertColumns(t, db, "transaction_jobs", "job_type", "entity_id", "status", "attempts", "run_after", "last_error")
}

func assertSeedData(t *testing.T, db *sql.DB) {
	t.Helper()
	assertTableCount(t, db, "providers", 6)
	assertTableCount(t, db, "trading_partners", 3)
	assertRowExists(t, db, "providers", "id", "provider-vitesse-temple")
	assertRowExists(t, db, "trading_partners", "id", "tp-greenstone-guild")
	assertRowExists(t, db, "trading_partners", "sender_id", "provider-vitesse-temple")

	var allowedTypes string
	require.NoError(t, db.QueryRow(`SELECT allowed_transaction_types FROM trading_partners WHERE id = 'tp-vitesse-temple'`).Scan(&allowedTypes))
	assert.Contains(t, allowedTypes, "275")
	assert.Contains(t, allowedTypes, "837")

	var validationProfile string
	require.NoError(t, db.QueryRow(`SELECT validation_profile FROM trading_partners WHERE id = 'tp-vitesse-temple'`).Scan(&validationProfile))
	assert.Contains(t, validationProfile, "allowedFileExtensions")
	assert.Contains(t, validationProfile, "maxAttachmentsPerPacket")
	assert.Contains(t, validationProfile, ".txt")
}

func writeMigration(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "migration.sql")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func migrationPath(t *testing.T, filename string) string {
	t.Helper()
	return filepath.Join("..", "..", "infra", "migrations", filename)
}

func resetMigrationSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	downSQL, err := os.ReadFile(migrationPath(t, "000001_init.down.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(downSQL))
	require.NoError(t, err)
}

func assertTableExists(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var exists bool
	require.NoError(t, db.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name = $1
	)`, table).Scan(&exists))
	assert.True(t, exists, table)
}

func assertColumns(t *testing.T, db *sql.DB, table string, columns ...string) {
	t.Helper()
	for _, column := range columns {
		var exists bool
		require.NoError(t, db.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2
		)`, table, column).Scan(&exists))
		assert.True(t, exists, table+"."+column)
	}
}

func assertRowExists(t *testing.T, db *sql.DB, table, column, value string) {
	t.Helper()
	var exists bool
	require.NoError(t, db.QueryRow("SELECT EXISTS (SELECT 1 FROM "+table+" WHERE "+column+" = $1)", value).Scan(&exists))
	assert.True(t, exists, table+"."+column+"="+value)
}

func assertTableCount(t *testing.T, db *sql.DB, table string, expected int) {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM "+table).Scan(&count))
	assert.Equal(t, expected, count, table)
}

func newMigrationMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}
