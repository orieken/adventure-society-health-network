package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"ashn/packages/asyncjobs"
	"ashn/packages/domain"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationPostgresPersistsAndHydratesWorkflow(t *testing.T) {
	if os.Getenv("ASHN_INTEGRATION") != "1" {
		t.Skip("set ASHN_INTEGRATION=1 to run Postgres-backed integration tests")
	}

	dsn := os.Getenv("DATABASE_URL")
	require.NotEmpty(t, dsn, "DATABASE_URL is required for integration tests")

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	resetIntegrationSchema(t, db)

	app := newIntegrationStore(db)
	mux := newPayerIntegrationMux(app)

	enrollment := serveJSON(t, mux, http.MethodPost, "/enrollments", domain.EnrollmentRequest{
		Name:   "Farros Integration",
		Rank:   domain.RankIron,
		Guild:  "Grim Foundations",
		Region: domain.RegionGreenstone,
	})
	require.Equal(t, http.StatusCreated, enrollment.Code)
	var adventurer domain.Adventurer
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, enrollment).Data, &adventurer))
	require.NotEmpty(t, adventurer.ID)

	eligibility := serveJSON(t, mux, http.MethodPost, "/eligibility/query", domain.EligibilityRequest{
		AdventurerID: adventurer.ID,
		ProviderID:   "provider-vitesse-temple",
	})
	require.Equal(t, http.StatusOK, eligibility.Code)
	require.Len(t, decodeEnvelope(t, eligibility).Transactions, 2)

	auth := serveJSON(t, mux, http.MethodPost, "/auth-requests", domain.PriorAuthRequest{
		AdventurerID:     adventurer.ID,
		ProviderID:       "provider-vitesse-temple",
		ServiceType:      "resurrection",
		IncidentSeverity: domain.SeverityDiamond,
	})
	require.Equal(t, http.StatusAccepted, auth.Code)
	require.NoError(t, forceDueJobs(db))
	processed, err := asyncjobs.ProcessDue(db, 5)
	require.NoError(t, err)
	assert.Equal(t, 1, processed)
	assertTransactionStatus(t, db, domain.Tx278, domain.TxStatusApproved)

	claimResponse := serveJSON(t, mux, http.MethodPost, "/claims", domain.ClaimRequest{
		AdventurerID:     adventurer.ID,
		ProviderID:       "provider-vitesse-temple",
		IncidentSeverity: domain.SeverityAwakened,
		AmountCents:      125000,
	})
	require.Equal(t, http.StatusCreated, claimResponse.Code)
	var claim domain.Claim
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, claimResponse).Data, &claim))

	require.NoError(t, forceDueJobs(db))
	processed, err = asyncjobs.ProcessDue(db, 5)
	require.NoError(t, err)
	assert.Equal(t, 1, processed)
	assertClaimStatus(t, db, claim.ID, domain.ClaimPending)

	require.NoError(t, forceDueJobs(db))
	processed, err = asyncjobs.ProcessDue(db, 5)
	require.NoError(t, err)
	assert.Equal(t, 1, processed)
	assertClaimStatus(t, db, claim.ID, domain.ClaimApproved)
	assertClaimAdjudication(t, db, claim.ID, 106250, 97750, 8500, 18750, "")

	payment := serveJSON(t, mux, http.MethodPost, "/claims/"+claim.ID+"/payment", domain.PaymentRequest{PaymentAmountCents: 100000})
	require.Equal(t, http.StatusOK, payment.Code)

	assertTableCount(t, db, "providers", 6)
	assertTableCount(t, db, "adventurers", 1)
	assertTableCount(t, db, "claims", 1)
	assertTableCount(t, db, "enrollments", 1)
	assertTableCount(t, db, "auth_requests", 1)
	assertTableCount(t, db, "transactions", 8)
	assertTableCount(t, db, "transaction_jobs", 3)

	reloaded := newIntegrationStore(db)
	assert.Contains(t, reloaded.adventurers, adventurer.ID)
	assert.Contains(t, reloaded.claims, claim.ID)
	assert.Len(t, reloaded.providers, 6)
	assert.Len(t, reloaded.transactions, 8)
	assert.Equal(t, domain.ClaimPaid, reloaded.claims[claim.ID].Status)

	listClaims := httptest.NewRecorder()
	mux.ServeHTTP(listClaims, httptest.NewRequest(http.MethodGet, "/claims?limit=10&status=Paid&providerId=provider-vitesse-temple&q=paid", nil))
	require.Equal(t, http.StatusOK, listClaims.Code)
	claimsEnvelope := decodeEnvelope(t, listClaims)
	var claims []domain.Claim
	require.NoError(t, json.Unmarshal(claimsEnvelope.Data, &claims))
	require.Len(t, claims, 1)
	assert.Equal(t, domain.ClaimPaid, claims[0].Status)
	require.NotNil(t, claimsEnvelope.Page)
	assert.Equal(t, 0, claimsEnvelope.Page.Offset)

	listTransactions := httptest.NewRecorder()
	mux.ServeHTTP(listTransactions, httptest.NewRequest(http.MethodGet, "/transactions?limit=1&type=835&status=Paid", nil))
	require.Equal(t, http.StatusOK, listTransactions.Code)
	transactionsEnvelope := decodeEnvelope(t, listTransactions)
	var transactions []domain.Transaction
	require.NoError(t, json.Unmarshal(transactionsEnvelope.Data, &transactions))
	require.Len(t, transactions, 1)
	assert.Equal(t, domain.Tx835, transactions[0].Type)
	var remittance map[string]any
	require.NoError(t, json.Unmarshal(transactions[0].Payload, &remittance))
	assert.Equal(t, float64(125000), remittance["billedAmountCents"])
	assert.Equal(t, float64(106250), remittance["allowedAmountCents"])
	assert.Equal(t, float64(97750), remittance["paymentAmountCents"])
	assert.Equal(t, float64(8500), remittance["patientResponsibilityCents"])
	assert.Equal(t, float64(18750), remittance["adjustmentAmountCents"])
	assert.Contains(t, transactions[0].RawX12, "ISA*")
	assert.Contains(t, transactions[0].RawX12, "ST*835")
	assert.Contains(t, transactions[0].RawX12, "CLP*")
	assert.Contains(t, transactions[0].RawX12, "*1250.00*977.50*85.00*")
	require.NotNil(t, transactionsEnvelope.Page)
	assert.False(t, transactionsEnvelope.Page.HasMore)

	listAdventurers := httptest.NewRecorder()
	mux.ServeHTTP(listAdventurers, httptest.NewRequest(http.MethodGet, "/adventurers?limit=10&q=Farros%20Integration&rank=Iron&region=Greenstone&coverageStatus=Active", nil))
	require.Equal(t, http.StatusOK, listAdventurers.Code)
	var adventurers []domain.Adventurer
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, listAdventurers).Data, &adventurers))
	require.Len(t, adventurers, 1)
	assert.Equal(t, adventurer.ID, adventurers[0].ID)
}

