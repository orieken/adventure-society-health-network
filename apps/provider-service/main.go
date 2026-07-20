package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"ashn/packages/ashnlog"
	"ashn/packages/domain"
	"ashn/packages/openapidocs"
	"ashn/packages/requestmeta"

	_ "github.com/lib/pq"
)

type providerApp struct {
	providers map[string]domain.Provider
	payerURL  string
	client    *http.Client
}

func main() {
	db := openDB()
	app := providerApp{providers: loadProviders(db), payerURL: env("PAYER_CORE_URL", "http://localhost:8081"), client: http.DefaultClient}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", openapidocs.HTMLHandler("ASHN Provider Service Docs"))
	mux.HandleFunc("GET /openapi.json", openapidocs.JSONHandler(providerOpenAPI()))
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /providers", app.listProviders)
	mux.HandleFunc("GET /providers/{id}", app.getProvider)
	mux.HandleFunc("POST /providers/{id}/verify-eligibility", app.verifyEligibility)
	mux.HandleFunc("POST /providers/{id}/submit-claim", app.submitClaim)
	addr := env("PROVIDER_SERVICE_ADDR", ":8082")
	ashnlog.Info("service_listening", "service", "provider-service", "addr", addr)
	ashnlog.Fatal("service_stopped", http.ListenAndServe(addr, requestmeta.Middleware("provider-service", logRequests(mux))), "service", "provider-service")
}

func (a providerApp) listProviders(w http.ResponseWriter, _ *http.Request) {
	providers := make([]domain.Provider, 0, len(a.providers))
	for _, provider := range a.providers {
		providers = append(providers, provider)
	}
	respond(w, http.StatusOK, domain.Envelope{Data: providers, Lore: "Provider registry opened by the Society scribe."})
}

func (a providerApp) getProvider(w http.ResponseWriter, r *http.Request) {
	provider, ok := a.providers[r.PathValue("id")]
	if !ok {
		fail(w, http.StatusNotFound, "provider not found", "No healer's seal matches that provider.")
		return
	}
	respond(w, http.StatusOK, domain.Envelope{Data: provider})
}

func (a providerApp) verifyEligibility(w http.ResponseWriter, r *http.Request) {
	providerID := r.PathValue("id")
	if _, ok := a.providers[providerID]; !ok {
		fail(w, http.StatusNotFound, "provider not found", "No healer's seal matches that provider.")
		return
	}
	var input struct {
		AdventurerID string `json:"adventurerId"`
	}
	if !decode(w, r, &input) {
		return
	}
	body := domain.EligibilityRequest{AdventurerID: input.AdventurerID, ProviderID: providerID}
	a.forward(w, r, http.MethodPost, "/eligibility/query", body)
}

func (a providerApp) submitClaim(w http.ResponseWriter, r *http.Request) {
	providerID := r.PathValue("id")
	if _, ok := a.providers[providerID]; !ok {
		fail(w, http.StatusNotFound, "provider not found", "No healer's seal matches that provider.")
		return
	}
	var input domain.ClaimRequest
	if !decode(w, r, &input) {
		return
	}
	input.ProviderID = providerID
	a.forward(w, r, http.MethodPost, "/claims", input)
}

