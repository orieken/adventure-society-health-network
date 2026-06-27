package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	assert.Len(t, providers, 6)
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
