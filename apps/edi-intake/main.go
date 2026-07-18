package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"

	"ashn/packages/ashnlog"
	"ashn/packages/domain"
	edimock "ashn/packages/edi-mock"
	"ashn/packages/openapidocs"
	"ashn/packages/requestmeta"

	_ "github.com/lib/pq"
)

type intakeApp struct {
	payerURL        string
	client          *http.Client
	db              *sql.DB
	tradingPartners map[string]domain.TradingPartner
}

type inboundTransaction struct {
	XMLName              xml.Name             `xml:"AshnX12Transaction" json:"-"`
	Type                 string               `xml:"type,attr" json:"type"`
	Sender               party                `xml:"Sender" json:"sender"`
	Receiver             party                `xml:"Receiver" json:"receiver"`
	Enrollment           *xmlEnrollment       `xml:"Enrollment" json:"enrollment,omitempty"`
	EligibilityInquiry   *xmlEligibility      `xml:"EligibilityInquiry" json:"eligibilityInquiry,omitempty"`
	PriorAuthorization   *xmlPriorAuth        `xml:"PriorAuthorization" json:"priorAuthorization,omitempty"`
	Attachment           *xmlAttachment       `xml:"Attachment" json:"attachment,omitempty"`
	AttachmentPacket     *xmlAttachmentPacket `xml:"AttachmentPacket" json:"attachmentPacket,omitempty"`
	Claim                *xmlClaim            `xml:"Claim" json:"claim,omitempty"`
	ClaimStatusRequest   *xmlClaimStatus      `xml:"ClaimStatusRequest" json:"claimStatusRequest,omitempty"`
	Payment              *xmlPayment          `xml:"Payment" json:"payment,omitempty"`
	PremiumPayment       *xmlPremiumPayment   `xml:"PremiumPayment" json:"premiumPayment,omitempty"`
	RawUnsupportedFields []xml.Name           `xml:",any" json:"-"`
}

type party struct {
	ID string `xml:"id,attr" json:"id"`
}

type xmlEnrollment struct {
	Name   string `xml:"Name" json:"name"`
	Rank   string `xml:"Rank" json:"rank"`
	Guild  string `xml:"Guild" json:"guild"`
	Region string `xml:"Region" json:"region"`
}

type xmlEligibility struct {
	AdventurerID string `xml:"AdventurerId" json:"adventurerId"`
	ProviderID   string `xml:"ProviderId" json:"providerId"`
	ServiceType  string `xml:"ServiceType" json:"serviceType,omitempty"`
}

type xmlPriorAuth struct {
	AdventurerID     string           `xml:"AdventurerId" json:"adventurerId"`
	ProviderID       string           `xml:"ProviderId" json:"providerId"`
	ServiceType      string           `xml:"ServiceType" json:"serviceType"`
	IncidentSeverity string           `xml:"IncidentSeverity" json:"incidentSeverity"`
	DentalService    xmlDentalService `xml:"DentalService" json:"dentalService,omitempty"`
}

type xmlClaim struct {
	AdventurerID               string                `xml:"AdventurerId" json:"adventurerId"`
	ProviderID                 string                `xml:"ProviderId" json:"providerId"`
	IncidentSeverity           string                `xml:"IncidentSeverity" json:"incidentSeverity"`
	AmountCents                string                `xml:"AmountCents" json:"amountCents"`
	AuthorizationTransactionID string                `xml:"AuthorizationTransactionId" json:"authorizationTransactionId,omitempty"`
	ServiceLines               []xmlClaimServiceLine `xml:"ServiceLine" json:"serviceLines,omitempty"`
	Diagnoses                  []xmlClaimDiagnosis   `xml:"Diagnosis" json:"diagnoses,omitempty"`
}

type xmlClaimServiceLine struct {
	LineNumber    int    `xml:"lineNumber,attr" json:"lineNumber,omitempty"`
	ProcedureCode string `xml:"ProcedureCode" json:"procedureCode"`
	Description   string `xml:"Description" json:"description,omitempty"`
	Units         int    `xml:"Units" json:"units,omitempty"`
	AmountCents   string `xml:"AmountCents" json:"amountCents"`
	CDTCode       string `xml:"CDTCode" json:"cdtCode,omitempty"`
	ToothNumber   string `xml:"ToothNumber" json:"toothNumber,omitempty"`
	Surface       string `xml:"Surface" json:"surface,omitempty"`
	Quadrant      string `xml:"Quadrant" json:"quadrant,omitempty"`
	Orthodontic   bool   `xml:"Orthodontic" json:"orthodontic,omitempty"`
}

type xmlDentalService struct {
	CDTCode     string `xml:"CDTCode" json:"cdtCode,omitempty"`
	ToothNumber string `xml:"ToothNumber" json:"toothNumber,omitempty"`
	Surface     string `xml:"Surface" json:"surface,omitempty"`
	Quadrant    string `xml:"Quadrant" json:"quadrant,omitempty"`
	Orthodontic bool   `xml:"Orthodontic" json:"orthodontic,omitempty"`
}

type xmlClaimDiagnosis struct {
	Qualifier   string `xml:"qualifier,attr" json:"qualifier,omitempty"`
	Code        string `xml:"Code" json:"code"`
	Description string `xml:"Description" json:"description,omitempty"`
	Primary     bool   `xml:"primary,attr" json:"primary,omitempty"`
}

type xmlAttachment struct {
	PacketID                   string `xml:"PacketId" json:"packetId,omitempty"`
	PacketSequence             int    `xml:"PacketSequence" json:"packetSequence,omitempty"`
	PacketCount                int    `xml:"PacketCount" json:"packetCount,omitempty"`
	ClaimID                    string `xml:"ClaimId" json:"claimId,omitempty"`
	AuthorizationTransactionID string `xml:"AuthorizationTransactionId" json:"authorizationTransactionId,omitempty"`
	ProviderID                 string `xml:"ProviderId" json:"providerId"`
	AttachmentPurpose          string `xml:"AttachmentPurpose" json:"attachmentPurpose,omitempty"`
	AttachmentTraceID          string `xml:"AttachmentTraceId" json:"attachmentTraceId,omitempty"`
	AttachmentFormatCode       string `xml:"AttachmentFormatCode" json:"attachmentFormatCode,omitempty"`
	AttachmentObjectType       string `xml:"AttachmentObjectType" json:"attachmentObjectType,omitempty"`
	AttachmentEncoding         string `xml:"AttachmentEncoding" json:"attachmentEncoding,omitempty"`
	AttachmentServiceDate      string `xml:"AttachmentServiceDate" json:"attachmentServiceDate,omitempty"`
	AttachmentType             string `xml:"AttachmentType" json:"attachmentType"`
	AttachmentControlNumber    string `xml:"AttachmentControlNumber" json:"attachmentControlNumber"`
	ReportTypeCode             string `xml:"ReportTypeCode" json:"reportTypeCode"`
	TransmissionCode           string `xml:"TransmissionCode" json:"transmissionCode"`
	ContentType                string `xml:"ContentType" json:"contentType"`
	FileName                   string `xml:"FileName" json:"fileName,omitempty"`
	Description                string `xml:"Description" json:"description"`
	Content                    string `xml:"Content" json:"content,omitempty"`
	DocumentReferenceID        string `xml:"DocumentReferenceId" json:"documentReferenceId,omitempty"`
	DocumentReferenceURL       string `xml:"DocumentReferenceUrl" json:"documentReferenceUrl,omitempty"`
}

type xmlAttachmentPacket struct {
	PacketID    string          `xml:"packetId,attr" json:"packetId,omitempty"`
	Attachments []xmlAttachment `xml:"Attachment" json:"attachments"`
}

type xmlClaimStatus struct {
	ClaimID string `xml:"ClaimId" json:"claimId"`
}

type xmlPayment struct {
	ClaimID            string `xml:"ClaimId" json:"claimId"`
	PaymentAmountCents string `xml:"PaymentAmountCents" json:"paymentAmountCents"`
}

type xmlPremiumPayment struct {
	AdventurerID string `xml:"AdventurerId" json:"adventurerId"`
	AmountCents  string `xml:"AmountCents" json:"amountCents"`
}

