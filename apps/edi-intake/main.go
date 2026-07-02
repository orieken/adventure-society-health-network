package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"ashn/packages/domain"
	edimock "ashn/packages/edi-mock"

	_ "github.com/lib/pq"
)

type intakeApp struct {
	payerURL        string
	client          *http.Client
	db              *sql.DB
	tradingPartners map[string]domain.TradingPartner
}

type inboundTransaction struct {
	XMLName              xml.Name           `xml:"AshnX12Transaction"`
	Type                 string             `xml:"type,attr"`
	Sender               party              `xml:"Sender"`
	Receiver             party              `xml:"Receiver"`
	Enrollment           *xmlEnrollment     `xml:"Enrollment"`
	EligibilityInquiry   *xmlEligibility    `xml:"EligibilityInquiry"`
	PriorAuthorization   *xmlPriorAuth      `xml:"PriorAuthorization"`
	Claim                *xmlClaim          `xml:"Claim"`
	ClaimStatusRequest   *xmlClaimStatus    `xml:"ClaimStatusRequest"`
	Payment              *xmlPayment        `xml:"Payment"`
	PremiumPayment       *xmlPremiumPayment `xml:"PremiumPayment"`
	RawUnsupportedFields []xml.Name         `xml:",any"`
}

type party struct {
	ID string `xml:"id,attr"`
}

type xmlEnrollment struct {
	Name   string `xml:"Name"`
	Rank   string `xml:"Rank"`
	Guild  string `xml:"Guild"`
	Region string `xml:"Region"`
}

type xmlEligibility struct {
	AdventurerID string `xml:"AdventurerId"`
	ProviderID   string `xml:"ProviderId"`
}

type xmlPriorAuth struct {
	AdventurerID     string `xml:"AdventurerId"`
	ProviderID       string `xml:"ProviderId"`
	ServiceType      string `xml:"ServiceType"`
	IncidentSeverity string `xml:"IncidentSeverity"`
}

type xmlClaim struct {
	AdventurerID     string `xml:"AdventurerId"`
	ProviderID       string `xml:"ProviderId"`
	IncidentSeverity string `xml:"IncidentSeverity"`
	AmountCents      string `xml:"AmountCents"`
}

type xmlClaimStatus struct {
	ClaimID string `xml:"ClaimId"`
}

type xmlPayment struct {
	ClaimID            string `xml:"ClaimId"`
	PaymentAmountCents string `xml:"PaymentAmountCents"`
}

type xmlPremiumPayment struct {
	AdventurerID string `xml:"AdventurerId"`
	AmountCents  string `xml:"AmountCents"`
}

