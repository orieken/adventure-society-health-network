package main

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvDurationUsesFallbackForMissingOrInvalidValues(t *testing.T) {
	t.Setenv("TX_TEST_DURATION", "")
	assert.Equal(t, 2*time.Second, envDuration("TX_TEST_DURATION", 2*time.Second))

	t.Setenv("TX_TEST_DURATION", "not-a-duration")
	assert.Equal(t, 2*time.Second, envDuration("TX_TEST_DURATION", 2*time.Second))
}

func TestEnvDurationParsesConfiguredValue(t *testing.T) {
	t.Setenv("TX_TEST_DURATION", "250ms")

	assert.Equal(t, 250*time.Millisecond, envDuration("TX_TEST_DURATION", time.Second))
}

func TestEnvIntUsesFallbackForMissingInvalidOrNonPositiveValues(t *testing.T) {
	t.Setenv("TX_TEST_INT", "")
	assert.Equal(t, 5, envInt("TX_TEST_INT", 5))

	t.Setenv("TX_TEST_INT", "nope")
	assert.Equal(t, 5, envInt("TX_TEST_INT", 5))

	t.Setenv("TX_TEST_INT", "0")
	assert.Equal(t, 5, envInt("TX_TEST_INT", 5))
}

func TestEnvIntParsesConfiguredValue(t *testing.T) {
	t.Setenv("TX_TEST_INT", "12")

	assert.Equal(t, 12, envInt("TX_TEST_INT", 5))
}

func TestOpenDBReturnsNilWithoutDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	assert.Nil(t, openDB())
}

func TestOpenDBWithHandlesOpenPingAndSuccess(t *testing.T) {
	assert.Nil(t, openDBWith("dsn", func(_, _ string) (*sql.DB, error) {
		return nil, assert.AnError
	}))

	pingDB, pingMock, pingCleanup := newWorkerMockDBWithPing(t)
	defer pingCleanup()
	pingMock.ExpectPing().WillReturnError(assert.AnError)
	assert.Nil(t, openDBWith("dsn", func(driverName, dsn string) (*sql.DB, error) {
		assert.Equal(t, "postgres", driverName)
		assert.Equal(t, "dsn", dsn)
		return pingDB, nil
	}))

	okDB, okMock, okCleanup := newWorkerMockDBWithPing(t)
	defer okCleanup()
	okMock.ExpectPing()
	assert.NotNil(t, openDBWith("dsn", func(_, _ string) (*sql.DB, error) {
		return okDB, nil
	}))
	require.NoError(t, okMock.ExpectationsWereMet())
}

func TestProcessHandlesNilDatabaseWithoutPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		process(nil, 1)
	})
}

func TestProcessHandlesNoReadyJobs(t *testing.T) {
	db, mock, cleanup := newWorkerMockDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, job_type, entity_id, status, attempts, run_after, COALESCE(last_error, ''), created_at, updated_at
		 FROM transaction_jobs
		 WHERE status = $1 AND run_after <= now()
		 ORDER BY run_after, created_at
		 FOR UPDATE SKIP LOCKED
		 LIMIT 1`)).
		WithArgs("pending").
		WillReturnRows(sqlmock.NewRows([]string{"id", "job_type", "entity_id", "status", "attempts", "run_after", "last_error", "created_at", "updated_at"}))
	mock.ExpectRollback()

	assert.NotPanics(t, func() {
		process(db, 1)
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProcessLogsProcessedJobs(t *testing.T) {
	db, mock, cleanup := newWorkerMockDB(t)
	defer cleanup()
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, job_type, entity_id, status, attempts, run_after, COALESCE(last_error, ''), created_at, updated_at
		 FROM transaction_jobs
		 WHERE status = $1 AND run_after <= now()
		 ORDER BY run_after, created_at
		 FOR UPDATE SKIP LOCKED
		 LIMIT 1`)).
		WithArgs("pending").
		WillReturnRows(sqlmock.NewRows([]string{"id", "job_type", "entity_id", "status", "attempts", "run_after", "last_error", "created_at", "updated_at"}).
			AddRow("job-1", "unsupported", "entity-1", "pending", 0, now, "", now, now))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE transaction_jobs SET status = $1, attempts = attempts + 1, updated_at = now() WHERE id = $2`)).
		WithArgs("processing", "job-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE transaction_jobs SET status = $1, run_after = $2, last_error = $3, updated_at = now() WHERE id = $4`)).
		WithArgs("pending", sqlmock.AnyArg(), workerJSONErrorArg{contains: "unsupported job type"}, "job-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	assert.NotPanics(t, func() {
		process(db, 1)
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func newWorkerMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}

func newWorkerMockDBWithPing(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}

type workerJSONErrorArg struct {
	contains string
}

func (arg workerJSONErrorArg) Match(value driver.Value) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return false
	}
	return strings.Contains(payload["error"], arg.contains)
}
