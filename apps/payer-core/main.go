package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ashn/packages/asyncjobs"
	"ashn/packages/domain"
	edimock "ashn/packages/edi-mock"
	"ashn/packages/lore"

	_ "github.com/lib/pq"
)

const societyID = "adventure-society"
const claimSelectColumns = `id, adventurer_id, provider_id, incident_severity, COALESCE(transaction_id, ''), amount_cents, allowed_amount_cents, paid_amount_cents, patient_responsibility_cents, adjustment_amount_cents, COALESCE(adjustment_reason, ''), COALESCE(denial_reason, ''), status`

type store struct {
	mu           sync.RWMutex
	adventurers  map[string]domain.Adventurer
	providers    map[string]domain.Provider
	claims       map[string]domain.Claim
	transactions map[string]domain.Transaction
	db           *sql.DB
}

type pageRequest struct {
	Limit  int
	Offset int
}

type adventurerFilters struct {
	Q              string
	Rank           string
	Region         string
	CoverageStatus string
}

type claimFilters struct {
	Q            string
	Status       string
	ProviderID   string
	AdventurerID string
	Severity     string
}

type transactionFilters struct {
	Q      string
	Type   string
	Status string
}

func main() {
	db := openDB()
	if db != nil && env("ASHN_AUTO_MIGRATE", "") == "true" {
		applyMigration(db)
	}
	app := &store{
		adventurers:  loadAdventurers(db),
		providers:    loadProviders(db),
		claims:       loadClaims(db),
		transactions: loadTransactions(db),
		db:           db,
	}
	if env("ASHN_EMBED_WORKER", "") == "true" {
		go runEmbeddedWorker(db)
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
	mux.HandleFunc("POST /transactions", app.recordTransaction)
	mux.HandleFunc("GET /transactions/{id}", app.getTransaction)
	mux.HandleFunc("GET /transactions/{id}/export", app.exportTransaction)
	mux.HandleFunc("POST /transactions/{id}/replay", app.replayTransaction)
	addr := env("PAYER_CORE_ADDR", ":8081")
	log.Printf("[ASHN] payer-core listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, logRequests(mux)))
}

func (s *store) listAdventurers(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r, 25)
	filters := parseAdventurerFilters(r)
	if s.db != nil {
		adventurers, pageInfo, err := s.queryAdventurers(page, filters)
		if err == nil {
			respond(w, http.StatusOK, domain.Envelope{Data: adventurers, Lore: "The Society opened its recent adventurer registry.", Page: &pageInfo})
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
	adventurers = filterAdventurers(adventurers, filters)
	adventurers, pageInfo := paginate(adventurers, page)
	respond(w, http.StatusOK, domain.Envelope{Data: adventurers, Lore: "The Society opened its adventurer registry from active memory.", Page: &pageInfo})
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
	s.saveTransaction(tx)
	s.saveAuthRequest(adventurer.ID, provider.ID, tx.ID, req.ServiceType, req.IncidentSeverity, string(domain.TxStatusPending))
	s.enqueueJob(asyncjobs.JobAuthReview, tx.ID, 2*time.Second)
	data := map[string]any{"authorizationStatus": domain.TxStatusPending, "serviceType": req.ServiceType, "incidentSeverity": req.IncidentSeverity, "review": "queued"}
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
	ack := edimock.Generate277CA(claim, tx.ID, true)
	s.saveTransaction(tx)
	s.saveTransaction(ack)
	s.saveClaim(claim)
	s.enqueueJob(asyncjobs.JobClaimAdjudication, claim.ID, 2*time.Second)
	respond(w, http.StatusCreated, domain.Envelope{Data: claim, Lore: lore.ThemeTransaction(domain.Tx837, adventurer.Name, provider.Name), Transaction: &tx, Transactions: []domain.Transaction{tx, ack}})
}

func (s *store) listClaims(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r, 25)
	filters := parseClaimFilters(r)
	if s.db != nil {
		claims, pageInfo, err := s.queryClaims(page, filters)
		if err == nil {
			respond(w, http.StatusOK, domain.Envelope{Data: claims, Lore: "Recent claim scrolls were pulled from the Society ledger.", Page: &pageInfo})
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
	claims = filterClaims(claims, filters)
	claims, pageInfo := paginate(claims, page)
	respond(w, http.StatusOK, domain.Envelope{Data: claims, Lore: "Recent claim scrolls were pulled from active memory.", Page: &pageInfo})
}

func (s *store) getClaim(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	claim, ok := s.findClaim(id)
	if !ok {
		fail(w, http.StatusNotFound, "claim not found", "No claim scroll with that seal exists in the Society ledger.")
		return
	}
	respond(w, http.StatusOK, domain.Envelope{Data: claim, Lore: "The Society retrieved a claim scroll from the ledger."})
}

func (s *store) claimStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	claim, ok := s.findClaim(id)
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
	claim, ok := s.findClaim(id)
	if !ok {
		fail(w, http.StatusNotFound, "claim not found", "The remittance scribe could not locate that claim.")
		return
	}
	paymentAmountCents := req.PaymentAmountCents
	if claim.PaidAmountCents > 0 || claim.Status == domain.ClaimDenied {
		paymentAmountCents = claim.PaidAmountCents
	}
	claim.Status = domain.ClaimPaid
	s.saveClaim(claim)
	tx := edimock.Generate835(claim, paymentAmountCents)
	s.saveTransaction(tx)
	respond(w, http.StatusOK, domain.Envelope{Data: claim, Lore: lore.ThemeTransaction(domain.Tx835, claim.ID, claim.ProviderID), Transaction: &tx})
}

func (s *store) getTransaction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	tx, ok := s.findTransaction(id)
	if !ok {
		fail(w, http.StatusNotFound, "transaction not found", "The transaction rune is absent from the ledger.")
		return
	}
	respond(w, http.StatusOK, domain.Envelope{Data: tx, Transaction: &tx})
}

func (s *store) exportTransaction(w http.ResponseWriter, r *http.Request) {
	tx, ok := s.findTransaction(r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, "transaction not found", "The transaction rune is absent from the ledger.")
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	switch format {
	case "x12":
		download(w, "text/plain; charset=utf-8", fmt.Sprintf("ashn-%s-%s.x12", tx.Type, tx.ID), []byte(tx.RawX12))
	case "xml":
		download(w, "application/xml; charset=utf-8", fmt.Sprintf("ashn-%s-%s.xml", tx.Type, tx.ID), []byte(transactionXML(tx)))
	default:
		payload, _ := json.MarshalIndent(tx, "", "  ")
		download(w, "application/json; charset=utf-8", fmt.Sprintf("ashn-%s-%s.json", tx.Type, tx.ID), payload)
	}
}

func (s *store) replayTransaction(w http.ResponseWriter, r *http.Request) {
	tx, ok := s.findTransaction(r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, "transaction not found", "The transaction rune is absent from the ledger.")
		return
	}
	tx.ID = domain.NewID()
	tx.RelatedID = r.PathValue("id")
	tx.CreatedAt = time.Now().UTC()
	tx.RawX12 = strings.ReplaceAll(tx.RawX12, r.PathValue("id"), tx.ID)
	s.saveTransaction(tx)
	respond(w, http.StatusCreated, domain.Envelope{Data: tx, Transaction: &tx, Lore: "The Society replayed a transaction rune into the ledger."})
}

func (s *store) listTransactions(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r, 25)
	filters := parseTransactionFilters(r)
	if s.db != nil {
		transactions, pageInfo, err := s.queryTransactions(page, filters)
		if err == nil {
			respond(w, http.StatusOK, domain.Envelope{Data: transactions, Lore: "Recent EDI runes were pulled from the transaction ledger.", Page: &pageInfo})
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
	transactions = filterTransactions(transactions, filters)
	transactions, pageInfo := paginate(transactions, page)
	respond(w, http.StatusOK, domain.Envelope{Data: transactions, Lore: "Recent EDI runes were pulled from active memory.", Page: &pageInfo})
}

func (s *store) recordTransaction(w http.ResponseWriter, r *http.Request) {
	var tx domain.Transaction
	if !decode(w, r, &tx) {
		return
	}
	if tx.ID == "" {
		tx.ID = domain.NewID()
	}
	if tx.CreatedAt.IsZero() {
		tx.CreatedAt = time.Now().UTC()
	}
	s.saveTransaction(tx)
	respond(w, http.StatusCreated, domain.Envelope{Data: tx, Transaction: &tx, Lore: "The Society ledger recorded an externally generated EDI rune."})
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

func (s *store) findClaim(id string) (domain.Claim, bool) {
	if s.db != nil {
		var claim domain.Claim
		err := s.db.QueryRow(`SELECT `+claimSelectColumns+` FROM claims WHERE id = $1`, id).
			Scan(scanClaimDest(&claim)...)
		if err == nil {
			s.mu.Lock()
			s.claims[claim.ID] = claim
			s.mu.Unlock()
			return claim, true
		}
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[ASHN] postgres claim lookup failed; using memory: %v", err)
		}
	}
	s.mu.RLock()
	claim, ok := s.claims[id]
	s.mu.RUnlock()
	return claim, ok
}

func (s *store) findTransaction(id string) (domain.Transaction, bool) {
	if s.db != nil {
		var tx domain.Transaction
		err := s.db.QueryRow(`SELECT id, type, status, sender_id, receiver_id, payload, COALESCE(raw_x12, ''), COALESCE(related_id, ''), created_at FROM transactions WHERE id = $1`, id).
			Scan(&tx.ID, &tx.Type, &tx.Status, &tx.SenderID, &tx.ReceiverID, &tx.Payload, &tx.RawX12, &tx.RelatedID, &tx.CreatedAt)
		if err == nil {
			s.mu.Lock()
			s.transactions[tx.ID] = tx
			s.mu.Unlock()
			return tx, true
		}
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[ASHN] postgres transaction lookup failed; using memory: %v", err)
		}
	}
	s.mu.RLock()
	tx, ok := s.transactions[id]
	s.mu.RUnlock()
	return tx, ok
}

func (s *store) enqueueJob(jobType, entityID string, delay time.Duration) {
	if err := asyncjobs.Enqueue(s.db, jobType, entityID, delay); err != nil {
		log.Printf("[ASHN] async job enqueue failed type=%s entity=%s: %v", jobType, entityID, err)
	}
}

func transactionXML(tx domain.Transaction) string {
	payload := string(tx.Payload)
	return fmt.Sprintf(`<AshnTransactionExport id="%s" type="%s" status="%s">
  <Sender id="%s" />
  <Receiver id="%s" />
  <RelatedId>%s</RelatedId>
  <CreatedAt>%s</CreatedAt>
  <Payload><![CDATA[%s]]></Payload>
  <RawX12><![CDATA[%s]]></RawX12>
</AshnTransactionExport>
`, xmlEscape(tx.ID), xmlEscape(string(tx.Type)), xmlEscape(string(tx.Status)), xmlEscape(tx.SenderID), xmlEscape(tx.ReceiverID), xmlEscape(tx.RelatedID), tx.CreatedAt.Format(time.RFC3339), payload, tx.RawX12)
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return replacer.Replace(value)
}

func download(w http.ResponseWriter, contentType, filename string, payload []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
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
		_, err := s.db.Exec(`INSERT INTO claims (id, adventurer_id, provider_id, incident_severity, transaction_id, amount_cents, allowed_amount_cents, paid_amount_cents, patient_responsibility_cents, adjustment_amount_cents, adjustment_reason, denial_reason, status) VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9, $10, NULLIF($11, ''), NULLIF($12, ''), $13) ON CONFLICT (id) DO UPDATE SET transaction_id = EXCLUDED.transaction_id, amount_cents = EXCLUDED.amount_cents, allowed_amount_cents = EXCLUDED.allowed_amount_cents, paid_amount_cents = EXCLUDED.paid_amount_cents, patient_responsibility_cents = EXCLUDED.patient_responsibility_cents, adjustment_amount_cents = EXCLUDED.adjustment_amount_cents, adjustment_reason = EXCLUDED.adjustment_reason, denial_reason = EXCLUDED.denial_reason, status = EXCLUDED.status`,
			claim.ID, claim.AdventurerID, claim.ProviderID, claim.IncidentSeverity, claim.TransactionID, claim.AmountCents, claim.AllowedAmountCents, claim.PaidAmountCents, claim.PatientResponsibilityCents, claim.AdjustmentAmountCents, claim.AdjustmentReason, claim.DenialReason, claim.Status)
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
		_, err := s.db.Exec(`INSERT INTO transactions (id, type, status, sender_id, receiver_id, payload, raw_x12, related_id, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9) ON CONFLICT (id) DO NOTHING`,
			tx.ID, tx.Type, tx.Status, tx.SenderID, tx.ReceiverID, []byte(tx.Payload), tx.RawX12, tx.RelatedID, tx.CreatedAt)
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

func runEmbeddedWorker(db *sql.DB) {
	if db == nil {
		log.Printf("[ASHN] embedded worker disabled: database unavailable")
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	log.Printf("[ASHN] embedded tx-worker started")
	for range ticker.C {
		processed, err := asyncjobs.ProcessDue(db, 5)
		if err != nil {
			log.Printf("[ASHN] embedded tx-worker failed: %v", err)
			continue
		}
		if processed > 0 {
			log.Printf("[ASHN] embedded tx-worker processed %d job(s)", processed)
		}
	}
}

func applyMigration(db *sql.DB) {
	migrationPath := env("ASHN_MIGRATION_PATH", "infra/migrations/000001_init.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		log.Printf("[ASHN] auto migration read failed: %v", err)
		return
	}
	if _, err := db.Exec(string(migration)); err != nil {
		log.Printf("[ASHN] auto migration failed: %v", err)
		return
	}
	log.Printf("[ASHN] auto migration applied")
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
	rows, err := db.Query(`SELECT ` + claimSelectColumns + ` FROM claims`)
	if err != nil {
		log.Printf("[ASHN] postgres claim load failed: %v", err)
		return claims
	}
	defer rows.Close()
	for rows.Next() {
		var claim domain.Claim
		if err := rows.Scan(scanClaimDest(&claim)...); err != nil {
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

func scanClaimDest(claim *domain.Claim) []any {
	return []any{
		&claim.ID,
		&claim.AdventurerID,
		&claim.ProviderID,
		&claim.IncidentSeverity,
		&claim.TransactionID,
		&claim.AmountCents,
		&claim.AllowedAmountCents,
		&claim.PaidAmountCents,
		&claim.PatientResponsibilityCents,
		&claim.AdjustmentAmountCents,
		&claim.AdjustmentReason,
		&claim.DenialReason,
		&claim.Status,
	}
}

func loadTransactions(db *sql.DB) map[string]domain.Transaction {
	transactions := map[string]domain.Transaction{}
	if db == nil {
		return transactions
	}
	rows, err := db.Query(`SELECT id, type, status, sender_id, receiver_id, payload, COALESCE(raw_x12, ''), COALESCE(related_id, ''), created_at FROM transactions`)
	if err != nil {
		log.Printf("[ASHN] postgres transaction load failed: %v", err)
		return transactions
	}
	defer rows.Close()
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(&tx.ID, &tx.Type, &tx.Status, &tx.SenderID, &tx.ReceiverID, &tx.Payload, &tx.RawX12, &tx.RelatedID, &tx.CreatedAt); err != nil {
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

func (s *store) queryAdventurers(page pageRequest, filters adventurerFilters) ([]domain.Adventurer, domain.PageInfo, error) {
	clauses, args := []string{}, []any{}
	addTextFilter(&clauses, &args, "rank", filters.Rank)
	addTextFilter(&clauses, &args, "region", filters.Region)
	addTextFilter(&clauses, &args, "coverage_status", filters.CoverageStatus)
	addSearchFilter(&clauses, &args, filters.Q, "id", "name", "guild", "rank", "region", "coverage_status")
	query := `SELECT id, name, rank, guild, region, coverage_status FROM adventurers`
	query = appendWhere(query, clauses)
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, page.Limit+1, page.Offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, domain.PageInfo{}, err
	}
	defer rows.Close()
	adventurers := []domain.Adventurer{}
	for rows.Next() {
		var adventurer domain.Adventurer
		if err := rows.Scan(&adventurer.ID, &adventurer.Name, &adventurer.Rank, &adventurer.Guild, &adventurer.Region, &adventurer.CoverageStatus); err != nil {
			return nil, domain.PageInfo{}, err
		}
		adventurers = append(adventurers, adventurer)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.PageInfo{}, err
	}
	adventurers, pageInfo := trimFetchedPage(adventurers, page)
	return adventurers, pageInfo, nil
}

func (s *store) queryClaims(page pageRequest, filters claimFilters) ([]domain.Claim, domain.PageInfo, error) {
	clauses, args := []string{}, []any{}
	addTextFilter(&clauses, &args, "status", filters.Status)
	addTextFilter(&clauses, &args, "provider_id", filters.ProviderID)
	addTextFilter(&clauses, &args, "adventurer_id", filters.AdventurerID)
	addTextFilter(&clauses, &args, "incident_severity", filters.Severity)
	addSearchFilter(&clauses, &args, filters.Q, "id", "adventurer_id", "provider_id", "incident_severity", "status")
	query := `SELECT ` + claimSelectColumns + ` FROM claims`
	query = appendWhere(query, clauses)
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, page.Limit+1, page.Offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, domain.PageInfo{}, err
	}
	defer rows.Close()
	claims := []domain.Claim{}
	for rows.Next() {
		var claim domain.Claim
		if err := rows.Scan(scanClaimDest(&claim)...); err != nil {
			return nil, domain.PageInfo{}, err
		}
		claims = append(claims, claim)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.PageInfo{}, err
	}
	claims, pageInfo := trimFetchedPage(claims, page)
	return claims, pageInfo, nil
}

func (s *store) queryTransactions(page pageRequest, filters transactionFilters) ([]domain.Transaction, domain.PageInfo, error) {
	clauses, args := []string{}, []any{}
	addTextFilter(&clauses, &args, "type", filters.Type)
	addTextFilter(&clauses, &args, "status", filters.Status)
	addSearchFilter(&clauses, &args, filters.Q, "id", "type", "status", "sender_id", "receiver_id", "payload::text", "COALESCE(raw_x12, '')", "COALESCE(related_id, '')")
	query := `SELECT id, type, status, sender_id, receiver_id, payload, COALESCE(raw_x12, ''), COALESCE(related_id, ''), created_at FROM transactions`
	query = appendWhere(query, clauses)
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, page.Limit+1, page.Offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, domain.PageInfo{}, err
	}
	defer rows.Close()
	transactions := []domain.Transaction{}
	for rows.Next() {
		var transaction domain.Transaction
		if err := rows.Scan(&transaction.ID, &transaction.Type, &transaction.Status, &transaction.SenderID, &transaction.ReceiverID, &transaction.Payload, &transaction.RawX12, &transaction.RelatedID, &transaction.CreatedAt); err != nil {
			return nil, domain.PageInfo{}, err
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.PageInfo{}, err
	}
	transactions, pageInfo := trimFetchedPage(transactions, page)
	return transactions, pageInfo, nil
}

func parsePage(r *http.Request, fallback int) pageRequest {
	return pageRequest{Limit: parseLimit(r, fallback), Offset: parseOffset(r)}
}

func parseAdventurerFilters(r *http.Request) adventurerFilters {
	query := r.URL.Query()
	return adventurerFilters{
		Q:              strings.TrimSpace(query.Get("q")),
		Rank:           strings.TrimSpace(query.Get("rank")),
		Region:         strings.TrimSpace(query.Get("region")),
		CoverageStatus: strings.TrimSpace(query.Get("coverageStatus")),
	}
}

func parseClaimFilters(r *http.Request) claimFilters {
	query := r.URL.Query()
	return claimFilters{
		Q:            strings.TrimSpace(query.Get("q")),
		Status:       strings.TrimSpace(query.Get("status")),
		ProviderID:   strings.TrimSpace(query.Get("providerId")),
		AdventurerID: strings.TrimSpace(query.Get("adventurerId")),
		Severity:     strings.TrimSpace(query.Get("severity")),
	}
}

func parseTransactionFilters(r *http.Request) transactionFilters {
	query := r.URL.Query()
	return transactionFilters{
		Q:      strings.TrimSpace(query.Get("q")),
		Type:   strings.TrimSpace(query.Get("type")),
		Status: strings.TrimSpace(query.Get("status")),
	}
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

func parseOffset(r *http.Request) int {
	value := r.URL.Query().Get("offset")
	if value == "" {
		return 0
	}
	offset, err := strconv.Atoi(value)
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}

func paginate[T any](items []T, page pageRequest) ([]T, domain.PageInfo) {
	if page.Offset >= len(items) {
		return []T{}, domain.PageInfo{Limit: page.Limit, Offset: page.Offset, Count: 0, HasMore: false}
	}
	end := page.Offset + page.Limit
	hasMore := end < len(items)
	if end > len(items) {
		end = len(items)
	}
	paged := items[page.Offset:end]
	return paged, domain.PageInfo{Limit: page.Limit, Offset: page.Offset, Count: len(paged), HasMore: hasMore}
}

func trimFetchedPage[T any](items []T, page pageRequest) ([]T, domain.PageInfo) {
	hasMore := len(items) > page.Limit
	if hasMore {
		items = items[:page.Limit]
	}
	return items, domain.PageInfo{Limit: page.Limit, Offset: page.Offset, Count: len(items), HasMore: hasMore}
}

func filterAdventurers(items []domain.Adventurer, filters adventurerFilters) []domain.Adventurer {
	filtered := []domain.Adventurer{}
	for _, item := range items {
		if filters.Rank != "" && !sameFold(string(item.Rank), filters.Rank) {
			continue
		}
		if filters.Region != "" && !sameFold(string(item.Region), filters.Region) {
			continue
		}
		if filters.CoverageStatus != "" && !sameFold(string(item.CoverageStatus), filters.CoverageStatus) {
			continue
		}
		if filters.Q != "" && !containsAny(filters.Q, item.ID, item.Name, item.Guild, string(item.Rank), string(item.Region), string(item.CoverageStatus)) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterClaims(items []domain.Claim, filters claimFilters) []domain.Claim {
	filtered := []domain.Claim{}
	for _, item := range items {
		if filters.Status != "" && !sameFold(string(item.Status), filters.Status) {
			continue
		}
		if filters.ProviderID != "" && !sameFold(item.ProviderID, filters.ProviderID) {
			continue
		}
		if filters.AdventurerID != "" && !sameFold(item.AdventurerID, filters.AdventurerID) {
			continue
		}
		if filters.Severity != "" && !sameFold(string(item.IncidentSeverity), filters.Severity) {
			continue
		}
		if filters.Q != "" && !containsAny(filters.Q, item.ID, item.AdventurerID, item.ProviderID, string(item.IncidentSeverity), string(item.Status)) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterTransactions(items []domain.Transaction, filters transactionFilters) []domain.Transaction {
	filtered := []domain.Transaction{}
	for _, item := range items {
		if filters.Type != "" && !sameFold(string(item.Type), filters.Type) {
			continue
		}
		if filters.Status != "" && !sameFold(string(item.Status), filters.Status) {
			continue
		}
		if filters.Q != "" && !containsAny(filters.Q, item.ID, string(item.Type), string(item.Status), item.SenderID, item.ReceiverID, string(item.Payload)) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func addTextFilter(clauses *[]string, args *[]any, column string, value string) {
	if value == "" {
		return
	}
	*args = append(*args, value)
	*clauses = append(*clauses, fmt.Sprintf("LOWER(%s) = LOWER($%d)", column, len(*args)))
}

func addSearchFilter(clauses *[]string, args *[]any, query string, columns ...string) {
	if query == "" {
		return
	}
	*args = append(*args, "%"+query+"%")
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, fmt.Sprintf("%s ILIKE $%d", column, len(*args)))
	}
	*clauses = append(*clauses, "("+strings.Join(parts, " OR ")+")")
}

func appendWhere(query string, clauses []string) string {
	if len(clauses) == 0 {
		return query
	}
	return query + " WHERE " + strings.Join(clauses, " AND ")
}

func sameFold(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right))
}

func containsAny(query string, values ...string) bool {
	needle := strings.ToLower(strings.TrimSpace(query))
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
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