func main() {
	db := openDB()
	app := intakeApp{payerURL: env("PAYER_CORE_URL", "http://localhost:8081"), client: http.DefaultClient, db: db, tradingPartners: loadTradingPartners(db)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /x12/xml", app.acceptXML)
	mux.HandleFunc("GET /x12/messages", app.listMessages)
	mux.HandleFunc("GET /x12/messages/{id}/export", app.exportMessage)
	mux.HandleFunc("POST /x12/messages/{id}/replay", app.replayMessage)
	mux.HandleFunc("GET /x12/trading-partners", app.listTradingPartners)
	addr := env("EDI_INTAKE_ADDR", ":8083")
	log.Printf("[ASHN] edi-intake listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, logRequests(mux)))
}

func (a intakeApp) acceptXML(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		messageID := a.auditInboundMessage(contentType, "", "", "", "rejected", "invalid xml", http.StatusBadRequest)
		a.record999(messageID, "", "", false, "invalid xml")
		fail(w, http.StatusBadRequest, "invalid xml", "The XML scroll faded before the scribe could read it.")
		return
	}
	rawXML := string(body)
	if !isXMLContent(contentType) {
		messageID := a.auditInboundMessage(contentType, "", "", rawXML, "rejected", "unsupported content type", http.StatusUnsupportedMediaType)
		a.record999(messageID, "", "", false, "unsupported content type")
		fail(w, http.StatusUnsupportedMediaType, "unsupported content type", "The intake scribe only accepts XML scrolls.")
		return
	}
	inbound, err := parseInboundXML(body)
	if err != nil {
		messageID := a.auditInboundMessage(contentType, "", "", rawXML, "rejected", "invalid xml", http.StatusBadRequest)
		a.record999(messageID, "", "", false, "invalid xml")
		fail(w, http.StatusBadRequest, "invalid xml", "The XML scroll does not match the Society intake format.")
		return
	}
	method, path, payload, err := inbound.toPayerRequest()
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not implemented") {
			status = http.StatusNotImplemented
		}
		messageID := a.auditInboundMessage(contentType, "", inbound.Type, rawXML, "rejected", err.Error(), status)
		a.record999(messageID, inbound.Type, inbound.Sender.ID, false, err.Error())
		fail(w, status, err.Error(), "The intake scribe rejected the XML scroll before it entered the ledger.")
		return
	}
	partner, err := a.validateTradingPartner(inbound)
	if err != nil {
		messageID := a.auditInboundMessage(contentType, "", inbound.Type, rawXML, "rejected", err.Error(), http.StatusBadRequest)
		a.record999(messageID, inbound.Type, inbound.Sender.ID, false, err.Error())
		fail(w, http.StatusBadRequest, err.Error(), "The trading partner seal did not match the Society routing rules.")
		return
	}
	status, forwardErr := a.forward(w, method, path, payload)
	if forwardErr != "" {
		messageID := a.auditInboundMessage(contentType, partner.ID, inbound.Type, rawXML, "rejected", forwardErr, status)
		a.record999(messageID, inbound.Type, inbound.Sender.ID, false, forwardErr)
		return
	}
	messageID := a.auditInboundMessage(contentType, partner.ID, inbound.Type, rawXML, "accepted", "", status)
	a.record999(messageID, inbound.Type, inbound.Sender.ID, true, "")
}

func parseInboundXML(body []byte) (inboundTransaction, error) {
	var inbound inboundTransaction
	decoder := xml.NewDecoder(bytes.NewReader(body))
	decoder.Strict = true
	if err := decoder.Decode(&inbound); err != nil {
		return inbound, err
	}
	if inbound.XMLName.Local != "AshnX12Transaction" {
		return inbound, fmt.Errorf("unexpected root element")
	}
	inbound.Type = strings.TrimSpace(inbound.Type)
	return inbound, nil
}

