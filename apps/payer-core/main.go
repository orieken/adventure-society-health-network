package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ashn/packages/ashnlog"
	"ashn/packages/asyncjobs"
	"ashn/packages/domain"
	edimock "ashn/packages/edi-mock"
	"ashn/packages/lore"
	"ashn/packages/openapidocs"
	"ashn/packages/requestmeta"

	_ "github.com/lib/pq"
)

const societyID = "adventure-society"
const claimSelectColumns = `id, adventurer_id, provider_id, incident_severity, COALESCE(transaction_id, ''), COALESCE(authorization_transaction_id, ''), COALESCE(authorization_status, ''), COALESCE(authorization_reason, ''), amount_cents, allowed_amount_cents, paid_amount_cents, patient_responsibility_cents, adjustment_amount_cents, COALESCE(adjustment_reason, ''), COALESCE(denial_reason, ''), status, COALESCE(service_lines, '[]'::jsonb), COALESCE(diagnoses, '[]'::jsonb)`

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
	mux.HandleFunc("GET /", openapidocs.HTMLHandler("ASHN Payer Core Docs"))
	mux.HandleFunc("GET /openapi.json", openapidocs.JSONHandler(payerOpenAPI()))
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /enrollments", app.enroll)
	mux.HandleFunc("GET /adventurers", app.listAdventurers)
	mux.HandleFunc("GET /adventurers/{id}", app.getAdventurer)
	mux.HandleFunc("POST /premium-payments", app.recordPremiumPayment)
	mux.HandleFunc("POST /eligibility/query", app.eligibility)
	mux.HandleFunc("POST /auth-requests", app.authRequest)
	mux.HandleFunc("POST /auth-requests/{id}/decision", app.decideAuthorization)
	mux.HandleFunc("POST /auth-requests/{id}/attachments", app.attachAuthorizationInformation)
	mux.HandleFunc("GET /claims", app.listClaims)
	mux.HandleFunc("POST /claims", app.submitClaim)
	mux.HandleFunc("GET /claims/{id}", app.getClaim)
	mux.HandleFunc("GET /claims/{id}/status", app.claimStatus)
	mux.HandleFunc("POST /claims/{id}/documentation-request", app.requestClaimDocumentation)
	mux.HandleFunc("POST /claims/{id}/attachments", app.attachClaimInformation)
	mux.HandleFunc("POST /claims/{id}/payment", app.payClaim)
	mux.HandleFunc("GET /transactions", app.listTransactions)
	mux.HandleFunc("POST /transactions", app.recordTransaction)
	mux.HandleFunc("GET /transactions/{id}", app.getTransaction)
	mux.HandleFunc("GET /transactions/{id}/export", app.exportTransaction)
	mux.HandleFunc("GET /transactions/{id}/document-reference", app.getTransactionDocumentReference)
	mux.HandleFunc("GET /transactions/{id}/document-reference/content", app.downloadTransactionDocumentContent)
	mux.HandleFunc("POST /transactions/{id}/replay", app.replayTransaction)
	mux.HandleFunc("POST /transactions/{id}/attachment-review", app.reviewAttachment)
	mux.HandleFunc("GET /jobs", app.listJobs)
	mux.HandleFunc("POST /jobs/{id}/replay", app.replayJob)
	addr := env("PAYER_CORE_ADDR", ":8081")
	ashnlog.Info("service_listening", "service", "payer-core", "addr", addr)
	ashnlog.Fatal("service_stopped", http.ListenAndServe(addr, requestmeta.Middleware("payer-core", logRequests(mux))), "service", "payer-core")
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
		ashnlog.Error("postgres_adventurer_list_failed_using_memory", err, "service", "payer-core")
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
	serviceType := strings.ToLower(strings.TrimSpace(req.ServiceType))
	inquiry := edimock.Generate270(adventurer, provider, serviceType)
	eligible := adventurer.CoverageStatus == domain.CoverageActive
	response := edimock.Generate271(adventurer, eligible, serviceType)
	s.saveTransaction(inquiry)
	s.saveTransaction(response)
	data := map[string]any{"eligible": eligible, "coverageStatus": adventurer.CoverageStatus, "adventurerId": adventurer.ID, "providerId": provider.ID}
	if serviceType != "" {
		data["serviceType"] = serviceType
	}
	if serviceType == "dental" || serviceType == "dental-eligibility" {
		data["dentalEligibility"] = edimock.DentalEligibility(adventurer, eligible)
	}
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
	tx := edimock.Generate278RequestWithDental(adventurer, provider, req.ServiceType, req.DentalService)
	s.saveTransaction(tx)
	s.saveAuthRequest(adventurer.ID, provider.ID, tx.ID, req.ServiceType, req.IncidentSeverity, string(domain.TxStatusPending))
	s.enqueueJob(asyncjobs.JobAuthReview, tx.ID, 2*time.Second)
	data := map[string]any{"authorizationStatus": domain.TxStatusPending, "serviceType": req.ServiceType, "incidentSeverity": req.IncidentSeverity, "review": "queued"}
	if req.DentalService != nil {
		data["dentalService"] = req.DentalService
	}
	respond(w, http.StatusAccepted, domain.Envelope{Data: data, Lore: lore.ThemeTransaction(domain.Tx278, adventurer.Name, provider.Name), Transaction: &tx})
}

func (s *store) decideAuthorization(w http.ResponseWriter, r *http.Request) {
	var req domain.AuthorizationDecisionRequest
	if !decode(w, r, &req) {
		return
	}
	transactionID := r.PathValue("id")
	decision, ok := parseAuthorizationDecision(req.Decision)
	if !ok {
		fail(w, http.StatusBadRequest, "invalid authorization decision", "The review council only accepts Approved or Denied decisions.")
		return
	}
	tx, ok := s.findTransaction(transactionID)
	if !ok {
		fail(w, http.StatusNotFound, "transaction not found", "The authorization rune is absent from the ledger.")
		return
	}
	if tx.Type != domain.Tx278 {
		fail(w, http.StatusBadRequest, "invalid authorization transaction", "Only 278 prior authorization runes can be reviewed here.")
		return
	}
	tx = edimock.WithStatus(tx, decision)
	if err := s.updateAuthorizationDecision(tx, strings.TrimSpace(req.Reason)); err != nil {
		fail(w, http.StatusInternalServerError, "authorization update failed", "The review council could not record its decision.")
		return
	}
	data := map[string]any{"authorizationStatus": decision, "transactionId": tx.ID, "reason": strings.TrimSpace(req.Reason)}
	respond(w, http.StatusOK, domain.Envelope{Data: data, Lore: fmt.Sprintf("Prior authorization %s by manual review.", strings.ToLower(string(decision))), Transaction: &tx})
}

func (s *store) attachAuthorizationInformation(w http.ResponseWriter, r *http.Request) {
	requests, ok := decodeAttachmentRequests(w, r)
	if !ok {
		return
	}
	transactionID := r.PathValue("id")
	auth, ok := s.findTransaction(transactionID)
	if !ok {
		fail(w, http.StatusNotFound, "transaction not found", "The attachment scribe could not locate that authorization rune.")
		return
	}
	if auth.Type != domain.Tx278 {
		fail(w, http.StatusBadRequest, "invalid authorization transaction", "Only 278 prior authorization runes can receive 275 attachments here.")
		return
	}
	normalizeAttachmentRequests(requests)
	if err := validateAttachmentPacketControls(requests); err != nil {
		fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
		return
	}
	if err := s.validatePriorAttachmentControls("authorizationTransactionId", auth.ID, requests); err != nil {
		fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
		return
	}
	providerID := transactionPayloadString(auth, "providerId")
	if providerID == "" {
		providerID = auth.SenderID
	}
	if err := validateAttachmentPacketLimit(providerID, requests); err != nil {
		fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
		return
	}
	txs := make([]domain.Transaction, 0, len(requests))
	for index, req := range requests {
		req = requests[index]
		if err := validateAttachmentRequest(req); err != nil {
			fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
			return
		}
		if err := validateAttachmentForProvider(providerID, req); err != nil {
			fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
			return
		}
		tx := edimock.Generate275ForAuthorization(auth, req)
		s.saveTransaction(tx)
		txs = append(txs, tx)
	}
	firstTx := firstTransaction(txs)
	data := map[string]any{
		"authorizationTransactionId": auth.ID,
		"packetId":                   packetIDFor(requests),
		"attachmentCount":            len(requests),
	}
	addAttachmentSummary(data, requests)
	respond(w, http.StatusCreated, domain.Envelope{Data: data, Lore: lore.ThemeTransaction(domain.Tx275, providerID, "Adventure Society"), Transaction: firstTx, Transactions: txs})
}

func parseAuthorizationDecision(value string) (domain.TransactionStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "approved", "approve":
		return domain.TxStatusApproved, true
	case "denied", "deny":
		return domain.TxStatusDenied, true
	default:
		return "", false
	}
}