func main() {
	db := openDB()
	app := intakeApp{payerURL: env("PAYER_CORE_URL", "http://localhost:8081"), client: http.DefaultClient, db: db, tradingPartners: loadTradingPartners(db)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", openapidocs.HTMLHandler("ASHN EDI Intake Docs"))
	mux.HandleFunc("GET /openapi.json", openapidocs.JSONHandler(ediOpenAPI()))
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /x12/transactions", app.acceptTransaction)
	mux.HandleFunc("POST /x12/xml", app.acceptTransaction)
	mux.HandleFunc("POST /x12/raw", app.acceptTransaction)
	mux.HandleFunc("POST /x12/batch", app.acceptBatch)
	mux.HandleFunc("GET /x12/messages", app.listMessages)
	mux.HandleFunc("GET /x12/messages/rejections", app.rejectionMetrics)
	mux.HandleFunc("GET /x12/messages/{id}/export", app.exportMessage)
	mux.HandleFunc("POST /x12/messages/{id}/replay", app.replayMessage)
	mux.HandleFunc("GET /x12/trading-partners", app.listTradingPartners)
	mux.HandleFunc("POST /x12/trading-partners", app.saveTradingPartner)
	mux.HandleFunc("PUT /x12/trading-partners/{id}", app.saveTradingPartner)
	mux.HandleFunc("DELETE /x12/trading-partners/{id}", app.deleteTradingPartner)
	addr := env("EDI_INTAKE_ADDR", ":8083")
	ashnlog.Info("service_listening", "service", "edi-intake", "addr", addr)
	ashnlog.Fatal("service_stopped", http.ListenAndServe(addr, requestmeta.Middleware("edi-intake", logRequests(mux))), "service", "edi-intake")
}

func (a intakeApp) acceptTransaction(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		messageID := a.auditInboundMessage(contentType, "", "", "", "rejected", "invalid payload", http.StatusBadRequest)
		a.record999(r, messageID, "", "", false, "invalid payload")
		fail(w, http.StatusBadRequest, "invalid payload", "The intake scroll faded before the scribe could read it.")
		return
	}
	rawPayload := string(body)
	inbound, err := parseInboundPayload(contentType, body)
	if errors.Is(err, errUnsupportedContentType) {
		messageID := a.auditInboundMessage(contentType, "", "", rawPayload, "rejected", "unsupported content type", http.StatusUnsupportedMediaType)
		a.record999(r, messageID, "", "", false, "unsupported content type")
		fail(w, http.StatusUnsupportedMediaType, "unsupported content type", "The intake scribe accepts canonical ASHN XML, JSON, or raw X12 scrolls.")
		return
	}
	if err != nil {
		errorText := invalidPayloadError(contentType)
		messageID := a.auditInboundMessage(contentType, "", "", rawPayload, "rejected", errorText, http.StatusBadRequest)
		a.record999(r, messageID, "", "", false, errorText)
		fail(w, http.StatusBadRequest, errorText, "The intake scroll does not match the Society canonical transaction format.")
		return
	}
	method, path, payload, err := inbound.toPayerRequest()
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not implemented") {
			status = http.StatusNotImplemented
		}
		messageID := a.auditInboundMessage(contentType, "", inbound.Type, rawPayload, "rejected", err.Error(), status)
		a.record999(r, messageID, inbound.Type, inbound.Sender.ID, false, err.Error())
		fail(w, status, err.Error(), "The intake scribe rejected the transaction scroll before it entered the ledger.")
		return
	}
	partner, err := a.validateTradingPartner(inbound)
	if err != nil {
		messageID := a.auditInboundMessage(contentType, "", inbound.Type, rawPayload, "rejected", err.Error(), http.StatusBadRequest)
		a.record999(r, messageID, inbound.Type, inbound.Sender.ID, false, err.Error())
		fail(w, http.StatusBadRequest, err.Error(), "The trading partner seal did not match the Society routing rules.")
		return
	}
	if err := validateTradingPartnerProfile(partner, inbound); err != nil {
		messageID := a.auditInboundMessage(contentType, partner.ID, inbound.Type, rawPayload, "rejected", err.Error(), http.StatusBadRequest)
		a.record999(r, messageID, inbound.Type, inbound.Sender.ID, false, err.Error())
		fail(w, http.StatusBadRequest, err.Error(), "The companion guide seal rejected the transaction scroll.")
		return
	}
	status, forwardErr := a.forward(w, r, method, path, payload)
	if forwardErr != "" {
		messageID := a.auditInboundMessage(contentType, partner.ID, inbound.Type, rawPayload, "rejected", forwardErr, status)
		a.record999(r, messageID, inbound.Type, inbound.Sender.ID, false, forwardErr)
		return
	}
	messageID := a.auditInboundMessage(contentType, partner.ID, inbound.Type, rawPayload, "accepted", "", status)
	a.record999(r, messageID, inbound.Type, inbound.Sender.ID, true, "")
}

func (a intakeApp) acceptXML(w http.ResponseWriter, r *http.Request) {
	a.acceptTransaction(w, r)
}

func (a intakeApp) acceptBatch(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		fail(w, http.StatusBadRequest, "invalid multipart payload", "The file-drop satchel could not be opened.")
		return
	}
	files := append([]*multipartFileHeader{}, multipartHeaders(r, "files")...)
	files = append(files, multipartHeaders(r, "file")...)
	if len(files) == 0 {
		fail(w, http.StatusBadRequest, "missing files", "The file-drop satchel was empty.")
		return
	}
	summary := domain.BatchIntakeSummary{Total: len(files)}
	for _, file := range files {
		result := a.processBatchFile(r, file)
		if result.Accepted {
			summary.Accepted++
		} else {
			summary.Rejected++
		}
		summary.Results = append(summary.Results, result)
	}
	status := http.StatusAccepted
	if summary.Accepted == 0 {
		status = http.StatusBadRequest
	} else if summary.Rejected > 0 {
		status = http.StatusMultiStatus
	}
	respond(w, status, domain.Envelope{Data: summary, Lore: "The intake file-drop processed its batch scrolls."})
}

type multipartFileHeader = struct {
	FileName    string
	ContentType string
	Open        func() (io.ReadCloser, error)
}

func multipartHeaders(r *http.Request, field string) []*multipartFileHeader {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil
	}
	headers := r.MultipartForm.File[field]
	files := make([]*multipartFileHeader, 0, len(headers))
	for _, header := range headers {
		fileHeader := header
		files = append(files, &multipartFileHeader{
			FileName:    fileHeader.Filename,
			ContentType: fileHeader.Header.Get("Content-Type"),
			Open: func() (io.ReadCloser, error) {
				return fileHeader.Open()
			},
		})
	}
	return files
}

func (a intakeApp) processBatchFile(parent *http.Request, file *multipartFileHeader) domain.BatchIntakeResult {
	result := domain.BatchIntakeResult{FileName: fallbackLabel(file.FileName, "unnamed"), ContentType: inferBatchContentType(file.FileName, file.ContentType)}
	reader, err := file.Open()
	if err != nil {
		result.StatusCode = http.StatusBadRequest
		result.Error = "file open failed"
		result.Lore = "The file-drop could not unroll one scroll."
		return result
	}
	defer reader.Close()
	body, err := io.ReadAll(io.LimitReader(reader, 1<<20))
	if err != nil {
		result.StatusCode = http.StatusBadRequest
		result.Error = "file read failed"
		result.Lore = "The file-drop could not read one scroll."
		return result
	}
	if inbound, err := parseInboundPayload(result.ContentType, body); err == nil {
		result.TransactionType = inbound.Type
	}
	request := httptestRequest(http.MethodPost, "/x12/transactions", result.ContentType, string(body))
	requestmeta.Propagate(parent, request)
	response := httptest.NewRecorder()
	a.acceptTransaction(response, request)
	result.StatusCode = response.Code
	result.Accepted = response.Code >= 200 && response.Code < 300
	var envelope domain.ErrorEnvelope
	if !result.Accepted && json.Unmarshal(response.Body.Bytes(), &envelope) == nil {
		result.Error = envelope.Error
		result.Lore = envelope.Lore
		return result
	}
	var accepted domain.Envelope
	if result.Accepted && json.Unmarshal(response.Body.Bytes(), &accepted) == nil {
		result.Lore = accepted.Lore
		if tx := accepted.Transaction; tx != nil {
			result.TransactionType = string(tx.Type)
		}
	}
	return result
}

func inferBatchContentType(filename, contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if contentType != "" && contentType != "application/octet-stream" {
		return contentType
	}
	name := strings.ToLower(strings.TrimSpace(filename))
	switch {
	case strings.HasSuffix(name, ".xml"):
		return "application/xml"
	case strings.HasSuffix(name, ".json"):
		return "application/json"
	case strings.HasSuffix(name, ".x12"), strings.HasSuffix(name, ".edi"), strings.HasSuffix(name, ".txt"):
		return "application/edi-x12"
	default:
		return "application/octet-stream"
	}
}

var errUnsupportedContentType = errors.New("unsupported content type")

func parseInboundPayload(contentType string, body []byte) (inboundTransaction, error) {
	switch {
	case isXMLContent(contentType):
		return parseInboundXML(body)
	case isJSONContent(contentType):
		return parseInboundJSON(body)
	case isRawX12Content(contentType):
		return parseInboundRawX12(body)
	default:
		return inboundTransaction{}, errUnsupportedContentType
	}
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

func parseInboundJSON(body []byte) (inboundTransaction, error) {
	var inbound inboundTransaction
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&inbound); err != nil {
		return inbound, err
	}
	inbound.Type = strings.TrimSpace(inbound.Type)
	if inbound.Type == "" {
		return inbound, fmt.Errorf("missing type")
	}
	return inbound, nil
}