func (t inboundTransaction) toPayerRequest() (string, string, any, error) {
	switch domain.TransactionType(t.Type) {
	case domain.Tx834:
		if t.Enrollment == nil {
			return "", "", nil, fmt.Errorf("missing enrollment")
		}
		if err := requireFields(map[string]string{
			"Name":   t.Enrollment.Name,
			"Rank":   t.Enrollment.Rank,
			"Guild":  t.Enrollment.Guild,
			"Region": t.Enrollment.Region,
		}); err != nil {
			return "", "", nil, err
		}
		if !validRank(t.Enrollment.Rank) {
			return "", "", nil, fmt.Errorf("invalid field Rank")
		}
		if !validRegion(t.Enrollment.Region) {
			return "", "", nil, fmt.Errorf("invalid field Region")
		}
		return http.MethodPost, "/enrollments", domain.EnrollmentRequest{
			Name: strings.TrimSpace(t.Enrollment.Name), Rank: domain.Rank(strings.TrimSpace(t.Enrollment.Rank)),
			Guild: strings.TrimSpace(t.Enrollment.Guild), Region: domain.Region(strings.TrimSpace(t.Enrollment.Region)),
		}, nil
	case domain.Tx270:
		if t.EligibilityInquiry == nil {
			return "", "", nil, fmt.Errorf("missing eligibility inquiry")
		}
		if err := requireFields(map[string]string{"AdventurerId": t.EligibilityInquiry.AdventurerID, "ProviderId": t.EligibilityInquiry.ProviderID}); err != nil {
			return "", "", nil, err
		}
		if err := validateProviderSender(t.Sender.ID, t.EligibilityInquiry.ProviderID); err != nil {
			return "", "", nil, err
		}
		return http.MethodPost, "/eligibility/query", domain.EligibilityRequest{
			AdventurerID: strings.TrimSpace(t.EligibilityInquiry.AdventurerID),
			ProviderID:   strings.TrimSpace(t.EligibilityInquiry.ProviderID),
		}, nil
	case domain.Tx278:
		if t.PriorAuthorization == nil {
			return "", "", nil, fmt.Errorf("missing prior authorization")
		}
		if err := requireFields(map[string]string{
			"AdventurerId":     t.PriorAuthorization.AdventurerID,
			"ProviderId":       t.PriorAuthorization.ProviderID,
			"ServiceType":      t.PriorAuthorization.ServiceType,
			"IncidentSeverity": t.PriorAuthorization.IncidentSeverity,
		}); err != nil {
			return "", "", nil, err
		}
		if err := validateProviderSender(t.Sender.ID, t.PriorAuthorization.ProviderID); err != nil {
			return "", "", nil, err
		}
		if !validSeverity(t.PriorAuthorization.IncidentSeverity) {
			return "", "", nil, fmt.Errorf("invalid field IncidentSeverity")
		}
		if !validServiceType(t.PriorAuthorization.ServiceType) {
			return "", "", nil, fmt.Errorf("invalid field ServiceType")
		}
		return http.MethodPost, "/auth-requests", domain.PriorAuthRequest{
			AdventurerID: strings.TrimSpace(t.PriorAuthorization.AdventurerID), ProviderID: strings.TrimSpace(t.PriorAuthorization.ProviderID),
			ServiceType: strings.TrimSpace(t.PriorAuthorization.ServiceType), IncidentSeverity: domain.IncidentSeverity(strings.TrimSpace(t.PriorAuthorization.IncidentSeverity)),
		}, nil
	case domain.Tx837:
		if t.Claim == nil {
			return "", "", nil, fmt.Errorf("missing claim")
		}
		if err := requireFields(map[string]string{
			"AdventurerId":     t.Claim.AdventurerID,
			"ProviderId":       t.Claim.ProviderID,
			"IncidentSeverity": t.Claim.IncidentSeverity,
			"AmountCents":      t.Claim.AmountCents,
		}); err != nil {
			return "", "", nil, err
		}
		if err := validateProviderSender(t.Sender.ID, t.Claim.ProviderID); err != nil {
			return "", "", nil, err
		}
		if !validSeverity(t.Claim.IncidentSeverity) {
			return "", "", nil, fmt.Errorf("invalid field IncidentSeverity")
		}
		amountCents, err := parsePositiveInt64("AmountCents", t.Claim.AmountCents)
		if err != nil {
			return "", "", nil, err
		}
		if amountCents > 500000000 {
			return "", "", nil, fmt.Errorf("invalid field AmountCents")
		}
		return http.MethodPost, "/claims", domain.ClaimRequest{
			AdventurerID: strings.TrimSpace(t.Claim.AdventurerID), ProviderID: strings.TrimSpace(t.Claim.ProviderID),
			IncidentSeverity: domain.IncidentSeverity(strings.TrimSpace(t.Claim.IncidentSeverity)), AmountCents: amountCents,
		}, nil
	case domain.Tx276:
		if t.ClaimStatusRequest == nil {
			return "", "", nil, fmt.Errorf("missing claim status request")
		}
		if err := requireFields(map[string]string{"ClaimId": t.ClaimStatusRequest.ClaimID}); err != nil {
			return "", "", nil, err
		}
		return http.MethodGet, "/claims/" + strings.TrimSpace(t.ClaimStatusRequest.ClaimID) + "/status", nil, nil
	case domain.Tx835:
		if t.Payment == nil {
			return "", "", nil, fmt.Errorf("missing payment")
		}
		if err := requireFields(map[string]string{"ClaimId": t.Payment.ClaimID, "PaymentAmountCents": t.Payment.PaymentAmountCents}); err != nil {
			return "", "", nil, err
		}
		amountCents, err := parsePositiveInt64("PaymentAmountCents", t.Payment.PaymentAmountCents)
		if err != nil {
			return "", "", nil, err
		}
		if amountCents > 500000000 {
			return "", "", nil, fmt.Errorf("invalid field PaymentAmountCents")
		}
		return http.MethodPost, "/claims/" + strings.TrimSpace(t.Payment.ClaimID) + "/payment", domain.PaymentRequest{PaymentAmountCents: amountCents}, nil
	case domain.Tx820:
		return "", "", nil, fmt.Errorf("transaction type 820 not implemented")
	default:
		return "", "", nil, fmt.Errorf("unsupported transaction type")
	}
}

