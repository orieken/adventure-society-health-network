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
	Data  json.RawMessage `json:"data"`
	Lore  string          `json:"lore"`
	Error string          `json:"error"`
}

func TestGatewayRoutesEnrollmentToPayerCore(t *testing.T) {
	var downstreamPath string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPath = r.URL.Path
		assert.Equal(t, http.MethodPost, r.Method)
		return jsonResponse(http.StatusCreated, domain.Envelope{
			Data:        domain.Adventurer{ID: "adv-1", Name: "Farros", CoverageStatus: domain.CoverageActive},
			Lore:        "Society registration accepted.",
			Transaction: &domain.Transaction{Type: domain.Tx834, Status: domain.TxStatusAccepted},
		})
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", client: client})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/adventurers", nil)
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, "/enrollments", downstreamPath)
	assert.Equal(t, "*", response.Header().Get("Access-Control-Allow-Origin"))
	envelope := decodeGatewayEnvelope(t, response)
	assert.Equal(t, "Society registration accepted.", envelope.Lore)
}

func TestGatewayRoutesProvidersToProviderService(t *testing.T) {
	var downstreamPath string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPath = r.URL.Path
		return jsonResponse(http.StatusOK, domain.Envelope{
			Data: []domain.Provider{{ID: "provider-vitesse-temple", Name: "Temple of the Healer, Vitesse"}},
			Lore: "Provider registry opened.",
		})
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", client: client})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/providers", nil)
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "/providers", downstreamPath)
	envelope := decodeGatewayEnvelope(t, response)
	assert.Equal(t, "Provider registry opened.", envelope.Lore)
}

func TestGatewayHealthAggregatesDownstreamServices(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/health", r.URL.Path)
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", client: client})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeGatewayEnvelope(t, response)
	var health map[string]string
	require.NoError(t, json.Unmarshal(envelope.Data, &health))
	assert.Equal(t, "ok", health["api-gateway"])
	assert.Equal(t, "ok", health["payer-core"])
	assert.Equal(t, "ok", health["provider-service"])
	assert.NotEmpty(t, envelope.Lore)
}

func TestGatewayOptionsReturnsCORSPreflight(t *testing.T) {
	handler := gatewayHandler(gateway{payerURL: "http://payer", providerURL: "http://provider"})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/v1/claims", nil)
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "*", response.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, response.Header().Get("Access-Control-Allow-Methods"), "POST")
}

func TestGatewayUnknownRouteReturnsErrorEnvelope(t *testing.T) {
	handler := gatewayHandler(gateway{payerURL: "http://payer", providerURL: "http://provider"})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNotFound, response.Code)
	envelope := decodeGatewayEnvelope(t, response)
	assert.Equal(t, "route not found", envelope.Error)
	assert.NotEmpty(t, envelope.Lore)
}

func gatewayHandler(g gateway) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/", g.route)
	return cors(mux)
}

func decodeGatewayEnvelope(t *testing.T, response *httptest.ResponseRecorder) testEnvelope {
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