func parseInboundRawX12(body []byte) (inboundTransaction, error) {
	segments := parseRawX12Segments(string(body))
	if len(segments) == 0 {
		return inboundTransaction{}, fmt.Errorf("missing X12 segments")
	}
	segmentMap := map[string][][]string{}
	for _, segment := range segments {
		if len(segment) == 0 {
			continue
		}
		segmentMap[segment[0]] = append(segmentMap[segment[0]], segment)
	}
	st := firstRawSegment(segmentMap, "ST")
	if len(st) < 2 || strings.TrimSpace(st[1]) == "" {
		return inboundTransaction{}, fmt.Errorf("missing ST transaction set")
	}
	inbound := inboundTransaction{
		Type:     strings.TrimSpace(st[1]),
		Sender:   party{ID: rawSenderID(segmentMap)},
		Receiver: party{ID: rawReceiverID(segmentMap)},
	}
	switch domain.TransactionType(inbound.Type) {
	case domain.Tx834:
		enrollment, err := raw834Enrollment(segmentMap)
		if err != nil {
			return inbound, err
		}
		inbound.Enrollment = &enrollment
	case domain.Tx820:
		premium, err := raw820PremiumPayment(segmentMap)
		if err != nil {
			return inbound, err
		}
		inbound.PremiumPayment = &premium
	case domain.Tx270:
		eligibility, err := raw270Eligibility(segmentMap, inbound.Sender.ID)
		if err != nil {
			return inbound, err
		}
		inbound.EligibilityInquiry = &eligibility
	case domain.Tx276:
		claimStatus, err := raw276ClaimStatus(segmentMap)
		if err != nil {
			return inbound, err
		}
		inbound.ClaimStatusRequest = &claimStatus
	case domain.Tx278:
		priorAuth, err := raw278PriorAuthorization(segmentMap, inbound.Sender.ID)
		if err != nil {
			return inbound, err
		}
		inbound.PriorAuthorization = &priorAuth
	case domain.Tx837:
		claim, err := raw837Claim(segmentMap, inbound.Sender.ID)
		if err != nil {
			return inbound, err
		}
		inbound.Claim = &claim
	case domain.Tx275:
		attachment, err := raw275Attachment(segmentMap, inbound.Sender.ID)
		if err != nil {
			return inbound, err
		}
		inbound.Attachment = &attachment
	case domain.Tx835:
		payment, err := raw835Payment(segmentMap)
		if err != nil {
			return inbound, err
		}
		inbound.Payment = &payment
	default:
		return inbound, fmt.Errorf("raw X12 transaction type %s not implemented", inbound.Type)
	}
	return inbound, nil
}

func raw834Enrollment(segmentMap map[string][][]string) (xmlEnrollment, error) {
	enrollment := xmlEnrollment{
		Name:   firstNonEmpty(rawNM1Name(segmentMap, "IL"), rawK3Value(segmentMap, "Name")),
		Rank:   rawK3Value(segmentMap, "Rank"),
		Guild:  rawK3Value(segmentMap, "Guild"),
		Region: rawK3Value(segmentMap, "Region"),
	}
	if enrollment.Name == "" {
		return xmlEnrollment{}, fmt.Errorf("missing subscriber NM1 segment")
	}
	if enrollment.Rank == "" || enrollment.Guild == "" || enrollment.Region == "" {
		return xmlEnrollment{}, fmt.Errorf("missing enrollment K3 metadata")
	}
	return enrollment, nil
}

func raw820PremiumPayment(segmentMap map[string][][]string) (xmlPremiumPayment, error) {
	premium := xmlPremiumPayment{
		AdventurerID: rawNM1ID(segmentMap, "IL"),
		AmountCents:  raw820AmountCents(segmentMap),
	}
	if premium.AdventurerID == "" {
		return xmlPremiumPayment{}, fmt.Errorf("missing subscriber NM1 segment")
	}
	if premium.AmountCents == "" {
		return xmlPremiumPayment{}, fmt.Errorf("invalid premium amount")
	}
	return premium, nil
}

func raw270Eligibility(segmentMap map[string][][]string, senderID string) (xmlEligibility, error) {
	eligibility := xmlEligibility{
		AdventurerID: rawNM1ID(segmentMap, "IL"),
		ProviderID:   firstNonEmpty(rawNM1ID(segmentMap, "1P"), rawNM1ID(segmentMap, "85"), senderID),
	}
	if rawServiceType(segmentMap) == "35" {
		eligibility.ServiceType = "dental"
	}
	if eligibility.AdventurerID == "" {
		return xmlEligibility{}, fmt.Errorf("missing subscriber NM1 segment")
	}
	if eligibility.ProviderID == "" {
		return xmlEligibility{}, fmt.Errorf("missing provider NM1 segment")
	}
	return eligibility, nil
}

func raw276ClaimStatus(segmentMap map[string][][]string) (xmlClaimStatus, error) {
	claimID := rawClaimReference(segmentMap)
	if claimID == "" {
		return xmlClaimStatus{}, fmt.Errorf("missing claim REF segment")
	}
	return xmlClaimStatus{ClaimID: claimID}, nil
}

func rawClaimReference(segmentMap map[string][][]string) string {
	for _, ref := range segmentMap["REF"] {
		if len(ref) >= 3 && strings.EqualFold(strings.TrimSpace(ref[1]), "1K") {
			return strings.TrimSpace(ref[2])
		}
	}
	return ""
}

func rawServiceType(segmentMap map[string][][]string) string {
	if eq := firstRawSegment(segmentMap, "EQ"); len(eq) > 1 {
		return strings.TrimSpace(eq[1])
	}
	return ""
}

func raw278PriorAuthorization(segmentMap map[string][][]string, senderID string) (xmlPriorAuth, error) {
	priorAuth := xmlPriorAuth{
		AdventurerID:     rawNM1ID(segmentMap, "IL"),
		ProviderID:       firstNonEmpty(rawNM1ID(segmentMap, "1P"), senderID),
		ServiceType:      raw278ServiceType(segmentMap),
		IncidentSeverity: rawSeverity(segmentMap),
	}
	if priorAuth.AdventurerID == "" {
		return xmlPriorAuth{}, fmt.Errorf("missing subscriber NM1 segment")
	}
	if priorAuth.ProviderID == "" {
		return xmlPriorAuth{}, fmt.Errorf("missing provider NM1 segment")
	}
	if priorAuth.ServiceType == "" {
		return xmlPriorAuth{}, fmt.Errorf("missing UM service type")
	}
	return priorAuth, nil
}

func raw278ServiceType(segmentMap map[string][][]string) string {
	if um := firstRawSegment(segmentMap, "UM"); len(um) > 6 && strings.TrimSpace(um[6]) != "" {
		return strings.TrimSpace(um[6])
	}
	if sv1 := firstRawSegment(segmentMap, "SV1"); len(sv1) > 1 {
		return rawProcedureCode(sv1[1])
	}
	return ""
}

func parseRawX12Segments(raw string) [][]string {
	parts := strings.Split(raw, "~")
	segments := make([][]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ReplaceAll(part, "\n", ""))
		if part == "" {
			continue
		}
		elements := strings.Split(part, "*")
		for index := range elements {
			elements[index] = strings.TrimSpace(elements[index])
		}
		segments = append(segments, elements)
	}
	return segments
}

func firstRawSegment(segmentMap map[string][][]string, id string) []string {
	segments := segmentMap[id]
	if len(segments) == 0 {
		return nil
	}
	return segments[0]
}

func rawSenderID(segmentMap map[string][][]string) string {
	if gs := firstRawSegment(segmentMap, "GS"); len(gs) > 2 {
		return strings.TrimSpace(gs[2])
	}
	if isa := firstRawSegment(segmentMap, "ISA"); len(isa) > 6 {
		return strings.TrimSpace(isa[6])
	}
	return ""
}

func rawReceiverID(segmentMap map[string][][]string) string {
	if gs := firstRawSegment(segmentMap, "GS"); len(gs) > 3 {
		return strings.TrimSpace(gs[3])
	}
	if isa := firstRawSegment(segmentMap, "ISA"); len(isa) > 8 {
		return strings.TrimSpace(isa[8])
	}
	return ""
}

func raw837Claim(segmentMap map[string][][]string, senderID string) (xmlClaim, error) {
	clm := firstRawSegment(segmentMap, "CLM")
	if len(clm) < 3 {
		return xmlClaim{}, fmt.Errorf("missing CLM claim segment")
	}
	claim := xmlClaim{
		ProviderID:       firstNonEmpty(rawNM1ID(segmentMap, "85"), rawNM1ID(segmentMap, "41"), senderID),
		AdventurerID:     rawNM1ID(segmentMap, "IL"),
		IncidentSeverity: rawSeverity(segmentMap),
		AmountCents:      rawAmountCents(clm[2]),
		ServiceLines:     raw837ServiceLines(segmentMap),
		Diagnoses:        raw837Diagnoses(segmentMap),
	}
	if claim.AdventurerID == "" {
		return xmlClaim{}, fmt.Errorf("missing subscriber NM1 segment")
	}
	if claim.ProviderID == "" {
		return xmlClaim{}, fmt.Errorf("missing provider NM1 segment")
	}
	if claim.AmountCents == "" {
		return xmlClaim{}, fmt.Errorf("invalid CLM amount")
	}
	if claim.IncidentSeverity == "" {
		claim.IncidentSeverity = string(domain.SeverityNormal)
	}
	return claim, nil
}

func raw837ServiceLines(segmentMap map[string][][]string) []xmlClaimServiceLine {
	serviceLines := []xmlClaimServiceLine{}
	for index, sv1 := range segmentMap["SV1"] {
		if len(sv1) < 3 {
			continue
		}
		amountCents := rawAmountCents(sv1[2])
		if amountCents == "" {
			continue
		}
		line := xmlClaimServiceLine{
			LineNumber:    index + 1,
			ProcedureCode: rawProcedureCode(sv1[1]),
			Description:   "Raw X12 service line",
			Units:         rawServiceUnits(sv1),
			AmountCents:   amountCents,
		}
		if len(sv1) > 7 {
			if parsed, err := strconv.Atoi(strings.TrimSpace(sv1[7])); err == nil && parsed > 0 {
				line.LineNumber = parsed
			}
		}
		serviceLines = append(serviceLines, line)
	}
	return serviceLines
}

