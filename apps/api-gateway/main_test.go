package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ashn/packages/domain"
	"ashn/packages/requestmeta"

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
		{http.MethodPost, "/v1/auth-requests/tx-278/decision"},
		{http.MethodPost, "/v1/auth-requests/tx-278/attachments"},
		{http.MethodGet, "/v1/claims/claim-1"},
		{http.MethodGet, "/v1/claims/claim-1/status"},
		{http.MethodPost, "/v1/claims/claim-1/documentation-request"},
		{http.MethodPost, "/v1/claims/claim-1/attachments"},
		{http.MethodPost, "/v1/claims/claim-1/payment"},
		{http.MethodGet, "/v1/premium-payments?adventurerId=adv-1&limit=10"},
		{http.MethodGet, "/v1/premium-payments/premium-1"},
		{http.MethodGet, "/v1/premium-payments/premium-1/export?format=xml"},
		{http.MethodPost, "/v1/premium-payments"},
		{http.MethodPost, "/v1/benefit-coordination"},
		{http.MethodGet, "/v1/transactions/tx-1"},
		{http.MethodGet, "/v1/transactions/tx-1/export?format=x12"},
		{http.MethodGet, "/v1/transactions/tx-275/document-reference"},
		{http.MethodGet, "/v1/transactions/tx-275/document-reference/content"},
		{http.MethodPost, "/v1/transactions/tx-1/replay"},
		{http.MethodPost, "/v1/transactions/tx-275/attachment-review"},
		{http.MethodGet, "/v1/jobs?limit=8"},
		{http.MethodPost, "/v1/jobs/job-1/replay"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(item.method, item.path, nil))
		assert.Equal(t, http.StatusOK, response.Code)
	}

	assert.Equal(t, []string{
		"POST /claims",
		"POST /auth-requests/tx-278/decision",
		"POST /auth-requests/tx-278/attachments",
		"GET /claims/claim-1",
		"GET /claims/claim-1/status",
		"POST /claims/claim-1/documentation-request",
		"POST /claims/claim-1/attachments",
		"POST /claims/claim-1/payment",
		"GET /premium-payments?adventurerId=adv-1&limit=10",
		"GET /premium-payments/premium-1",
		"GET /premium-payments/premium-1/export?format=xml",
		"POST /premium-payments",
		"POST /benefit-coordination",
		"GET /transactions/tx-1",
		"GET /transactions/tx-1/export?format=x12",
		"GET /transactions/tx-275/document-reference",
		"GET /transactions/tx-275/document-reference/content",
		"POST /transactions/tx-1/replay",
		"POST /transactions/tx-275/attachment-review",
		"GET /jobs?limit=8",
		"POST /jobs/job-1/replay",
	}, paths)
}

func TestGatewayRoutesXMLToEDIIntake(t *testing.T) {
	downstreamPaths := []string{}
	downstreamContentTypes := []string{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPaths = append(downstreamPaths, r.URL.Path)
		downstreamContentTypes = append(downstreamContentTypes, r.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPost, r.Method)
		return jsonResponse(http.StatusCreated, domain.Envelope{Lore: "Intake accepted."})
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", ediURL: "http://edi-intake", client: client})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/x12/xml", strings.NewReader(`<AshnX12Transaction type="837" />`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, "Intake accepted.", decodeGatewayEnvelope(t, response).Lore)

	jsonResponseRecorder := httptest.NewRecorder()
	jsonRequest := httptest.NewRequest(http.MethodPost, "/v1/x12/transactions", strings.NewReader(`{"type":"837"}`))
	jsonRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(jsonResponseRecorder, jsonRequest)

	rawResponseRecorder := httptest.NewRecorder()
	rawRequest := httptest.NewRequest(http.MethodPost, "/v1/x12/raw", strings.NewReader(`ST*837*0001~SE*2*0001~`))
	rawRequest.Header.Set("Content-Type", "application/edi-x12")
	handler.ServeHTTP(rawResponseRecorder, rawRequest)

	assert.Equal(t, http.StatusCreated, jsonResponseRecorder.Code)
	assert.Equal(t, http.StatusCreated, rawResponseRecorder.Code)
	assert.Equal(t, []string{"/x12/xml", "/x12/transactions", "/x12/raw"}, downstreamPaths)
	assert.Equal(t, []string{"application/xml", "application/json", "application/edi-x12"}, downstreamContentTypes)
}

func TestGatewayPropagatesRequestAndCorrelationIDs(t *testing.T) {
	var downstreamRequestID string
	var downstreamCorrelationID string
	var downstreamTraceparent string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamRequestID = r.Header.Get(requestmeta.RequestIDHeader)
		downstreamCorrelationID = r.Header.Get(requestmeta.CorrelationIDHeader)
		downstreamTraceparent = r.Header.Get(requestmeta.TraceparentHeader)
		return jsonResponse(http.StatusOK, domain.Envelope{Lore: "Traced."})
	})}
	handler := requestmeta.Middleware("api-gateway-test", gatewayHandler(gateway{payerURL: "http://payer-core", client: client}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/transactions", nil)
	request.Header.Set(requestmeta.RequestIDHeader, "req-demo")
	request.Header.Set(requestmeta.CorrelationIDHeader, "corr-demo")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "req-demo", response.Header().Get(requestmeta.RequestIDHeader))
	assert.Equal(t, "corr-demo", response.Header().Get(requestmeta.CorrelationIDHeader))
	assert.Equal(t, "req-demo", downstreamRequestID)
	assert.Equal(t, "corr-demo", downstreamCorrelationID)
	assert.NotEmpty(t, response.Header().Get(requestmeta.TraceparentHeader))
	assert.Equal(t, response.Header().Get(requestmeta.TraceparentHeader), downstreamTraceparent)
}

