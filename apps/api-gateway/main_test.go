package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestGatewayRoutesPersistedListsToPayerCore(t *testing.T) {
	paths := []string{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.RequestURI())
		return jsonResponse(http.StatusOK, domain.Envelope{Data: []string{}, Lore: "List returned."})
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", client: client})
	for _, path := range []string{
		"/v1/adventurers?limit=10&offset=10&q=farros",
		"/v1/claims?limit=10&offset=20&status=Paid&providerId=provider-vitesse-temple",
		"/v1/transactions?limit=25&offset=25&type=837&status=Accepted",
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		handler.ServeHTTP(response, request)
		assert.Equal(t, http.StatusOK, response.Code)
	}

	assert.Equal(t, []string{
		"/adventurers?limit=10&offset=10&q=farros",
		"/claims?limit=10&offset=20&status=Paid&providerId=provider-vitesse-temple",
		"/transactions?limit=25&offset=25&type=837&status=Accepted",
	}, paths)
}

func TestGatewayRoutesClaimAndTransactionActionsToPayerCore(t *testing.T) {
	paths := []string{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		return jsonResponse(http.StatusOK, domain.Envelope{Lore: "payer route"})
	})}
	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", client: client})

	for _, item := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/claims"},
		{http.MethodGet, "/v1/claims/claim-1"},
		{http.MethodGet, "/v1/claims/claim-1/status"},
		{http.MethodPost, "/v1/claims/claim-1/payment"},
		{http.MethodGet, "/v1/transactions/tx-1"},
		{http.MethodGet, "/v1/transactions/tx-1/export?format=x12"},
		{http.MethodPost, "/v1/transactions/tx-1/replay"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(item.method, item.path, nil))
		assert.Equal(t, http.StatusOK, response.Code)
	}

	assert.Equal(t, []string{
		"POST /claims",
		"GET /claims/claim-1",
		"GET /claims/claim-1/status",
		"POST /claims/claim-1/payment",
		"GET /transactions/tx-1",
		"GET /transactions/tx-1/export?format=x12",
		"POST /transactions/tx-1/replay",
	}, paths)
}

func TestGatewayRoutesXMLToEDIIntake(t *testing.T) {
	var downstreamPath string
	var downstreamContentType string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPath = r.URL.Path
		downstreamContentType = r.Header.Get("Content-Type")
		assert.Equal(t, http.MethodPost, r.Method)
		return jsonResponse(http.StatusCreated, domain.Envelope{Lore: "XML accepted."})
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", ediURL: "http://edi-intake", client: client})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/x12/xml", strings.NewReader(`<AshnX12Transaction type="837" />`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, "/x12/xml", downstreamPath)
	assert.Equal(t, "application/xml", downstreamContentType)
	assert.Equal(t, "XML accepted.", decodeGatewayEnvelope(t, response).Lore)
}

func TestGatewayRoutesXMLAuditMessagesToEDIIntake(t *testing.T) {
	var downstreamURI string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamURI = r.URL.RequestURI()
		assert.Equal(t, http.MethodGet, r.Method)
		return jsonResponse(http.StatusOK, domain.Envelope{Data: []domain.InboundMessage{}, Lore: "Messages returned."})
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", ediURL: "http://edi-intake", client: client})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/x12/messages?limit=10&offset=20&status=accepted&type=834&q=farros", nil)
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "/x12/messages?limit=10&offset=20&status=accepted&type=834&q=farros", downstreamURI)
	assert.Equal(t, "Messages returned.", decodeGatewayEnvelope(t, response).Lore)
}

func TestGatewayRoutesXMLMessageActionsAndTradingPartnersToEDIIntake(t *testing.T) {
	paths := []string{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.Method+" "+r.URL.RequestURI())
		return jsonResponse(http.StatusOK, domain.Envelope{Lore: "edi route"})
	})}
	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", ediURL: "http://edi-intake", client: client})

	for _, item := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/x12/messages/msg-1/export?format=json"},
		{http.MethodPost, "/v1/x12/messages/msg-1/replay"},
		{http.MethodGet, "/v1/x12/trading-partners"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(item.method, item.path, nil))
		assert.Equal(t, http.StatusOK, response.Code)
	}

	assert.Equal(t, []string{
		"GET /x12/messages/msg-1/export?format=json",
		"POST /x12/messages/msg-1/replay",
		"GET /x12/trading-partners",
	}, paths)
}

func TestGatewayHealthAggregatesDownstreamServices(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/health", r.URL.Path)
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", ediURL: "http://edi-intake", client: client})
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
	assert.Equal(t, "ok", health["edi-intake"])
	assert.NotEmpty(t, envelope.Lore)
}

