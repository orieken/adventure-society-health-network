package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"ashn/packages/domain"
	edimock "ashn/packages/edi-mock"
	"ashn/packages/lore"

	_ "github.com/lib/pq"
)

const societyID = "adventure-society"

type store struct {
	mu           sync.RWMutex
	adventurers  map[string]domain.Adventurer
	providers    map[string]domain.Provider
	claims       map[string]domain.Claim
	transactions map[string]domain.Transaction
	db           *sql.DB
}

func main() {
	db := openDB()
	app := &store{
		adventurers:  loadAdventurers(db),
		providers:    loadProviders(db),
		claims:       loadClaims(db),
		transactions: loadTransactions(db),
		db:           db,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /enrollments", app.enroll)
	mux.HandleFunc("GET /adventurers", app.listAdventurers)
	mux.HandleFunc("GET /adventurers/{id}", app.getAdventurer)
	mux.HandleFunc("POST /eligibility/query", app.eligibility)
	mux.HandleFunc("POST /auth-requests", app.authRequest)
	mux.HandleFunc("GET /claims", app.listClaims)
	mux.HandleFunc("POST /claims", app.submitClaim)
	mux.HandleFunc("GET /claims/{id}", app.getClaim)
	mux.HandleFunc("GET /claims/{id}/status", app.claimStatus)
	mux.HandleFunc("POST /claims/{id}/payment", app.payClaim)
	mux.HandleFunc("GET /transactions", app.listTransactions)
	mux.HandleFunc("GET /transactions/{id}", app.getTransaction)
	addr := env("PAYER_CORE_ADDR", ":8081")
	log.Printf("[ASHN] payer-core listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, logRequests(mux)))
}

func (s *store) listAdventurers(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 25)
	if s.db != nil {
		adventurers, err := s.queryAdventurers(limit)
		if err == nil {
			respond(w, http.StatusOK, domain.Envelope{Data: adventurers, Lore: "The Society opened its recent adventurer registry."})
			return
		}
		log.Printf("[ASHN] postgres adventurer list failed; using memory: %v", err)
	}
	s.mu.RLock()
	adventurers := make([]domain.Adventurer, 0, len(s.adventurers))
	for _, adventurer := range s.adventurers {
		adventurers = append(adventurers, adventurer)
	}
	s.mu.RUnlock()
	sort.Slice(adventurers, func(i, j int) bool {
		return adventurers[i].Name < adventurers[j].Name
	})
	respond(w, http.StatusOK, domain.Envelope{Data: clamp(adventurers, limit), Lore: "The Society opened its adventurer registry from active memory."})
}

func (s *store) enroll(w http.ResponseWriter, r *http.Request) {
	var req domain.EnrollmentRequest
	if !decode(w, r, &req) {
		return
	}
	adventurer := domain.Adventurer{
		ID: domain.NewID(), Name: req.Name, Rank: req.Rank, Guild: req.Guild, Region: req.Region,
		CoverageStatus: domain.CoverageActive,
	}
	tx := edimock.Generate834(adventurer, societyID)
	s.saveAdventurer(adventurer)
	s.saveTransaction(tx)
	s.saveEnrollment(adventurer.ID, tx.ID, string(domain.TxStatusAccepted))
	respond(w, http.StatusCreated, domain.Envelope{Data: adventurer, Lore: lore.ThemeTransaction(domain.Tx834, adventurer.Name, "Adventure Society"), Transaction: &tx})
}

func (s *store) getAdventurer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	adventurer, ok := s.adventurers[id]
	s.mu.RUnlock()
	if !ok {
		fail(w, http.StatusNotFound, "adventurer not found", "The Society archives contain no record of that adventurer.")
		return
	}
	respond(w, http.StatusOK, domain.Envelope{Data: adventurer})
}