func TestGatewayAuthIsDisabledWhenNoAPIKeysAreConfigured(t *testing.T) {
	called := false
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		assert.Equal(t, "/x12/xml", r.URL.Path)
		return jsonResponse(http.StatusCreated, domain.Envelope{Lore: "Intake accepted."})
	})}
	handler := gatewayHandler(gateway{ediURL: "http://edi-intake", client: client})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/x12/xml", strings.NewReader(`<AshnX12Transaction type="837" />`))
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.True(t, called)
}

func TestGatewayRequiresConfiguredAPIKeyForV1Routes(t *testing.T) {
	called := false
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		called = true
		return jsonResponse(http.StatusCreated, domain.Envelope{Lore: "Intake accepted."})
	})}
	handler := gatewayHandler(gateway{ediURL: "http://edi-intake", client: client, apiKeys: []string{"society-secret"}})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/x12/xml", strings.NewReader(`<AshnX12Transaction type="837" />`))
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.False(t, called)
	assert.Equal(t, "unauthorized", decodeGatewayEnvelope(t, response).Error)
	assert.Contains(t, response.Header().Get("WWW-Authenticate"), "Bearer")
}

func TestGatewayAcceptsBearerOrAPIKeyHeader(t *testing.T) {
	paths := []string{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		return jsonResponse(http.StatusOK, domain.Envelope{Lore: "Authorized."})
	})}
	handler := gatewayHandler(gateway{ediURL: "http://edi-intake", client: client, apiKeys: []string{"society-secret"}})

	bearerResponse := httptest.NewRecorder()
	bearerRequest := httptest.NewRequest(http.MethodPost, "/v1/x12/xml", strings.NewReader(`<AshnX12Transaction type="837" />`))
	bearerRequest.Header.Set("Authorization", "Bearer society-secret")
	handler.ServeHTTP(bearerResponse, bearerRequest)

	headerResponse := httptest.NewRecorder()
	headerRequest := httptest.NewRequest(http.MethodPost, "/v1/x12/transactions", strings.NewReader(`{"type":"837"}`))
	headerRequest.Header.Set("X-ASHN-API-Key", "society-secret")
	handler.ServeHTTP(headerResponse, headerRequest)

	assert.Equal(t, http.StatusOK, bearerResponse.Code)
	assert.Equal(t, http.StatusOK, headerResponse.Code)
	assert.Equal(t, []string{"/x12/xml", "/x12/transactions"}, paths)
}

func TestGatewayHealthAndPreflightBypassAPIKey(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/health", r.URL.Path)
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	})}
	handler := gatewayHandler(gateway{payerURL: "http://payer-core", client: client, apiKeys: []string{"society-secret"}})

	healthResponse := httptest.NewRecorder()
	handler.ServeHTTP(healthResponse, httptest.NewRequest(http.MethodGet, "/v1/health", nil))

	optionsResponse := httptest.NewRecorder()
	handler.ServeHTTP(optionsResponse, httptest.NewRequest(http.MethodOptions, "/v1/x12/xml", nil))

	assert.Equal(t, http.StatusOK, healthResponse.Code)
	assert.Equal(t, http.StatusNoContent, optionsResponse.Code)
}

