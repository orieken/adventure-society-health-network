package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ashn/packages/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEnvelope struct {
	Data         json.RawMessage      `json:"data"`
	Lore         string               `json:"lore"`
	Transaction  *domain.Transaction  `json:"transaction"`
	Transactions []domain.Transaction `json:"transactions"`
	Error        string               `json:"error"`
}

func TestEnrollCreatesActiveAdventurerAnd834(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	response := serveJSON(t, mux, http.MethodPost, "/enrollments", domain.EnrollmentRequest{
		Name:   "Farros",
		Rank:   domain.RankIron,
		Guild:  "Grim Foundations",
		Region: domain.RegionGreenstone,
	})

	assert.Equal(t, http.StatusCreated, response.Code)
	envelope := decodeEnvelope(t, response)
	require.NotNil(t, envelope.Transaction)
	assert.Equal(t, domain.Tx834, envelope.Transaction.Type)
	assert.Equal(t, domain.TxStatusAccepted, envelope.Transaction.Status)
	assert.NotEmpty(t, envelope.Lore)

	var adventurer domain.Adventurer
	require.NoError(t, json.Unmarshal(envelope.Data, &adventurer))
	assert.Equal(t, domain.CoverageActive, adventurer.CoverageStatus)
	assert.Equal(t, adventurer, app.adventurers[adventurer.ID])
}

func TestEligibilityReturns270And271Pair(t *testing.T) {
	app := newTestStore()
	adventurer := domain.Adventurer{ID: "adv-1", Name: "Farros", Rank: domain.RankIron, Guild: "Grim Foundations", Region: domain.RegionGreenstone, CoverageStatus: domain.CoverageActive}
	app.adventurers[adventurer.ID] = adventurer
	mux := newPayerTestMux(app)

	response := serveJSON(t, mux, http.MethodPost, "/eligibility/query", domain.EligibilityRequest{
		AdventurerID: adventurer.ID,
		ProviderID:   "provider-vitesse-temple",
	})

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeEnvelope(t, response)
	require.Len(t, envelope.Transactions, 2)
	assert.Equal(t, domain.Tx270, envelope.Transactions[0].Type)
	assert.Equal(t, domain.TxStatusDispatched, envelope.Transactions[0].Status)
	assert.Equal(t, domain.Tx271, envelope.Transactions[1].Type)
	assert.Equal(t, domain.TxStatusAccepted, envelope.Transactions[1].Status)
	assert.Len(t, app.transactions, 2)
}

func TestClaimPaymentUpdatesClaimAndEmits835(t *testing.T) {
	app := newTestStore()
	adventurer := domain.Adventurer{ID: "adv-1", Name: "Farros", Rank: domain.RankIron, Guild: "Grim Foundations", Region: domain.RegionGreenstone, CoverageStatus: domain.CoverageActive}
	app.adventurers[adventurer.ID] = adventurer
	mux := newPayerTestMux(app)

	claimResponse := serveJSON(t, mux, http.MethodPost, "/claims", domain.ClaimRequest{
		AdventurerID:     adventurer.ID,
		ProviderID:       "provider-vitesse-temple",
		IncidentSeverity: domain.SeverityAwakened,
		AmountCents:      125000,
	})
	require.Equal(t, http.StatusCreated, claimResponse.Code)
	claimEnvelope := decodeEnvelope(t, claimResponse)
	var claim domain.Claim
	require.NoError(t, json.Unmarshal(claimEnvelope.Data, &claim))

	paymentResponse := serveJSON(t, mux, http.MethodPost, "/claims/"+claim.ID+"/payment", domain.PaymentRequest{PaymentAmountCents: 100000})

	assert.Equal(t, http.StatusOK, paymentResponse.Code)
	paymentEnvelope := decodeEnvelope(t, paymentResponse)
	require.NotNil(t, paymentEnvelope.Transaction)
	assert.Equal(t, domain.Tx835, paymentEnvelope.Transaction.Type)
	assert.Equal(t, domain.TxStatusPaid, paymentEnvelope.Transaction.Status)
	assert.Equal(t, domain.ClaimPaid, app.claims[claim.ID].Status)
}

func TestGetClaimReturnsClaimDetail(t *testing.T) {
	app := newTestStore()
	app.claims["claim-1"] = domain.Claim{ID: "claim-1", AdventurerID: "adv-1", ProviderID: "provider-vitesse-temple", IncidentSeverity: domain.SeverityAwakened, AmountCents: 125000, Status: domain.ClaimSubmitted}
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/claims/claim-1", nil)
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeEnvelope(t, response)
	var claim domain.Claim
	require.NoError(t, json.Unmarshal(envelope.Data, &claim))
	assert.Equal(t, "claim-1", claim.ID)
	assert.Equal(t, domain.ClaimSubmitted, claim.Status)
	assert.NotEmpty(t, envelope.Lore)
}