func (s *store) submitClaim(w http.ResponseWriter, r *http.Request) {
	var req domain.ClaimRequest
	if !decode(w, r, &req) {
		return
	}
	serviceLines, amountCents, err := normalizeClaimServiceLines(req)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid service lines", err.Error())
		return
	}
	diagnoses := normalizeClaimDiagnoses(req)
	adventurer, provider, ok := s.findAdventurerProvider(w, req.AdventurerID, req.ProviderID)
	if !ok {
		return
	}
	claim := domain.Claim{
		ID: domain.NewID(), AdventurerID: adventurer.ID, ProviderID: provider.ID,
		IncidentSeverity: req.IncidentSeverity, AmountCents: amountCents, ServiceLines: serviceLines, Diagnoses: diagnoses, Status: domain.ClaimSubmitted,
	}
	if strings.TrimSpace(req.AuthorizationTransactionID) != "" {
		authStatus, authReason, ok := s.authorizationForClaim(req.AuthorizationTransactionID, adventurer.ID, provider.ID)
		if !ok {
			fail(w, http.StatusBadRequest, "invalid authorization link", "The claim references a prior authorization that does not match this adventurer and provider.")
			return
		}
		claim.AuthorizationTransactionID = strings.TrimSpace(req.AuthorizationTransactionID)
		claim.AuthorizationStatus = authStatus
		claim.AuthorizationReason = authReason
	}
	tx := edimock.Generate837(claim)
	claim.TransactionID = tx.ID
	ack := edimock.Generate277CA(claim, tx.ID, true)
	s.saveTransaction(tx)
	s.saveTransaction(ack)
	s.saveClaim(claim)
	s.enqueueJob(asyncjobs.JobClaimAdjudication, claim.ID, 2*time.Second)
	respond(w, http.StatusCreated, domain.Envelope{Data: claim, Lore: lore.ThemeTransaction(tx.Type, adventurer.Name, provider.Name), Transaction: &tx, Transactions: []domain.Transaction{tx, ack}})
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
		ashnlog.Error("postgres_claim_list_failed_using_memory", err, "service", "payer-core")
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

func (s *store) requestClaimDocumentation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	claim, ok := s.findClaim(id)
	if !ok {
		fail(w, http.StatusNotFound, "claim not found", "No claim scroll with that seal exists in the Society ledger.")
		return
	}
	documentationRequest, ok := decodeClaimDocumentationRequest(w, r)
	if !ok {
		return
	}
	claim.Status = domain.ClaimPendingDocumentation
	if err := s.updateClaimStatus(claim); err != nil {
		fail(w, http.StatusInternalServerError, "claim update failed", "The Society could not mark this claim for documentation.")
		return
	}
	request := edimock.Generate277(claim.ID, claim.Status)
	request.RelatedID = claim.TransactionID
	request.Payload = domain.Payload(map[string]any{
		"x12": "277 Claim Status Response", "claimId": claim.ID, "claimStatus": claim.Status,
		"documentationRequest": map[string]any{
			"reason":              documentationRequest.Reason,
			"dueDate":             documentationRequest.DueDate,
			"expectedTransaction": domain.Tx275,
			"attachmentTraceId":   request.ID,
			"requiredDocuments":   documentationRequest.RequiredDocuments,
		},
		"relatedId": claim.TransactionID,
	})
	s.saveTransaction(request)
	data := map[string]any{
		"claimId":               claim.ID,
		"status":                claim.Status,
		"requestedTransaction":  domain.Tx275,
		"reason":                documentationRequest.Reason,
		"dueDate":               documentationRequest.DueDate,
		"attachmentTraceId":     request.ID,
		"requiredDocuments":     documentationRequest.RequiredDocuments,
		"requiredDocumentCount": len(documentationRequest.RequiredDocuments),
	}
	respond(w, http.StatusAccepted, domain.Envelope{Data: data, Lore: "The Society requested supporting documentation before adjudication.", Transaction: &request})
}

func decodeClaimDocumentationRequest(w http.ResponseWriter, r *http.Request) (domain.ClaimDocumentationRequest, bool) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid json", "The documentation request could not be read by the Society scribe.")
		return domain.ClaimDocumentationRequest{}, false
	}
	request := defaultClaimDocumentationRequest()
	if len(strings.TrimSpace(string(body))) == 0 {
		return request, true
	}
	if err := json.Unmarshal(body, &request); err != nil {
		fail(w, http.StatusBadRequest, "invalid json", "The documentation request could not be read by the Society scribe.")
		return domain.ClaimDocumentationRequest{}, false
	}
	request.Reason = strings.TrimSpace(request.Reason)
	request.DueDate = strings.TrimSpace(request.DueDate)
	if request.Reason == "" {
		request.Reason = defaultClaimDocumentationRequest().Reason
	}
	if len(request.RequiredDocuments) == 0 {
		request.RequiredDocuments = defaultClaimDocumentationRequest().RequiredDocuments
	}
	return request, true
}

func defaultClaimDocumentationRequest() domain.ClaimDocumentationRequest {
	return domain.ClaimDocumentationRequest{
		Reason:  "Additional supporting documentation required before adjudication.",
		DueDate: time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02"),
		RequiredDocuments: []domain.DocumentationChecklistItem{
			{Code: "MED-NEC", Label: "Medical necessity letter", AttachmentType: "OZ", ReportTypeCode: "B4", ContentType: "text/plain", Required: true},
			{Code: "ENC-NOTE", Label: "Encounter notes", AttachmentType: "OZ", ReportTypeCode: "B4", ContentType: "text/plain", Required: true},
			{Code: "ITEM-BILL", Label: "Itemized bill narrative", AttachmentType: "OZ", ReportTypeCode: "B4", ContentType: "text/plain", Required: false},
		},
	}
}

func (s *store) attachClaimInformation(w http.ResponseWriter, r *http.Request) {
	requests, ok := decodeAttachmentRequests(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	claim, ok := s.findClaim(id)
	if !ok {
		fail(w, http.StatusNotFound, "claim not found", "The attachment scribe could not locate that claim.")
		return
	}
	normalizeAttachmentRequests(requests)
	if err := validateAttachmentPacketControls(requests); err != nil {
		fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
		return
	}
	if err := s.validatePriorAttachmentControls("claimId", claim.ID, requests); err != nil {
		fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
		return
	}
	if err := validateAttachmentPacketLimit(claim.ProviderID, requests); err != nil {
		fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
		return
	}
	if err := s.validateUnsolicitedAttachmentTiming(claim, requests); err != nil {
		fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
		return
	}
	txs := make([]domain.Transaction, 0, len(requests))
	for index, req := range requests {
		req = requests[index]
		if err := validateAttachmentRequest(req); err != nil {
			fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
			return
		}
		if err := validateAttachmentForProvider(claim.ProviderID, req); err != nil {
			fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
			return
		}
		if err := s.validateSolicitedAttachmentTrace(claim, req); err != nil {
			fail(w, http.StatusBadRequest, "invalid attachment", err.Error())
			return
		}
		tx := edimock.Generate275(claim, req, claim.TransactionID)
		s.saveTransaction(tx)
		txs = append(txs, tx)
	}
	claimStatus := claim.Status
	if claim.Status == domain.ClaimPendingDocumentation {
		claim.Status = domain.ClaimPending
		if err := s.updateClaimStatus(claim); err != nil {
			fail(w, http.StatusInternalServerError, "claim update failed", "The Society could not clear the documentation hold.")
			return
		}
		claimStatus = claim.Status
		s.enqueueJob(asyncjobs.JobClaimFinalization, claim.ID, 2*time.Second)
	}
	data := map[string]any{
		"claimId":         claim.ID,
		"claimStatus":     claimStatus,
		"packetId":        packetIDFor(requests),
		"attachmentCount": len(requests),
	}
	addAttachmentSummary(data, requests)
	respond(w, http.StatusCreated, domain.Envelope{Data: data, Lore: lore.ThemeTransaction(domain.Tx275, claim.ProviderID, "Adventure Society"), Transaction: firstTransaction(txs), Transactions: txs})
}

func (s *store) validateSolicitedAttachmentTrace(claim domain.Claim, req domain.AttachmentRequest) error {
	if req.AttachmentPurpose != "solicited" {
		return nil
	}
	expectedTraceID := s.latestDocumentationRequestTraceID(claim)
	if expectedTraceID == "" {
		return fmt.Errorf("solicited attachment has no matching documentation request trace")
	}
	if strings.TrimSpace(req.AttachmentTraceID) == "" {
		return fmt.Errorf("solicited attachment must include attachmentTraceId %s", expectedTraceID)
	}
	if strings.TrimSpace(req.AttachmentTraceID) != expectedTraceID {
		return fmt.Errorf("solicited attachment trace %s does not match expected %s", req.AttachmentTraceID, expectedTraceID)
	}
	return nil
}

func (s *store) latestDocumentationRequestTraceID(claim domain.Claim) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest domain.Transaction
	for _, tx := range s.transactions {
		if tx.Type != domain.Tx277 {
			continue
		}
		if tx.RelatedID != "" && tx.RelatedID != claim.TransactionID {
			continue
		}
		if !transactionDocumentsClaim(tx, claim.ID) {
			continue
		}
		if latest.ID == "" || tx.CreatedAt.After(latest.CreatedAt) {
			latest = tx
		}
	}
	return latest.ID
}