func (s *store) eligibility(w http.ResponseWriter, r *http.Request) {
	var req domain.EligibilityRequest
	if !decode(w, r, &req) {
		return
	}
	adventurer, provider, ok := s.findAdventurerProvider(w, req.AdventurerID, req.ProviderID)
	if !ok {
		return
	}
	inquiry := edimock.Generate270(adventurer, provider)
	eligible := adventurer.CoverageStatus == domain.CoverageActive
	response := edimock.Generate271(adventurer, eligible)
	s.saveTransaction(inquiry)
	s.saveTransaction(response)
	data := map[string]any{"eligible": eligible, "coverageStatus": adventurer.CoverageStatus, "adventurerId": adventurer.ID, "providerId": provider.ID}
	respond(w, http.StatusOK, domain.Envelope{Data: data, Lore: lore.ThemeTransaction(domain.Tx271, adventurer.Name, "Adventure Society"), Transactions: []domain.Transaction{inquiry, response}})
}

func (s *store) authRequest(w http.ResponseWriter, r *http.Request) {
	var req domain.PriorAuthRequest
	if !decode(w, r, &req) {
		return
	}
	adventurer, provider, ok := s.findAdventurerProvider(w, req.AdventurerID, req.ProviderID)
	if !ok {
		return
	}
	tx := edimock.Generate278Request(adventurer, provider, req.ServiceType)
	decision := domain.TxStatusPending
	if req.IncidentSeverity == domain.SeverityDiamond && strings.Contains(strings.ToLower(req.ServiceType), "resurrection") {
		decision = domain.TxStatusApproved
	}
	s.saveTransaction(tx)
	s.saveAuthRequest(adventurer.ID, provider.ID, tx.ID, req.ServiceType, req.IncidentSeverity, string(decision))
	data := map[string]any{"authorizationStatus": decision, "serviceType": req.ServiceType, "incidentSeverity": req.IncidentSeverity}
	respond(w, http.StatusAccepted, domain.Envelope{Data: data, Lore: lore.ThemeTransaction(domain.Tx278, adventurer.Name, provider.Name), Transaction: &tx})
}

func (s *store) submitClaim(w http.ResponseWriter, r *http.Request) {
	var req domain.ClaimRequest
	if !decode(w, r, &req) {
		return
	}
	adventurer, provider, ok := s.findAdventurerProvider(w, req.AdventurerID, req.ProviderID)
	if !ok {
		return
	}
	claim := domain.Claim{
		ID: domain.NewID(), AdventurerID: adventurer.ID, ProviderID: provider.ID,
		IncidentSeverity: req.IncidentSeverity, AmountCents: req.AmountCents, Status: domain.ClaimSubmitted,
	}
	tx := edimock.Generate837(claim)
	claim.TransactionID = tx.ID
	s.saveTransaction(tx)
	s.saveClaim(claim)
	respond(w, http.StatusCreated, domain.Envelope{Data: claim, Lore: lore.ThemeTransaction(domain.Tx837, adventurer.Name, provider.Name), Transaction: &tx})
}

func (s *store) listClaims(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 25)
	if s.db != nil {
		claims, err := s.queryClaims(limit)
		if err == nil {
			respond(w, http.StatusOK, domain.Envelope{Data: claims, Lore: "Recent claim scrolls were pulled from the Society ledger."})
			return
		}
		log.Printf("[ASHN] postgres claim list failed; using memory: %v", err)
	}
	s.mu.RLock()
	claims := make([]domain.Claim, 0, len(s.claims))
	for _, claim := range s.claims {
		claims = append(claims, claim)
	}
	s.mu.RUnlock()
	sort.Slice(claims, func(i, j int) bool {
		return claims[i].ID > claims[j].ID
	})
	respond(w, http.StatusOK, domain.Envelope{Data: clamp(claims, limit), Lore: "Recent claim scrolls were pulled from active memory."})
}

func (s *store) getClaim(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	claim, ok := s.claims[id]
	s.mu.RUnlock()
	if !ok {
		fail(w, http.StatusNotFound, "claim not found", "No claim scroll with that seal exists in the Society ledger.")
		return
	}
	respond(w, http.StatusOK, domain.Envelope{Data: claim, Lore: "The Society retrieved a claim scroll from the ledger."})
}

