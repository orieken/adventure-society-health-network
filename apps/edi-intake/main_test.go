package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ashn/packages/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEnvelope struct {
	Data  json.RawMessage  `json:"data"`
	Lore  string           `json:"lore"`
	Page  *domain.PageInfo `json:"page"`
	Error string           `json:"error"`
}

func TestAcceptXMLRoutesClaimToPayerCore(t *testing.T) {
	var downstreamPath string
	var downstreamMethod string
	var claimRequest domain.ClaimRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPath = r.URL.Path
		downstreamMethod = r.Method
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&claimRequest))
		return jsonResponse(http.StatusCreated, domain.Envelope{
			Data:        domain.Claim{ID: "claim-1", AdventurerID: claimRequest.AdventurerID, ProviderID: claimRequest.ProviderID, Status: domain.ClaimSubmitted},
			Lore:        "Incident claim submitted.",
			Transaction: &domain.Transaction{Type: domain.Tx837, Status: domain.TxStatusAccepted},
		})
	})}
	handler := newIntakeTestMux(intakeApp{payerURL: "http://payer-core", client: client})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="837">
  <Sender id="provider-vitesse-temple" />
  <Receiver id="Adventure Society" />
  <Claim>
    <AdventurerId>adv-1</AdventurerId>
    <ProviderId>provider-vitesse-temple</ProviderId>
    <IncidentSeverity>Awakened</IncidentSeverity>
    <AmountCents>125000</AmountCents>
  </Claim>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, http.MethodPost, downstreamMethod)
	assert.Equal(t, "/claims", downstreamPath)
	assert.Equal(t, "adv-1", claimRequest.AdventurerID)
	assert.Equal(t, domain.SeverityAwakened, claimRequest.IncidentSeverity)
	assert.Equal(t, int64(125000), claimRequest.AmountCents)
	envelope := decodeEnvelope(t, response)
	assert.Equal(t, "Incident claim submitted.", envelope.Lore)
}

func TestAcceptXMLRoutesClaimStatusToPayerCore(t *testing.T) {
	var downstreamPath string
	var downstreamMethod string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPath = r.URL.Path
		downstreamMethod = r.Method
		return jsonResponse(http.StatusOK, domain.Envelope{Data: map[string]string{"claimId": "claim-1", "status": "Paid"}, Lore: "Claim status returned."})
	})}
	handler := newIntakeTestMux(intakeApp{payerURL: "http://payer-core", client: client})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="276">
  <ClaimStatusRequest>
    <ClaimId>claim-1</ClaimId>
  </ClaimStatusRequest>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "text/xml; charset=utf-8")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, http.MethodGet, downstreamMethod)
	assert.Equal(t, "/claims/claim-1/status", downstreamPath)
}

func TestInboundXMLMapsSupportedTransactionTypes(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantMethod string
		wantPath   string
	}{
		{
			name: "834 enrollment", wantMethod: http.MethodPost, wantPath: "/enrollments",
			body: `<AshnX12Transaction type="834"><Enrollment><Name>Farros</Name><Rank>Iron</Rank><Guild>Grim Foundations</Guild><Region>Greenstone</Region></Enrollment></AshnX12Transaction>`,
		},
		{
			name: "270 eligibility", wantMethod: http.MethodPost, wantPath: "/eligibility/query",
			body: `<AshnX12Transaction type="270"><EligibilityInquiry><AdventurerId>adv-1</AdventurerId><ProviderId>provider-vitesse-temple</ProviderId></EligibilityInquiry></AshnX12Transaction>`,
		},
		{
			name: "278 prior authorization", wantMethod: http.MethodPost, wantPath: "/auth-requests",
			body: `<AshnX12Transaction type="278"><PriorAuthorization><AdventurerId>adv-1</AdventurerId><ProviderId>provider-vitesse-temple</ProviderId><ServiceType>resurrection</ServiceType><IncidentSeverity>Diamond</IncidentSeverity></PriorAuthorization></AshnX12Transaction>`,
		},
		{
			name: "835 payment", wantMethod: http.MethodPost, wantPath: "/claims/claim-1/payment",
			body: `<AshnX12Transaction type="835"><Payment><ClaimId>claim-1</ClaimId><PaymentAmountCents>100000</PaymentAmountCents></Payment></AshnX12Transaction>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound, err := parseInboundXML([]byte(tt.body))
			require.NoError(t, err)
			method, path, payload, err := inbound.toPayerRequest()
			require.NoError(t, err)
			assert.Equal(t, tt.wantMethod, method)
			assert.Equal(t, tt.wantPath, path)
			assert.NotNil(t, payload)
		})
	}
}

func TestAcceptXMLRejectsUnsupportedContentType(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`<AshnX12Transaction type="837" />`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusUnsupportedMediaType, response.Code)
	assert.Equal(t, "unsupported content type", decodeEnvelope(t, response).Error)
}

func TestAcceptXMLRejectsMalformedXML(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`<AshnX12Transaction type="837">`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "invalid xml", decodeEnvelope(t, response).Error)
}

func TestAcceptXMLRejectsMissingRequiredFields(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="270">
  <EligibilityInquiry>
    <AdventurerId>adv-1</AdventurerId>
  </EligibilityInquiry>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, decodeEnvelope(t, response).Error, "missing field")
}

func TestAcceptXMLRejectsUnimplemented820(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="820">
  <PremiumPayment>
    <AdventurerId>adv-1</AdventurerId>
    <AmountCents>5000</AmountCents>
  </PremiumPayment>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNotImplemented, response.Code)
	assert.Equal(t, "transaction type 820 not implemented", decodeEnvelope(t, response).Error)
}

func TestListMessagesWithoutDatabaseReturnsEmptyPage(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/x12/messages?limit=5&offset=10", nil)
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeEnvelope(t, response)
	require.NotNil(t, envelope.Page)
	assert.Equal(t, 5, envelope.Page.Limit)
	assert.Equal(t, 10, envelope.Page.Offset)
	assert.Equal(t, 0, envelope.Page.Count)
}

func newIntakeTestMux(app intakeApp) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /x12/xml", app.acceptXML)
	mux.HandleFunc("GET /x12/messages", app.listMessages)
	return mux
}

func decodeEnvelope(t *testing.T, response *httptest.ResponseRecorder) testEnvelope {
	t.Helper()
	var envelope testEnvelope
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	return envelope
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, value any) (*http.Response, error) {
	payload, _ := json.Marshal(value)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(payload))),
	}, nil
}