func transactionDocumentsClaim(tx domain.Transaction, claimID string) bool {
	var payload map[string]any
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		return false
	}
	if strings.TrimSpace(fmt.Sprint(payload["claimId"])) != claimID {
		return false
	}
	_, ok := payload["documentationRequest"]
	return ok
}

func normalizeAttachmentRequest(req domain.AttachmentRequest) domain.AttachmentRequest {
	req.PacketID = strings.TrimSpace(req.PacketID)
	req.AttachmentPurpose = normalizeAttachmentPurpose(req.AttachmentPurpose)
	req.AttachmentTraceID = strings.TrimSpace(req.AttachmentTraceID)
	req.AttachmentFormatCode = strings.ToUpper(strings.TrimSpace(req.AttachmentFormatCode))
	req.AttachmentObjectType = strings.ToUpper(strings.TrimSpace(req.AttachmentObjectType))
	req.AttachmentEncoding = strings.ToUpper(strings.TrimSpace(req.AttachmentEncoding))
	req.AttachmentServiceDate = strings.TrimSpace(req.AttachmentServiceDate)
	req.AttachmentType = strings.TrimSpace(req.AttachmentType)
	req.AttachmentControlNumber = strings.TrimSpace(req.AttachmentControlNumber)
	req.ReportTypeCode = strings.TrimSpace(req.ReportTypeCode)
	req.TransmissionCode = strings.TrimSpace(req.TransmissionCode)
	req.ContentType = strings.TrimSpace(req.ContentType)
	req.FileName = strings.TrimSpace(req.FileName)
	req.Description = strings.TrimSpace(req.Description)
	req.Content = strings.TrimSpace(req.Content)
	req.DocumentReferenceID = strings.TrimSpace(req.DocumentReferenceID)
	req.DocumentReferenceURL = strings.TrimSpace(req.DocumentReferenceURL)
	return req
}

func normalizeAttachmentRequests(requests []domain.AttachmentRequest) {
	for index := range requests {
		requests[index] = normalizeAttachmentRequest(requests[index])
	}
}

func normalizeAttachmentPurpose(purpose string) string {
	purpose = strings.ToLower(strings.TrimSpace(purpose))
	switch purpose {
	case "02":
		return "unsolicited"
	case "11":
		return "solicited"
	default:
		return purpose
	}
}

func decodeAttachmentRequests(w http.ResponseWriter, r *http.Request) ([]domain.AttachmentRequest, bool) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid json", "The submitted scroll could not be read by the Society scribe.")
		return nil, false
	}
	var packet domain.AttachmentPacketRequest
	if err := json.Unmarshal(body, &packet); err == nil && len(packet.Attachments) > 0 {
		return normalizeAttachmentPacket(packet), true
	}
	var req domain.AttachmentRequest
	if err := json.Unmarshal(body, &req); err != nil {
		fail(w, http.StatusBadRequest, "invalid json", "The submitted scroll could not be read by the Society scribe.")
		return nil, false
	}
	return normalizeAttachmentPacket(domain.AttachmentPacketRequest{PacketID: req.PacketID, Attachments: []domain.AttachmentRequest{req}}), true
}

func normalizeAttachmentPacket(packet domain.AttachmentPacketRequest) []domain.AttachmentRequest {
	requests := packet.Attachments
	if len(requests) == 0 {
		return nil
	}
	packetID := strings.TrimSpace(packet.PacketID)
	if packetID == "" {
		packetID = strings.TrimSpace(requests[0].PacketID)
	}
	if packetID == "" && len(requests) > 1 {
		packetID = "packet-" + domain.NewID()
	}
	for index := range requests {
		if requests[index].PacketID == "" {
			requests[index].PacketID = packetID
		}
		if len(requests) > 1 {
			if requests[index].PacketSequence == 0 {
				requests[index].PacketSequence = index + 1
			}
			if requests[index].PacketCount == 0 {
				requests[index].PacketCount = len(requests)
			}
		}
	}
	return requests
}

func validateAttachmentPacketControls(requests []domain.AttachmentRequest) error {
	seen := map[string]string{}
	for _, req := range requests {
		controlNumber := strings.TrimSpace(req.AttachmentControlNumber)
		if controlNumber == "" {
			continue
		}
		key := strings.ToUpper(controlNumber)
		if first := seen[key]; first != "" {
			return fmt.Errorf("duplicate attachment control number %s in packet", first)
		}
		seen[key] = controlNumber
	}
	return nil
}

func (s *store) validatePriorAttachmentControls(contextKey, contextValue string, requests []domain.AttachmentRequest) error {
	contextValue = strings.TrimSpace(contextValue)
	if contextValue == "" {
		return nil
	}
	controls := map[string]string{}
	for _, req := range requests {
		controlNumber := strings.TrimSpace(req.AttachmentControlNumber)
		if controlNumber != "" {
			controls[strings.ToUpper(controlNumber)] = controlNumber
		}
	}
	if len(controls) == 0 {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, tx := range s.transactions {
		if tx.Type != domain.Tx275 {
			continue
		}
		if !strings.EqualFold(transactionPayloadString(tx, contextKey), contextValue) {
			continue
		}
		existingControl := strings.TrimSpace(transactionPayloadString(tx, "attachmentControlNumber"))
		if existingControl == "" {
			continue
		}
		if requestedControl := controls[strings.ToUpper(existingControl)]; requestedControl != "" {
			return fmt.Errorf("attachment control number %s was already submitted for this %s", requestedControl, attachmentContextLabel(contextKey))
		}
	}
	return nil
}

func attachmentContextLabel(contextKey string) string {
	if contextKey == "authorizationTransactionId" {
		return "authorization"
	}
	return "claim"
}

func packetIDFor(requests []domain.AttachmentRequest) string {
	if len(requests) == 0 {
		return ""
	}
	return strings.TrimSpace(requests[0].PacketID)
}

func firstTransaction(txs []domain.Transaction) *domain.Transaction {
	if len(txs) == 0 {
		return nil
	}
	return &txs[0]
}

func addAttachmentSummary(data map[string]any, requests []domain.AttachmentRequest) {
	if len(requests) == 0 {
		return
	}
	req := requests[0]
	data["attachmentType"] = req.AttachmentType
	data["attachmentControlNumber"] = req.AttachmentControlNumber
	data["reportTypeCode"] = req.ReportTypeCode
	data["transmissionCode"] = req.TransmissionCode
	data["contentType"] = req.ContentType
	data["fileName"] = req.FileName
	data["description"] = req.Description
	data["attachmentPurpose"] = req.AttachmentPurpose
	data["attachmentTraceId"] = req.AttachmentTraceID
	data["attachmentFormatCode"] = req.AttachmentFormatCode
	data["attachmentObjectType"] = req.AttachmentObjectType
	data["attachmentEncoding"] = req.AttachmentEncoding
	data["attachmentServiceDate"] = req.AttachmentServiceDate
	data["documentReferenceId"] = req.DocumentReferenceID
	data["documentReferenceUrl"] = req.DocumentReferenceURL
}

func validateAttachmentRequest(req domain.AttachmentRequest) error {
	if req.AttachmentType == "" || req.AttachmentControlNumber == "" || req.ReportTypeCode == "" || req.TransmissionCode == "" || req.ContentType == "" || req.Description == "" {
		return fmt.Errorf("The supporting scroll is missing required patient information.")
	}
	if req.Content == "" && req.DocumentReferenceURL == "" {
		return fmt.Errorf("The supporting scroll needs embedded content or an external document reference URL.")
	}
	if req.DocumentReferenceURL != "" && !(strings.HasPrefix(req.DocumentReferenceURL, "https://") || strings.HasPrefix(req.DocumentReferenceURL, "s3://") || strings.HasPrefix(req.DocumentReferenceURL, "gs://")) {
		return fmt.Errorf("document reference URL must start with https://, s3://, or gs://")
	}
	if req.AttachmentPurpose != "" && req.AttachmentPurpose != "solicited" && req.AttachmentPurpose != "unsolicited" {
		return fmt.Errorf("attachment purpose must be solicited or unsolicited")
	}
	if err := validateAttachmentEncoding(req); err != nil {
		return err
	}
	return nil
}

func validateAttachmentEncoding(req domain.AttachmentRequest) error {
	switch req.AttachmentEncoding {
	case "":
		return nil
	case "ASC":
		if strings.TrimSpace(req.Content) == "" {
			return fmt.Errorf("ASC attachment encoding requires embedded content")
		}
		if !utf8.ValidString(req.Content) {
			return fmt.Errorf("ASC attachment content must be valid text")
		}
		for _, char := range req.Content {
			if char == '\n' || char == '\r' || char == '\t' {
				continue
			}
			if char < 0x20 {
				return fmt.Errorf("ASC attachment content contains unsupported control characters")
			}
		}
	case "B64":
		if strings.TrimSpace(req.Content) == "" {
			return fmt.Errorf("B64 attachment encoding requires embedded content")
		}
		if _, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.Content)); err != nil {
			return fmt.Errorf("B64 attachment content must be valid base64")
		}
	case "REF":
		if strings.TrimSpace(req.DocumentReferenceURL) == "" {
			return fmt.Errorf("REF attachment encoding requires documentReferenceUrl")
		}
	default:
		return fmt.Errorf("attachment encoding must be ASC, B64, or REF")
	}
	return nil
}