func TestGatewayRateLimitBlocksAfterConfiguredLimit(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusOK, domain.Envelope{Lore: "Ledger returned."})
	})}
	clock := fixedClock(time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	handler := gatewayHandler(gateway{
		payerURL:    "http://payer-core",
		client:      client,
		rateLimiter: newRateLimiter(2, time.Minute, clock),
	})

	for requestNumber := 1; requestNumber <= 2; requestNumber++ {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/v1/transactions", nil)
		request.RemoteAddr = "192.0.2.10:12000"
		handler.ServeHTTP(response, request)

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "2", response.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, strconv.Itoa(2-requestNumber), response.Header().Get("X-RateLimit-Remaining"))
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/transactions", nil)
	request.RemoteAddr = "192.0.2.10:12000"
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusTooManyRequests, response.Code)
	assert.Equal(t, 2, calls)
	assert.Equal(t, "rate limit exceeded", decodeGatewayEnvelope(t, response).Error)
	assert.Equal(t, "60", response.Header().Get("Retry-After"))
}

func TestGatewayRateLimitSeparatesAPIKeyBuckets(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusCreated, domain.Envelope{Lore: "Intake accepted."})
	})}
	handler := gatewayHandler(gateway{
		ediURL:      "http://edi-intake",
		client:      client,
		apiKeys:     []string{"alpha", "beta"},
		rateLimiter: newRateLimiter(1, time.Minute, fixedClock(time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))),
	})

	alphaOne := httptest.NewRecorder()
	alphaRequest := httptest.NewRequest(http.MethodPost, "/v1/x12/xml", strings.NewReader(`<AshnX12Transaction type="837" />`))
	alphaRequest.Header.Set("Authorization", "Bearer alpha")
	handler.ServeHTTP(alphaOne, alphaRequest)

	alphaTwo := httptest.NewRecorder()
	alphaRetry := httptest.NewRequest(http.MethodPost, "/v1/x12/xml", strings.NewReader(`<AshnX12Transaction type="837" />`))
	alphaRetry.Header.Set("Authorization", "Bearer alpha")
	handler.ServeHTTP(alphaTwo, alphaRetry)

	betaOne := httptest.NewRecorder()
	betaRequest := httptest.NewRequest(http.MethodPost, "/v1/x12/xml", strings.NewReader(`<AshnX12Transaction type="837" />`))
	betaRequest.Header.Set("X-ASHN-API-Key", "beta")
	handler.ServeHTTP(betaOne, betaRequest)

	assert.Equal(t, http.StatusCreated, alphaOne.Code)
	assert.Equal(t, http.StatusTooManyRequests, alphaTwo.Code)
	assert.Equal(t, http.StatusCreated, betaOne.Code)
	assert.Equal(t, 2, calls)
}

func TestGatewayRateLimitExemptsHealthAndPreflight(t *testing.T) {
	handler := gatewayHandler(gateway{
		payerURL:    "",
		client:      http.DefaultClient,
		apiKeys:     []string{"society-secret"},
		rateLimiter: newRateLimiter(1, time.Minute, fixedClock(time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))),
	})

	for requestNumber := 0; requestNumber < 2; requestNumber++ {
		healthResponse := httptest.NewRecorder()
		handler.ServeHTTP(healthResponse, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
		assert.Equal(t, http.StatusOK, healthResponse.Code)
		assert.Empty(t, healthResponse.Header().Get("X-RateLimit-Limit"))

		optionsResponse := httptest.NewRecorder()
		handler.ServeHTTP(optionsResponse, httptest.NewRequest(http.MethodOptions, "/v1/x12/xml", nil))
		assert.Equal(t, http.StatusNoContent, optionsResponse.Code)
		assert.Empty(t, optionsResponse.Header().Get("X-RateLimit-Limit"))
	}
}