func (s *store) claimStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	claim, ok := s.claims[id]
	s.mu.RUnlock()
	if !ok {
		fail(w, http.StatusNotFound, "claim not found", "No claim scroll with that seal exists in the Society ledger.")
		return
	}
	request := edimock.Generate276(claim.ID)
	response := edimock.Generate277(claim.ID, claim.Status)
	s.saveTransaction(request)
	s.saveTransaction(response)
	respond(w, http.StatusOK, domain.Envelope{Data: map[string]any{"claimId": claim.ID, "status": claim.Status}, Lore: lore.ThemeTransaction(domain.Tx277, claim.ID, "Adventure Society"), Transactions: []domain.Transaction{request, response}})
}

func (s *store) payClaim(w http.ResponseWriter, r *http.Request) {
	var req domain.PaymentRequest
	if !decode(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	s.mu.Lock()
	claim, ok := s.claims[id]
	if ok {
		claim.Status = domain.ClaimPaid
		s.claims[id] = claim
	}
	s.mu.Unlock()
	if !ok {
		fail(w, http.StatusNotFound, "claim not found", "The remittance scribe could not locate that claim.")
		return
	}
	s.saveClaim(claim)
	tx := edimock.Generate835(claim, req.PaymentAmountCents)
	s.saveTransaction(tx)
	respond(w, http.StatusOK, domain.Envelope{Data: claim, Lore: lore.ThemeTransaction(domain.Tx835, claim.ID, claim.ProviderID), Transaction: &tx})
}

func (s *store) getTransaction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.RLock()
	tx, ok := s.transactions[id]
	s.mu.RUnlock()
	if !ok {
		fail(w, http.StatusNotFound, "transaction not found", "The transaction rune is absent from the ledger.")
		return
	}
	respond(w, http.StatusOK, domain.Envelope{Data: tx, Transaction: &tx})
}

func (s *store) listTransactions(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r, 25)
	if s.db != nil {
		transactions, err := s.queryTransactions(limit)
		if err == nil {
			respond(w, http.StatusOK, domain.Envelope{Data: transactions, Lore: "Recent EDI runes were pulled from the transaction ledger."})
			return
		}
		log.Printf("[ASHN] postgres transaction list failed; using memory: %v", err)
	}
	s.mu.RLock()
	transactions := make([]domain.Transaction, 0, len(s.transactions))
	for _, transaction := range s.transactions {
		transactions = append(transactions, transaction)
	}
	s.mu.RUnlock()
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].CreatedAt.After(transactions[j].CreatedAt)
	})
	respond(w, http.StatusOK, domain.Envelope{Data: clamp(transactions, limit), Lore: "Recent EDI runes were pulled from active memory."})
}

func (s *store) findAdventurerProvider(w http.ResponseWriter, adventurerID, providerID string) (domain.Adventurer, domain.Provider, bool) {
	s.mu.RLock()
	adventurer, adventurerOK := s.adventurers[adventurerID]
	provider, providerOK := s.providers[providerID]
	s.mu.RUnlock()
	if !adventurerOK {
		fail(w, http.StatusNotFound, "adventurer not found", "The Society archives contain no record of that adventurer.")
		return domain.Adventurer{}, domain.Provider{}, false
	}
	if !providerOK {
		fail(w, http.StatusNotFound, "provider not found", "No healer's seal matches that provider.")
		return domain.Adventurer{}, domain.Provider{}, false
	}
	return adventurer, provider, true
}

func (s *store) saveAdventurer(adventurer domain.Adventurer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adventurers[adventurer.ID] = adventurer
	if s.db != nil {
		_, err := s.db.Exec(`INSERT INTO adventurers (id, name, rank, guild, region, coverage_status) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, rank = EXCLUDED.rank, guild = EXCLUDED.guild, region = EXCLUDED.region, coverage_status = EXCLUDED.coverage_status`,
			adventurer.ID, adventurer.Name, adventurer.Rank, adventurer.Guild, adventurer.Region, adventurer.CoverageStatus)
		if err != nil {
			log.Printf("[ASHN] postgres adventurer persistence failed: %v", err)
		}
	}
}