func TestGetTransactionReturnsTransactionDetail(t *testing.T) {
	app := newTestStore()
	app.transactions["tx-1"] = domain.Transaction{ID: "tx-1", Type: domain.Tx837, Status: domain.TxStatusAccepted, CreatedAt: time.Now()}
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/transactions/tx-1", nil)
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeEnvelope(t, response)
	require.NotNil(t, envelope.Transaction)
	assert.Equal(t, "tx-1", envelope.Transaction.ID)
	assert.Equal(t, domain.Tx837, envelope.Transaction.Type)
}

func TestMissingClaimReturnsErrorEnvelope(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/claims/missing/status", nil)
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNotFound, response.Code)
	envelope := decodeEnvelope(t, response)
	assert.Equal(t, "claim not found", envelope.Error)
	assert.NotEmpty(t, envelope.Lore)
}

func TestListEndpointsReturnPersistedMemory(t *testing.T) {
	app := newTestStore()
	app.adventurers["adv-1"] = domain.Adventurer{ID: "adv-1", Name: "Farros", Rank: domain.RankIron, Guild: "Grim Foundations", Region: domain.RegionGreenstone, CoverageStatus: domain.CoverageActive}
	app.claims["claim-1"] = domain.Claim{ID: "claim-1", AdventurerID: "adv-1", ProviderID: "provider-vitesse-temple", IncidentSeverity: domain.SeverityAwakened, AmountCents: 125000, Status: domain.ClaimSubmitted}
	app.transactions["tx-old"] = domain.Transaction{ID: "tx-old", Type: domain.Tx834, Status: domain.TxStatusAccepted, CreatedAt: time.Now().Add(-time.Hour)}
	app.transactions["tx-new"] = domain.Transaction{ID: "tx-new", Type: domain.Tx837, Status: domain.TxStatusAccepted, CreatedAt: time.Now()}
	mux := newPayerTestMux(app)

	adventurersResponse := httptest.NewRecorder()
	mux.ServeHTTP(adventurersResponse, httptest.NewRequest(http.MethodGet, "/adventurers?limit=10", nil))
	assert.Equal(t, http.StatusOK, adventurersResponse.Code)
	var adventurers []domain.Adventurer
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, adventurersResponse).Data, &adventurers))
	assert.Len(t, adventurers, 1)

	claimsResponse := httptest.NewRecorder()
	mux.ServeHTTP(claimsResponse, httptest.NewRequest(http.MethodGet, "/claims?limit=10", nil))
	assert.Equal(t, http.StatusOK, claimsResponse.Code)
	var claims []domain.Claim
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, claimsResponse).Data, &claims))
	assert.Len(t, claims, 1)

	transactionsResponse := httptest.NewRecorder()
	mux.ServeHTTP(transactionsResponse, httptest.NewRequest(http.MethodGet, "/transactions?limit=1", nil))
	assert.Equal(t, http.StatusOK, transactionsResponse.Code)
	var transactions []domain.Transaction
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, transactionsResponse).Data, &transactions))
	require.Len(t, transactions, 1)
	assert.Equal(t, "tx-new", transactions[0].ID)
}

func newTestStore() *store {
	return &store{
		adventurers:  map[string]domain.Adventurer{},
		providers:    seedProviders(),
		claims:       map[string]domain.Claim{},
		transactions: map[string]domain.Transaction{},
	}
}

func newPayerTestMux(app *store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /enrollments", app.enroll)
	mux.HandleFunc("GET /adventurers", app.listAdventurers)
	mux.HandleFunc("POST /eligibility/query", app.eligibility)
	mux.HandleFunc("GET /claims", app.listClaims)
	mux.HandleFunc("POST /claims", app.submitClaim)
	mux.HandleFunc("GET /claims/{id}", app.getClaim)
	mux.HandleFunc("GET /claims/{id}/status", app.claimStatus)
	mux.HandleFunc("POST /claims/{id}/payment", app.payClaim)
	mux.HandleFunc("GET /transactions", app.listTransactions)
	mux.HandleFunc("GET /transactions/{id}", app.getTransaction)
	return mux
}

func serveJSON(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)
	return response
}

func decodeEnvelope(t *testing.T, response *httptest.ResponseRecorder) testEnvelope {
	t.Helper()
	var envelope testEnvelope
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	return envelope
}
