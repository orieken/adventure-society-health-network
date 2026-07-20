package main

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"ashn/packages/ashnlog"
	"ashn/packages/domain"
	"ashn/packages/openapidocs"
	"ashn/packages/requestmeta"
)

type gateway struct {
	payerURL               string
	providerURL            string
	ediURL                 string
	client                 *http.Client
	coldStartRetryAttempts int
	coldStartRetryDelay    time.Duration
	apiKeys                []string
	rateLimiter            *rateLimiter
}

func main() {
	g := gateway{
		payerURL:    env("PAYER_CORE_URL", "http://localhost:8081"),
		providerURL: env("PROVIDER_SERVICE_URL", "http://localhost:8082"),
		ediURL:      env("EDI_INTAKE_URL", "http://localhost:8083"),
		client:      &http.Client{Timeout: 35 * time.Second},
		apiKeys:     parseAPIKeys(env("ASHN_API_KEYS", "")),
		rateLimiter: rateLimiterFromEnv(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", openapidocs.HTMLHandler("ASHN API Gateway Docs"))
	mux.HandleFunc("GET /openapi.json", openapidocs.JSONHandler(apiGatewayOpenAPI()))
	mux.HandleFunc("GET /v1/", g.route)
	mux.HandleFunc("POST /v1/", g.route)
	mux.HandleFunc("PUT /v1/", g.route)
	mux.HandleFunc("DELETE /v1/", g.route)
	mux.HandleFunc("OPTIONS /v1/", g.route)
	addr := env("API_GATEWAY_ADDR", ":8080")
	ashnlog.Info("service_listening", "service", "api-gateway", "addr", addr)
	ashnlog.Fatal("service_stopped", http.ListenAndServe(addr, requestmeta.Middleware("api-gateway", cors(logRequests(mux)))), "service", "api-gateway")
}

func (g gateway) route(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1")
	if g.requiresAPIKey(path, r.Method) && !g.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="ashn-api"`)
		fail(w, http.StatusUnauthorized, "unauthorized", "The gateway ward rejected the courier seal.")
		return
	}
	if !g.allowRequest(w, r, path) {
		return
	}
	switch {
	case path == "/health" && r.Method == http.MethodGet:
		g.health(w)
	case path == "/system/readiness" && r.Method == http.MethodGet:
		g.systemReadiness(w, r)
	case path == "/metrics/summary" && r.Method == http.MethodGet:
		g.metricsSummary(w, r)
	case path == "/adventurers" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, "/enrollments")
	case path == "/adventurers" && r.Method == http.MethodGet:
		g.proxy(w, r, g.payerURL, path)
	case strings.HasPrefix(path, "/adventurers/") && r.Method == http.MethodGet:
		g.proxy(w, r, g.payerURL, path)
	case path == "/premium-payments" && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		g.proxy(w, r, g.payerURL, path)
	case path == "/eligibility" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, "/eligibility/query")
	case path == "/benefit-coordination" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, path)
	case path == "/auth-requests" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, path)
	case strings.HasPrefix(path, "/auth-requests/") && r.Method == http.MethodPost:
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
	case path == "/jobs" && r.Method == http.MethodGet:
		g.proxy(w, r, g.payerURL, path)
	case strings.HasPrefix(path, "/jobs/") && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, path)
	case path == "/x12/transactions" && r.Method == http.MethodPost:
		g.proxy(w, r, g.ediURL, path)
	case path == "/x12/xml" && r.Method == http.MethodPost:
		g.proxy(w, r, g.ediURL, path)
	case path == "/x12/raw" && r.Method == http.MethodPost:
		g.proxy(w, r, g.ediURL, path)
	case path == "/x12/batch" && r.Method == http.MethodPost:
		g.proxy(w, r, g.ediURL, path)
	case path == "/x12/messages" && r.Method == http.MethodGet:
		g.proxy(w, r, g.ediURL, path)
	case strings.HasPrefix(path, "/x12/messages/") && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		g.proxy(w, r, g.ediURL, path)
	case path == "/x12/trading-partners" && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		g.proxy(w, r, g.ediURL, path)
	case strings.HasPrefix(path, "/x12/trading-partners/") && (r.Method == http.MethodPut || r.Method == http.MethodDelete):
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

type readinessReport struct {
	Status      string            `json:"status"`
	GeneratedAt string            `json:"generatedAt"`
	Version     string            `json:"version"`
	Commit      string            `json:"commit,omitempty"`
	Services    map[string]string `json:"services"`
	Checks      []readinessCheck  `json:"checks"`
	Summary     map[string]int    `json:"summary"`
	Links       map[string]string `json:"links"`
}

type readinessCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Count  int    `json:"count,omitempty"`
}

type readinessEnvelope struct {
	Data json.RawMessage  `json:"data,omitempty"`
	Page *domain.PageInfo `json:"page,omitempty"`
}

type readinessMetricsEnvelope struct {
	Data struct {
		Total int `json:"total"`
	} `json:"data"`
}

func (g gateway) systemReadiness(w http.ResponseWriter, r *http.Request) {
	services := map[string]string{
		"api-gateway":      "ok",
		"payer-core":       g.downstreamHealth(g.payerURL),
		"provider-service": g.downstreamHealth(g.providerURL),
		"edi-intake":       g.downstreamHealth(g.ediURL),
	}
	checks := []readinessCheck{
		g.readinessCollectionCheck(r, "ledger transactions", g.payerURL, "/transactions?limit=1"),
		g.readinessCollectionCheck(r, "async jobs", g.payerURL, "/jobs?limit=8"),
		g.readinessCollectionCheck(r, "provider registry", g.providerURL, "/providers"),
		g.readinessCollectionCheck(r, "intake audit", g.ediURL, "/x12/messages?limit=1"),
		g.readinessRejectionCheck(r),
	}
	summary := map[string]int{"ok": 0, "degraded": 0, "unavailable": 0}
	ready := true
	for _, status := range services {
		if status != "ok" {
			ready = false
		}
	}
	for _, check := range checks {
		summary[check.Status]++
		if check.Status != "ok" {
			ready = false
		}
	}
	status := "ready"
	if !ready {
		status = "degraded"
	}
	respond(w, http.StatusOK, domain.Envelope{
		Data: readinessReport{
			Status:      status,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Version:     "0.1.0",
			Commit:      buildCommit(),
			Services:    services,
			Checks:      checks,
			Summary:     summary,
			Links: map[string]string{
				"openapi": "/openapi.json",
				"health":  "/v1/health",
			},
		},
		Lore: "The Society readiness board gathered health, ledger, queue, partner, and intake signals.",
	})
}

func (g gateway) readinessCollectionCheck(r *http.Request, name, baseURL, path string) readinessCheck {
	check := g.readinessGET(r, name, baseURL, path)
	if check.Status != "ok" {
		return check
	}
	var envelope readinessEnvelope
	if err := json.Unmarshal([]byte(check.Detail), &envelope); err != nil {
		check.Status = "degraded"
		check.Detail = "response shape changed"
		return check
	}
	check.Detail = "reachable"
	if envelope.Page != nil {
		check.Count = envelope.Page.Count
		return check
	}
	var items []any
	if err := json.Unmarshal(envelope.Data, &items); err == nil {
		check.Count = len(items)
	}
	return check
}

func (g gateway) readinessRejectionCheck(r *http.Request) readinessCheck {
	check := g.readinessGET(r, "intake rejections", g.ediURL, "/x12/messages/rejections")
	if check.Status != "ok" {
		return check
	}
	var envelope readinessMetricsEnvelope
	if err := json.Unmarshal([]byte(check.Detail), &envelope); err != nil {
		check.Status = "degraded"
		check.Detail = "response shape changed"
		return check
	}
	check.Detail = "reachable"
	check.Count = envelope.Data.Total
	return check
}

func (g gateway) readinessGET(r *http.Request, name, baseURL, path string) readinessCheck {
	if strings.TrimSpace(baseURL) == "" {
		return readinessCheck{Name: name, Status: "unavailable", Detail: "service URL is not configured"}
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return readinessCheck{Name: name, Status: "unavailable", Detail: "request creation failed"}
	}
	req.Header = r.Header.Clone()
	requestmeta.Propagate(r, req)
	resp, err := g.httpClient().Do(req)
	if err != nil {
		return readinessCheck{Name: name, Status: "unavailable", Detail: "request failed"}
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= http.StatusInternalServerError {
		return readinessCheck{Name: name, Status: "unavailable", Detail: strconv.Itoa(resp.StatusCode)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return readinessCheck{Name: name, Status: "degraded", Detail: strconv.Itoa(resp.StatusCode)}
	}
	return readinessCheck{Name: name, Status: "ok", Detail: string(payload)}
}

func buildCommit() string {
	for _, key := range []string{"RENDER_GIT_COMMIT", "GIT_COMMIT", "SOURCE_VERSION", "COMMIT_SHA"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

type metricsSummary struct {
	GeneratedAt       string                    `json:"generatedAt"`
	Window            string                    `json:"window"`
	Transactions      metricsTransactionSummary `json:"transactions"`
	Claims            metricsClaimSummary       `json:"claims"`
	Intake            metricsIntakeSummary      `json:"intake"`
	AsyncJobs         metricsAsyncJobSummary    `json:"asyncJobs"`
	Financials        metricsFinancialSummary   `json:"financials"`
	OperationalStatus string                    `json:"operationalStatus"`
	Highlights        []string                  `json:"highlights"`
}

type metricsTransactionSummary struct {
	TotalLoaded int            `json:"totalLoaded"`
	ByType      map[string]int `json:"byType"`
	ByStatus    map[string]int `json:"byStatus"`
}

type metricsClaimSummary struct {
	TotalLoaded int            `json:"totalLoaded"`
	ByStatus    map[string]int `json:"byStatus"`
	ByProvider  map[string]int `json:"byProvider"`
}

type metricsIntakeSummary struct {
	RejectionTotal int            `json:"rejectionTotal"`
	ByPartner      map[string]int `json:"byPartner"`
	ByType         map[string]int `json:"byType"`
	ByReason       map[string]int `json:"byReason"`
}

type metricsAsyncJobSummary struct {
	TotalLoaded int            `json:"totalLoaded"`
	ByStatus    map[string]int `json:"byStatus"`
	DeadLetters int            `json:"deadLetters"`
}

type metricsFinancialSummary struct {
	BilledCents                int64 `json:"billedCents"`
	AllowedCents               int64 `json:"allowedCents"`
	PaidCents                  int64 `json:"paidCents"`
	PatientResponsibilityCents int64 `json:"patientResponsibilityCents"`
	AdjustmentCents            int64 `json:"adjustmentCents"`
}

type metricsCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
	Type  string `json:"type,omitempty"`
}

type metricsRejectionData struct {
	Total     int            `json:"total"`
	ByPartner []metricsCount `json:"byPartner"`
	ByType    []metricsCount `json:"byType"`
	ByReason  []metricsCount `json:"byReason"`
}

type metricsJob struct {
	Status     string `json:"status"`
	DeadLetter bool   `json:"deadLetter"`
}

type metricsEnvelope struct {
	Data json.RawMessage `json:"data,omitempty"`
}

func (g gateway) metricsSummary(w http.ResponseWriter, r *http.Request) {
	transactions := []domain.Transaction{}
	claims := []domain.Claim{}
	jobs := []metricsJob{}
	rejections := metricsRejectionData{}
	highlights := []string{}

	if err := g.fetchMetricsData(r, g.payerURL, "/transactions?limit=100", &transactions); err != nil {
		highlights = append(highlights, "Ledger metrics unavailable")
	}
	if err := g.fetchMetricsData(r, g.payerURL, "/claims?limit=100", &claims); err != nil {
		highlights = append(highlights, "Claim metrics unavailable")
	}
	if err := g.fetchMetricsData(r, g.payerURL, "/jobs?limit=100", &jobs); err != nil {
		highlights = append(highlights, "Async job metrics unavailable")
	}
	if err := g.fetchMetricsData(r, g.ediURL, "/x12/messages/rejections", &rejections); err != nil {
		highlights = append(highlights, "Intake rejection metrics unavailable")
	}

	summary := metricsSummary{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Window:      "latest 100 records per source",
		Transactions: metricsTransactionSummary{
			TotalLoaded: len(transactions),
			ByType:      map[string]int{},
			ByStatus:    map[string]int{},
		},
		Claims: metricsClaimSummary{
			TotalLoaded: len(claims),
			ByStatus:    map[string]int{},
			ByProvider:  map[string]int{},
		},
		Intake: metricsIntakeSummary{
			RejectionTotal: rejections.Total,
			ByPartner:      countsToMap(rejections.ByPartner),
			ByType:         countsToMap(rejections.ByType),
			ByReason:       countsToMap(rejections.ByReason),
		},
		AsyncJobs: metricsAsyncJobSummary{
			TotalLoaded: len(jobs),
			ByStatus:    map[string]int{},
		},
		Financials:        metricsFinancialSummary{},
		OperationalStatus: "healthy",
		Highlights:        highlights,
	}

	for _, tx := range transactions {
		summary.Transactions.ByType[string(tx.Type)]++
		summary.Transactions.ByStatus[string(tx.Status)]++
	}
	for _, claim := range claims {
		summary.Claims.ByStatus[string(claim.Status)]++
		summary.Claims.ByProvider[claim.ProviderID]++
		summary.Financials.BilledCents += claim.AmountCents
		summary.Financials.AllowedCents += claim.AllowedAmountCents
		summary.Financials.PaidCents += claim.PaidAmountCents
		summary.Financials.PatientResponsibilityCents += claim.PatientResponsibilityCents
		summary.Financials.AdjustmentCents += claim.AdjustmentAmountCents
	}
	for _, job := range jobs {
		summary.AsyncJobs.ByStatus[job.Status]++
		if job.DeadLetter {
			summary.AsyncJobs.DeadLetters++
		}
	}
	if len(highlights) > 0 || summary.AsyncJobs.DeadLetters > 0 {
		summary.OperationalStatus = "attention"
	}
	if summary.Intake.RejectionTotal > 0 {
		summary.Highlights = append(summary.Highlights, "Partner rejection activity detected")
	}
	if summary.Financials.PaidCents > 0 {
		summary.Highlights = append(summary.Highlights, "Paid claims are flowing through remittance")
	}
	if len(summary.Highlights) == 0 {
		summary.Highlights = append(summary.Highlights, "No critical metric signals in the current sample")
	}

	respond(w, http.StatusOK, domain.Envelope{Data: summary, Lore: "The Guild Operations Board collected ledger, claim, intake, and worker metrics."})
}

func (g gateway) fetchMetricsData(r *http.Request, baseURL, path string, target any) error {
	if strings.TrimSpace(baseURL) == "" {
		return errRequestCreation
	}
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header = r.Header.Clone()
	requestmeta.Propagate(r, req)
	resp, err := g.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return http.ErrAbortHandler
	}
	var envelope metricsEnvelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope.Data) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Data, target)
}

func countsToMap(counts []metricsCount) map[string]int {
	result := map[string]int{}
	for _, count := range counts {
		label := strings.TrimSpace(count.Label)
		if label == "" {
			label = strings.TrimSpace(count.Type)
		}
		if label == "" {
			label = "unknown"
		}
		result[label] = count.Count
	}
	return result
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
		requestmeta.Propagate(r, req)
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

func (g gateway) requiresAPIKey(path, method string) bool {
	if len(g.apiKeys) == 0 || method == http.MethodOptions {
		return false
	}
	return !(path == "/health" && method == http.MethodGet)
}

func (g gateway) authorized(r *http.Request) bool {
	return g.validAPIKey(bearerToken(r.Header.Get("Authorization"))) || g.validAPIKey(r.Header.Get("X-ASHN-API-Key"))
}

func (g gateway) allowRequest(w http.ResponseWriter, r *http.Request, path string) bool {
	if g.rateLimiter == nil || g.exemptFromRateLimit(path, r.Method) {
		return true
	}
	result := g.rateLimiter.allow(g.rateLimitKey(r))
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.reset.Unix(), 10))
	if result.allowed {
		return true
	}
	retryAfter := int(math.Ceil(g.rateLimiter.retryAfter(result.reset).Seconds()))
	if retryAfter < 1 {
		retryAfter = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	fail(w, http.StatusTooManyRequests, "rate limit exceeded", "Too many couriers reached the gateway at once. Wait for the crystal to clear.")
	return false
}

func (g gateway) exemptFromRateLimit(path, method string) bool {
	return method == http.MethodOptions || (path == "/health" && method == http.MethodGet)
}

func (g gateway) rateLimitKey(r *http.Request) string {
	if token := bearerToken(r.Header.Get("Authorization")); token != "" {
		return "api-key:" + token
	}
	if key := strings.TrimSpace(r.Header.Get("X-ASHN-API-Key")); key != "" {
		return "api-key:" + key
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return "ip:" + host
	}
	if r.RemoteAddr != "" {
		return "ip:" + r.RemoteAddr
	}
	return "ip:unknown"
}

func (g gateway) validAPIKey(candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}
	for _, key := range g.apiKeys {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(key)) == 1 {
			return true
		}
	}
	return false
}

func bearerToken(header string) string {
	scheme, value, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(value)
}

func apiGatewayOpenAPI() map[string]any {
	spec := openapidocs.Spec(openapidocs.Service{
		Title:       "ASHN API Gateway",
		Description: "Public facade for the Adventure Society Health Network demo APIs.",
		Version:     "0.1.0",
		Paths: map[string]map[string]openapidocs.Operation{
			"/v1/health":           {"get": {Summary: "Check service health", Tags: []string{"gateway"}}},
			"/v1/system/readiness": {"get": {Summary: "Check deploy readiness signals", Tags: []string{"gateway", "operations"}}},
			"/v1/metrics/summary":  {"get": {Summary: "Collect operational metrics summary", Tags: []string{"gateway", "metrics", "operations"}}},
			"/v1/adventurers": {
				"get":  {Summary: "List adventurers", Tags: []string{"adventurers"}},
				"post": {Summary: "Create an enrollment", Tags: []string{"adventurers", "x12"}, RequestBody: true},
			},
			"/v1/adventurers/{id}": {"get": {Summary: "Get an adventurer", Tags: []string{"adventurers"}}},
			"/v1/eligibility":      {"post": {Summary: "Run 270/271 eligibility", Tags: []string{"eligibility", "x12"}, RequestBody: true}},
			"/v1/benefit-coordination": {
				"post": {Summary: "Record 269 benefit coordination", Tags: []string{"benefits", "x12"}, RequestBody: true},
			},
			"/v1/auth-requests": {"post": {Summary: "Submit 278 authorization", Tags: []string{"authorizations", "x12"}, RequestBody: true}},
			"/v1/auth-requests/{id}/decision": {
				"post": {Summary: "Approve or deny a 278 authorization", Tags: []string{"authorizations", "x12"}, RequestBody: true},
			},
			"/v1/auth-requests/{id}/attachments": {
				"post": {Summary: "Submit one 275 attachment or a packet for a 278 authorization", Tags: []string{"authorizations", "attachments", "x12"}, RequestBody: true},
			},
			"/v1/claims": {
				"get":  {Summary: "List claims", Tags: []string{"claims"}},
				"post": {Summary: "Submit 837 claim", Tags: []string{"claims", "x12"}, RequestBody: true},
			},
			"/v1/claims/{id}":                       {"get": {Summary: "Get claim detail", Tags: []string{"claims"}}},
			"/v1/claims/{id}/status":                {"get": {Summary: "Get claim status", Tags: []string{"claims"}}},
			"/v1/claims/{id}/documentation-request": {"post": {Summary: "Request 275 supporting documentation", Tags: []string{"claims", "attachments", "x12"}, RequestBody: true}},
			"/v1/claims/{id}/attachments":           {"post": {Summary: "Submit one 275 patient information attachment or a packet", Tags: []string{"claims", "attachments", "x12"}, RequestBody: true}},
			"/v1/claims/{id}/payment":               {"post": {Summary: "Create 835 payment", Tags: []string{"claims", "x12"}, RequestBody: true}},
			"/v1/premium-payments": {
				"get":  {Summary: "List 820 premium payment history", Tags: []string{"premium", "x12"}},
				"post": {Summary: "Record 820 premium payment", Tags: []string{"premium", "x12"}, RequestBody: true},
			},
			"/v1/transactions":                         {"get": {Summary: "List ledger transactions", Tags: []string{"transactions"}}},
			"/v1/transactions/{id}":                    {"get": {Summary: "Get transaction detail", Tags: []string{"transactions"}}},
			"/v1/transactions/{id}/export":             {"get": {Summary: "Export transaction as JSON, XML, or X12", Tags: []string{"transactions", "export"}}},
			"/v1/transactions/{id}/document-reference": {"get": {Summary: "Resolve 275 document reference metadata", Tags: []string{"transactions", "attachments"}}},
			"/v1/transactions/{id}/document-reference/content": {
				"get": {Summary: "Download embedded 275 document content", Tags: []string{"transactions", "attachments", "export"}},
			},
			"/v1/transactions/{id}/replay":            {"post": {Summary: "Replay transaction", Tags: []string{"transactions", "replay"}}},
			"/v1/transactions/{id}/attachment-review": {"post": {Summary: "Record 275 attachment review outcome", Tags: []string{"transactions", "attachments"}, RequestBody: true}},
			"/v1/jobs":                     {"get": {Summary: "List async transaction jobs", Tags: []string{"async jobs"}}},
			"/v1/jobs/{id}/replay":         {"post": {Summary: "Replay a dead-lettered async job", Tags: []string{"async jobs", "replay"}}},
			"/v1/x12/transactions":         {"post": {Summary: "Accept canonical ASHN transaction intake as XML or JSON", Tags: []string{"intake", "x12"}, RequestBody: true}},
			"/v1/x12/xml":                  {"post": {Summary: "Accept XML intake compatibility route", Tags: []string{"xml", "x12"}, RequestBody: true}},
			"/v1/x12/raw":                  {"post": {Summary: "Accept raw delimiter-based X12 intake", Tags: []string{"raw x12", "x12"}, RequestBody: true}},
			"/v1/x12/batch":                {"post": {Summary: "Accept multipart XML/JSON/raw X12 batch files", Tags: []string{"intake", "batch"}, RequestBody: true}},
			"/v1/x12/messages":             {"get": {Summary: "List XML intake audits", Tags: []string{"xml"}}},
			"/v1/x12/messages/rejections":  {"get": {Summary: "Summarize XML intake rejections", Tags: []string{"xml", "operations"}}},
			"/v1/x12/messages/{id}/export": {"get": {Summary: "Export XML intake audit", Tags: []string{"xml", "export"}}},
			"/v1/x12/messages/{id}/replay": {"post": {Summary: "Replay XML intake", Tags: []string{"xml", "replay"}}},
			"/v1/x12/trading-partners": {
				"get":  {Summary: "List trading partners", Tags: []string{"trading partners", "x12"}},
				"post": {Summary: "Create trading partner", Tags: []string{"trading partners", "x12"}, RequestBody: true},
			},
			"/v1/x12/trading-partners/{id}": {
				"put":    {Summary: "Update trading partner", Tags: []string{"trading partners", "x12"}, RequestBody: true},
				"delete": {Summary: "Delete trading partner", Tags: []string{"trading partners", "x12"}},
			},
			"/v1/providers":                         {"get": {Summary: "List providers", Tags: []string{"providers"}}},
			"/v1/providers/{id}":                    {"get": {Summary: "Get provider detail", Tags: []string{"providers"}}},
			"/v1/providers/{id}/submit-claim":       {"post": {Summary: "Submit claim through provider workflow", Tags: []string{"providers", "claims"}, RequestBody: true}},
			"/v1/providers/{id}/verify-eligibility": {"post": {Summary: "Verify eligibility through provider workflow", Tags: []string{"providers", "eligibility"}, RequestBody: true}},
		},
	})
	spec["components"] = map[string]any{
		"securitySchemes": map[string]any{
			"bearerAuth": map[string]string{
				"type":   "http",
				"scheme": "bearer",
			},
			"apiKeyAuth": map[string]string{
				"type": "apiKey",
				"in":   "header",
				"name": "X-ASHN-API-Key",
			},
		},
	}
	return spec
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-ASHN-API-Key, X-Request-ID, X-Correlation-ID, traceparent, tracestate")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-Correlation-ID, traceparent, tracestate")
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

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func rateLimiterFromEnv() *rateLimiter {
	limit := envInt("ASHN_RATE_LIMIT_REQUESTS", 300)
	window := envDuration("ASHN_RATE_LIMIT_WINDOW", time.Minute)
	if limit <= 0 || window <= 0 {
		return nil
	}
	return newRateLimiter(limit, window, time.Now)
}

func parseAPIKeys(value string) []string {
	keys := []string{}
	for _, item := range strings.Split(value, ",") {
		key := strings.TrimSpace(item)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ashnlog.Request("api-gateway", r)
		next.ServeHTTP(w, r)
	})
}

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	buckets map[string]rateLimitBucket
}

type rateLimitBucket struct {
	count int
	reset time.Time
}

type rateLimitResult struct {
	allowed   bool
	limit     int
	remaining int
	reset     time.Time
}

func newRateLimiter(limit int, window time.Duration, now func() time.Time) *rateLimiter {
	if now == nil {
		now = time.Now
	}
	return &rateLimiter{
		limit:   limit,
		window:  window,
		now:     now,
		buckets: map[string]rateLimitBucket{},
	}
}

func (l *rateLimiter) allow(key string) rateLimitResult {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	bucket := l.buckets[key]
	if bucket.reset.IsZero() || !now.Before(bucket.reset) {
		bucket = rateLimitBucket{reset: now.Add(l.window)}
	}
	if bucket.count >= l.limit {
		l.buckets[key] = bucket
		return rateLimitResult{allowed: false, limit: l.limit, remaining: 0, reset: bucket.reset}
	}
	bucket.count++
	l.buckets[key] = bucket
	remaining := l.limit - bucket.count
	if remaining < 0 {
		remaining = 0
	}
	return rateLimitResult{allowed: true, limit: l.limit, remaining: remaining, reset: bucket.reset}
}

func (l *rateLimiter) retryAfter(reset time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return reset.Sub(l.now())
}