func forceDueJobs(db *sql.DB) error {
	_, err := db.Exec(`UPDATE transaction_jobs SET run_after = now() WHERE status = $1`, asyncjobs.StatusPending)
	return err
}

func assertClaimStatus(t *testing.T, db *sql.DB, claimID string, expected domain.ClaimStatus) {
	t.Helper()
	var status domain.ClaimStatus
	require.NoError(t, db.QueryRow(`SELECT status FROM claims WHERE id = $1`, claimID).Scan(&status))
	assert.Equal(t, expected, status)
}

func assertClaimAdjudication(t *testing.T, db *sql.DB, claimID string, allowed, paid, patient, adjustment int64, denialReason string) {
	t.Helper()
	var actualAllowed, actualPaid, actualPatient, actualAdjustment int64
	var actualDenialReason string
	require.NoError(t, db.QueryRow(`SELECT allowed_amount_cents, paid_amount_cents, patient_responsibility_cents, adjustment_amount_cents, COALESCE(denial_reason, '') FROM claims WHERE id = $1`, claimID).
		Scan(&actualAllowed, &actualPaid, &actualPatient, &actualAdjustment, &actualDenialReason))
	assert.Equal(t, allowed, actualAllowed)
	assert.Equal(t, paid, actualPaid)
	assert.Equal(t, patient, actualPatient)
	assert.Equal(t, adjustment, actualAdjustment)
	assert.Equal(t, denialReason, actualDenialReason)
}

func assertTransactionStatus(t *testing.T, db *sql.DB, txType domain.TransactionType, expected domain.TransactionStatus) {
	t.Helper()
	var status domain.TransactionStatus
	require.NoError(t, db.QueryRow(`SELECT status FROM transactions WHERE type = $1 ORDER BY created_at DESC LIMIT 1`, txType).Scan(&status))
	assert.Equal(t, expected, status)
}

func newIntegrationStore(db *sql.DB) *store {
	return &store{
		adventurers:  loadAdventurers(db),
		providers:    loadProviders(db),
		claims:       loadClaims(db),
		transactions: loadTransactions(db),
		db:           db,
	}
}

func newPayerIntegrationMux(app *store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /enrollments", app.enroll)
	mux.HandleFunc("GET /adventurers", app.listAdventurers)
	mux.HandleFunc("GET /adventurers/{id}", app.getAdventurer)
	mux.HandleFunc("POST /eligibility/query", app.eligibility)
	mux.HandleFunc("POST /auth-requests", app.authRequest)
	mux.HandleFunc("GET /claims", app.listClaims)
	mux.HandleFunc("POST /claims", app.submitClaim)
	mux.HandleFunc("GET /claims/{id}", app.getClaim)
	mux.HandleFunc("GET /claims/{id}/status", app.claimStatus)
	mux.HandleFunc("POST /claims/{id}/payment", app.payClaim)
	mux.HandleFunc("GET /transactions", app.listTransactions)
	mux.HandleFunc("GET /transactions/{id}", app.getTransaction)
	return mux
}

func resetIntegrationSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	root := filepath.Join("..", "..")
	downSQL, err := os.ReadFile(filepath.Join(root, "infra", "migrations", "000001_init.down.sql"))
	require.NoError(t, err)
	upSQL, err := os.ReadFile(filepath.Join(root, "infra", "migrations", "000001_init.up.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(downSQL))
	require.NoError(t, err)
	_, err = db.Exec(string(upSQL))
	require.NoError(t, err)
}

func assertTableCount(t *testing.T, db *sql.DB, table string, expected int) {
	t.Helper()
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM "+table).Scan(&count))
	assert.Equal(t, expected, count, table)
}