func raw837Diagnoses(segmentMap map[string][][]string) []xmlClaimDiagnosis {
	diagnoses := []xmlClaimDiagnosis{}
	for _, hi := range segmentMap["HI"] {
		for _, rawElement := range hi[1:] {
			parts := strings.SplitN(rawElement, ":", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
				continue
			}
			qualifier := strings.TrimSpace(parts[0])
			diagnoses = append(diagnoses, xmlClaimDiagnosis{
				Qualifier: qualifier,
				Code:      strings.TrimSpace(parts[1]),
				Primary:   strings.EqualFold(qualifier, "ABK") || len(diagnoses) == 0,
			})
		}
	}
	return diagnoses
}

func rawProcedureCode(composite string) string {
	parts := strings.Split(composite, ":")
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(composite)
}

func rawServiceUnits(segment []string) int {
	if len(segment) > 4 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(segment[4])); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 1
}

func raw275Attachment(segmentMap map[string][][]string, senderID string) (xmlAttachment, error) {
	attachment := xmlAttachment{
		ProviderID:       firstNonEmpty(rawNM1ID(segmentMap, "1P"), senderID),
		ContentType:      "text/plain",
		Description:      "Raw X12 patient information attachment",
		TransmissionCode: "EL",
	}
	if bgn := firstRawSegment(segmentMap, "BGN"); len(bgn) > 2 {
		attachment.AttachmentPurpose = attachmentPurposeFromBGN01(bgn[1])
		attachment.AttachmentTraceID = strings.TrimSpace(bgn[2])
	}
	for _, ref := range segmentMap["REF"] {
		if len(ref) < 3 {
			continue
		}
		switch strings.TrimSpace(ref[1]) {
		case "1K":
			attachment.ClaimID = strings.TrimSpace(ref[2])
		case "G1":
			attachment.AuthorizationTransactionID = strings.TrimSpace(ref[2])
		case "6R":
			if attachment.AttachmentControlNumber == "" {
				attachment.AttachmentControlNumber = strings.TrimSpace(ref[2])
			}
		case "F8":
			attachment.PacketID, attachment.PacketSequence, attachment.PacketCount = rawPacketReference(ref[2])
		}
	}
	if pwk := firstRawSegment(segmentMap, "PWK"); len(pwk) > 2 {
		attachment.ReportTypeCode = strings.TrimSpace(pwk[1])
		attachment.TransmissionCode = strings.TrimSpace(pwk[2])
		if len(pwk) > 6 && attachment.AttachmentControlNumber == "" {
			attachment.AttachmentControlNumber = strings.TrimSpace(pwk[6])
		}
	}
	if dtp := firstRawSegment(segmentMap, "DTP"); len(dtp) > 3 && strings.TrimSpace(dtp[1]) == "472" {
		attachment.AttachmentServiceDate = rawDate(dtp[3])
	}
	if cat := firstRawSegment(segmentMap, "CAT"); len(cat) > 2 {
		if attachment.ReportTypeCode == "" {
			attachment.ReportTypeCode = strings.TrimSpace(cat[1])
		}
		attachment.AttachmentFormatCode = strings.TrimSpace(cat[2])
	}
	if ooi := firstRawSegment(segmentMap, "OOI"); len(ooi) > 1 {
		attachment.AttachmentObjectType = strings.TrimSpace(ooi[1])
	}
	if bds := firstRawSegment(segmentMap, "BDS"); len(bds) > 1 {
		attachment.AttachmentEncoding = strings.TrimSpace(bds[1])
		if len(bds) > 3 && attachment.ContentType == "" {
			applyRawK3(&attachment, bds[3])
		}
	}
	if lq := firstRawSegment(segmentMap, "LQ"); len(lq) > 2 {
		attachment.AttachmentType = strings.TrimSpace(lq[2])
	}
	if nte := firstRawSegment(segmentMap, "NTE"); len(nte) > 2 {
		attachment.Description = strings.TrimSpace(nte[2])
	}
	if k3 := firstRawSegment(segmentMap, "K3"); len(k3) > 1 {
		applyRawK3(&attachment, k3[1])
	}
	if bin := firstRawSegment(segmentMap, "BIN"); len(bin) > 2 {
		attachment.Content = strings.TrimSpace(bin[2])
	}
	if attachment.DocumentReferenceURL == "" && attachment.Content == "" {
		attachment.Content = "Raw X12 attachment metadata only."
	}
	if attachment.ProviderID == "" {
		return xmlAttachment{}, fmt.Errorf("missing provider NM1 segment")
	}
	return attachment, nil
}

func raw835Payment(segmentMap map[string][][]string) (xmlPayment, error) {
	payment := xmlPayment{}
	if clp := firstRawSegment(segmentMap, "CLP"); len(clp) > 4 {
		payment.ClaimID = strings.TrimSpace(clp[1])
		payment.PaymentAmountCents = rawAmountCents(clp[4])
	}
	if payment.PaymentAmountCents == "" {
		payment.PaymentAmountCents = raw835BPRAmountCents(segmentMap)
	}
	if payment.ClaimID == "" {
		return xmlPayment{}, fmt.Errorf("missing CLP claim segment")
	}
	if payment.PaymentAmountCents == "" {
		return xmlPayment{}, fmt.Errorf("invalid payment amount")
	}
	return payment, nil
}

func rawNM1ID(segmentMap map[string][][]string, entityCode string) string {
	for _, nm1 := range segmentMap["NM1"] {
		if len(nm1) < 4 || !strings.EqualFold(nm1[1], entityCode) {
			continue
		}
		if len(nm1) > 9 && strings.TrimSpace(nm1[9]) != "" {
			return strings.TrimSpace(nm1[9])
		}
		return strings.TrimSpace(nm1[3])
	}
	return ""
}

func rawNM1Name(segmentMap map[string][][]string, entityCode string) string {
	for _, nm1 := range segmentMap["NM1"] {
		if len(nm1) >= 4 && strings.EqualFold(nm1[1], entityCode) {
			return strings.TrimSpace(nm1[3])
		}
	}
	return ""
}

func rawK3Value(segmentMap map[string][][]string, key string) string {
	prefix := strings.ToLower(strings.TrimSpace(key)) + ":"
	for _, k3 := range segmentMap["K3"] {
		if len(k3) < 2 {
			continue
		}
		value := strings.TrimSpace(k3[1])
		if strings.HasPrefix(strings.ToLower(value), prefix) {
			return strings.TrimSpace(value[len(prefix):])
		}
	}
	return ""
}

func rawSeverity(segmentMap map[string][][]string) string {
	for _, hi := range segmentMap["HI"] {
		for _, element := range hi[1:] {
			parts := strings.SplitN(element, ":", 2)
			if len(parts) != 2 {
				continue
			}
			switch strings.TrimSpace(parts[1]) {
			case "S610":
				return string(domain.SeverityNormal)
			case "T509":
				return string(domain.SeverityAwakened)
			case "S062X9A":
				return string(domain.SeverityDiamond)
			}
		}
	}
	return string(domain.SeverityNormal)
}

func rawAmountCents(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	if strings.Contains(normalized, ".") {
		amount, err := strconv.ParseFloat(normalized, 64)
		if err != nil || amount <= 0 {
			return ""
		}
		return strconv.FormatInt(int64(amount*100+0.5), 10)
	}
	amount, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil || amount <= 0 {
		return ""
	}
	return strconv.FormatInt(amount*100, 10)
}

func rawDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 8 {
		return value[:4] + "-" + value[4:6] + "-" + value[6:]
	}
	return value
}

func raw820AmountCents(segmentMap map[string][][]string) string {
	if rmr := firstRawSegment(segmentMap, "RMR"); len(rmr) > 4 && strings.TrimSpace(rmr[4]) != "" {
		return rawAmountCents(rmr[4])
	}
	if bpr := firstRawSegment(segmentMap, "BPR"); len(bpr) > 2 {
		return rawAmountCents(bpr[2])
	}
	return ""
}

func raw835BPRAmountCents(segmentMap map[string][][]string) string {
	if bpr := firstRawSegment(segmentMap, "BPR"); len(bpr) > 2 {
		return rawAmountCents(bpr[2])
	}
	return ""
}

func rawPacketReference(value string) (string, int, int) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "-")
	if len(parts) >= 4 && strings.EqualFold(parts[len(parts)-2], "OF") {
		sequence, _ := strconv.Atoi(parts[len(parts)-3])
		count, _ := strconv.Atoi(parts[len(parts)-1])
		return strings.Join(parts[:len(parts)-3], "-"), sequence, count
	}
	return value, 0, 0
}

func applyRawK3(attachment *xmlAttachment, value string) {
	value = strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(value, "Document-Reference:"):
		reference := strings.TrimSpace(strings.TrimPrefix(value, "Document-Reference:"))
		if strings.HasPrefix(reference, "https://") || strings.HasPrefix(reference, "s3://") || strings.HasPrefix(reference, "gs://") {
			attachment.DocumentReferenceURL = reference
		} else {
			attachment.DocumentReferenceID = reference
		}
	case strings.HasPrefix(value, "Content-Type:"):
		attachment.ContentType = strings.TrimSpace(strings.TrimPrefix(value, "Content-Type:"))
	}
}