func TestGatewayRateLimitConfigurationFromEnv(t *testing.T) {
	t.Setenv("ASHN_RATE_LIMIT_REQUESTS", "7")
	t.Setenv("ASHN_RATE_LIMIT_WINDOW", "2m")
	limiter := rateLimiterFromEnv()

	require.NotNil(t, limiter)
	assert.Equal(t, 7, limiter.limit)
	assert.Equal(t, 2*time.Minute, limiter.window)

	t.Setenv("ASHN_RATE_LIMIT_REQUESTS", "0")
	assert.Nil(t, rateLimiterFromEnv())
}

func TestGatewayAPIKeyParsingAndBearerToken(t *testing.T) {
	assert.Equal(t, []string{"alpha", "beta"}, parseAPIKeys(" alpha, ,beta "))
	assert.Equal(t, "secret", bearerToken("Bearer secret"))
	assert.Equal(t, "secret", bearerToken("bearer secret"))
	assert.Empty(t, bearerToken("Basic secret"))
	assert.Empty(t, bearerToken("Bearer"))
}

func TestGatewayRoutesXMLAuditMessagesToEDIIntake(t *testing.T) {
	var downstreamURI string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamURI = r.URL.RequestURI()
		assert.Equal(t, http.MethodGet, r.Method)
		if r.URL.Path == "/x12/messages/rejections" {
			return jsonResponse(http.StatusOK, domain.Envelope{Data: domain.InboundRejectionMetrics{Total: 2}, Lore: "Rejections returned."})
		}
		return jsonResponse(http.StatusOK, domain.Envelope{Data: []domain.InboundMessage{}, Lore: "Messages returned."})
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", ediURL: "http://edi-intake", client: client})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/x12/messages?limit=10&offset=20&status=accepted&type=834&q=farros", nil)
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "/x12/messages?limit=10&offset=20&status=accepted&type=834&q=farros", downstreamURI)
	assert.Equal(t, "Messages returned.", decodeGatewayEnvelope(t, response).Lore)

	rejectionResponse := httptest.NewRecorder()
	rejectionRequest := httptest.NewRequest(http.MethodGet, "/v1/x12/messages/rejections?type=837&q=diagnosis", nil)
	handler.ServeHTTP(rejectionResponse, rejectionRequest)

	assert.Equal(t, http.StatusOK, rejectionResponse.Code)
	assert.Equal(t, "/x12/messages/rejections?type=837&q=diagnosis", downstreamURI)
	assert.Equal(t, "Rejections returned.", decodeGatewayEnvelope(t, rejectionResponse).Lore)
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
		{http.MethodPost, "/v1/x12/batch"},
		{http.MethodGet, "/v1/x12/messages/msg-1/export?format=json"},
		{http.MethodPost, "/v1/x12/messages/msg-1/replay"},
		{http.MethodGet, "/v1/x12/trading-partners"},
		{http.MethodPost, "/v1/x12/trading-partners"},
		{http.MethodPut, "/v1/x12/trading-partners/tp-1"},
		{http.MethodDelete, "/v1/x12/trading-partners/tp-1"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(item.method, item.path, nil))
		assert.Equal(t, http.StatusOK, response.Code)
	}

	assert.Equal(t, []string{
		"POST /x12/batch",
		"GET /x12/messages/msg-1/export?format=json",
		"POST /x12/messages/msg-1/replay",
		"GET /x12/trading-partners",
		"POST /x12/trading-partners",
		"PUT /x12/trading-partners/tp-1",
		"DELETE /x12/trading-partners/tp-1",
	}, paths)
}

