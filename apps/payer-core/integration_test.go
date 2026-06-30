package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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

	claimResponse := serveJSON(t, mux, http.MethodPost, "/claims", domain.ClaimRequest{
		AdventurerID:     adventurer.ID,
		ProviderID:       "provider-vitesse-temple",
		IncidentSeverity: domain.SeverityAwakened,
		AmountCents:      125000,
	})
	require.Equal(t, http.StatusCreated, claimResponse.Code)
	var claim domain.Claim
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, claimResponse).Data, &claim))

	payment := serveJSON(t, mux, http.MethodPost, "/claims/"+claim.ID+"/payment", domain.PaymentRequest{PaymentAmountCents: 100000})
	require.Equal(t, http.StatusOK, payment.Code)

	assertTableCount(t, db, "providers", 6)
	assertTableCount(t, db, "adventurers", 1)
	assertTableCount(t, db, "claims", 1)
	assertTableCount(t, db, "enrollments", 1)
	assertTableCount(t, db, "auth_requests", 1)
	assertTableCount(t, db, "transactions", 6)

	reloaded := newIntegrationStore(db)
	assert.Contains(t, reloaded.adventurers, adventurer.ID)
	assert.Contains(t, reloaded.claims, claim.ID)
	assert.Len(t, reloaded.providers, 6)
	assert.Len(t, reloaded.transactions, 6)
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
	assert.Contains(t, transactions[0].RawX12, "ISA*")
	assert.Contains(t, transactions[0].RawX12, "ST*835")
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