func TestGatewayHealthMarksUnavailableServices(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Host, "down") {
			return nil, assert.AnError
		}
		return jsonResponse(http.StatusInternalServerError, map[string]string{"status": "bad"})
	})}

	response := httptest.NewRecorder()
	gateway{payerURL: "http://down", providerURL: "http://provider", ediURL: "", client: client}.health(response)

	assert.Equal(t, http.StatusOK, response.Code)
	var envelope testEnvelope
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	var health map[string]string
	require.NoError(t, json.Unmarshal(envelope.Data, &health))
	assert.Equal(t, "unavailable", health["payer-core"])
	assert.Equal(t, "unavailable", health["provider-service"])
	assert.NotContains(t, health, "edi-intake")
}

func TestGatewayHealthRetriesColdStartBadGateway(t *testing.T) {
	attemptsByHost := map[string]int{}
	var attemptsMu sync.Mutex
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attemptsMu.Lock()
		defer attemptsMu.Unlock()
		attemptsByHost[r.URL.Host]++
		if attemptsByHost[r.URL.Host] == 1 {
			return jsonResponse(http.StatusBadGateway, map[string]string{"status": "starting"})
		}
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	})}

	response := httptest.NewRecorder()
	gateway{
		payerURL:               "http://payer-core",
		providerURL:            "http://provider-service",
		ediURL:                 "http://edi-intake",
		client:                 client,
		coldStartRetryAttempts: 2,
	}.health(response)

	assert.Equal(t, http.StatusOK, response.Code)
	var envelope testEnvelope
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	var health map[string]string
	require.NoError(t, json.Unmarshal(envelope.Data, &health))
	assert.Equal(t, "ok", health["payer-core"])
	assert.Equal(t, "ok", health["provider-service"])
	assert.Equal(t, "ok", health["edi-intake"])
	assert.Equal(t, 2, attemptsByHost["payer-core"])
	assert.Equal(t, 2, attemptsByHost["provider-service"])
	assert.Equal(t, 2, attemptsByHost["edi-intake"])
}

func TestGatewayRetriesGETProxyOnColdStartBadGateway(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return jsonResponse(http.StatusBadGateway, domain.ErrorEnvelope{Error: "starting"})
		}
		return jsonResponse(http.StatusOK, domain.Envelope{Lore: "Provider registry opened."})
	})}

	handler := gatewayHandler(gateway{
		providerURL:            "http://provider-service",
		client:                 client,
		coldStartRetryAttempts: 2,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/providers", nil))

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 2, attempts)
	assert.Equal(t, "Provider registry opened.", decodeGatewayEnvelope(t, response).Lore)
}

func TestGatewayDoesNotRetryPOSTProxyToAvoidDuplicateWrites(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		return jsonResponse(http.StatusBadGateway, domain.ErrorEnvelope{Error: "starting"})
	})}

	handler := gatewayHandler(gateway{
		payerURL:               "http://payer-core",
		client:                 client,
		coldStartRetryAttempts: 2,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/adventurers", strings.NewReader(`{"name":"Farros"}`)))

	assert.Equal(t, http.StatusBadGateway, response.Code)
	assert.Equal(t, 1, attempts)
}

func TestGatewayProxyHandlesRequestCreationAndDownstreamErrors(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	gateway{}.proxy(response, request, "://bad-url", "/test")
	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "request creation failed", decodeGatewayEnvelope(t, response).Error)

	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, assert.AnError
	})}
	response = httptest.NewRecorder()
	gateway{client: client}.proxy(response, request, "http://downstream", "/test")
	assert.Equal(t, http.StatusBadGateway, response.Code)
	assert.Equal(t, "downstream unavailable", decodeGatewayEnvelope(t, response).Error)
}

func TestGatewayEnvHTTPClientAndLogMiddleware(t *testing.T) {
	t.Setenv("GATEWAY_TEST_ENV", "configured")
	assert.Equal(t, "configured", env("GATEWAY_TEST_ENV", "fallback"))
	assert.Equal(t, "fallback", env("GATEWAY_MISSING_ENV", "fallback"))
	assert.Same(t, http.DefaultClient, gateway{}.httpClient())

	called := false
	handler := logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.True(t, called)
	assert.Equal(t, http.StatusNoContent, response.Code)
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

func TestAPIGatewayOpenAPIIncludesPublicRoutes(t *testing.T) {
	spec := apiGatewayOpenAPI()

	info := spec["info"].(map[string]string)
	assert.Equal(t, "ASHN API Gateway", info["title"])
	paths := spec["paths"].(map[string]any)
	assert.Contains(t, paths, "/v1/health")
	assert.Contains(t, paths, "/v1/x12/xml")
	assert.Contains(t, paths, "/v1/transactions/{id}/export")
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