func (a intakeApp) forward(w http.ResponseWriter, method, path string, body any) (int, string) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			fail(w, http.StatusInternalServerError, "request creation failed", "The intake scribe could not translate the XML scroll.")
			return http.StatusInternalServerError, "request creation failed"
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, a.payerURL+path, reader)
	if err != nil {
		fail(w, http.StatusInternalServerError, "request creation failed", "The intake courier could not bind the payer route.")
		return http.StatusInternalServerError, "request creation failed"
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.httpClient().Do(req)
	if err != nil {
		fail(w, http.StatusBadGateway, "payer-core unavailable", "The intake courier could not reach the Adventure Society.")
		return http.StatusBadGateway, "payer-core unavailable"
	}
	defer resp.Body.Close()
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
	if resp.StatusCode >= 400 {
		return resp.StatusCode, "payer-core rejected XML-derived request"
	}
	return resp.StatusCode, ""
}

func (a intakeApp) validateTradingPartner(inbound inboundTransaction) (domain.TradingPartner, error) {
	senderID := strings.TrimSpace(inbound.Sender.ID)
	if senderID == "" {
		return domain.TradingPartner{}, fmt.Errorf("missing trading partner sender")
	}
	receiverID := strings.TrimSpace(inbound.Receiver.ID)
	if receiverID == "" {
		return domain.TradingPartner{}, fmt.Errorf("missing trading partner receiver")
	}
	partner, ok := a.partnerBySenderID(senderID)
	if !ok {
		return domain.TradingPartner{}, fmt.Errorf("unknown trading partner")
	}
	if !strings.EqualFold(partner.Status, "active") {
		return domain.TradingPartner{}, fmt.Errorf("inactive trading partner")
	}
	if !strings.EqualFold(partner.ReceiverID, receiverID) {
		return domain.TradingPartner{}, fmt.Errorf("trading partner receiver mismatch")
	}
	if !partnerAllows(partner, inbound.Type) {
		return domain.TradingPartner{}, fmt.Errorf("transaction type %s not allowed for trading partner", inbound.Type)
	}
	if !strings.EqualFold(partner.RouteTarget, "payer-core") {
		return domain.TradingPartner{}, fmt.Errorf("unsupported trading partner route")
	}
	return partner, nil
}

func (a intakeApp) partnerBySenderID(senderID string) (domain.TradingPartner, bool) {
	partners := a.tradingPartners
	if len(partners) == 0 {
		partners = seedTradingPartners()
	}
	for _, partner := range partners {
		if strings.EqualFold(partner.SenderID, senderID) {
			return partner, true
		}
	}
	return domain.TradingPartner{}, false
}

func partnerAllows(partner domain.TradingPartner, txType string) bool {
	for _, allowed := range partner.AllowedTransactionTypes {
		if strings.EqualFold(strings.TrimSpace(allowed), strings.TrimSpace(txType)) {
			return true
		}
	}
	return false
}

