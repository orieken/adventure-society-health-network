package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"ashn/packages/domain"
	"ashn/packages/openapidocs"
)

type gateway struct {
	payerURL               string
	providerURL            string
	ediURL                 string
	client                 *http.Client
	coldStartRetryAttempts int
	coldStartRetryDelay    time.Duration
}

func main() {
	g := gateway{
		payerURL:    env("PAYER_CORE_URL", "http://localhost:8081"),
		providerURL: env("PROVIDER_SERVICE_URL", "http://localhost:8082"),
		ediURL:      env("EDI_INTAKE_URL", "http://localhost:8083"),
		client:      &http.Client{Timeout: 35 * time.Second},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", openapidocs.HTMLHandler("ASHN API Gateway Docs"))
	mux.HandleFunc("GET /openapi.json", openapidocs.JSONHandler(apiGatewayOpenAPI()))
	mux.HandleFunc("GET /v1/", g.route)
	mux.HandleFunc("POST /v1/", g.route)
	mux.HandleFunc("OPTIONS /v1/", g.route)
	addr := env("API_GATEWAY_ADDR", ":8080")
	log.Printf("[ASHN] api-gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, cors(logRequests(mux))))
}

func (g gateway) route(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1")
	switch {
	case path == "/health" && r.Method == http.MethodGet:
		g.health(w)
	case path == "/adventurers" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, "/enrollments")
	case path == "/adventurers" && r.Method == http.MethodGet:
		g.proxy(w, r, g.payerURL, path)
	case strings.HasPrefix(path, "/adventurers/") && r.Method == http.MethodGet:
		g.proxy(w, r, g.payerURL, path)
	case path == "/eligibility" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, "/eligibility/query")
	case path == "/auth-requests" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, path)
	case path == "/claims" && r.Method == http.MethodGet:
		g.proxy(w, r, g.payerURL, path)
	case path == "/claims" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, path)
	case strings.HasPrefix(path, "/claims/"):
		g.proxy(w, r, g.payerURL, path)
	case path == "/transactions" && r.Method == http.MethodGet:
		g.proxy(w, r, g.payerURL, path)
	case strings.HasPrefix(path, "/transactions/") && r.Method == http.MethodGet:
		g.proxy(w, r, g.payerURL, path)
	case strings.HasPrefix(path, "/transactions/") && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, path)
	case path == "/x12/xml" && r.Method == http.MethodPost:
		g.proxy(w, r, g.ediURL, path)
	case path == "/x12/messages" && r.Method == http.MethodGet:
		g.proxy(w, r, g.ediURL, path)
	case strings.HasPrefix(path, "/x12/messages/") && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		g.proxy(w, r, g.ediURL, path)
	case path == "/x12/trading-partners" && r.Method == http.MethodGet:
		g.proxy(w, r, g.ediURL, path)
	case strings.HasPrefix(path, "/providers"):
		g.proxy(w, r, g.providerURL, path)
	default:
		fail(w, http.StatusNotFound, "route not found", "No known Society road leads to that endpoint.")
	}
}

func (g gateway) proxy(w http.ResponseWriter, r *http.Request, baseURL, path string) {
	targetURL := baseURL + path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}
	resp, err := g.doProxyRequest(r, targetURL)
	if err != nil {
		if strings.Contains(err.Error(), "request creation failed") {
			fail(w, http.StatusInternalServerError, "request creation failed", "The gateway scribe could not bind the courier spell.")
			return
		}
		fail(w, http.StatusBadGateway, "downstream unavailable", "The gateway courier vanished somewhere between towers.")
		return
	}
	defer resp.Body.Close()
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (g gateway) health(w http.ResponseWriter) {
	status := map[string]string{"api-gateway": "ok"}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for service, url := range map[string]string{"payer-core": g.payerURL, "provider-service": g.providerURL, "edi-intake": g.ediURL} {
		if url == "" {
			continue
		}
		wg.Add(1)
		go func(serviceName, serviceURL string) {
			defer wg.Done()
			serviceStatus := g.downstreamHealth(serviceURL)
			mu.Lock()
			status[serviceName] = serviceStatus
			mu.Unlock()
		}(service, url)
	}
	wg.Wait()
	respond(w, http.StatusOK, domain.Envelope{Data: status, Lore: "The gateway crystal checked every downstream beacon."})
}

func (g gateway) doProxyRequest(r *http.Request, targetURL string) (*http.Response, error) {
	attempts, delay := g.coldStartRetryPolicy()
	if r.Method != http.MethodGet {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequest(r.Method, targetURL, r.Body)
		if err != nil {
			return nil, errRequestCreation
		}
		req.Header = r.Header.Clone()
		resp, err := g.httpClient().Do(req)
		if err != nil {
			return nil, err
		}
		if r.Method == http.MethodGet && retryableColdStartStatus(resp.StatusCode) && attempt < attempts {
			_ = resp.Body.Close()
			g.sleep(delay)
			continue
		}
		return resp, nil
	}
	return nil, http.ErrAbortHandler
}