func TestGatewayMuxRegistersTradingPartnerMutationMethods(t *testing.T) {
	paths := []string{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		return jsonResponse(http.StatusOK, domain.Envelope{Lore: "edi route"})
	})}
	handler := gatewayMainHandler(gateway{ediURL: "http://edi-intake", client: client})

	for _, item := range []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/v1/x12/trading-partners/tp-1"},
		{http.MethodDelete, "/v1/x12/trading-partners/tp-1"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(item.method, item.path, nil))
		assert.Equal(t, http.StatusOK, response.Code)
	}

	assert.Equal(t, []string{
		"PUT /x12/trading-partners/tp-1",
		"DELETE /x12/trading-partners/tp-1",
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

func TestGatewaySystemReadinessAggregatesOperationalSignals(t *testing.T) {
	t.Setenv("RENDER_GIT_COMMIT", "abc123")
	requestIDs := []string{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestIDs = append(requestIDs, r.Header.Get("X-Request-ID"))
		switch r.URL.Path {
		case "/health":
			return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
		case "/transactions":
			return jsonResponse(http.StatusOK, domain.Envelope{Data: []map[string]string{{"id": "tx-1"}}, Page: &domain.PageInfo{Limit: 1, Count: 1}})
		case "/jobs":
			return jsonResponse(http.StatusOK, domain.Envelope{Data: []map[string]string{{"id": "job-1"}}})
		case "/providers":
			return jsonResponse(http.StatusOK, domain.Envelope{Data: []map[string]string{{"id": "provider-1"}}})
		case "/x12/messages":
			return jsonResponse(http.StatusOK, domain.Envelope{Data: []map[string]string{{"id": "msg-1"}}, Page: &domain.PageInfo{Limit: 1, Count: 1}})
		case "/x12/messages/rejections":
			return jsonResponse(http.StatusOK, domain.Envelope{Data: map[string]int{"total": 2}})
		default:
			return jsonResponse(http.StatusNotFound, domain.ErrorEnvelope{Error: "missing"})
		}
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", ediURL: "http://edi-intake", client: client})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/system/readiness", nil)
	request.Header.Set("X-Request-ID", "req-readiness")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeGatewayEnvelope(t, response)
	var readiness map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &readiness))
	assert.Equal(t, "ready", readiness["status"])
	assert.Equal(t, "abc123", readiness["commit"])
	assert.NotEmpty(t, readiness["generatedAt"])
	services := readiness["services"].(map[string]any)
	assert.Equal(t, "ok", services["payer-core"])
	assert.Equal(t, "ok", services["provider-service"])
	assert.Equal(t, "ok", services["edi-intake"])
	checks := readiness["checks"].([]any)
	assert.Len(t, checks, 5)
	assert.Contains(t, requestIDs, "req-readiness")
	assert.NotEmpty(t, envelope.Lore)
}

func TestGatewaySystemReadinessReportsDegradedSignals(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/health" {
			return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
		}
		if r.URL.Path == "/jobs" {
			return jsonResponse(http.StatusBadRequest, domain.ErrorEnvelope{Error: "bad filter"})
		}
		if r.URL.Path == "/x12/messages/rejections" {
			return jsonResponse(http.StatusInternalServerError, domain.ErrorEnvelope{Error: "down"})
		}
		return jsonResponse(http.StatusOK, domain.Envelope{Data: []map[string]string{}})
	})}

	response := httptest.NewRecorder()
	gateway{payerURL: "http://payer-core", providerURL: "http://provider-service", ediURL: "http://edi-intake", client: client}.systemReadiness(response, httptest.NewRequest(http.MethodGet, "/v1/system/readiness", nil))

	assert.Equal(t, http.StatusOK, response.Code)
	var envelope testEnvelope
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	var readiness map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &readiness))
	assert.Equal(t, "degraded", readiness["status"])
	summary := readiness["summary"].(map[string]any)
	assert.Equal(t, float64(1), summary["degraded"])
	assert.Equal(t, float64(1), summary["unavailable"])
}