func (a providerApp) forward(w http.ResponseWriter, inbound *http.Request, method, path string, body any) {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequest(method, a.payerURL+path, bytes.NewReader(payload))
	if err != nil {
		fail(w, http.StatusInternalServerError, "request creation failed", "The provider courier tripped before leaving the clinic.")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	requestmeta.Propagate(inbound, req)
	resp, err := a.httpClient().Do(req)
	if err != nil {
		fail(w, http.StatusBadGateway, "payer-core unavailable", "The provider courier could not reach the Adventure Society.")
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (a providerApp) httpClient() *http.Client {
	if a.client != nil {
		return a.client
	}
	return http.DefaultClient
}

func seedProviders() map[string]domain.Provider {
	providers := map[string]domain.Provider{}
	for _, provider := range []domain.Provider{
		{ID: "provider-greenstone-roadside", Name: "Greenstone Roadside Clinic", ProviderType: domain.ProviderTypeClinic, TierRank: domain.RankIron, Region: domain.RegionGreenstone},
		{ID: "provider-westbridge-outpost", Name: "Westbridge Outpost", ProviderType: domain.ProviderTypeOutpost, TierRank: domain.RankIron, Region: domain.RegionGreenstone},
		{ID: "provider-yaresh-regional", Name: "Yaresh Regional Healing Centre", ProviderType: domain.ProviderTypeClinic, TierRank: domain.RankSilver, Region: domain.RegionYaresh},
		{ID: "provider-jungle-wardens", Name: "Jungle Warden's Guild", ProviderType: domain.ProviderTypeClinic, TierRank: domain.RankSilver, Region: domain.RegionYaresh},
		{ID: "provider-rimaros-hospital", Name: "Rimaros City Hospital", ProviderType: domain.ProviderTypeClinic, TierRank: domain.RankGold, Region: domain.RegionRimaros},
		{ID: "provider-crown-dental", Name: "Crown Dental Clearinghouse", ProviderType: domain.ProviderTypeClinic, TierRank: domain.RankGold, Region: domain.RegionRimaros},
		{ID: "provider-vitesse-temple", Name: "Temple of the Healer, Vitesse", ProviderType: domain.ProviderTypeTemple, TierRank: domain.RankDiamond, Region: domain.RegionVitesse},
	} {
		providers[provider.ID] = provider
	}
	return providers
}

func loadProviders(db *sql.DB) map[string]domain.Provider {
	if db == nil {
		return seedProviders()
	}
	rows, err := db.Query(`SELECT id, name, provider_type, tier_rank, region FROM providers ORDER BY name`)
	if err != nil {
		ashnlog.Error("postgres_provider_load_failed_using_seed", err, "service", "provider-service")
		return seedProviders()
	}
	defer rows.Close()
	providers := map[string]domain.Provider{}
	for rows.Next() {
		var provider domain.Provider
		if err := rows.Scan(&provider.ID, &provider.Name, &provider.ProviderType, &provider.TierRank, &provider.Region); err != nil {
			ashnlog.Error("postgres_provider_row_skipped", err, "service", "provider-service")
			continue
		}
		providers[provider.ID] = provider
	}
	if err := rows.Err(); err != nil {
		ashnlog.Error("postgres_provider_rows_failed_using_seed", err, "service", "provider-service")
		return seedProviders()
	}
	if len(providers) == 0 {
		ashnlog.Info("postgres_provider_table_empty_using_seed", "service", "provider-service")
		return seedProviders()
	}
	ashnlog.Info("postgres_providers_loaded", "service", "provider-service", "count", len(providers))
	return providers
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		fail(w, http.StatusBadRequest, "invalid json", "The submitted clinic scroll could not be read.")
		return false
	}
	return true
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func fail(w http.ResponseWriter, status int, message, loreText string) {
	respond(w, status, domain.ErrorEnvelope{Error: message, Lore: loreText})
}

func health(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok", "service": "provider-service"})
}

func providerOpenAPI() map[string]any {
	return openapidocs.Spec(openapidocs.Service{
		Title:       "ASHN Provider Service",
		Description: "Provider registry and provider-facing workflows for eligibility and claim submission.",
		Version:     "0.1.0",
		Paths: map[string]map[string]openapidocs.Operation{
			"/health": {"get": {Summary: "Check provider-service health", Tags: []string{"health"}}},
			"/providers": {
				"get": {Summary: "List providers", Tags: []string{"providers"}},
			},
			"/providers/{id}": {
				"get": {Summary: "Get provider detail", Tags: []string{"providers"}},
			},
			"/providers/{id}/verify-eligibility": {
				"post": {Summary: "Verify eligibility through payer-core", Tags: []string{"providers", "eligibility"}, RequestBody: true},
			},
			"/providers/{id}/submit-claim": {
				"post": {Summary: "Submit claim through payer-core", Tags: []string{"providers", "claims"}, RequestBody: true},
			},
		},
	})
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func openDB() *sql.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		ashnlog.Info("database_url_missing_using_seed", "service", "provider-service")
		return nil
	}
	return openDBWith(dsn, sql.Open)
}

func openDBWith(dsn string, open func(string, string) (*sql.DB, error)) *sql.DB {
	db, err := open("postgres", dsn)
	if err != nil {
		ashnlog.Error("postgres_open_failed_using_seed", err, "service", "provider-service")
		return nil
	}
	if err := db.Ping(); err != nil {
		ashnlog.Error("postgres_ping_failed_using_seed", err, "service", "provider-service")
		_ = db.Close()
		return nil
	}
	ashnlog.Info("postgres_connected", "service", "provider-service")
	return db
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ashnlog.Request("provider-service", r)
		next.ServeHTTP(w, r)
	})
}