func (s *store) saveClaim(claim domain.Claim) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims[claim.ID] = claim
	if s.db != nil {
		_, err := s.db.Exec(`INSERT INTO claims (id, adventurer_id, provider_id, incident_severity, transaction_id, amount_cents, status) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO UPDATE SET transaction_id = EXCLUDED.transaction_id, amount_cents = EXCLUDED.amount_cents, status = EXCLUDED.status`,
			claim.ID, claim.AdventurerID, claim.ProviderID, claim.IncidentSeverity, claim.TransactionID, claim.AmountCents, claim.Status)
		if err != nil {
			log.Printf("[ASHN] postgres claim persistence failed: %v", err)
		}
	}
}

func (s *store) saveTransaction(tx domain.Transaction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transactions[tx.ID] = tx
	if s.db != nil {
		_, err := s.db.Exec(`INSERT INTO transactions (id, type, status, sender_id, receiver_id, payload, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING`,
			tx.ID, tx.Type, tx.Status, tx.SenderID, tx.ReceiverID, []byte(tx.Payload), tx.CreatedAt)
		if err != nil {
			log.Printf("[ASHN] postgres transaction persistence failed: %v", err)
		}
	}
	log.Printf("[ASHN] transaction=%s type=%s status=%s lore=%s", tx.ID, tx.Type, tx.Status, lore.ThemeTransaction(tx.Type, tx.SenderID, tx.ReceiverID))
}

func (s *store) saveEnrollment(adventurerID, transactionID, status string) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO enrollments (id, adventurer_id, transaction_id, status) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING`,
		domain.NewID(), adventurerID, transactionID, status)
	if err != nil {
		log.Printf("[ASHN] postgres enrollment persistence failed: %v", err)
	}
}

func (s *store) saveAuthRequest(adventurerID, providerID, transactionID, serviceType string, severity domain.IncidentSeverity, status string) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO auth_requests (id, adventurer_id, provider_id, transaction_id, service_type, incident_severity, status) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING`,
		domain.NewID(), adventurerID, providerID, transactionID, serviceType, severity, status)
	if err != nil {
		log.Printf("[ASHN] postgres auth request persistence failed: %v", err)
	}
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

func loadAdventurers(db *sql.DB) map[string]domain.Adventurer {
	adventurers := map[string]domain.Adventurer{}
	if db == nil {
		return adventurers
	}
	rows, err := db.Query(`SELECT id, name, rank, guild, region, coverage_status FROM adventurers`)
	if err != nil {
		log.Printf("[ASHN] postgres adventurer load failed: %v", err)
		return adventurers
	}
	defer rows.Close()
	for rows.Next() {
		var adventurer domain.Adventurer
		if err := rows.Scan(&adventurer.ID, &adventurer.Name, &adventurer.Rank, &adventurer.Guild, &adventurer.Region, &adventurer.CoverageStatus); err != nil {
			log.Printf("[ASHN] postgres adventurer row skipped: %v", err)
			continue
		}
		adventurers[adventurer.ID] = adventurer
	}
	if err := rows.Err(); err != nil {
		log.Printf("[ASHN] postgres adventurer rows failed: %v", err)
	}
	log.Printf("[ASHN] loaded %d adventurers from Postgres", len(adventurers))
	return adventurers
}

func loadClaims(db *sql.DB) map[string]domain.Claim {
	claims := map[string]domain.Claim{}
	if db == nil {
		return claims
	}
	rows, err := db.Query(`SELECT id, adventurer_id, provider_id, incident_severity, COALESCE(transaction_id, ''), amount_cents, status FROM claims`)
	if err != nil {
		log.Printf("[ASHN] postgres claim load failed: %v", err)
		return claims
	}
	defer rows.Close()
	for rows.Next() {
		var claim domain.Claim
		if err := rows.Scan(&claim.ID, &claim.AdventurerID, &claim.ProviderID, &claim.IncidentSeverity, &claim.TransactionID, &claim.AmountCents, &claim.Status); err != nil {
			log.Printf("[ASHN] postgres claim row skipped: %v", err)
			continue
		}
		claims[claim.ID] = claim
	}
	if err := rows.Err(); err != nil {
		log.Printf("[ASHN] postgres claim rows failed: %v", err)
	}
	log.Printf("[ASHN] loaded %d claims from Postgres", len(claims))
	return claims
}

