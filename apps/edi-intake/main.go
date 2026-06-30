package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"ashn/packages/domain"
)

type intakeApp struct {
	payerURL string
	client   *http.Client
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
	app := intakeApp{payerURL: env("PAYER_CORE_URL", "http://localhost:8081"), client: http.DefaultClient}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /x12/xml", app.acceptXML)
	addr := env("EDI_INTAKE_ADDR", ":8083")
	log.Printf("[ASHN] edi-intake listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, logRequests(mux)))
}

func (a intakeApp) acceptXML(w http.ResponseWriter, r *http.Request) {
	if !isXMLContent(r.Header.Get("Content-Type")) {
		fail(w, http.StatusUnsupportedMediaType, "unsupported content type", "The intake scribe only accepts XML scrolls.")
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid xml", "The XML scroll faded before the scribe could read it.")
		return
	}
	inbound, err := parseInboundXML(body)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid xml", "The XML scroll does not match the Society intake format.")
		return
	}
	method, path, payload, err := inbound.toPayerRequest()
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not implemented") {
			status = http.StatusNotImplemented
		}
		fail(w, status, err.Error(), "The intake scribe rejected the XML scroll before it entered the ledger.")
		return
	}
	a.forward(w, method, path, payload)
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
		amountCents, err := parsePositiveInt64("AmountCents", t.Claim.AmountCents)
		if err != nil {
			return "", "", nil, err
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
		return http.MethodPost, "/claims/" + strings.TrimSpace(t.Payment.ClaimID) + "/payment", domain.PaymentRequest{PaymentAmountCents: amountCents}, nil
	case domain.Tx820:
		return "", "", nil, fmt.Errorf("transaction type 820 not implemented")
	default:
		return "", "", nil, fmt.Errorf("unsupported transaction type")
	}
}

func (a intakeApp) forward(w http.ResponseWriter, method, path string, body any) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			fail(w, http.StatusInternalServerError, "request creation failed", "The intake scribe could not translate the XML scroll.")
			return
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, a.payerURL+path, reader)
	if err != nil {
		fail(w, http.StatusInternalServerError, "request creation failed", "The intake courier could not bind the payer route.")
		return
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.httpClient().Do(req)
	if err != nil {
		fail(w, http.StatusBadGateway, "payer-core unavailable", "The intake courier could not reach the Adventure Society.")
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

func isXMLContent(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return contentType == "application/xml" || contentType == "text/xml"
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

func fail(w http.ResponseWriter, status int, message, loreText string) {
	respond(w, status, domain.ErrorEnvelope{Error: message, Lore: loreText})
}

func health(w http.ResponseWriter, _ *http.Request) {
	respond(w, http.StatusOK, map[string]string{"status": "ok", "service": "edi-intake"})
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