func invalidPayloadError(contentType string) string {
	if isXMLContent(contentType) {
		return "invalid xml"
	}
	if isJSONContent(contentType) {
		return "invalid json"
	}
	if isRawX12Content(contentType) {
		return "invalid raw x12"
	}
	return "invalid payload"
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
			ServiceType:  strings.TrimSpace(t.EligibilityInquiry.ServiceType),
		}, nil
	case domain.Tx275:
		attachments, packetID, claimID, authorizationTransactionID, err := t.attachmentRequests()
		if err != nil {
			return "", "", nil, err
		}
		if len(attachments) == 0 {
			return "", "", nil, fmt.Errorf("missing attachment")
		}
		path := "/claims/" + claimID + "/attachments"
		if authorizationTransactionID != "" {
			path = "/auth-requests/" + authorizationTransactionID + "/attachments"
		}
		if len(attachments) == 1 {
			return http.MethodPost, path, attachments[0], nil
		}
		return http.MethodPost, path, domain.AttachmentPacketRequest{PacketID: packetID, Attachments: attachments}, nil
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
		request := domain.PriorAuthRequest{
			AdventurerID: strings.TrimSpace(t.PriorAuthorization.AdventurerID), ProviderID: strings.TrimSpace(t.PriorAuthorization.ProviderID),
			ServiceType: strings.TrimSpace(t.PriorAuthorization.ServiceType), IncidentSeverity: domain.IncidentSeverity(strings.TrimSpace(t.PriorAuthorization.IncidentSeverity)),
		}
		if dentalService := t.PriorAuthorization.DentalService.toDomain(); dentalService != nil {
			request.DentalService = dentalService
		}
		return http.MethodPost, "/auth-requests", request, nil
	case domain.Tx837, domain.Tx837D:
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
		serviceLines, err := t.Claim.toServiceLines()
		if err != nil {
			return "", "", nil, err
		}
		diagnoses := t.Claim.toDiagnoses()
		return http.MethodPost, "/claims", domain.ClaimRequest{
			AdventurerID: strings.TrimSpace(t.Claim.AdventurerID), ProviderID: strings.TrimSpace(t.Claim.ProviderID),
			IncidentSeverity: domain.IncidentSeverity(strings.TrimSpace(t.Claim.IncidentSeverity)), AmountCents: amountCents,
			AuthorizationTransactionID: strings.TrimSpace(t.Claim.AuthorizationTransactionID),
			ServiceLines:               serviceLines,
			Diagnoses:                  diagnoses,
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
		if t.PremiumPayment == nil {
			return "", "", nil, fmt.Errorf("missing premium payment")
		}
		if err := requireFields(map[string]string{"AdventurerId": t.PremiumPayment.AdventurerID, "AmountCents": t.PremiumPayment.AmountCents}); err != nil {
			return "", "", nil, err
		}
		amountCents, err := parsePositiveInt64("AmountCents", t.PremiumPayment.AmountCents)
		if err != nil {
			return "", "", nil, err
		}
		return http.MethodPost, "/premium-payments", domain.PremiumPaymentRequest{AdventurerID: strings.TrimSpace(t.PremiumPayment.AdventurerID), AmountCents: amountCents}, nil
	default:
		return "", "", nil, fmt.Errorf("unsupported transaction type")
	}
}

func (claim xmlClaim) toServiceLines() ([]domain.ClaimServiceLine, error) {
	if len(claim.ServiceLines) == 0 {
		return nil, nil
	}
	serviceLines := make([]domain.ClaimServiceLine, 0, len(claim.ServiceLines))
	for index, raw := range claim.ServiceLines {
		amountCents, err := parsePositiveInt64("ServiceLine.AmountCents", raw.AmountCents)
		if err != nil {
			return nil, err
		}
		if amountCents > 500000000 {
			return nil, fmt.Errorf("invalid field ServiceLine.AmountCents")
		}
		lineNumber := raw.LineNumber
		if lineNumber <= 0 {
			lineNumber = index + 1
		}
		units := raw.Units
		if units <= 0 {
			units = 1
		}
		serviceLines = append(serviceLines, domain.ClaimServiceLine{
			LineNumber:    lineNumber,
			ProcedureCode: strings.TrimSpace(raw.ProcedureCode),
			Description:   strings.TrimSpace(raw.Description),
			Units:         units,
			AmountCents:   amountCents,
			CDTCode:       strings.TrimSpace(raw.CDTCode),
			ToothNumber:   strings.TrimSpace(raw.ToothNumber),
			Surface:       strings.TrimSpace(raw.Surface),
			Quadrant:      strings.TrimSpace(raw.Quadrant),
			Orthodontic:   raw.Orthodontic,
		})
	}
	return serviceLines, nil
}

func (service xmlDentalService) toDomain() *domain.DentalServiceDetail {
	detail := domain.DentalServiceDetail{
		CDTCode:     strings.TrimSpace(service.CDTCode),
		ToothNumber: strings.TrimSpace(service.ToothNumber),
		Surface:     strings.TrimSpace(service.Surface),
		Quadrant:    strings.TrimSpace(service.Quadrant),
		Orthodontic: service.Orthodontic,
	}
	if detail.CDTCode == "" && detail.ToothNumber == "" && detail.Surface == "" && detail.Quadrant == "" && !detail.Orthodontic {
		return nil
	}
	return &detail
}

func (claim xmlClaim) toDiagnoses() []domain.ClaimDiagnosis {
	if len(claim.Diagnoses) == 0 {
		return nil
	}
	diagnoses := make([]domain.ClaimDiagnosis, 0, len(claim.Diagnoses))
	for index, raw := range claim.Diagnoses {
		code := strings.ToUpper(strings.TrimSpace(raw.Code))
		if code == "" {
			continue
		}
		qualifier := strings.ToUpper(strings.TrimSpace(raw.Qualifier))
		if qualifier == "" {
			qualifier = "ABF"
		}
		primary := raw.Primary || index == 0
		if primary {
			qualifier = "ABK"
		}
		diagnoses = append(diagnoses, domain.ClaimDiagnosis{
			Qualifier:   qualifier,
			Code:        code,
			Description: strings.TrimSpace(raw.Description),
			Primary:     primary,
		})
	}
	return diagnoses
}