func transactionPayloadString(tx domain.Transaction, key string) string {
	var payload map[string]any
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

type attachmentCompanionRule struct {
	ProviderID            string
	AllowedTypes          []string
	AllowedReportTypes    []string
	AllowedTransmissions  []string
	AllowedContentTypes   []string
	AllowedExtensions     []string
	ControlPrefixes       []string
	MaxContentBytes       int
	MaxAttachments        int
	UnsolicitedWindowDays int
}

func validateAttachmentPacketLimit(providerID string, requests []domain.AttachmentRequest) error {
	rule := companionRuleForProvider(providerID)
	if rule.MaxAttachments <= 0 || len(requests) <= rule.MaxAttachments {
		return nil
	}
	return fmt.Errorf("attachment packet contains %d LX loops; provider %s allows %d", len(requests), providerID, rule.MaxAttachments)
}

func (s *store) validateUnsolicitedAttachmentTiming(claim domain.Claim, requests []domain.AttachmentRequest) error {
	if strings.TrimSpace(claim.TransactionID) == "" {
		return nil
	}
	hasUnsolicited := false
	for _, req := range requests {
		if req.AttachmentPurpose != "solicited" {
			hasUnsolicited = true
			break
		}
	}
	if !hasUnsolicited {
		return nil
	}
	claimTx, ok := s.findTransaction(claim.TransactionID)
	if !ok || claimTx.CreatedAt.IsZero() {
		return nil
	}
	rule := companionRuleForProvider(claim.ProviderID)
	windowDays := rule.UnsolicitedWindowDays
	deadline := claimTx.CreatedAt.UTC().AddDate(0, 0, windowDays+1)
	if time.Now().UTC().Before(deadline) {
		return nil
	}
	if windowDays == 0 {
		return fmt.Errorf("unsolicited 275 attachments for provider %s must be submitted on the same day as the originating 837 claim", claim.ProviderID)
	}
	return fmt.Errorf("unsolicited 275 attachments for provider %s must be submitted within %d days of the originating 837 claim", claim.ProviderID, windowDays)
}

func validateAttachmentForProvider(providerID string, req domain.AttachmentRequest) error {
	rule := companionRuleForProvider(providerID)
	if !containsCode(rule.AllowedTypes, req.AttachmentType) {
		return fmt.Errorf("attachment type %s is not allowed for provider %s; allowed: %s", req.AttachmentType, providerID, strings.Join(rule.AllowedTypes, ", "))
	}
	if !containsCode(rule.AllowedReportTypes, req.ReportTypeCode) {
		return fmt.Errorf("report type %s is not allowed for provider %s; allowed: %s", req.ReportTypeCode, providerID, strings.Join(rule.AllowedReportTypes, ", "))
	}
	if !containsCode(rule.AllowedTransmissions, req.TransmissionCode) {
		return fmt.Errorf("transmission code %s is not allowed for provider %s; allowed: %s", req.TransmissionCode, providerID, strings.Join(rule.AllowedTransmissions, ", "))
	}
	if !containsCode(rule.AllowedContentTypes, req.ContentType) {
		return fmt.Errorf("content type %s is not allowed for provider %s; allowed: %s", req.ContentType, providerID, strings.Join(rule.AllowedContentTypes, ", "))
	}
	if err := validateAttachmentFileExtension(rule, req); err != nil {
		return err
	}
	if err := validateAttachmentContentTypeMatch(req); err != nil {
		return err
	}
	if !hasPrefix(req.AttachmentControlNumber, rule.ControlPrefixes) {
		return fmt.Errorf("attachment control number must start with one of: %s", strings.Join(rule.ControlPrefixes, ", "))
	}
	if len([]byte(req.Content)) > rule.MaxContentBytes {
		return fmt.Errorf("attachment content exceeds %d byte limit for provider %s", rule.MaxContentBytes, providerID)
	}
	return nil
}

func companionRuleForProvider(providerID string) attachmentCompanionRule {
	switch providerID {
	case "provider-rimaros-hospital":
		return attachmentCompanionRule{
			ProviderID:            providerID,
			AllowedTypes:          []string{"OZ", "PN"},
			AllowedReportTypes:    []string{"03", "B4"},
			AllowedTransmissions:  []string{"EL"},
			AllowedContentTypes:   []string{"text/plain", "application/pdf"},
			AllowedExtensions:     []string{".txt", ".pdf"},
			ControlPrefixes:       []string{"RIM-", "ATTACH-", "XML-"},
			MaxContentBytes:       8192,
			MaxAttachments:        5,
			UnsolicitedWindowDays: 7,
		}
	default:
		return attachmentCompanionRule{
			ProviderID:            providerID,
			AllowedTypes:          []string{"OZ"},
			AllowedReportTypes:    []string{"B4"},
			AllowedTransmissions:  []string{"EL"},
			AllowedContentTypes:   []string{"text/plain"},
			AllowedExtensions:     []string{".txt"},
			ControlPrefixes:       []string{"TEMPLE-", "ATTACH-", "XML-"},
			MaxContentBytes:       4096,
			MaxAttachments:        3,
			UnsolicitedWindowDays: 0,
		}
	}
}

func validateAttachmentFileExtension(rule attachmentCompanionRule, req domain.AttachmentRequest) error {
	if len(rule.AllowedExtensions) == 0 {
		return nil
	}
	extension := attachmentFileExtension(req)
	if extension == "" {
		return nil
	}
	if !containsCode(rule.AllowedExtensions, extension) {
		return fmt.Errorf("attachment file extension %s is not allowed for provider %s; allowed: %s", extension, rule.ProviderID, strings.Join(rule.AllowedExtensions, ", "))
	}
	return nil
}

func validateAttachmentContentTypeMatch(req domain.AttachmentRequest) error {
	extension := attachmentFileExtension(req)
	if extension != "" {
		expected := contentTypeForExtension(extension)
		if expected != "" && !strings.EqualFold(req.ContentType, expected) {
			return fmt.Errorf("attachment content type %s does not match file extension %s; expected %s", req.ContentType, extension, expected)
		}
	}
	if req.AttachmentEncoding != "B64" || strings.TrimSpace(req.Content) == "" {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(req.Content))
	if err != nil {
		return nil
	}
	mimeText := strings.TrimSpace(string(decoded))
	if mimeText == "" || !strings.Contains(strings.ToLower(mimeText), "content-type:") {
		return nil
	}
	lowerMimeText := strings.ToLower(mimeText)
	lowerContentType := strings.ToLower(req.ContentType)
	if strings.Contains(lowerMimeText, "multipart/") || strings.Contains(lowerMimeText, "boundary=") {
		return fmt.Errorf("single-part MIME packaging is required for B64 attachments")
	}
	if !strings.Contains(lowerMimeText, "content-type: "+lowerContentType) && !strings.Contains(lowerMimeText, "content-type:"+lowerContentType) {
		return fmt.Errorf("B64 MIME content type does not match declared content type %s", req.ContentType)
	}
	return nil
}

func contentTypeForExtension(extension string) string {
	switch strings.ToLower(strings.TrimSpace(extension)) {
	case ".txt":
		return "text/plain"
	case ".pdf":
		return "application/pdf"
	default:
		return ""
	}
}

func attachmentFileExtension(req domain.AttachmentRequest) string {
	for _, candidate := range []string{req.FileName, req.DocumentReferenceURL, req.DocumentReferenceID} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if value, _, ok := strings.Cut(candidate, "?"); ok {
			candidate = value
		}
		if value, _, ok := strings.Cut(candidate, "#"); ok {
			candidate = value
		}
		if slash := strings.LastIndex(candidate, "/"); slash >= 0 {
			candidate = candidate[slash+1:]
		}
		if dot := strings.LastIndex(candidate, "."); dot >= 0 && dot < len(candidate)-1 {
			return strings.ToLower(candidate[dot:])
		}
	}
	return ""
}

func containsCode(values []string, candidate string) bool {
	for _, value := range values {
		if strings.EqualFold(value, candidate) {
			return true
		}
	}
	return false
}

func hasPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToUpper(value), strings.ToUpper(prefix)) {
			return true
		}
	}
	return false
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

func (s *store) recordPremiumPayment(w http.ResponseWriter, r *http.Request) {
	var req domain.PremiumPaymentRequest
	if !decode(w, r, &req) {
		return
	}
	req.AdventurerID = strings.TrimSpace(req.AdventurerID)
	if req.AdventurerID == "" || req.AmountCents <= 0 {
		fail(w, http.StatusBadRequest, "invalid premium payment", "The dues ledger needs an adventurer and a positive amount.")
		return
	}
	s.mu.RLock()
	adventurer, ok := s.adventurers[req.AdventurerID]
	s.mu.RUnlock()
	if !ok {
		fail(w, http.StatusNotFound, "adventurer not found", "The Society archives contain no record of that adventurer.")
		return
	}
	tx := edimock.Generate820(adventurer, req.AmountCents)
	s.saveTransaction(tx)
	s.savePremiumPayment(tx, req.AmountCents)
	respond(w, http.StatusCreated, domain.Envelope{Data: map[string]any{"adventurerId": adventurer.ID, "amountCents": req.AmountCents, "status": "Accepted"}, Lore: lore.ThemeTransaction(domain.Tx820, adventurer.Name, "Adventure Society"), Transaction: &tx})
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

func (s *store) getTransactionDocumentReference(w http.ResponseWriter, r *http.Request) {
	reference, ok := s.documentReferenceForTransaction(w, r.PathValue("id"))
	if !ok {
		return
	}
	respond(w, http.StatusOK, domain.Envelope{
		Data: reference,
		Lore: "The Society document vault resolved the 275 reference without fetching external scrolls.",
	})
}

func (s *store) downloadTransactionDocumentContent(w http.ResponseWriter, r *http.Request) {
	tx, reference, payload, ok := s.documentReferencePayload(w, r.PathValue("id"))
	if !ok {
		return
	}
	content := strings.TrimSpace(fmt.Sprint(payload["content"]))
	if content == "" {
		fail(w, http.StatusNotFound, "document content unavailable", "This 275 points to an external vault reference; ASHN will not fetch arbitrary outside scrolls.")
		return
	}
	contentType := reference.ContentType
	if contentType == "" {
		contentType = "text/plain"
	}
	filename := fmt.Sprintf("ashn-%s-document.txt", tx.ID)
	download(w, contentType+"; charset=utf-8", filename, []byte(content))
}

func (s *store) documentReferenceForTransaction(w http.ResponseWriter, transactionID string) (domain.DocumentReference, bool) {
	_, reference, _, ok := s.documentReferencePayload(w, transactionID)
	return reference, ok
}

func (s *store) documentReferencePayload(w http.ResponseWriter, transactionID string) (domain.Transaction, domain.DocumentReference, map[string]any, bool) {
	tx, ok := s.findTransaction(transactionID)
	if !ok {
		fail(w, http.StatusNotFound, "transaction not found", "The transaction rune is absent from the ledger.")
		return domain.Transaction{}, domain.DocumentReference{}, nil, false
	}
	if tx.Type != domain.Tx275 {
		fail(w, http.StatusBadRequest, "invalid attachment transaction", "Only 275 patient information runes can resolve document references.")
		return domain.Transaction{}, domain.DocumentReference{}, nil, false
	}
	var payload map[string]any
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		fail(w, http.StatusBadRequest, "invalid attachment payload", "The document vault could not read this 275 payload.")
		return domain.Transaction{}, domain.DocumentReference{}, nil, false
	}
	reference := domain.DocumentReference{
		TransactionID:              tx.ID,
		ClaimID:                    payloadStringValue(payload, "claimId"),
		AuthorizationTransactionID: payloadStringValue(payload, "authorizationTransactionId"),
		AttachmentType:             payloadStringValue(payload, "attachmentType"),
		AttachmentControlNumber:    payloadStringValue(payload, "attachmentControlNumber"),
		ReportTypeCode:             payloadStringValue(payload, "reportTypeCode"),
		ContentType:                payloadStringValue(payload, "contentType"),
		FileName:                   payloadStringValue(payload, "fileName"),
		Description:                payloadStringValue(payload, "description"),
		DocumentReferenceID:        payloadStringValue(payload, "documentReferenceId"),
		DocumentReferenceURL:       payloadStringValue(payload, "documentReferenceUrl"),
		EmbeddedContentAvailable:   strings.TrimSpace(payloadStringValue(payload, "content")) != "",
	}
	if reference.DocumentReferenceID == "" && reference.DocumentReferenceURL == "" && !reference.EmbeddedContentAvailable {
		fail(w, http.StatusNotFound, "document reference not found", "This 275 does not include a document reference or embedded document content.")
		return domain.Transaction{}, domain.DocumentReference{}, nil, false
	}
	if reference.EmbeddedContentAvailable {
		reference.RetrievalMode = "embedded"
		reference.RetrievalStatus = "available"
		reference.RetrievalInstructions = fmt.Sprintf("Download embedded content from /transactions/%s/document-reference/content.", tx.ID)
	} else {
		reference.RetrievalMode = externalReferenceMode(reference.DocumentReferenceURL)
		reference.RetrievalStatus = "external-reference"
		reference.RetrievalInstructions = "ASHN records the vault pointer for review; clients should retrieve it with their authorized document-vault credentials."
	}
	return tx, reference, payload, true
}

func payloadStringValue(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func externalReferenceMode(referenceURL string) string {
	switch {
	case strings.HasPrefix(referenceURL, "s3://"):
		return "s3"
	case strings.HasPrefix(referenceURL, "gs://"):
		return "gcs"
	case strings.HasPrefix(referenceURL, "https://"):
		return "https"
	default:
		return "external"
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
		ashnlog.Error("postgres_transaction_list_failed_using_memory", err, "service", "payer-core")
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

func (s *store) reviewAttachment(w http.ResponseWriter, r *http.Request) {
	var req domain.AttachmentReviewRequest
	if !decode(w, r, &req) {
		return
	}
	tx, ok := s.findTransaction(r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, "transaction not found", "No attachment rune with that seal exists in the ledger.")
		return
	}
	if tx.Type != domain.Tx275 {
		fail(w, http.StatusBadRequest, "invalid attachment transaction", "Only 275 patient information runes can receive attachment review outcomes.")
		return
	}
	reviewStatus, ok := parseAttachmentReviewStatus(req.Status)
	if !ok {
		fail(w, http.StatusBadRequest, "invalid attachment review status", "Attachment review accepts In Review, Accepted, or Rejected.")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	tx.Payload = mergePayload(tx.Payload, map[string]any{
		"attachmentReviewStatus": reviewStatus,
		"attachmentReviewReason": reason,
		"attachmentReviewedAt":   time.Now().UTC().Format(time.RFC3339),
	})
	if err := s.updateTransaction(tx); err != nil {
		fail(w, http.StatusInternalServerError, "attachment review update failed", "The review council could not record the attachment outcome.")
		return
	}
	data := map[string]any{"transactionId": tx.ID, "attachmentReviewStatus": reviewStatus, "reason": reason}
	respond(w, http.StatusOK, domain.Envelope{Data: data, Lore: fmt.Sprintf("Attachment review marked %s.", strings.ToLower(reviewStatus)), Transaction: &tx})
}

func (s *store) listJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := asyncjobs.List(s.db, parseLimit(r, 25))
	if err != nil {
		fail(w, http.StatusInternalServerError, "job list failed", "The worker ledger could not be opened.")
		return
	}
	respond(w, http.StatusOK, domain.Envelope{Data: jobs, Lore: "The Society inspected its async worker queue."})
}

func (s *store) replayJob(w http.ResponseWriter, r *http.Request) {
	job, err := asyncjobs.Replay(s.db, r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, "job not replayable", "Only dead-lettered worker jobs can be replayed.")
		return
	}
	respond(w, http.StatusAccepted, domain.Envelope{Data: job, Lore: "The Society returned a dead-letter job to the pending queue."})
}

func parseAttachmentReviewStatus(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "in review", "in_review", "review", "reviewing":
		return "In Review", true
	case "accepted", "accept", "approved", "approve":
		return "Accepted", true
	case "rejected", "reject", "denied", "deny":
		return "Rejected", true
	default:
		return "", false
	}
}

func mergePayload(payload json.RawMessage, updates map[string]any) json.RawMessage {
	merged := map[string]any{}
	if err := json.Unmarshal(payload, &merged); err != nil {
		merged = map[string]any{}
	}
	for key, value := range updates {
		merged[key] = value
	}
	return domain.Payload(merged)
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
			ashnlog.Error("postgres_claim_lookup_failed_using_memory", err, "service", "payer-core")
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
			ashnlog.Error("postgres_transaction_lookup_failed_using_memory", err, "service", "payer-core")
		}
	}
	s.mu.RLock()
	tx, ok := s.transactions[id]
	s.mu.RUnlock()
	return tx, ok
}