func (a intakeApp) auditInboundMessage(contentType, partnerID, transactionType, rawPayload, status, errorText string, downstreamStatus int) string {
	id := domain.NewID()
	if a.db == nil {
		return id
	}
	_, err := a.db.Exec(`INSERT INTO inbound_messages (id, partner_id, content_type, transaction_type, raw_payload, status, error, downstream_status) VALUES ($1, NULLIF($2, ''), $3, NULLIF($4, ''), $5, $6, NULLIF($7, ''), $8)`,
		id, partnerID, contentType, transactionType, rawPayload, status, errorText, downstreamStatus)
	if err != nil {
		log.Printf("[ASHN] postgres inbound message audit failed: %v", err)
	}
	return id
}

func (a intakeApp) listTradingPartners(w http.ResponseWriter, _ *http.Request) {
	partners := make([]domain.TradingPartner, 0, len(a.tradingPartners))
	source := a.tradingPartners
	if len(source) == 0 {
		source = seedTradingPartners()
	}
	for _, partner := range source {
		partners = append(partners, partner)
	}
	respond(w, http.StatusOK, domain.Envelope{Data: partners, Lore: "Trading partner seals and routing rules were opened for inspection."})
}

func (a intakeApp) record999(relatedID string, transactionType string, receiverID string, accepted bool, errorText string) {
	if relatedID == "" {
		return
	}
	if receiverID == "" {
		receiverID = "external-partner"
	}
	ack := edimock.Generate999(relatedID, domain.TransactionType(transactionType), "edi-intake", receiverID, accepted, errorText)
	payload, err := json.Marshal(ack)
	if err != nil {
		log.Printf("[ASHN] 999 marshal failed: %v", err)
		return
	}
	req, err := http.NewRequest(http.MethodPost, a.payerURL+"/transactions", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[ASHN] 999 request creation failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient().Do(req)
	if err != nil {
		log.Printf("[ASHN] 999 persistence failed: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Printf("[ASHN] 999 persistence rejected by payer-core: %s", resp.Status)
	}
}

func (a intakeApp) listMessages(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r, 25)
	if a.db == nil {
		pageInfo := domain.PageInfo{Limit: page.Limit, Offset: page.Offset, Count: 0, HasMore: false}
		respond(w, http.StatusOK, domain.Envelope{Data: []domain.InboundMessage{}, Lore: "The XML intake archive is not connected to a database.", Page: &pageInfo})
		return
	}
	messages, pageInfo, err := a.queryMessages(page, parseMessageFilters(r))
	if err != nil {
		log.Printf("[ASHN] postgres inbound message list failed: %v", err)
		fail(w, http.StatusInternalServerError, "message list failed", "The intake archive could not be opened.")
		return
	}
	respond(w, http.StatusOK, domain.Envelope{Data: messages, Lore: "The XML intake archive opened its scroll case.", Page: &pageInfo})
}

func (a intakeApp) exportMessage(w http.ResponseWriter, r *http.Request) {
	message, ok := a.findMessage(r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, "message not found", "The intake archive has no scroll with that seal.")
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	switch format {
	case "json":
		payload, _ := json.MarshalIndent(message, "", "  ")
		download(w, "application/json; charset=utf-8", fmt.Sprintf("ashn-xml-message-%s.json", message.ID), payload)
	default:
		download(w, "application/xml; charset=utf-8", fmt.Sprintf("ashn-xml-message-%s.xml", message.ID), []byte(message.RawPayload))
	}
}

func (a intakeApp) replayMessage(w http.ResponseWriter, r *http.Request) {
	message, ok := a.findMessage(r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, "message not found", "The intake archive has no scroll with that seal.")
		return
	}
	replayRequest := httptestRequest(http.MethodPost, "/x12/xml", message.ContentType, message.RawPayload)
	a.acceptXML(w, replayRequest)
}