func loadTransactions(db *sql.DB) map[string]domain.Transaction {
	transactions := map[string]domain.Transaction{}
	if db == nil {
		return transactions
	}
	rows, err := db.Query(`SELECT id, type, status, sender_id, receiver_id, payload, created_at FROM transactions`)
	if err != nil {
		log.Printf("[ASHN] postgres transaction load failed: %v", err)
		return transactions
	}
	defer rows.Close()
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(&tx.ID, &tx.Type, &tx.Status, &tx.SenderID, &tx.ReceiverID, &tx.Payload, &tx.CreatedAt); err != nil {
			log.Printf("[ASHN] postgres transaction row skipped: %v", err)
			continue
		}
		transactions[tx.ID] = tx
	}
	if err := rows.Err(); err != nil {
		log.Printf("[ASHN] postgres transaction rows failed: %v", err)
	}
	log.Printf("[ASHN] loaded %d transactions from Postgres", len(transactions))
	return transactions
}

func (s *store) queryAdventurers(limit int) ([]domain.Adventurer, error) {
	rows, err := s.db.Query(`SELECT id, name, rank, guild, region, coverage_status FROM adventurers ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	adventurers := []domain.Adventurer{}
	for rows.Next() {
		var adventurer domain.Adventurer
		if err := rows.Scan(&adventurer.ID, &adventurer.Name, &adventurer.Rank, &adventurer.Guild, &adventurer.Region, &adventurer.CoverageStatus); err != nil {
			return nil, err
		}
		adventurers = append(adventurers, adventurer)
	}
	return adventurers, rows.Err()
}

func (s *store) queryClaims(limit int) ([]domain.Claim, error) {
	rows, err := s.db.Query(`SELECT id, adventurer_id, provider_id, incident_severity, COALESCE(transaction_id, ''), amount_cents, status FROM claims ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	claims := []domain.Claim{}
	for rows.Next() {
		var claim domain.Claim
		if err := rows.Scan(&claim.ID, &claim.AdventurerID, &claim.ProviderID, &claim.IncidentSeverity, &claim.TransactionID, &claim.AmountCents, &claim.Status); err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}
	return claims, rows.Err()
}

func (s *store) queryTransactions(limit int) ([]domain.Transaction, error) {
	rows, err := s.db.Query(`SELECT id, type, status, sender_id, receiver_id, payload, created_at FROM transactions ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	transactions := []domain.Transaction{}
	for rows.Next() {
		var transaction domain.Transaction
		if err := rows.Scan(&transaction.ID, &transaction.Type, &transaction.Status, &transaction.SenderID, &transaction.ReceiverID, &transaction.Payload, &transaction.CreatedAt); err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}
	return transactions, rows.Err()
}

func parseLimit(r *http.Request, fallback int) int {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return fallback
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 {
		return fallback
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func clamp[T any](items []T, limit int) []T {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		fail(w, http.StatusBadRequest, "invalid json", "The submitted scroll could not be read by the Society scribe.")
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
	respond(w, http.StatusOK, map[string]string{"status": "ok", "service": "payer-core"})
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
		log.Printf("[ASHN] DATABASE_URL not set; payer-core using in-memory persistence")
		return nil
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("[ASHN] postgres open failed; using in-memory persistence: %v", err)
		return nil
	}
	if err := db.Ping(); err != nil {
		log.Printf("[ASHN] postgres ping failed; using in-memory persistence: %v", err)
		_ = db.Close()
		return nil
	}
	log.Printf("[ASHN] payer-core connected to Postgres")
	return db
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[ASHN] %s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
