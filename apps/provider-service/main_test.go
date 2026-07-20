package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ashn/packages/domain"

	"github.com/DATA-DOG/go-sqlmock"
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

func TestListProvidersReturnsSeededCatalog(t *testing.T) {
	app := providerApp{providers: seedProviders(), payerURL: "http://unused"}
	mux := newProviderTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/providers", nil)
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeProviderEnvelope(t, response)
	assert.NotEmpty(t, envelope.Lore)
	var providers []domain.Provider
	require.NoError(t, json.Unmarshal(envelope.Data, &providers))
	assert.Len(t, providers, 7)
}

func TestGetProviderReturnsNotFoundEnvelope(t *testing.T) {
	app := providerApp{providers: seedProviders(), payerURL: "http://unused"}
	mux := newProviderTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/providers/not-a-provider", nil)
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNotFound, response.Code)
	envelope := decodeProviderEnvelope(t, response)
	assert.Equal(t, "provider not found", envelope.Error)
	assert.NotEmpty(t, envelope.Lore)
}

func TestGetProviderReturnsProviderDetail(t *testing.T) {
	app := providerApp{providers: seedProviders(), payerURL: "http://unused"}
	mux := newProviderTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/providers/provider-vitesse-temple", nil)
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	var provider domain.Provider
	require.NoError(t, json.Unmarshal(decodeProviderEnvelope(t, response).Data, &provider))
	assert.Equal(t, "Temple of the Healer, Vitesse", provider.Name)
}

func TestVerifyEligibilityForwardsProviderIDToPayer(t *testing.T) {
	var forwarded domain.EligibilityRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/eligibility/query", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&forwarded))
		return jsonResponse(http.StatusOK, domain.Envelope{
			Data: map[string]any{"eligible": true},
			Lore: "Eligibility confirmed by mock payer.",
			Transactions: []domain.Transaction{
				{Type: domain.Tx270, Status: domain.TxStatusDispatched},
				{Type: domain.Tx271, Status: domain.TxStatusAccepted},
			},
		})
	})}

	app := providerApp{providers: seedProviders(), payerURL: "http://payer-core", client: client}
	mux := newProviderTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/providers/provider-vitesse-temple/verify-eligibility", jsonBody(t, map[string]string{"adventurerId": "adv-1"}))
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "adv-1", forwarded.AdventurerID)
	assert.Equal(t, "provider-vitesse-temple", forwarded.ProviderID)
	envelope := decodeProviderEnvelope(t, response)
	require.Len(t, envelope.Transactions, 2)
	assert.Equal(t, domain.Tx270, envelope.Transactions[0].Type)
	assert.Equal(t, domain.Tx271, envelope.Transactions[1].Type)
}

func TestSubmitClaimOverridesProviderIDFromRoute(t *testing.T) {
	var forwarded domain.ClaimRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/claims", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&forwarded))
		return jsonResponse(http.StatusCreated, domain.Envelope{
			Data:        domain.Claim{ID: "claim-1", ProviderID: forwarded.ProviderID, Status: domain.ClaimSubmitted},
			Lore:        "Claim accepted by mock payer.",
			Transaction: &domain.Transaction{Type: domain.Tx837, Status: domain.TxStatusAccepted},
		})
	})}

	app := providerApp{providers: seedProviders(), payerURL: "http://payer-core", client: client}
	mux := newProviderTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/providers/provider-rimaros-hospital/submit-claim", jsonBody(t, domain.ClaimRequest{
		AdventurerID:     "adv-1",
		ProviderID:       "malicious-provider",
		IncidentSeverity: domain.SeverityAwakened,
		AmountCents:      125000,
	}))
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, "provider-rimaros-hospital", forwarded.ProviderID)
	assert.Equal(t, "adv-1", forwarded.AdventurerID)
}