func (a intakeApp) queryMessages(page pageRequest, filters messageFilters) ([]domain.InboundMessage, domain.PageInfo, error) {
	clauses, args := []string{}, []any{}
	addTextFilter(&clauses, &args, "status", filters.Status)
	addTextFilter(&clauses, &args, "transaction_type", filters.Type)
	addSearchFilter(&clauses, &args, filters.Q, "id", "COALESCE(partner_id, '')", "content_type", "COALESCE(transaction_type, '')", "raw_payload", "status", "COALESCE(error, '')")
	query := `SELECT id, COALESCE(partner_id, ''), content_type, COALESCE(transaction_type, ''), raw_payload, status, COALESCE(error, ''), COALESCE(downstream_status, 0), created_at FROM inbound_messages`
	query = appendWhere(query, clauses)
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, page.Limit+1, page.Offset)
	rows, err := a.db.Query(query, args...)
	if err != nil {
		return nil, domain.PageInfo{}, err
	}
	defer rows.Close()
	messages := []domain.InboundMessage{}
	for rows.Next() {
		var message domain.InboundMessage
		if err := rows.Scan(&message.ID, &message.PartnerID, &message.ContentType, &message.TransactionType, &message.RawPayload, &message.Status, &message.Error, &message.DownstreamStatus, &message.CreatedAt); err != nil {
			return nil, domain.PageInfo{}, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, domain.PageInfo{}, err
	}
	messages, pageInfo := trimFetchedPage(messages, page)
	return messages, pageInfo, nil
}

func (a intakeApp) findMessage(id string) (domain.InboundMessage, bool) {
	if a.db == nil {
		return domain.InboundMessage{}, false
	}
	var message domain.InboundMessage
	err := a.db.QueryRow(`SELECT id, COALESCE(partner_id, ''), content_type, COALESCE(transaction_type, ''), raw_payload, status, COALESCE(error, ''), COALESCE(downstream_status, 0), created_at FROM inbound_messages WHERE id = $1`, id).
		Scan(&message.ID, &message.PartnerID, &message.ContentType, &message.TransactionType, &message.RawPayload, &message.Status, &message.Error, &message.DownstreamStatus, &message.CreatedAt)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[ASHN] postgres inbound message lookup failed: %v", err)
		}
		return domain.InboundMessage{}, false
	}
	return message, true
}

type pageRequest struct {
	Limit  int
	Offset int
}

type messageFilters struct {
	Status string
	Type   string
	Q      string
}

func parsePage(r *http.Request, fallback int) pageRequest {
	return pageRequest{Limit: parseLimit(r, fallback), Offset: parseOffset(r)}
}

func parseMessageFilters(r *http.Request) messageFilters {
	query := r.URL.Query()
	return messageFilters{
		Status: strings.TrimSpace(query.Get("status")),
		Type:   strings.TrimSpace(query.Get("type")),
		Q:      strings.TrimSpace(query.Get("q")),
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

func trimFetchedPage[T any](items []T, page pageRequest) ([]T, domain.PageInfo) {
	hasMore := len(items) > page.Limit
	if hasMore {
		items = items[:page.Limit]
	}
	return items, domain.PageInfo{Limit: page.Limit, Offset: page.Offset, Count: len(items), HasMore: hasMore}
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

func requireFields(fields map[string]string) error {
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("missing field %s", name)
		}
	}
	return nil
}

func parsePositiveInt64(name, value string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 1 {
		return 0, fmt.Errorf("invalid field %s", name)
	}
	return parsed, nil
}

func validateProviderSender(senderID, providerID string) error {
	senderID = strings.TrimSpace(senderID)
	providerID = strings.TrimSpace(providerID)
	if senderID == "" {
		return nil
	}
	if senderID != providerID {
		return fmt.Errorf("sender must match ProviderId")
	}
	return nil
}

func validRank(value string) bool {
	switch domain.Rank(strings.TrimSpace(value)) {
	case domain.RankIron, domain.RankBronze, domain.RankSilver, domain.RankGold, domain.RankDiamond:
		return true
	default:
		return false
	}
}

func validRegion(value string) bool {
	switch domain.Region(strings.TrimSpace(value)) {
	case domain.RegionGreenstone, domain.RegionYaresh, domain.RegionRimaros, domain.RegionVitesse:
		return true
	default:
		return false
	}
}

func validSeverity(value string) bool {
	switch domain.IncidentSeverity(strings.TrimSpace(value)) {
	case domain.SeverityNormal, domain.SeverityAwakened, domain.SeverityDiamond:
		return true
	default:
		return false
	}
}

func validServiceType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "resurrection", "restoration", "curse-removal", "trauma-care":
		return true
	default:
		return false
	}
}