func (s *store) authorizationForClaim(transactionID, adventurerID, providerID string) (string, string, bool) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return "", "", false
	}
	tx, ok := s.findTransaction(transactionID)
	if !ok || tx.Type != domain.Tx278 {
		return "", "", false
	}
	if payloadAdventurer := transactionPayloadString(tx, "adventurerId"); payloadAdventurer != "" && payloadAdventurer != adventurerID {
		return "", "", false
	}
	if payloadProvider := transactionPayloadString(tx, "providerId"); payloadProvider != "" && payloadProvider != providerID {
		return "", "", false
	}
	reason := transactionPayloadString(tx, "authorizationReason")
	if s.db != nil {
		var status string
		err := s.db.QueryRow(
			`SELECT status FROM auth_requests WHERE transaction_id = $1 AND adventurer_id = $2 AND provider_id = $3`,
			transactionID, adventurerID, providerID,
		).Scan(&status)
		if err == nil {
			return status, reason, true
		}
		if !errors.Is(err, sql.ErrNoRows) {
			ashnlog.Error("postgres_auth_lookup_failed_using_transaction_status", err, "service", "payer-core")
		}
	}
	return string(tx.Status), reason, true
}

func (s *store) enqueueJob(jobType, entityID string, delay time.Duration) {
	if err := asyncjobs.Enqueue(s.db, jobType, entityID, delay); err != nil {
		ashnlog.Error("async_job_enqueue_failed", err, "service", "payer-core", "jobType", jobType, "entityId", entityID)
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
			ashnlog.Error("postgres_adventurer_persistence_failed", err, "service", "payer-core", "adventurerId", adventurer.ID)
		}
	}
}

func (s *store) saveClaim(claim domain.Claim) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims[claim.ID] = claim
	if s.db != nil {
		serviceLines := jsonArrayString(claim.ServiceLines)
		diagnoses := jsonArrayString(claim.Diagnoses)
		_, err := s.db.Exec(`INSERT INTO claims (id, adventurer_id, provider_id, incident_severity, transaction_id, authorization_transaction_id, authorization_status, authorization_reason, amount_cents, allowed_amount_cents, paid_amount_cents, patient_responsibility_cents, adjustment_amount_cents, adjustment_reason, denial_reason, service_lines, diagnoses, status) VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11, $12, $13, NULLIF($14, ''), NULLIF($15, ''), $16::jsonb, $17::jsonb, $18) ON CONFLICT (id) DO UPDATE SET transaction_id = EXCLUDED.transaction_id, authorization_transaction_id = EXCLUDED.authorization_transaction_id, authorization_status = EXCLUDED.authorization_status, authorization_reason = EXCLUDED.authorization_reason, amount_cents = EXCLUDED.amount_cents, allowed_amount_cents = EXCLUDED.allowed_amount_cents, paid_amount_cents = EXCLUDED.paid_amount_cents, patient_responsibility_cents = EXCLUDED.patient_responsibility_cents, adjustment_amount_cents = EXCLUDED.adjustment_amount_cents, adjustment_reason = EXCLUDED.adjustment_reason, denial_reason = EXCLUDED.denial_reason, service_lines = EXCLUDED.service_lines, diagnoses = EXCLUDED.diagnoses, status = EXCLUDED.status`,
			claim.ID, claim.AdventurerID, claim.ProviderID, claim.IncidentSeverity, claim.TransactionID, claim.AuthorizationTransactionID, claim.AuthorizationStatus, claim.AuthorizationReason, claim.AmountCents, claim.AllowedAmountCents, claim.PaidAmountCents, claim.PatientResponsibilityCents, claim.AdjustmentAmountCents, claim.AdjustmentReason, claim.DenialReason, serviceLines, diagnoses, claim.Status)
		if err != nil {
			ashnlog.Error("postgres_claim_persistence_failed", err, "service", "payer-core", "claimId", claim.ID)
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
			ashnlog.Error("postgres_transaction_persistence_failed", err, "service", "payer-core", "transactionId", tx.ID)
		}
	}
	ashnlog.Info("transaction_saved", "service", "payer-core", "transactionId", tx.ID, "type", tx.Type, "status", tx.Status, "lore", lore.ThemeTransaction(tx.Type, tx.SenderID, tx.ReceiverID))
}

func (s *store) savePremiumPayment(tx domain.Transaction, amountCents int64) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO premium_payments (id, adventurer_id, transaction_id, amount_cents, status, created_at) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING`,
		domain.NewID(), tx.SenderID, tx.ID, amountCents, string(tx.Status), tx.CreatedAt)
	if err != nil {
		ashnlog.Error("postgres_premium_payment_persistence_failed", err, "service", "payer-core", "transactionId", tx.ID)
	}
}

func (s *store) updateTransaction(tx domain.Transaction) error {
	s.mu.Lock()
	s.transactions[tx.ID] = tx
	s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	_, err := s.db.Exec(`UPDATE transactions SET status = $1, payload = $2, raw_x12 = $3, related_id = NULLIF($4, '') WHERE id = $5`,
		tx.Status, []byte(tx.Payload), tx.RawX12, tx.RelatedID, tx.ID)
	if err != nil {
		ashnlog.Error("postgres_transaction_update_failed", err, "service", "payer-core", "transactionId", tx.ID)
		return err
	}
	return nil
}

func (s *store) updateAuthorizationDecision(tx domain.Transaction, reason string) error {
	s.mu.Lock()
	s.transactions[tx.ID] = tx
	for id, claim := range s.claims {
		if claim.AuthorizationTransactionID == tx.ID {
			claim.AuthorizationStatus = string(tx.Status)
			claim.AuthorizationReason = reason
			s.claims[id] = claim
		}
	}
	s.mu.Unlock()
	if s.db == nil {
		ashnlog.Info("authorization_decision_recorded", "service", "payer-core", "transactionId", tx.ID, "decision", tx.Status, "reason", reason)
		return nil
	}
	_, err := s.db.Exec(`UPDATE auth_requests SET status = $1 WHERE transaction_id = $2`, string(tx.Status), tx.ID)
	if err != nil {
		ashnlog.Error("postgres_auth_decision_update_failed", err, "service", "payer-core", "transactionId", tx.ID)
		return err
	}
	_, err = s.db.Exec(`UPDATE transactions SET status = $1, raw_x12 = $2 WHERE id = $3 AND type = $4`, string(tx.Status), tx.RawX12, tx.ID, string(domain.Tx278))
	if err != nil {
		ashnlog.Error("postgres_auth_transaction_update_failed", err, "service", "payer-core", "transactionId", tx.ID)
		return err
	}
	if _, err = s.db.Exec(`UPDATE claims SET authorization_status = $1, authorization_reason = NULLIF($2, '') WHERE authorization_transaction_id = $3`, string(tx.Status), reason, tx.ID); err != nil {
		ashnlog.Error("postgres_linked_claim_auth_update_failed", err, "service", "payer-core", "transactionId", tx.ID)
		return err
	}
	ashnlog.Info("authorization_decision_recorded", "service", "payer-core", "transactionId", tx.ID, "decision", tx.Status, "reason", reason)
	return nil
}

func (s *store) updateClaimStatus(claim domain.Claim) error {
	s.mu.Lock()
	s.claims[claim.ID] = claim
	s.mu.Unlock()
	if s.db == nil {
		ashnlog.Info("claim_status_updated", "service", "payer-core", "claimId", claim.ID, "status", claim.Status)
		return nil
	}
	_, err := s.db.Exec(`UPDATE claims SET status = $1 WHERE id = $2`, string(claim.Status), claim.ID)
	if err != nil {
		ashnlog.Error("postgres_claim_status_update_failed", err, "service", "payer-core", "claimId", claim.ID)
		return err
	}
	ashnlog.Info("claim_status_updated", "service", "payer-core", "claimId", claim.ID, "status", claim.Status)
	return nil
}

func (s *store) saveEnrollment(adventurerID, transactionID, status string) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO enrollments (id, adventurer_id, transaction_id, status) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO NOTHING`,
		domain.NewID(), adventurerID, transactionID, status)
	if err != nil {
		ashnlog.Error("postgres_enrollment_persistence_failed", err, "service", "payer-core", "adventurerId", adventurerID, "transactionId", transactionID)
	}
}

