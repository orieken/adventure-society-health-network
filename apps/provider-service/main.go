package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"ashn/packages/domain"

	_ "github.com/lib/pq"
)

type providerApp struct {
	providers map[string]domain.Provider
	payerURL  string
}

func main() {
	db := openDB()
	app := providerApp{providers: loadProviders(db), payerURL: env("PAYER_CORE_URL", "http://localhost:8081")}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /providers", app.listProviders)
	mux.HandleFunc("GET /providers/{id}", app.getProvider)
	mux.HandleFunc("POST /providers/{id}/verify-eligibility", app.verifyEligibility)
	mux.HandleFunc("POST /providers/{id}/submit-claim", app.submitClaim)
	addr := env("PROVIDER_SERVICE_ADDR", ":8082")
	log.Printf("[ASHN] provider-service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, logRequests(mux)))
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
	a.forward(w, http.MethodPost, "/eligibility/query", body)
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
	a.forward(w, http.MethodPost, "/claims", input)
}

func (a providerApp) forward(w http.ResponseWriter, method, path string, body any) {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequest(method, a.payerURL+path, bytes.NewReader(payload))
	if err != nil {
		fail(w, http.StatusInternalServerError, "request creation failed", "The provider courier tripped before leaving the clinic.")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fail(w, http.StatusBadGateway, "payer-core unavailable", "The provider courier could not reach the Adventure Society.")
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func seedProviders() map[string]domain.Provider {
	providers := map[string]domain.Provider{}
	for _, provider := range []domain.Provider{
		{ID: "provider-greenstone-roadside", Name: "Greenstone Roadside Clinic", ProviderType: domain.ProviderTypeClinic, TierRank: domain.RankIron, Region: domain.RegionGreenstone},
		{ID: "provider-westbridge-outpost", Name: "Westbridge Outpost", ProviderType: domain.ProviderTypeOutpost, TierRank: domain.RankIron, Region: domain.RegionGreenstone},
		{ID: "provider-yaresh-regional", Name: "Yaresh Regional Healing Centre", ProviderType: domain.ProviderTypeClinic, TierRank: domain.RankSilver, Region: domain.RegionYaresh},
		{ID: "provider-jungle-wardens", Name: "Jungle Warden's Guild", ProviderType: domain.ProviderTypeClinic, TierRank: domain.RankSilver, Region: domain.RegionYaresh},
		{ID: "provider-rimaros-hospital", Name: "Rimaros City Hospital", ProviderType: domain.ProviderTypeClinic, TierRank: domain.RankGold, Region: domain.RegionRimaros},
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
		log.Printf("[ASHN] postgres provider load failed; using seed providers: %v", err)
		return seedProviders()
	}
	defer rows.Close()
	providers := map[string]domain.Provider{}
	for rows.Next() {
		var provider domain.Provider
		if err := rows.Scan(&provider.ID, &provider.Name, &provider.ProviderType, &provider.TierRank, &provider.Region); err != nil {
			log.Printf("[ASHN] postgres provider row skipped: %v", err)
			continue
		}
		providers[provider.ID] = provider
	}
	if err := rows.Err(); err != nil {
		log.Printf("[ASHN] postgres provider rows failed; using seed providers: %v", err)
		return seedProviders()
	}
	if len(providers) == 0 {
		log.Printf("[ASHN] postgres provider table empty; using seed providers")
		return seedProviders()
	}
	log.Printf("[ASHN] loaded %d providers from Postgres", len(providers))
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

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func openDB() *sql.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Printf("[ASHN] DATABASE_URL not set; provider-service using seed providers")
		return nil
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("[ASHN] postgres open failed; using seed providers: %v", err)
		return nil
	}
	if err := db.Ping(); err != nil {
		log.Printf("[ASHN] postgres ping failed; using seed providers: %v", err)
		_ = db.Close()
		return nil
	}
	log.Printf("[ASHN] provider-service connected to Postgres")
	return db
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[ASHN] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