func TestGatewayMetricsSummaryAggregatesOperationalData(t *testing.T) {
	paths := []string{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.RequestURI())
		switch r.URL.Path {
		case "/transactions":
			return jsonResponse(http.StatusOK, domain.Envelope{Data: []domain.Transaction{
				{ID: "tx-837", Type: domain.Tx837, Status: domain.TxStatusAccepted},
				{ID: "tx-835", Type: domain.Tx835, Status: domain.TxStatusPaid},
				{ID: "tx-275", Type: domain.Tx275, Status: domain.TxStatusAccepted},
			}})
		case "/claims":
			return jsonResponse(http.StatusOK, domain.Envelope{Data: []domain.Claim{
				{ID: "claim-1", ProviderID: "provider-vitesse-temple", Status: domain.ClaimPaid, AmountCents: 10000, AllowedAmountCents: 8000, PaidAmountCents: 7200, PatientResponsibilityCents: 800, AdjustmentAmountCents: 2000},
				{ID: "claim-2", ProviderID: "provider-greenstone-roadside", Status: domain.ClaimDenied, AmountCents: 5000, AdjustmentAmountCents: 5000},
			}})
		case "/jobs":
			return jsonResponse(http.StatusOK, domain.Envelope{Data: []metricsJob{
				{Status: "completed"},
				{Status: "failed", DeadLetter: true},
			}})
		case "/x12/messages/rejections":
			return jsonResponse(http.StatusOK, domain.Envelope{Data: metricsRejectionData{
				Total:     3,
				ByPartner: []metricsCount{{Label: "tp-vitesse-temple", Count: 2}},
				ByType:    []metricsCount{{Label: "837", Type: "837", Count: 3}},
				ByReason:  []metricsCount{{Label: "diagnosis not allowed", Count: 3}},
			}})
		default:
			return jsonResponse(http.StatusNotFound, domain.ErrorEnvelope{Error: "missing"})
		}
	})}

	handler := gatewayHandler(gateway{payerURL: "http://payer-core", ediURL: "http://edi-intake", client: client})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/metrics/summary", nil))

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeGatewayEnvelope(t, response)
	var summary map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &summary))
	assert.Equal(t, "attention", summary["operationalStatus"])
	transactions := summary["transactions"].(map[string]any)
	assert.Equal(t, float64(3), transactions["totalLoaded"])
	transactionTypes := transactions["byType"].(map[string]any)
	assert.Equal(t, float64(1), transactionTypes["837"])
	claims := summary["claims"].(map[string]any)
	claimStatuses := claims["byStatus"].(map[string]any)
	assert.Equal(t, float64(1), claimStatuses["Paid"])
	financials := summary["financials"].(map[string]any)
	assert.Equal(t, float64(15000), financials["billedCents"])
	assert.Equal(t, float64(7200), financials["paidCents"])
	intake := summary["intake"].(map[string]any)
	assert.Equal(t, float64(3), intake["rejectionTotal"])
	asyncJobs := summary["asyncJobs"].(map[string]any)
	assert.Equal(t, float64(1), asyncJobs["deadLetters"])
	assert.ElementsMatch(t, []string{"/transactions?limit=100", "/claims?limit=100", "/jobs?limit=100", "/x12/messages/rejections"}, paths)
	assert.NotEmpty(t, envelope.Lore)
}

func TestGatewayMetricsSummarySurvivesPartialDownstreamFailure(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/transactions" {
			return jsonResponse(http.StatusInternalServerError, domain.ErrorEnvelope{Error: "down"})
		}
		return jsonResponse(http.StatusOK, domain.Envelope{Data: []domain.Claim{}})
	})}

	response := httptest.NewRecorder()
	gateway{payerURL: "http://payer-core", ediURL: "http://edi-intake", client: client}.metricsSummary(response, httptest.NewRequest(http.MethodGet, "/v1/metrics/summary", nil))

	assert.Equal(t, http.StatusOK, response.Code)
	var envelope testEnvelope
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	var summary map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &summary))
	assert.Equal(t, "attention", summary["operationalStatus"])
	highlights := summary["highlights"].([]any)
	assert.Contains(t, highlights, "Ledger metrics unavailable")
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
	assert.Contains(t, paths, "/v1/system/readiness")
	assert.Contains(t, paths, "/v1/metrics/summary")
	assert.Contains(t, paths, "/v1/premium-payments")
	assert.Contains(t, paths, "/v1/benefit-coordination")
	assert.Contains(t, paths, "/v1/x12/xml")
	assert.Contains(t, paths, "/v1/x12/raw")
	assert.Contains(t, paths, "/v1/x12/batch")
	assert.Contains(t, paths, "/v1/x12/messages/rejections")
	assert.Contains(t, paths, "/v1/transactions/{id}/export")
	assert.Contains(t, paths, "/v1/transactions/{id}/document-reference")
	components := spec["components"].(map[string]any)
	securitySchemes := components["securitySchemes"].(map[string]any)
	assert.Contains(t, securitySchemes, "bearerAuth")
	assert.Contains(t, securitySchemes, "apiKeyAuth")
}

func gatewayHandler(g gateway) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/", g.route)
	return cors(mux)
}

func gatewayMainHandler(g gateway) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/", g.route)
	mux.HandleFunc("POST /v1/", g.route)
	mux.HandleFunc("PUT /v1/", g.route)
	mux.HandleFunc("DELETE /v1/", g.route)
	mux.HandleFunc("OPTIONS /v1/", g.route)
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

func fixedClock(now time.Time) func() time.Time {
	return func() time.Time {
		return now
	}
}