func (s *store) saveAuthRequest(adventurerID, providerID, transactionID, serviceType string, severity domain.IncidentSeverity, status string) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO auth_requests (id, adventurer_id, provider_id, transaction_id, service_type, incident_severity, status) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (id) DO NOTHING`,
		domain.NewID(), adventurerID, providerID, transactionID, serviceType, severity, status)
	if err != nil {
		ashnlog.Error("postgres_auth_request_persistence_failed", err, "service", "payer-core", "adventurerId", adventurerID, "providerId", providerID, "transactionId", transactionID)
	}
}

func runEmbeddedWorker(db *sql.DB) {
	if db == nil {
		ashnlog.Info("embedded_worker_disabled_database_unavailable", "service", "payer-core")
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	ashnlog.Info("embedded_worker_started", "service", "payer-core")
	for range ticker.C {
		processed, err := asyncjobs.ProcessDue(db, 5)
		if err != nil {
			ashnlog.Error("embedded_worker_failed", err, "service", "payer-core")
			continue
		}
		if processed > 0 {
			ashnlog.Info("embedded_worker_processed_jobs", "service", "payer-core", "count", processed)
		}
	}
}

func applyMigration(db *sql.DB) {
	migrationPath := env("ASHN_MIGRATION_PATH", "infra/migrations/000001_init.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		ashnlog.Error("auto_migration_read_failed", err, "service", "payer-core", "path", migrationPath)
		return
	}
	if _, err := db.Exec(string(migration)); err != nil {
		ashnlog.Error("auto_migration_failed", err, "service", "payer-core", "path", migrationPath)
		return
	}
	ashnlog.Info("auto_migration_applied", "service", "payer-core", "path", migrationPath)
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
		ashnlog.Error("postgres_provider_load_failed_using_seed", err, "service", "payer-core")
		return seedProviders()
	}
	defer rows.Close()
	providers := map[string]domain.Provider{}
	for rows.Next() {
		var provider domain.Provider
		if err := rows.Scan(&provider.ID, &provider.Name, &provider.ProviderType, &provider.TierRank, &provider.Region); err != nil {
			ashnlog.Error("postgres_provider_row_skipped", err, "service", "payer-core")
			continue
		}
		providers[provider.ID] = provider
	}
	if err := rows.Err(); err != nil {
		ashnlog.Error("postgres_provider_rows_failed_using_seed", err, "service", "payer-core")
		return seedProviders()
	}
	if len(providers) == 0 {
		ashnlog.Info("postgres_provider_table_empty_using_seed", "service", "payer-core")
		return seedProviders()
	}
	ashnlog.Info("postgres_providers_loaded", "service", "payer-core", "count", len(providers))
	return providers
}

func loadAdventurers(db *sql.DB) map[string]domain.Adventurer {
	adventurers := map[string]domain.Adventurer{}
	if db == nil {
		return adventurers
	}
	rows, err := db.Query(`SELECT id, name, rank, guild, region, coverage_status FROM adventurers`)
	if err != nil {
		ashnlog.Error("postgres_adventurer_load_failed", err, "service", "payer-core")
		return adventurers
	}
	defer rows.Close()
	for rows.Next() {
		var adventurer domain.Adventurer
		if err := rows.Scan(&adventurer.ID, &adventurer.Name, &adventurer.Rank, &adventurer.Guild, &adventurer.Region, &adventurer.CoverageStatus); err != nil {
			ashnlog.Error("postgres_adventurer_row_skipped", err, "service", "payer-core")
			continue
		}
		adventurers[adventurer.ID] = adventurer
	}
	if err := rows.Err(); err != nil {
		ashnlog.Error("postgres_adventurer_rows_failed", err, "service", "payer-core")
	}
	ashnlog.Info("postgres_adventurers_loaded", "service", "payer-core", "count", len(adventurers))
	return adventurers
}

func loadClaims(db *sql.DB) map[string]domain.Claim {
	claims := map[string]domain.Claim{}
	if db == nil {
		return claims
	}
	rows, err := db.Query(`SELECT ` + claimSelectColumns + ` FROM claims`)
	if err != nil {
		ashnlog.Error("postgres_claim_load_failed", err, "service", "payer-core")
		return claims
	}
	defer rows.Close()
	for rows.Next() {
		var claim domain.Claim
		if err := rows.Scan(scanClaimDest(&claim)...); err != nil {
			ashnlog.Error("postgres_claim_row_skipped", err, "service", "payer-core")
			continue
		}
		claims[claim.ID] = claim
	}
	if err := rows.Err(); err != nil {
		ashnlog.Error("postgres_claim_rows_failed", err, "service", "payer-core")
	}
	ashnlog.Info("postgres_claims_loaded", "service", "payer-core", "count", len(claims))
	return claims
}

func scanClaimDest(claim *domain.Claim) []any {
	var serviceLinesJSON jsonScanner[domain.ClaimServiceLine]
	serviceLinesJSON.target = &claim.ServiceLines
	var diagnosesJSON jsonScanner[domain.ClaimDiagnosis]
	diagnosesJSON.target = &claim.Diagnoses
	return []any{
		&claim.ID,
		&claim.AdventurerID,
		&claim.ProviderID,
		&claim.IncidentSeverity,
		&claim.TransactionID,
		&claim.AuthorizationTransactionID,
		&claim.AuthorizationStatus,
		&claim.AuthorizationReason,
		&claim.AmountCents,
		&claim.AllowedAmountCents,
		&claim.PaidAmountCents,
		&claim.PatientResponsibilityCents,
		&claim.AdjustmentAmountCents,
		&claim.AdjustmentReason,
		&claim.DenialReason,
		&claim.Status,
		&serviceLinesJSON,
		&diagnosesJSON,
	}
}

func normalizeClaimServiceLines(req domain.ClaimRequest) ([]domain.ClaimServiceLine, int64, error) {
	if len(req.ServiceLines) == 0 {
		if req.AmountCents <= 0 {
			return nil, 0, errors.New("amountCents must be greater than zero")
		}
		return []domain.ClaimServiceLine{{
			LineNumber:    1,
			ProcedureCode: "ASHN1",
			Description:   defaultClaimLineDescription(req.IncidentSeverity),
			Units:         1,
			AmountCents:   req.AmountCents,
		}}, req.AmountCents, nil
	}
	lines := make([]domain.ClaimServiceLine, 0, len(req.ServiceLines))
	var total int64
	for index, raw := range req.ServiceLines {
		line := domain.ClaimServiceLine{
			LineNumber:    raw.LineNumber,
			ProcedureCode: strings.ToUpper(strings.TrimSpace(raw.ProcedureCode)),
			Description:   strings.TrimSpace(raw.Description),
			Units:         raw.Units,
			AmountCents:   raw.AmountCents,
			CDTCode:       strings.ToUpper(strings.TrimSpace(raw.CDTCode)),
			ToothNumber:   strings.TrimSpace(raw.ToothNumber),
			Surface:       strings.ToUpper(strings.TrimSpace(raw.Surface)),
			Quadrant:      strings.TrimSpace(raw.Quadrant),
			Orthodontic:   raw.Orthodontic,
		}
		if line.LineNumber <= 0 {
			line.LineNumber = index + 1
		}
		isDentalLine := claimServiceLineIsDental(line)
		if line.ProcedureCode == "" && isDentalLine && line.CDTCode != "" {
			line.ProcedureCode = line.CDTCode
		}
		if line.CDTCode == "" && isDentalLine && validCDTCode(line.ProcedureCode) {
			line.CDTCode = line.ProcedureCode
		}
		if line.ProcedureCode == "" {
			line.ProcedureCode = fmt.Sprintf("ASHN%d", line.LineNumber)
		}
		if isDentalLine && !validCDTCode(line.CDTCode) {
			return nil, 0, fmt.Errorf("service line %d cdtCode must start with D", line.LineNumber)
		}
		if !isDentalLine && !validProcedureCode(line.ProcedureCode) {
			return nil, 0, fmt.Errorf("service line %d procedureCode must start with ASHN", line.LineNumber)
		}
		if line.Description == "" {
			line.Description = defaultClaimLineDescription(req.IncidentSeverity)
		}
		if line.Units <= 0 {
			line.Units = 1
		}
		if line.AmountCents <= 0 {
			return nil, 0, fmt.Errorf("service line %d amountCents must be greater than zero", line.LineNumber)
		}
		total += line.AmountCents
		lines = append(lines, line)
	}
	return lines, total, nil
}

func validProcedureCode(code string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	return strings.HasPrefix(code, "ASHN") && len(code) >= 5
}

func validCDTCode(code string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	return strings.HasPrefix(code, "D") && len(code) >= 5
}

func claimServiceLineIsDental(line domain.ClaimServiceLine) bool {
	return strings.TrimSpace(line.CDTCode) != "" || strings.TrimSpace(line.ToothNumber) != "" || strings.TrimSpace(line.Surface) != "" || strings.TrimSpace(line.Quadrant) != "" || line.Orthodontic
}

func normalizeClaimDiagnoses(req domain.ClaimRequest) []domain.ClaimDiagnosis {
	if len(req.Diagnoses) == 0 {
		return []domain.ClaimDiagnosis{defaultClaimDiagnosis(req.IncidentSeverity)}
	}
	diagnoses := make([]domain.ClaimDiagnosis, 0, len(req.Diagnoses))
	hasPrimary := false
	for index, raw := range req.Diagnoses {
		diagnosis := domain.ClaimDiagnosis{
			Qualifier:   strings.ToUpper(strings.TrimSpace(raw.Qualifier)),
			Code:        strings.ToUpper(strings.TrimSpace(raw.Code)),
			Description: strings.TrimSpace(raw.Description),
			Primary:     raw.Primary,
		}
		if diagnosis.Qualifier == "" {
			diagnosis.Qualifier = "ABF"
		}
		if index == 0 && !hasPrimary && !diagnosis.Primary {
			diagnosis.Primary = true
		}
		if diagnosis.Primary {
			diagnosis.Qualifier = "ABK"
			hasPrimary = true
		}
		if diagnosis.Code == "" {
			continue
		}
		if diagnosis.Description == "" {
			diagnosis.Description = diagnosisDescription(diagnosis.Code)
		}
		diagnoses = append(diagnoses, diagnosis)
	}
	if len(diagnoses) == 0 {
		return []domain.ClaimDiagnosis{defaultClaimDiagnosis(req.IncidentSeverity)}
	}
	if !hasPrimary {
		diagnoses[0].Primary = true
		diagnoses[0].Qualifier = "ABK"
	}
	return diagnoses
}

func defaultClaimDiagnosis(severity domain.IncidentSeverity) domain.ClaimDiagnosis {
	code := diagnosisCodeForSeverity(severity)
	return domain.ClaimDiagnosis{Qualifier: "ABK", Code: code, Description: diagnosisDescription(code), Primary: true}
}

func diagnosisCodeForSeverity(severity domain.IncidentSeverity) string {
	switch severity {
	case domain.SeverityNormal:
		return "S610"
	case domain.SeverityAwakened:
		return "T509"
	case domain.SeverityDiamond:
		return "S062X9A"
	default:
		return "ASHN"
	}
}

func diagnosisDescription(code string) string {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "S610":
		return "Minor wound encounter"
	case "T509":
		return "Awakened injury stabilization"
	case "S062X9A":
		return "Catastrophic injury with loss of consciousness"
	default:
		return "ASHN diagnosis"
	}
}

func defaultClaimLineDescription(severity domain.IncidentSeverity) string {
	switch severity {
	case domain.SeverityDiamond:
		return "Catastrophic resurrection encounter"
	case domain.SeverityAwakened:
		return "Awakened injury stabilization"
	default:
		return "Guild clinic encounter"
	}
}

func jsonArrayString[T any](items []T) string {
	if items == nil {
		items = []T{}
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

type jsonScanner[T any] struct {
	target *[]T
}

func (scanner *jsonScanner[T]) Scan(value any) error {
	if scanner.target == nil {
		return nil
	}
	if value == nil {
		*scanner.target = nil
		return nil
	}
	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	default:
		return fmt.Errorf("unsupported claim service lines type %T", value)
	}
	if len(data) == 0 {
		*scanner.target = nil
		return nil
	}
	return json.Unmarshal(data, scanner.target)
}

func loadTransactions(db *sql.DB) map[string]domain.Transaction {
	transactions := map[string]domain.Transaction{}
	if db == nil {
		return transactions
	}
	rows, err := db.Query(`SELECT id, type, status, sender_id, receiver_id, payload, COALESCE(raw_x12, ''), COALESCE(related_id, ''), created_at FROM transactions`)
	if err != nil {
		ashnlog.Error("postgres_transaction_load_failed", err, "service", "payer-core")
		return transactions
	}
	defer rows.Close()
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(&tx.ID, &tx.Type, &tx.Status, &tx.SenderID, &tx.ReceiverID, &tx.Payload, &tx.RawX12, &tx.RelatedID, &tx.CreatedAt); err != nil {
			ashnlog.Error("postgres_transaction_row_skipped", err, "service", "payer-core")
			continue
		}
		transactions[tx.ID] = tx
	}
	if err := rows.Err(); err != nil {
		ashnlog.Error("postgres_transaction_rows_failed", err, "service", "payer-core")
	}
	ashnlog.Info("postgres_transactions_loaded", "service", "payer-core", "count", len(transactions))
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

func payerOpenAPI() map[string]any {
	return openapidocs.Spec(openapidocs.Service{
		Title:       "ASHN Payer Core",
		Description: "Core payer API for enrollment, eligibility, authorization, claims, transactions, exports, and replay.",
		Version:     "0.1.0",
		Paths: map[string]map[string]openapidocs.Operation{
			"/health": {"get": {Summary: "Check payer-core health", Tags: []string{"health"}}},
			"/enrollments": {
				"post": {Summary: "Create 834 enrollment", Tags: []string{"adventurers", "x12"}, RequestBody: true},
			},
			"/adventurers": {
				"get": {Summary: "List adventurers", Tags: []string{"adventurers"}},
			},
			"/adventurers/{id}": {
				"get": {Summary: "Get adventurer detail", Tags: []string{"adventurers"}},
			},
			"/premium-payments": {
				"post": {Summary: "Record 820 premium payment", Tags: []string{"premium", "x12"}, RequestBody: true},
			},
			"/eligibility/query": {
				"post": {Summary: "Run 270/271 eligibility", Tags: []string{"eligibility", "x12"}, RequestBody: true},
			},
			"/auth-requests": {
				"post": {Summary: "Submit 278 authorization", Tags: []string{"authorizations", "x12"}, RequestBody: true},
			},
			"/auth-requests/{id}/decision": {
				"post": {Summary: "Approve or deny a 278 authorization", Tags: []string{"authorizations", "x12"}, RequestBody: true},
			},
			"/auth-requests/{id}/attachments": {
				"post": {Summary: "Submit one 275 attachment or a packet for a 278 authorization", Tags: []string{"authorizations", "attachments", "x12"}, RequestBody: true},
			},
			"/claims": {
				"get":  {Summary: "List claims", Tags: []string{"claims"}},
				"post": {Summary: "Submit 837 claim", Tags: []string{"claims", "x12"}, RequestBody: true},
			},
			"/claims/{id}": {
				"get": {Summary: "Get claim detail", Tags: []string{"claims"}},
			},
			"/claims/{id}/status": {
				"get": {Summary: "Get claim status", Tags: []string{"claims"}},
			},
			"/claims/{id}/documentation-request": {
				"post": {Summary: "Request 275 supporting documentation", Tags: []string{"claims", "attachments", "x12"}, RequestBody: true},
			},
			"/claims/{id}/attachments": {
				"post": {Summary: "Submit one 275 patient information attachment or a packet", Tags: []string{"claims", "attachments", "x12"}, RequestBody: true},
			},
			"/claims/{id}/payment": {
				"post": {Summary: "Create 835 payment", Tags: []string{"claims", "x12"}, RequestBody: true},
			},
			"/transactions": {
				"get":  {Summary: "List ledger transactions", Tags: []string{"transactions"}},
				"post": {Summary: "Record transaction", Tags: []string{"transactions"}, RequestBody: true},
			},
			"/transactions/{id}": {
				"get": {Summary: "Get transaction detail", Tags: []string{"transactions"}},
			},
			"/transactions/{id}/export": {
				"get": {Summary: "Export transaction as JSON, XML, or X12", Tags: []string{"transactions", "export"}},
			},
			"/transactions/{id}/document-reference": {
				"get": {Summary: "Resolve 275 document reference metadata", Tags: []string{"transactions", "attachments"}},
			},
			"/transactions/{id}/document-reference/content": {
				"get": {Summary: "Download embedded 275 document content", Tags: []string{"transactions", "attachments", "export"}},
			},
			"/transactions/{id}/replay": {
				"post": {Summary: "Replay transaction", Tags: []string{"transactions", "replay"}},
			},
			"/transactions/{id}/attachment-review": {
				"post": {Summary: "Record 275 attachment review outcome", Tags: []string{"transactions", "attachments"}, RequestBody: true},
			},
			"/jobs": {
				"get": {Summary: "List async transaction jobs", Tags: []string{"async jobs"}},
			},
			"/jobs/{id}/replay": {
				"post": {Summary: "Replay a dead-lettered async job", Tags: []string{"async jobs", "replay"}},
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
		ashnlog.Info("database_url_missing_using_memory", "service", "payer-core")
		return nil
	}
	return openDBWith(dsn, sql.Open)
}

func openDBWith(dsn string, open func(string, string) (*sql.DB, error)) *sql.DB {
	db, err := open("postgres", dsn)
	if err != nil {
		ashnlog.Error("postgres_open_failed_using_memory", err, "service", "payer-core")
		return nil
	}
	if err := db.Ping(); err != nil {
		ashnlog.Error("postgres_ping_failed_using_memory", err, "service", "payer-core")
		_ = db.Close()
		return nil
	}
	ashnlog.Info("postgres_connected", "service", "payer-core")
	return db
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ashnlog.Request("payer-core", r)
		next.ServeHTTP(w, r)
	})
}