func TestProviderWorkflowMissingProviderAndInvalidJSON(t *testing.T) {
	app := providerApp{providers: seedProviders(), payerURL: "http://unused"}
	mux := newProviderTestMux(app)

	missingEligibility := httptest.NewRecorder()
	mux.ServeHTTP(missingEligibility, httptest.NewRequest(http.MethodPost, "/providers/missing/verify-eligibility", jsonBody(t, map[string]string{"adventurerId": "adv-1"})))
	assert.Equal(t, http.StatusNotFound, missingEligibility.Code)
	assert.Equal(t, "provider not found", decodeProviderEnvelope(t, missingEligibility).Error)

	missingClaim := httptest.NewRecorder()
	mux.ServeHTTP(missingClaim, httptest.NewRequest(http.MethodPost, "/providers/missing/submit-claim", jsonBody(t, domain.ClaimRequest{})))
	assert.Equal(t, http.StatusNotFound, missingClaim.Code)
	assert.Equal(t, "provider not found", decodeProviderEnvelope(t, missingClaim).Error)

	invalidJSON := httptest.NewRecorder()
	mux.ServeHTTP(invalidJSON, httptest.NewRequest(http.MethodPost, "/providers/provider-vitesse-temple/submit-claim", strings.NewReader("{")))
	assert.Equal(t, http.StatusBadRequest, invalidJSON.Code)
	assert.Equal(t, "invalid json", decodeProviderEnvelope(t, invalidJSON).Error)
}

func TestProviderForwardHandlesDownstreamUnavailable(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, assert.AnError
	})}
	app := providerApp{providers: seedProviders(), payerURL: "http://payer-core", client: client}
	mux := newProviderTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/providers/provider-vitesse-temple/verify-eligibility", jsonBody(t, map[string]string{"adventurerId": "adv-1"}))
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadGateway, response.Code)
	assert.Equal(t, "payer-core unavailable", decodeProviderEnvelope(t, response).Error)
}

func TestProviderForwardHandlesRequestCreationFailureAndHTTPClientFallback(t *testing.T) {
	app := providerApp{providers: seedProviders(), payerURL: "://bad-url"}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/providers/provider-vitesse-temple/submit-claim", nil)

	app.forward(response, request, http.MethodPost, "/claims", map[string]string{"ok": "true"})

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "request creation failed", decodeProviderEnvelope(t, response).Error)
	assert.Same(t, http.DefaultClient, providerApp{}.httpClient())
}

func TestProviderLoadProvidersReadsDatabaseAndFallsBackWhenEmpty(t *testing.T) {
	db, mock, cleanup := newProviderMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, name, provider_type, tier_rank, region FROM providers").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "provider_type", "tier_rank", "region"}).
			AddRow("provider-1", "Provider One", domain.ProviderTypeClinic, domain.RankGold, domain.RegionVitesse))

	providers := loadProviders(db)

	require.Len(t, providers, 1)
	assert.Equal(t, "Provider One", providers["provider-1"].Name)
	require.NoError(t, mock.ExpectationsWereMet())

	emptyDB, emptyMock, emptyCleanup := newProviderMockDB(t)
	defer emptyCleanup()
	emptyMock.ExpectQuery("SELECT id, name, provider_type, tier_rank, region FROM providers").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "provider_type", "tier_rank", "region"}))
	assert.Len(t, loadProviders(emptyDB), 7)
	require.NoError(t, emptyMock.ExpectationsWereMet())
}

func TestProviderLoadProvidersFallsBackOnDatabaseErrorAndOpenDBNoEnv(t *testing.T) {
	db, mock, cleanup := newProviderMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, name, provider_type, tier_rank, region FROM providers").
		WillReturnError(assert.AnError)
	assert.Len(t, loadProviders(db), 7)
	require.NoError(t, mock.ExpectationsWereMet())

	t.Setenv("DATABASE_URL", "")
	assert.Nil(t, openDB())

	assert.Nil(t, openDBWith("dsn", func(_, _ string) (*sql.DB, error) {
		return nil, assert.AnError
	}))

	pingDB, pingMock, pingCleanup := newProviderMockDBWithPing(t)
	defer pingCleanup()
	pingMock.ExpectPing().WillReturnError(assert.AnError)
	assert.Nil(t, openDBWith("dsn", func(_, _ string) (*sql.DB, error) {
		return pingDB, nil
	}))

	okDB, okMock, okCleanup := newProviderMockDBWithPing(t)
	defer okCleanup()
	okMock.ExpectPing()
	assert.NotNil(t, openDBWith("dsn", func(driverName, dsn string) (*sql.DB, error) {
		assert.Equal(t, "postgres", driverName)
		assert.Equal(t, "dsn", dsn)
		return okDB, nil
	}))
	require.NoError(t, okMock.ExpectationsWereMet())
}