func (g gateway) downstreamHealth(baseURL string) string {
	attempts, delay := g.coldStartRetryPolicy()
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := g.httpClient().Get(baseURL + "/health")
		if err != nil {
			return "unavailable"
		}
		statusCode := resp.StatusCode
		_ = resp.Body.Close()
		if statusCode < http.StatusInternalServerError {
			return "ok"
		}
		if !retryableColdStartStatus(statusCode) {
			return "unavailable"
		}
		if attempt < attempts {
			g.sleep(delay)
		}
	}
	return "unavailable"
}

func (g gateway) coldStartRetryPolicy() (int, time.Duration) {
	if g.coldStartRetryAttempts > 0 {
		return g.coldStartRetryAttempts, g.coldStartRetryDelay
	}
	return 6, 5 * time.Second
}

func (g gateway) sleep(delay time.Duration) {
	if delay > 0 {
		time.Sleep(delay)
	}
}

func retryableColdStartStatus(status int) bool {
	return status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func (g gateway) httpClient() *http.Client {
	if g.client != nil {
		return g.client
	}
	return http.DefaultClient
}

func apiGatewayOpenAPI() map[string]any {
	return openapidocs.Spec(openapidocs.Service{
		Title:       "ASHN API Gateway",
		Description: "Public facade for the Adventure Society Health Network demo APIs.",
		Version:     "0.1.0",
		Paths: map[string]map[string]openapidocs.Operation{
			"/v1/health": {"get": {Summary: "Check service health", Tags: []string{"gateway"}}},
			"/v1/adventurers": {
				"get":  {Summary: "List adventurers", Tags: []string{"adventurers"}},
				"post": {Summary: "Create an enrollment", Tags: []string{"adventurers", "x12"}, RequestBody: true},
			},
			"/v1/adventurers/{id}": {"get": {Summary: "Get an adventurer", Tags: []string{"adventurers"}}},
			"/v1/eligibility":      {"post": {Summary: "Run 270/271 eligibility", Tags: []string{"eligibility", "x12"}, RequestBody: true}},
			"/v1/auth-requests":    {"post": {Summary: "Submit 278 authorization", Tags: []string{"authorizations", "x12"}, RequestBody: true}},
			"/v1/claims": {
				"get":  {Summary: "List claims", Tags: []string{"claims"}},
				"post": {Summary: "Submit 837 claim", Tags: []string{"claims", "x12"}, RequestBody: true},
			},
			"/v1/claims/{id}":                       {"get": {Summary: "Get claim detail", Tags: []string{"claims"}}},
			"/v1/claims/{id}/status":                {"get": {Summary: "Get claim status", Tags: []string{"claims"}}},
			"/v1/claims/{id}/attachments":           {"post": {Summary: "Submit 275 patient information attachment", Tags: []string{"claims", "attachments", "x12"}, RequestBody: true}},
			"/v1/claims/{id}/payment":               {"post": {Summary: "Create 835 payment", Tags: []string{"claims", "x12"}, RequestBody: true}},
			"/v1/transactions":                      {"get": {Summary: "List ledger transactions", Tags: []string{"transactions"}}},
			"/v1/transactions/{id}":                 {"get": {Summary: "Get transaction detail", Tags: []string{"transactions"}}},
			"/v1/transactions/{id}/export":          {"get": {Summary: "Export transaction as JSON, XML, or X12", Tags: []string{"transactions", "export"}}},
			"/v1/transactions/{id}/replay":          {"post": {Summary: "Replay transaction", Tags: []string{"transactions", "replay"}}},
			"/v1/x12/xml":                           {"post": {Summary: "Accept XML intake", Tags: []string{"xml", "x12"}, RequestBody: true}},
			"/v1/x12/messages":                      {"get": {Summary: "List XML intake audits", Tags: []string{"xml"}}},
			"/v1/x12/messages/{id}/export":          {"get": {Summary: "Export XML intake audit", Tags: []string{"xml", "export"}}},
			"/v1/x12/messages/{id}/replay":          {"post": {Summary: "Replay XML intake", Tags: []string{"xml", "replay"}}},
			"/v1/x12/trading-partners":              {"get": {Summary: "List trading partners", Tags: []string{"trading partners", "x12"}}},
			"/v1/providers":                         {"get": {Summary: "List providers", Tags: []string{"providers"}}},
			"/v1/providers/{id}":                    {"get": {Summary: "Get provider detail", Tags: []string{"providers"}}},
			"/v1/providers/{id}/submit-claim":       {"post": {Summary: "Submit claim through provider workflow", Tags: []string{"providers", "claims"}, RequestBody: true}},
			"/v1/providers/{id}/verify-eligibility": {"post": {Summary: "Verify eligibility through provider workflow", Tags: []string{"providers", "eligibility"}, RequestBody: true}},
		},
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		next.ServeHTTP(w, r)
	})
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func fail(w http.ResponseWriter, status int, message, loreText string) {
	respond(w, status, domain.ErrorEnvelope{Error: message, Lore: loreText})
}

var errRequestCreation = &gatewayError{message: "request creation failed"}

type gatewayError struct {
	message string
}

func (e *gatewayError) Error() string {
	return e.message
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[ASHN] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
