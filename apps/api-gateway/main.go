package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"ashn/packages/domain"
)

type gateway struct {
	payerURL    string
	providerURL string
}

func main() {
	g := gateway{payerURL: env("PAYER_CORE_URL", "http://localhost:8081"), providerURL: env("PROVIDER_SERVICE_URL", "http://localhost:8082")}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/", g.route)
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
	case strings.HasPrefix(path, "/adventurers/") && r.Method == http.MethodGet:
		g.proxy(w, r, g.payerURL, path)
	case path == "/eligibility" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, "/eligibility/query")
	case path == "/auth-requests" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, path)
	case path == "/claims" && r.Method == http.MethodPost:
		g.proxy(w, r, g.payerURL, path)
	case strings.HasPrefix(path, "/claims/"):
		g.proxy(w, r, g.payerURL, path)
	case strings.HasPrefix(path, "/transactions/") && r.Method == http.MethodGet:
		g.proxy(w, r, g.payerURL, path)
	case strings.HasPrefix(path, "/providers"):
		g.proxy(w, r, g.providerURL, path)
	default:
		fail(w, http.StatusNotFound, "route not found", "No known Society road leads to that endpoint.")
	}
}

func (g gateway) proxy(w http.ResponseWriter, r *http.Request, baseURL, path string) {
	req, err := http.NewRequest(r.Method, baseURL+path, r.Body)
	if err != nil {
		fail(w, http.StatusInternalServerError, "request creation failed", "The gateway scribe could not bind the courier spell.")
		return
	}
	req.Header = r.Header.Clone()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
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
	status := map[string]string{"api-gateway": "ok", "payer-core": "unknown", "provider-service": "unknown"}
	for service, url := range map[string]string{"payer-core": g.payerURL, "provider-service": g.providerURL} {
		resp, err := http.Get(url + "/health")
		if err == nil && resp.StatusCode < 500 {
			status[service] = "ok"
		} else {
			status[service] = "unavailable"
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	respond(w, http.StatusOK, domain.Envelope{Data: status, Lore: "The gateway crystal checked every downstream beacon."})
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