func TestProviderLoadProvidersSkipsBadRowsAndFallsBackOnRowsError(t *testing.T) {
	db, mock, cleanup := newProviderMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, name, provider_type, tier_rank, region FROM providers").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "provider_type", "tier_rank", "region"}).
			AddRow("provider-good", "Provider Good", domain.ProviderTypeClinic, domain.RankGold, domain.RegionVitesse).
			AddRow(nil, "Provider Bad", domain.ProviderTypeClinic, domain.RankIron, domain.RegionGreenstone))

	providers := loadProviders(db)

	require.Len(t, providers, 1)
	assert.Contains(t, providers, "provider-good")
	require.NoError(t, mock.ExpectationsWereMet())

	rowsErrDB, rowsErrMock, rowsErrCleanup := newProviderMockDB(t)
	defer rowsErrCleanup()
	rows := sqlmock.NewRows([]string{"id", "name", "provider_type", "tier_rank", "region"}).
		AddRow("provider-1", "Provider One", domain.ProviderTypeClinic, domain.RankGold, domain.RegionVitesse).
		RowError(0, assert.AnError)
	rowsErrMock.ExpectQuery("SELECT id, name, provider_type, tier_rank, region FROM providers").WillReturnRows(rows)

	assert.Len(t, loadProviders(rowsErrDB), 7)
	require.NoError(t, rowsErrMock.ExpectationsWereMet())
}

func TestProviderHealthEnvAndLogMiddleware(t *testing.T) {
	t.Setenv("PROVIDER_TEST_ENV", "configured")
	assert.Equal(t, "configured", env("PROVIDER_TEST_ENV", "fallback"))
	assert.Equal(t, "fallback", env("PROVIDER_MISSING_ENV", "fallback"))

	healthResponse := httptest.NewRecorder()
	health(healthResponse, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, healthResponse.Code)
	assert.Contains(t, healthResponse.Body.String(), "provider-service")

	called := false
	handler := logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/anything", nil))
	assert.True(t, called)
	assert.Equal(t, http.StatusNoContent, response.Code)
}

func TestProviderOpenAPIIncludesProviderRoutes(t *testing.T) {
	spec := providerOpenAPI()

	info := spec["info"].(map[string]string)
	assert.Equal(t, "ASHN Provider Service", info["title"])
	paths := spec["paths"].(map[string]any)
	assert.Contains(t, paths, "/health")
	assert.Contains(t, paths, "/providers")
	assert.Contains(t, paths, "/providers/{id}/submit-claim")
}

func newProviderTestMux(app providerApp) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /providers", app.listProviders)
	mux.HandleFunc("GET /providers/{id}", app.getProvider)
	mux.HandleFunc("POST /providers/{id}/verify-eligibility", app.verifyEligibility)
	mux.HandleFunc("POST /providers/{id}/submit-claim", app.submitClaim)
	return mux
}

func jsonBody(t *testing.T, value any) *strings.Reader {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	return strings.NewReader(string(payload))
}

func decodeProviderEnvelope(t *testing.T, response *httptest.ResponseRecorder) testEnvelope {
	t.Helper()
	var envelope testEnvelope
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	return envelope
}

func newProviderMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}

func newProviderMockDBWithPing(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, value any) (*http.Response, error) {
	payload, _ := json.Marshal(value)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(payload))),
	}, nil
}