func (t inboundTransaction) attachmentRequests() ([]domain.AttachmentRequest, string, string, string, error) {
	xmlAttachments := []xmlAttachment{}
	packetID := ""
	if t.Attachment != nil {
		xmlAttachments = append(xmlAttachments, *t.Attachment)
		packetID = strings.TrimSpace(t.Attachment.PacketID)
	}
	if t.AttachmentPacket != nil {
		packetID = strings.TrimSpace(t.AttachmentPacket.PacketID)
		xmlAttachments = append(xmlAttachments, t.AttachmentPacket.Attachments...)
	}
	if len(xmlAttachments) == 0 {
		return nil, "", "", "", nil
	}
	if packetID == "" && len(xmlAttachments) > 1 {
		packetID = "xml-packet-" + domain.NewID()
	}
	requests := make([]domain.AttachmentRequest, 0, len(xmlAttachments))
	claimID := ""
	authorizationTransactionID := ""
	providerID := ""
	for index, attachment := range xmlAttachments {
		itemClaimID := strings.TrimSpace(attachment.ClaimID)
		itemAuthorizationTransactionID := strings.TrimSpace(attachment.AuthorizationTransactionID)
		if (itemClaimID == "") == (itemAuthorizationTransactionID == "") {
			return nil, "", "", "", fmt.Errorf("attachment requires exactly one of ClaimId or AuthorizationTransactionId")
		}
		if claimID == "" {
			claimID = itemClaimID
		}
		if authorizationTransactionID == "" {
			authorizationTransactionID = itemAuthorizationTransactionID
		}
		if claimID != itemClaimID || authorizationTransactionID != itemAuthorizationTransactionID {
			return nil, "", "", "", fmt.Errorf("attachment packet must target one claim or authorization")
		}
		if err := requireFields(map[string]string{
			"ProviderId":              attachment.ProviderID,
			"AttachmentType":          attachment.AttachmentType,
			"AttachmentControlNumber": attachment.AttachmentControlNumber,
			"ReportTypeCode":          attachment.ReportTypeCode,
			"TransmissionCode":        attachment.TransmissionCode,
			"ContentType":             attachment.ContentType,
			"Description":             attachment.Description,
		}); err != nil {
			return nil, "", "", "", err
		}
		if strings.TrimSpace(attachment.Content) == "" && strings.TrimSpace(attachment.DocumentReferenceURL) == "" {
			return nil, "", "", "", fmt.Errorf("missing Content or DocumentReferenceUrl")
		}
		if providerID == "" {
			providerID = strings.TrimSpace(attachment.ProviderID)
		}
		if !strings.EqualFold(providerID, strings.TrimSpace(attachment.ProviderID)) {
			return nil, "", "", "", fmt.Errorf("attachment packet must use one ProviderId")
		}
		if err := validateProviderSender(t.Sender.ID, attachment.ProviderID); err != nil {
			return nil, "", "", "", err
		}
		sequence := attachment.PacketSequence
		count := attachment.PacketCount
		if len(xmlAttachments) > 1 {
			if sequence == 0 {
				sequence = index + 1
			}
			if count == 0 {
				count = len(xmlAttachments)
			}
		}
		requests = append(requests, domain.AttachmentRequest{
			PacketID:                firstNonEmpty(strings.TrimSpace(attachment.PacketID), packetID),
			PacketSequence:          sequence,
			PacketCount:             count,
			AttachmentPurpose:       strings.TrimSpace(attachment.AttachmentPurpose),
			AttachmentTraceID:       strings.TrimSpace(attachment.AttachmentTraceID),
			AttachmentFormatCode:    strings.TrimSpace(attachment.AttachmentFormatCode),
			AttachmentObjectType:    strings.TrimSpace(attachment.AttachmentObjectType),
			AttachmentEncoding:      strings.TrimSpace(attachment.AttachmentEncoding),
			AttachmentServiceDate:   strings.TrimSpace(attachment.AttachmentServiceDate),
			AttachmentType:          strings.TrimSpace(attachment.AttachmentType),
			AttachmentControlNumber: strings.TrimSpace(attachment.AttachmentControlNumber),
			ReportTypeCode:          strings.TrimSpace(attachment.ReportTypeCode),
			TransmissionCode:        strings.TrimSpace(attachment.TransmissionCode),
			ContentType:             strings.TrimSpace(attachment.ContentType),
			FileName:                strings.TrimSpace(attachment.FileName),
			Description:             strings.TrimSpace(attachment.Description),
			Content:                 strings.TrimSpace(attachment.Content),
			DocumentReferenceID:     strings.TrimSpace(attachment.DocumentReferenceID),
			DocumentReferenceURL:    strings.TrimSpace(attachment.DocumentReferenceURL),
		})
	}
	return requests, packetID, claimID, authorizationTransactionID, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func attachmentPurposeFromBGN01(code string) string {
	switch strings.TrimSpace(code) {
	case "02":
		return "unsolicited"
	case "11":
		return "solicited"
	default:
		return strings.TrimSpace(code)
	}
}

func (a intakeApp) forward(w http.ResponseWriter, inbound *http.Request, method, path string, body any) (int, string) {
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
	requestmeta.Propagate(inbound, req)
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
		return resp.StatusCode, "payer-core rejected intake-derived request"
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

func validateTradingPartnerProfile(partner domain.TradingPartner, inbound inboundTransaction) error {
	profile := partner.ValidationProfile
	switch domain.TransactionType(inbound.Type) {
	case domain.Tx275:
		attachments := inbound.attachmentsForValidation()
		if len(attachments) == 0 {
			return nil
		}
		if profile.MaxAttachmentsPerPacket > 0 && len(attachments) > profile.MaxAttachmentsPerPacket {
			return fmt.Errorf("attachment packet contains %d LX loops; trading partner %s allows %d", len(attachments), partner.ID, profile.MaxAttachmentsPerPacket)
		}
		for _, attachment := range attachments {
			if err := validateProfileCode(partner.ID, "attachment type", attachment.AttachmentType, profile.AttachmentTypes); err != nil {
				return err
			}
			if err := validateProfileCode(partner.ID, "report type", attachment.ReportTypeCode, profile.ReportTypeCodes); err != nil {
				return err
			}
			if err := validateProfileCode(partner.ID, "transmission code", attachment.TransmissionCode, profile.TransmissionCodes); err != nil {
				return err
			}
			if err := validateProfileCode(partner.ID, "content type", attachment.ContentType, profile.ContentTypes); err != nil {
				return err
			}
			if err := validateAttachmentExtensionProfile(partner.ID, attachment, profile.AllowedFileExtensions); err != nil {
				return err
			}
			if err := validateAttachmentContentTypeProfile(attachment); err != nil {
				return err
			}
			if len(profile.ControlNumberPrefixes) > 0 && !hasProfilePrefix(attachment.AttachmentControlNumber, profile.ControlNumberPrefixes) {
				return fmt.Errorf("attachment control number must start with one of: %s", strings.Join(profile.ControlNumberPrefixes, ", "))
			}
			if profile.MaxEmbeddedContentBytes > 0 && len([]byte(strings.TrimSpace(attachment.Content))) > profile.MaxEmbeddedContentBytes {
				return fmt.Errorf("attachment content exceeds %d byte limit for trading partner %s", profile.MaxEmbeddedContentBytes, partner.ID)
			}
		}
	case domain.Tx278:
		if inbound.PriorAuthorization == nil {
			return nil
		}
		if err := validateProfileCode(partner.ID, "service type", inbound.PriorAuthorization.ServiceType, profile.ServiceTypes); err != nil {
			return err
		}
		if err := validateProfileCode(partner.ID, "incident severity", inbound.PriorAuthorization.IncidentSeverity, profile.IncidentSeverities); err != nil {
			return err
		}
	case domain.Tx837, domain.Tx837D:
		if inbound.Claim == nil {
			return nil
		}
		for _, diagnosis := range claimDiagnosesForValidation(*inbound.Claim) {
			if err := validateProfileCode(partner.ID, "diagnosis qualifier", diagnosis.Qualifier, profile.DiagnosisQualifiers); err != nil {
				return err
			}
			if err := validateProfileCode(partner.ID, "diagnosis code", diagnosis.Code, profile.DiagnosisCodes); err != nil {
				return err
			}
		}
		for _, serviceLine := range inbound.Claim.ServiceLines {
			if err := validateProcedureProfile(partner.ID, serviceLine.ProcedureCode, profile); err != nil {
				return err
			}
		}
	}
	return nil
}

func claimDiagnosesForValidation(claim xmlClaim) []xmlClaimDiagnosis {
	if len(claim.Diagnoses) > 0 {
		return claim.Diagnoses
	}
	code := defaultDiagnosisCodeForSeverity(claim.IncidentSeverity)
	if code == "" {
		return nil
	}
	return []xmlClaimDiagnosis{{Qualifier: "ABK", Code: code, Primary: true}}
}

func defaultDiagnosisCodeForSeverity(severity string) string {
	switch strings.TrimSpace(severity) {
	case string(domain.SeverityNormal):
		return "S610"
	case string(domain.SeverityAwakened):
		return "T509"
	case string(domain.SeverityDiamond):
		return "S062X9A"
	default:
		return ""
	}
}

func (inbound inboundTransaction) attachmentsForValidation() []xmlAttachment {
	if inbound.Attachment != nil {
		return []xmlAttachment{*inbound.Attachment}
	}
	if inbound.AttachmentPacket != nil {
		return inbound.AttachmentPacket.Attachments
	}
	return nil
}

func validateProfileCode(partnerID, label, value string, allowed []string) error {
	if len(allowed) == 0 || containsProfileCode(allowed, value) {
		return nil
	}
	return fmt.Errorf("%s %s is not allowed for trading partner %s; allowed: %s", label, strings.TrimSpace(value), partnerID, strings.Join(allowed, ", "))
}

func validateAttachmentExtensionProfile(partnerID string, attachment xmlAttachment, allowed []string) error {
	if len(allowed) == 0 {
		return nil
	}
	extension := attachmentExtension(attachment)
	if extension == "" {
		return nil
	}
	if containsProfileCode(allowed, extension) {
		return nil
	}
	return fmt.Errorf("attachment file extension %s is not allowed for trading partner %s; allowed: %s", extension, partnerID, strings.Join(allowed, ", "))
}

func validateAttachmentContentTypeProfile(attachment xmlAttachment) error {
	extension := attachmentExtension(attachment)
	if extension == "" {
		return nil
	}
	expected := contentTypeForExtension(extension)
	if expected == "" || strings.EqualFold(attachment.ContentType, expected) {
		return nil
	}
	return fmt.Errorf("attachment content type %s does not match file extension %s; expected %s", attachment.ContentType, extension, expected)
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

func attachmentExtension(attachment xmlAttachment) string {
	for _, candidate := range []string{attachment.FileName, attachment.DocumentReferenceURL, attachment.DocumentReferenceID} {
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

func validateProcedureProfile(partnerID, procedureCode string, profile domain.PartnerValidationProfile) error {
	procedureCode = strings.TrimSpace(procedureCode)
	if procedureCode == "" {
		return nil
	}
	if len(profile.ProcedureCodes) > 0 && containsProfileCode(profile.ProcedureCodes, procedureCode) {
		return nil
	}
	if len(profile.ProcedureCodePrefixes) > 0 && hasProfilePrefix(procedureCode, profile.ProcedureCodePrefixes) {
		return nil
	}
	if len(profile.ProcedureCodes) == 0 && len(profile.ProcedureCodePrefixes) == 0 {
		return nil
	}
	allowed := append([]string{}, profile.ProcedureCodes...)
	allowed = append(allowed, profile.ProcedureCodePrefixes...)
	return fmt.Errorf("procedure code %s is not allowed for trading partner %s; allowed: %s", procedureCode, partnerID, strings.Join(allowed, ", "))
}

func containsProfileCode(values []string, candidate string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func hasProfilePrefix(value string, prefixes []string) bool {
	trimmed := strings.TrimSpace(value)
	for _, prefix := range prefixes {
		if strings.HasPrefix(trimmed, strings.TrimSpace(prefix)) {
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
		ashnlog.Error("postgres_inbound_message_audit_failed", err, "service", "edi-intake", "messageId", id, "transactionType", transactionType)
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

func (a intakeApp) saveTradingPartner(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var partner domain.TradingPartner
	if err := json.NewDecoder(r.Body).Decode(&partner); err != nil {
		fail(w, http.StatusBadRequest, "invalid json", "The partner profile scroll could not be read.")
		return
	}
	if pathID := strings.TrimSpace(r.PathValue("id")); pathID != "" {
		partner.ID = pathID
	}
	partner = normalizeTradingPartner(partner)
	if err := validatePartnerProfile(partner); err != nil {
		fail(w, http.StatusBadRequest, err.Error(), "The trading partner profile is missing required routing seals.")
		return
	}
	if err := a.persistTradingPartner(partner); err != nil {
		fail(w, http.StatusConflict, "partner save failed", err.Error())
		return
	}
	if a.tradingPartners != nil {
		a.tradingPartners[partner.ID] = partner
	}
	status := http.StatusCreated
	if r.Method == http.MethodPut {
		status = http.StatusOK
	}
	respond(w, status, domain.Envelope{Data: partner, Lore: "Trading partner profile saved for routing."})
}

func (a intakeApp) deleteTradingPartner(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		fail(w, http.StatusBadRequest, "missing trading partner id", "The routing seal could not be found.")
		return
	}
	if err := a.removeTradingPartner(id); err != nil {
		fail(w, http.StatusConflict, "partner delete failed", err.Error())
		return
	}
	if a.tradingPartners != nil {
		delete(a.tradingPartners, id)
	}
	respond(w, http.StatusOK, domain.Envelope{Data: map[string]string{"id": id}, Lore: "Trading partner profile removed from routing."})
}

func normalizeTradingPartner(partner domain.TradingPartner) domain.TradingPartner {
	partner.ID = strings.TrimSpace(partner.ID)
	partner.Name = strings.TrimSpace(partner.Name)
	partner.SenderID = strings.TrimSpace(partner.SenderID)
	partner.ReceiverID = strings.TrimSpace(partner.ReceiverID)
	partner.RouteTarget = strings.TrimSpace(partner.RouteTarget)
	partner.Status = strings.TrimSpace(partner.Status)
	if partner.ID == "" && partner.SenderID != "" {
		partner.ID = "tp-" + strings.ToLower(strings.ReplaceAll(partner.SenderID, "_", "-"))
	}
	if partner.RouteTarget == "" {
		partner.RouteTarget = "payer-core"
	}
	if partner.Status == "" {
		partner.Status = "active"
	}
	allowed := []string{}
	for _, txType := range partner.AllowedTransactionTypes {
		if trimmed := strings.TrimSpace(txType); trimmed != "" {
			allowed = append(allowed, trimmed)
		}
	}
	partner.AllowedTransactionTypes = allowed
	return partner
}

func validatePartnerProfile(partner domain.TradingPartner) error {
	if partner.Name == "" {
		return fmt.Errorf("missing trading partner name")
	}
	if partner.SenderID == "" {
		return fmt.Errorf("missing trading partner sender")
	}
	if partner.ID == "" {
		return fmt.Errorf("missing trading partner id")
	}
	if partner.ReceiverID == "" {
		return fmt.Errorf("missing trading partner receiver")
	}
	if len(partner.AllowedTransactionTypes) == 0 {
		return fmt.Errorf("missing allowed transaction types")
	}
	if !strings.EqualFold(partner.RouteTarget, "payer-core") {
		return fmt.Errorf("unsupported trading partner route")
	}
	return nil
}

func (a intakeApp) persistTradingPartner(partner domain.TradingPartner) error {
	if a.db == nil {
		return nil
	}
	profile, err := json.Marshal(partner.ValidationProfile)
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`INSERT INTO trading_partners (id, name, sender_id, receiver_id, allowed_transaction_types, validation_profile, route_target, status)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
		ON CONFLICT (sender_id) DO UPDATE SET
			id = EXCLUDED.id,
			name = EXCLUDED.name,
			receiver_id = EXCLUDED.receiver_id,
			allowed_transaction_types = EXCLUDED.allowed_transaction_types,
			validation_profile = EXCLUDED.validation_profile,
			route_target = EXCLUDED.route_target,
			status = EXCLUDED.status`,
		partner.ID, partner.Name, partner.SenderID, partner.ReceiverID, strings.Join(partner.AllowedTransactionTypes, ","), string(profile), partner.RouteTarget, partner.Status)
	return err
}

func (a intakeApp) removeTradingPartner(id string) error {
	if a.db == nil {
		return nil
	}
	result, err := a.db.Exec(`DELETE FROM trading_partners WHERE id = $1`, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("trading partner not found")
	}
	return nil
}

func (a intakeApp) record999(inbound *http.Request, relatedID string, transactionType string, receiverID string, accepted bool, errorText string) {
	if relatedID == "" {
		return
	}
	if receiverID == "" {
		receiverID = "external-partner"
	}
	ack := edimock.Generate999(relatedID, domain.TransactionType(transactionType), "edi-intake", receiverID, accepted, errorText)
	payload, err := json.Marshal(ack)
	if err != nil {
		ashnlog.Error("ack_999_marshal_failed", err, "service", "edi-intake", "relatedId", relatedID)
		return
	}
	req, err := http.NewRequest(http.MethodPost, a.payerURL+"/transactions", bytes.NewReader(payload))
	if err != nil {
		ashnlog.Error("ack_999_request_creation_failed", err, "service", "edi-intake", "relatedId", relatedID)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	requestmeta.Propagate(inbound, req)
	resp, err := a.httpClient().Do(req)
	if err != nil {
		ashnlog.Error("ack_999_persistence_failed", err, "service", "edi-intake", "relatedId", relatedID)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		ashnlog.Info("ack_999_persistence_rejected", "service", "edi-intake", "relatedId", relatedID, "status", resp.Status)
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
		ashnlog.Error("postgres_inbound_message_list_failed", err, "service", "edi-intake")
		fail(w, http.StatusInternalServerError, "message list failed", "The intake archive could not be opened.")
		return
	}
	respond(w, http.StatusOK, domain.Envelope{Data: messages, Lore: "The XML intake archive opened its scroll case.", Page: &pageInfo})
}

func (a intakeApp) rejectionMetrics(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		respond(w, http.StatusOK, domain.Envelope{Data: domain.InboundRejectionMetrics{}, Lore: "The XML rejection dashboard is not connected to a database."})
		return
	}
	metrics, err := a.queryRejectionMetrics(parseMessageFilters(r))
	if err != nil {
		ashnlog.Error("postgres_inbound_rejection_metrics_failed", err, "service", "edi-intake")
		fail(w, http.StatusInternalServerError, "rejection metrics failed", "The intake rejection dashboard could not read the archive.")
		return
	}
	respond(w, http.StatusOK, domain.Envelope{Data: metrics, Lore: "The intake rejection dashboard lit its warning runes."})
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

func (a intakeApp) queryRejectionMetrics(filters messageFilters) (domain.InboundRejectionMetrics, error) {
	rejectionFilters := filters
	rejectionFilters.Status = "rejected"
	messages, _, err := a.queryMessages(pageRequest{Limit: 100, Offset: 0}, rejectionFilters)
	if err != nil {
		return domain.InboundRejectionMetrics{}, err
	}
	metrics := domain.InboundRejectionMetrics{Total: len(messages), Latest: messages}
	partnerCounts := map[string]int{}
	typeCounts := map[string]int{}
	reasonCounts := map[string]int{}
	reasonQueries := map[string]string{}
	trendCounts := map[string]int{}
	for _, message := range messages {
		partnerLabel := fallbackLabel(message.PartnerID, "Unknown partner")
		typeLabel := fallbackLabel(message.TransactionType, "Unknown type")
		reasonLabel, reasonQuery := rejectionReason(message.Error)
		partnerCounts[partnerLabel]++
		typeCounts[typeLabel]++
		reasonCounts[reasonLabel]++
		reasonQueries[reasonLabel] = reasonQuery
		trendCounts[message.CreatedAt.Format("2006-01-02")]++
	}
	metrics.ByPartner = countItems(partnerCounts, 5, func(label string) domain.InboundRejectionCount {
		return domain.InboundRejectionCount{Label: label, Count: partnerCounts[label], Query: label, PartnerID: label}
	})
	metrics.ByType = countItems(typeCounts, 5, func(label string) domain.InboundRejectionCount {
		return domain.InboundRejectionCount{Label: label, Count: typeCounts[label], Type: label}
	})
	metrics.ByReason = countItems(reasonCounts, 5, func(label string) domain.InboundRejectionCount {
		return domain.InboundRejectionCount{Label: label, Count: reasonCounts[label], Query: reasonQueries[label]}
	})
	metrics.Trend = trendItems(trendCounts)
	if len(metrics.Latest) > 5 {
		metrics.Latest = metrics.Latest[:5]
	}
	return metrics, nil
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
			ashnlog.Error("postgres_inbound_message_lookup_failed", err, "service", "edi-intake", "messageId", id)
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

func fallbackLabel(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func rejectionReason(errorText string) (string, string) {
	text := strings.ToLower(strings.TrimSpace(errorText))
	switch {
	case strings.Contains(text, "diagnosis code"):
		return "Diagnosis code profile", "diagnosis code"
	case strings.Contains(text, "diagnosis qualifier"):
		return "Diagnosis qualifier profile", "diagnosis qualifier"
	case strings.Contains(text, "procedure code"):
		return "Procedure profile", "procedure code"
	case strings.Contains(text, "attachment type"):
		return "Attachment type profile", "attachment type"
	case strings.Contains(text, "report type"):
		return "Report type profile", "report type"
	case strings.Contains(text, "trading partner"):
		return "Trading partner routing", "trading partner"
	case strings.Contains(text, "transaction type"):
		return "Transaction set profile", "transaction type"
	case strings.TrimSpace(errorText) == "":
		return "Unknown rejection", "Unknown rejection"
	default:
		return strings.TrimSpace(errorText), strings.TrimSpace(errorText)
	}
}

func countItems(counts map[string]int, limit int, build func(string) domain.InboundRejectionCount) []domain.InboundRejectionCount {
	labels := make([]string, 0, len(counts))
	for label := range counts {
		labels = append(labels, label)
	}
	sort.Slice(labels, func(i, j int) bool {
		if counts[labels[i]] == counts[labels[j]] {
			return labels[i] < labels[j]
		}
		return counts[labels[i]] > counts[labels[j]]
	})
	if len(labels) > limit {
		labels = labels[:limit]
	}
	items := make([]domain.InboundRejectionCount, 0, len(labels))
	for _, label := range labels {
		items = append(items, build(label))
	}
	return items
}

func trendItems(counts map[string]int) []domain.InboundRejectionTrend {
	dates := make([]string, 0, len(counts))
	for date := range counts {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	items := make([]domain.InboundRejectionTrend, 0, len(dates))
	for _, date := range dates {
		items = append(items, domain.InboundRejectionTrend{Date: date, Count: counts[date]})
	}
	return items
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
	case "resurrection", "restoration", "curse-removal", "trauma-care", "dental-predetermination":
		return true
	default:
		return false
	}
}

func isXMLContent(contentType string) bool {
	mediaType := mediaType(contentType)
	return mediaType == "application/xml" || mediaType == "text/xml" || strings.HasSuffix(mediaType, "+xml")
}

func isJSONContent(contentType string) bool {
	mediaType := mediaType(contentType)
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func isRawX12Content(contentType string) bool {
	mediaType := mediaType(contentType)
	return mediaType == "text/plain" || mediaType == "application/edi-x12" || mediaType == "application/x12"
}

func mediaType(contentType string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
}

func seedTradingPartners() map[string]domain.TradingPartner {
	partners := map[string]domain.TradingPartner{}
	for _, partner := range []domain.TradingPartner{
		{ID: "tp-greenstone-guild", Name: "Greenstone Employer Guild", SenderID: "partner-greenstone", ReceiverID: "Adventure Society", AllowedTransactionTypes: []string{"834", "820"}, RouteTarget: "payer-core", Status: "active"},
		{ID: "tp-vitesse-temple", Name: "Temple of the Healer, Vitesse", SenderID: "provider-vitesse-temple", ReceiverID: "Adventure Society", AllowedTransactionTypes: []string{"270", "275", "276", "278", "837", "837D"}, RouteTarget: "payer-core", Status: "active", ValidationProfile: vitesseValidationProfile()},
		{ID: "tp-rimaros-hospital", Name: "Rimaros City Hospital", SenderID: "provider-rimaros-hospital", ReceiverID: "Adventure Society", AllowedTransactionTypes: []string{"270", "275", "276", "278", "837", "837D"}, RouteTarget: "payer-core", Status: "active", ValidationProfile: rimarosValidationProfile()},
		{ID: "tp-adventure-society-remittance", Name: "Adventure Society Remittance", SenderID: "Adventure Society", ReceiverID: "provider-vitesse-temple", AllowedTransactionTypes: []string{"835"}, RouteTarget: "payer-core", Status: "active"},
	} {
		partners[partner.ID] = partner
	}
	return partners
}

func vitesseValidationProfile() domain.PartnerValidationProfile {
	return domain.PartnerValidationProfile{
		AttachmentTypes:         []string{"OZ"},
		ReportTypeCodes:         []string{"B4"},
		TransmissionCodes:       []string{"EL"},
		ContentTypes:            []string{"text/plain"},
		AllowedFileExtensions:   []string{".txt"},
		ControlNumberPrefixes:   []string{"TEMPLE-", "ATTACH-", "XML-"},
		MaxEmbeddedContentBytes: 4096,
		MaxAttachmentsPerPacket: 3,
		ServiceTypes:            []string{"resurrection", "restoration", "curse-removal", "trauma-care", "dental-predetermination"},
		IncidentSeverities:      []string{"Normal", "Awakened", "Diamond"},
		DiagnosisQualifiers:     []string{"ABK", "ABF"},
		DiagnosisCodes:          []string{"S610", "T509", "S062X9A", "K021"},
		ProcedureCodePrefixes:   []string{"ASHN", "D"},
	}
}

func rimarosValidationProfile() domain.PartnerValidationProfile {
	return domain.PartnerValidationProfile{
		AttachmentTypes:         []string{"OZ", "PN"},
		ReportTypeCodes:         []string{"03", "B4"},
		TransmissionCodes:       []string{"EL"},
		ContentTypes:            []string{"text/plain", "application/pdf"},
		AllowedFileExtensions:   []string{".txt", ".pdf"},
		ControlNumberPrefixes:   []string{"RIM-", "ATTACH-", "XML-"},
		MaxEmbeddedContentBytes: 8192,
		MaxAttachmentsPerPacket: 5,
		ServiceTypes:            []string{"resurrection", "restoration", "curse-removal", "trauma-care", "dental-predetermination"},
		IncidentSeverities:      []string{"Normal", "Awakened", "Diamond"},
		DiagnosisQualifiers:     []string{"ABK", "ABF"},
		DiagnosisCodes:          []string{"S610", "T509", "S062X9A", "M542", "K021"},
		ProcedureCodePrefixes:   []string{"ASHN", "RIM", "D"},
	}
}

func loadTradingPartners(db *sql.DB) map[string]domain.TradingPartner {
	if db == nil {
		return seedTradingPartners()
	}
	rows, err := db.Query(`SELECT id, name, sender_id, receiver_id, allowed_transaction_types, validation_profile::text, route_target, status FROM trading_partners ORDER BY name`)
	if err != nil {
		ashnlog.Error("postgres_trading_partner_load_failed_using_seed", err, "service", "edi-intake")
		return seedTradingPartners()
	}
	defer rows.Close()
	partners := map[string]domain.TradingPartner{}
	for rows.Next() {
		var partner domain.TradingPartner
		var allowed string
		var validationProfile string
		if err := rows.Scan(&partner.ID, &partner.Name, &partner.SenderID, &partner.ReceiverID, &allowed, &validationProfile, &partner.RouteTarget, &partner.Status); err != nil {
			ashnlog.Error("postgres_trading_partner_row_skipped", err, "service", "edi-intake")
			continue
		}
		partner.AllowedTransactionTypes = splitCSV(allowed)
		if err := json.Unmarshal([]byte(validationProfile), &partner.ValidationProfile); err != nil {
			ashnlog.Error("postgres_trading_partner_profile_skipped", err, "service", "edi-intake", "partnerId", partner.ID)
		}
		partners[partner.ID] = partner
	}
	if err := rows.Err(); err != nil {
		ashnlog.Error("postgres_trading_partner_rows_failed_using_seed", err, "service", "edi-intake")
		return seedTradingPartners()
	}
	if len(partners) == 0 {
		ashnlog.Info("postgres_trading_partner_table_empty_using_seed", "service", "edi-intake")
		return seedTradingPartners()
	}
	ashnlog.Info("postgres_trading_partners_loaded", "service", "edi-intake", "count", len(partners))
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

func ediOpenAPI() map[string]any {
	return openapidocs.Spec(openapidocs.Service{
		Title:       "ASHN EDI Intake",
		Description: "XML intake, audit visibility, trading partner lookup, acknowledgment, export, and replay endpoints.",
		Version:     "0.1.0",
		Paths: map[string]map[string]openapidocs.Operation{
			"/health": {"get": {Summary: "Check edi-intake health", Tags: []string{"health"}}},
			"/x12/transactions": {
				"post": {Summary: "Accept canonical ASHN transaction intake as XML or JSON", Tags: []string{"intake", "x12"}, RequestBody: true},
			},
			"/x12/xml": {
				"post": {Summary: "Accept XML transaction intake compatibility route", Tags: []string{"xml", "x12"}, RequestBody: true},
			},
			"/x12/raw": {
				"post": {Summary: "Accept raw delimiter-based X12 intake", Tags: []string{"raw x12", "x12"}, RequestBody: true},
			},
			"/x12/batch": {
				"post": {Summary: "Accept multipart XML/JSON/raw X12 batch files", Tags: []string{"intake", "batch"}, RequestBody: true},
			},
			"/x12/messages": {
				"get": {Summary: "List XML intake audit messages", Tags: []string{"xml", "audit"}},
			},
			"/x12/messages/rejections": {
				"get": {Summary: "Summarize rejected XML intake messages", Tags: []string{"xml", "audit", "operations"}},
			},
			"/x12/messages/{id}/export": {
				"get": {Summary: "Export XML intake audit as JSON or XML", Tags: []string{"xml", "export"}},
			},
			"/x12/messages/{id}/replay": {
				"post": {Summary: "Replay XML intake message", Tags: []string{"xml", "replay"}},
			},
			"/x12/trading-partners": {
				"get":  {Summary: "List trading partner profiles", Tags: []string{"trading partners", "x12"}},
				"post": {Summary: "Create trading partner profile", Tags: []string{"trading partners", "x12"}, RequestBody: true},
			},
			"/x12/trading-partners/{id}": {
				"put":    {Summary: "Update trading partner profile", Tags: []string{"trading partners", "x12"}, RequestBody: true},
				"delete": {Summary: "Delete trading partner profile", Tags: []string{"trading partners", "x12"}},
			},
		},
	})
}

func openDB() *sql.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		ashnlog.Info("database_url_missing_audit_disabled", "service", "edi-intake")
		return nil
	}
	return openDBWith(dsn, sql.Open)
}

func openDBWith(dsn string, open func(string, string) (*sql.DB, error)) *sql.DB {
	db, err := open("postgres", dsn)
	if err != nil {
		ashnlog.Error("postgres_open_failed_audit_disabled", err, "service", "edi-intake")
		return nil
	}
	if err := db.Ping(); err != nil {
		ashnlog.Error("postgres_ping_failed_audit_disabled", err, "service", "edi-intake")
		_ = db.Close()
		return nil
	}
	ashnlog.Info("postgres_connected", "service", "edi-intake")
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
		ashnlog.Request("edi-intake", r)
		next.ServeHTTP(w, r)
	})
}