func isXMLContent(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return contentType == "application/xml" || contentType == "text/xml"
}

func seedTradingPartners() map[string]domain.TradingPartner {
	partners := map[string]domain.TradingPartner{}
	for _, partner := range []domain.TradingPartner{
		{ID: "tp-greenstone-guild", Name: "Greenstone Employer Guild", SenderID: "partner-greenstone", ReceiverID: "Adventure Society", AllowedTransactionTypes: []string{"834", "820"}, RouteTarget: "payer-core", Status: "active"},
		{ID: "tp-vitesse-temple", Name: "Temple of the Healer, Vitesse", SenderID: "provider-vitesse-temple", ReceiverID: "Adventure Society", AllowedTransactionTypes: []string{"270", "276", "278", "837"}, RouteTarget: "payer-core", Status: "active"},
		{ID: "tp-rimaros-hospital", Name: "Rimaros City Hospital", SenderID: "provider-rimaros-hospital", ReceiverID: "Adventure Society", AllowedTransactionTypes: []string{"270", "276", "278", "837"}, RouteTarget: "payer-core", Status: "active"},
	} {
		partners[partner.ID] = partner
	}
	return partners
}

func loadTradingPartners(db *sql.DB) map[string]domain.TradingPartner {
	if db == nil {
		return seedTradingPartners()
	}
	rows, err := db.Query(`SELECT id, name, sender_id, receiver_id, allowed_transaction_types, route_target, status FROM trading_partners ORDER BY name`)
	if err != nil {
		log.Printf("[ASHN] postgres trading partner load failed; using seed partners: %v", err)
		return seedTradingPartners()
	}
	defer rows.Close()
	partners := map[string]domain.TradingPartner{}
	for rows.Next() {
		var partner domain.TradingPartner
		var allowed string
		if err := rows.Scan(&partner.ID, &partner.Name, &partner.SenderID, &partner.ReceiverID, &allowed, &partner.RouteTarget, &partner.Status); err != nil {
			log.Printf("[ASHN] postgres trading partner row skipped: %v", err)
			continue
		}
		partner.AllowedTransactionTypes = splitCSV(allowed)
		partners[partner.ID] = partner
	}
	if err := rows.Err(); err != nil {
		log.Printf("[ASHN] postgres trading partner rows failed; using seed partners: %v", err)
		return seedTradingPartners()
	}
	if len(partners) == 0 {
		log.Printf("[ASHN] postgres trading partner table empty; using seed partners")
		return seedTradingPartners()
	}
	log.Printf("[ASHN] loaded %d trading partners from Postgres", len(partners))
	return partners
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func (a intakeApp) httpClient() *http.Client {
	if a.client != nil {
		return a.client
	}
	return http.DefaultClient
}

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func httptestRequest(method, path, contentType, body string) *http.Request {
	request, _ := http.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	return request
}

func download(w http.ResponseWriter, contentType, filename string, payload []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func fail(w http.ResponseWriter, status int, message, loreText string) {
	respond(w, status, domain.ErrorEnvelope{Error: message, Lore: loreText})
}

func health(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok", "service": "edi-intake"})
}

func openDB() *sql.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Printf("[ASHN] DATABASE_URL not set; edi-intake audit persistence disabled")
		return nil
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("[ASHN] postgres open failed; edi-intake audit persistence disabled: %v", err)
		return nil
	}
	if err := db.Ping(); err != nil {
		log.Printf("[ASHN] postgres ping failed; edi-intake audit persistence disabled: %v", err)
		_ = db.Close()
		return nil
	}
	log.Printf("[ASHN] edi-intake connected to Postgres")
	return db
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
