package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ashn/packages/domain"

	"github.com/DATA-DOG/go-sqlmock"
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
	downstreamPaths := []string{}
	var claimRequest domain.ClaimRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPaths = append(downstreamPaths, r.URL.Path)
		switch r.URL.Path {
		case "/claims":
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.NoError(t, json.NewDecoder(r.Body).Decode(&claimRequest))
			return jsonResponse(http.StatusCreated, domain.Envelope{
				Data:        domain.Claim{ID: "claim-1", AdventurerID: claimRequest.AdventurerID, ProviderID: claimRequest.ProviderID, Status: domain.ClaimSubmitted},
				Lore:        "Incident claim submitted.",
				Transaction: &domain.Transaction{Type: domain.Tx837, Status: domain.TxStatusAccepted},
			})
		case "/transactions":
			var ack domain.Transaction
			require.NoError(t, json.NewDecoder(r.Body).Decode(&ack))
			assert.Equal(t, domain.Tx999, ack.Type)
			assert.Equal(t, domain.TxStatusAccepted, ack.Status)
			assert.NotEmpty(t, ack.RelatedID)
			return jsonResponse(http.StatusCreated, domain.Envelope{Transaction: &ack})
		default:
			t.Fatalf("unexpected downstream path %s", r.URL.Path)
			return nil, nil
		}
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
    <Diagnosis qualifier="ABK" primary="true">
      <Code>T509</Code>
      <Description>Awakened injury stabilization</Description>
    </Diagnosis>
    <Diagnosis qualifier="ABF">
      <Code>S610</Code>
      <Description>Minor wound encounter</Description>
    </Diagnosis>
    <ServiceLine lineNumber="1">
      <ProcedureCode>ASHN1</ProcedureCode>
      <Description>Resurrection stabilization</Description>
      <Units>1</Units>
      <AmountCents>95000</AmountCents>
    </ServiceLine>
    <ServiceLine lineNumber="2">
      <ProcedureCode>ASHN2</ProcedureCode>
      <Description>Dragonfire trauma supplies</Description>
      <Units>1</Units>
      <AmountCents>30000</AmountCents>
    </ServiceLine>
  </Claim>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, []string{"/claims", "/transactions"}, downstreamPaths)
	assert.Equal(t, "adv-1", claimRequest.AdventurerID)
	assert.Equal(t, domain.SeverityAwakened, claimRequest.IncidentSeverity)
	assert.Equal(t, int64(125000), claimRequest.AmountCents)
	require.Len(t, claimRequest.Diagnoses, 2)
	assert.Equal(t, "ABK", claimRequest.Diagnoses[0].Qualifier)
	assert.Equal(t, "T509", claimRequest.Diagnoses[0].Code)
	assert.True(t, claimRequest.Diagnoses[0].Primary)
	require.Len(t, claimRequest.ServiceLines, 2)
	assert.Equal(t, "ASHN1", claimRequest.ServiceLines[0].ProcedureCode)
	assert.Equal(t, int64(95000), claimRequest.ServiceLines[0].AmountCents)
	assert.Equal(t, "ASHN2", claimRequest.ServiceLines[1].ProcedureCode)
	assert.Equal(t, int64(30000), claimRequest.ServiceLines[1].AmountCents)
	envelope := decodeEnvelope(t, response)
	assert.Equal(t, "Incident claim submitted.", envelope.Lore)
}

func TestAcceptTransactionRoutesCanonicalJSONClaimToPayerCore(t *testing.T) {
	downstreamPaths := []string{}
	var claimRequest domain.ClaimRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPaths = append(downstreamPaths, r.URL.Path)
		switch r.URL.Path {
		case "/claims":
			assert.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&claimRequest))
			return jsonResponse(http.StatusCreated, domain.Envelope{
				Data:        domain.Claim{ID: "claim-json-1", AdventurerID: claimRequest.AdventurerID, ProviderID: claimRequest.ProviderID, Status: domain.ClaimSubmitted},
				Lore:        "JSON claim submitted.",
				Transaction: &domain.Transaction{Type: domain.Tx837, Status: domain.TxStatusAccepted},
			})
		case "/transactions":
			var ack domain.Transaction
			require.NoError(t, json.NewDecoder(r.Body).Decode(&ack))
			assert.Equal(t, domain.Tx999, ack.Type)
			assert.Equal(t, domain.TxStatusAccepted, ack.Status)
			return jsonResponse(http.StatusCreated, domain.Envelope{Transaction: &ack})
		default:
			t.Fatalf("unexpected downstream path %s", r.URL.Path)
			return nil, nil
		}
	})}
	handler := newIntakeTestMux(intakeApp{payerURL: "http://payer-core", client: client})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/transactions", strings.NewReader(`{
  "type": "837",
  "sender": { "id": "provider-vitesse-temple" },
  "receiver": { "id": "Adventure Society" },
  "claim": {
    "adventurerId": "adv-json-1",
    "providerId": "provider-vitesse-temple",
    "incidentSeverity": "Diamond",
    "amountCents": "250000",
    "authorizationTransactionId": "tx-278-approved",
    "diagnoses": [
      { "qualifier": "ABK", "code": "S062X9A", "description": "Catastrophic injury", "primary": true },
      { "qualifier": "ABF", "code": "T509", "description": "Awakened complication" }
    ],
    "serviceLines": [
      { "lineNumber": 1, "procedureCode": "ASHN1", "description": "Resurrection stabilization", "units": 1, "amountCents": "200000" },
      { "lineNumber": 2, "procedureCode": "ASHN3", "description": "High-acuity magic supplies", "units": 2, "amountCents": "50000" }
    ]
  }
}`))
	request.Header.Set("Content-Type", "application/vnd.ashn+x12+json")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, []string{"/claims", "/transactions"}, downstreamPaths)
	assert.Equal(t, "adv-json-1", claimRequest.AdventurerID)
	assert.Equal(t, domain.SeverityDiamond, claimRequest.IncidentSeverity)
	assert.Equal(t, int64(250000), claimRequest.AmountCents)
	assert.Equal(t, "tx-278-approved", claimRequest.AuthorizationTransactionID)
	require.Len(t, claimRequest.Diagnoses, 2)
	assert.Equal(t, "S062X9A", claimRequest.Diagnoses[0].Code)
	require.Len(t, claimRequest.ServiceLines, 2)
	assert.Equal(t, 2, claimRequest.ServiceLines[1].Units)
	assert.Equal(t, int64(50000), claimRequest.ServiceLines[1].AmountCents)
	assert.Equal(t, "JSON claim submitted.", decodeEnvelope(t, response).Lore)
}

func TestAcceptRawX12RoutesClaimToPayerCore(t *testing.T) {
	downstreamPaths := []string{}
	var claimRequest domain.ClaimRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPaths = append(downstreamPaths, r.URL.Path)
		switch r.URL.Path {
		case "/claims":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&claimRequest))
			return jsonResponse(http.StatusCreated, domain.Envelope{
				Data:        domain.Claim{ID: "claim-raw-1", AdventurerID: claimRequest.AdventurerID, ProviderID: claimRequest.ProviderID, Status: domain.ClaimSubmitted},
				Lore:        "Raw X12 claim submitted.",
				Transaction: &domain.Transaction{Type: domain.Tx837, Status: domain.TxStatusAccepted},
			})
		case "/transactions":
			var ack domain.Transaction
			require.NoError(t, json.NewDecoder(r.Body).Decode(&ack))
			assert.Equal(t, domain.Tx999, ack.Type)
			assert.Equal(t, domain.Tx837, acknowledgedTypeFromPayload(t, ack.Payload))
			return jsonResponse(http.StatusCreated, domain.Envelope{Transaction: &ack})
		default:
			t.Fatalf("unexpected downstream path %s", r.URL.Path)
			return nil, nil
		}
	})}
	handler := newIntakeTestMux(intakeApp{payerURL: "http://payer-core", client: client})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/raw", strings.NewReader(raw837Fixture()))
	request.Header.Set("Content-Type", "application/edi-x12")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, []string{"/claims", "/transactions"}, downstreamPaths)
	assert.Equal(t, "adv-raw-1", claimRequest.AdventurerID)
	assert.Equal(t, "provider-vitesse-temple", claimRequest.ProviderID)
	assert.Equal(t, domain.SeverityDiamond, claimRequest.IncidentSeverity)
	assert.Equal(t, int64(125000), claimRequest.AmountCents)
	require.Len(t, claimRequest.Diagnoses, 2)
	assert.Equal(t, "S062X9A", claimRequest.Diagnoses[0].Code)
	assert.True(t, claimRequest.Diagnoses[0].Primary)
	assert.Equal(t, "T509", claimRequest.Diagnoses[1].Code)
	require.Len(t, claimRequest.ServiceLines, 2)
	assert.Equal(t, "ASHN1", claimRequest.ServiceLines[0].ProcedureCode)
	assert.Equal(t, int64(95000), claimRequest.ServiceLines[0].AmountCents)
	assert.Equal(t, "ASHN2", claimRequest.ServiceLines[1].ProcedureCode)
	assert.Equal(t, int64(30000), claimRequest.ServiceLines[1].AmountCents)
	assert.Equal(t, "Raw X12 claim submitted.", decodeEnvelope(t, response).Lore)
}

func TestAcceptRawX12RoutesAttachmentToPayerCore(t *testing.T) {
	downstreamPaths := []string{}
	var attachmentRequest domain.AttachmentRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPaths = append(downstreamPaths, r.URL.Path)
		switch r.URL.Path {
		case "/claims/claim-raw-1/attachments":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&attachmentRequest))
			return jsonResponse(http.StatusCreated, domain.Envelope{
				Lore:        "Raw X12 attachment accepted.",
				Transaction: &domain.Transaction{Type: domain.Tx275, Status: domain.TxStatusAccepted, RelatedID: "claim-raw-1"},
			})
		case "/transactions":
			var ack domain.Transaction
			require.NoError(t, json.NewDecoder(r.Body).Decode(&ack))
			assert.Equal(t, domain.Tx999, ack.Type)
			assert.Equal(t, domain.Tx275, acknowledgedTypeFromPayload(t, ack.Payload))
			return jsonResponse(http.StatusCreated, domain.Envelope{Transaction: &ack})
		default:
			t.Fatalf("unexpected downstream path %s", r.URL.Path)
			return nil, nil
		}
	})}
	handler := newIntakeTestMux(intakeApp{payerURL: "http://payer-core", client: client})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/raw", strings.NewReader(raw275Fixture()))
	request.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, []string{"/claims/claim-raw-1/attachments", "/transactions"}, downstreamPaths)
	assert.Equal(t, "OZ", attachmentRequest.AttachmentType)
	assert.Equal(t, "ATTACH-RAW-1", attachmentRequest.AttachmentControlNumber)
	assert.Equal(t, "B4", attachmentRequest.ReportTypeCode)
	assert.Equal(t, "EL", attachmentRequest.TransmissionCode)
	assert.Equal(t, "text/plain", attachmentRequest.ContentType)
	assert.Equal(t, "Raw resurrection notes", attachmentRequest.Description)
	assert.Equal(t, "Patient survived raw X12 dragonfire.", attachmentRequest.Content)
}

func TestAcceptXMLRoutesClaimStatusToPayerCore(t *testing.T) {
	downstreamPaths := []string{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPaths = append(downstreamPaths, r.URL.Path)
		switch r.URL.Path {
		case "/claims/claim-1/status":
			assert.Equal(t, http.MethodGet, r.Method)
			return jsonResponse(http.StatusOK, domain.Envelope{Data: map[string]string{"claimId": "claim-1", "status": "Paid"}, Lore: "Claim status returned."})
		case "/transactions":
			var ack domain.Transaction
			require.NoError(t, json.NewDecoder(r.Body).Decode(&ack))
			assert.Equal(t, domain.Tx999, ack.Type)
			return jsonResponse(http.StatusCreated, domain.Envelope{Transaction: &ack})
		default:
			t.Fatalf("unexpected downstream path %s", r.URL.Path)
			return nil, nil
		}
	})}
	handler := newIntakeTestMux(intakeApp{payerURL: "http://payer-core", client: client})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="276">
  <Sender id="provider-vitesse-temple" />
  <Receiver id="Adventure Society" />
  <ClaimStatusRequest>
    <ClaimId>claim-1</ClaimId>
  </ClaimStatusRequest>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "text/xml; charset=utf-8")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, []string{"/claims/claim-1/status", "/transactions"}, downstreamPaths)
}

func TestAcceptXMLRoutesAttachmentToPayerCore(t *testing.T) {
	downstreamPaths := []string{}
	var attachmentRequest domain.AttachmentRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPaths = append(downstreamPaths, r.URL.Path)
		switch r.URL.Path {
		case "/claims/claim-1/attachments":
			assert.Equal(t, http.MethodPost, r.Method)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&attachmentRequest))
			return jsonResponse(http.StatusCreated, domain.Envelope{
				Lore:        "Patient information attachment accepted.",
				Transaction: &domain.Transaction{Type: domain.Tx275, Status: domain.TxStatusAccepted, RelatedID: "tx-837"},
			})
		case "/transactions":
			var ack domain.Transaction
			require.NoError(t, json.NewDecoder(r.Body).Decode(&ack))
			assert.Equal(t, domain.Tx999, ack.Type)
			assert.Equal(t, domain.Tx275, acknowledgedTypeFromPayload(t, ack.Payload))
			return jsonResponse(http.StatusCreated, domain.Envelope{Transaction: &ack})
		default:
			t.Fatalf("unexpected downstream path %s", r.URL.Path)
			return nil, nil
		}
	})}
	handler := newIntakeTestMux(intakeApp{payerURL: "http://payer-core", client: client})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="275">
  <Sender id="provider-vitesse-temple" />
  <Receiver id="Adventure Society" />
  <Attachment>
    <ClaimId>claim-1</ClaimId>
    <ProviderId>provider-vitesse-temple</ProviderId>
    <AttachmentType>OZ</AttachmentType>
    <AttachmentControlNumber>ATTACH-1</AttachmentControlNumber>
    <ReportTypeCode>B4</ReportTypeCode>
    <TransmissionCode>EL</TransmissionCode>
    <ContentType>text/plain</ContentType>
    <Description>Resurrection notes</Description>
    <Content>Patient survived a dragonfire incident.</Content>
    <DocumentReferenceId>doc-xml-001</DocumentReferenceId>
    <DocumentReferenceUrl>https://docs.example.test/doc-xml-001.txt</DocumentReferenceUrl>
  </Attachment>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, []string{"/claims/claim-1/attachments", "/transactions"}, downstreamPaths)
	assert.Equal(t, "OZ", attachmentRequest.AttachmentType)
	assert.Equal(t, "ATTACH-1", attachmentRequest.AttachmentControlNumber)
	assert.Equal(t, "B4", attachmentRequest.ReportTypeCode)
	assert.Equal(t, "EL", attachmentRequest.TransmissionCode)
	assert.Equal(t, "text/plain", attachmentRequest.ContentType)
	assert.Equal(t, "doc-xml-001", attachmentRequest.DocumentReferenceID)
	assert.Equal(t, "https://docs.example.test/doc-xml-001.txt", attachmentRequest.DocumentReferenceURL)
}

func TestAcceptXMLRoutesAttachmentPacketToPayerCore(t *testing.T) {
	downstreamPaths := []string{}
	var packetRequest domain.AttachmentPacketRequest
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPaths = append(downstreamPaths, r.URL.Path)
		switch r.URL.Path {
		case "/claims/claim-1/attachments":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&packetRequest))
			return jsonResponse(http.StatusCreated, domain.Envelope{
				Lore: "Patient information attachment packet accepted.",
				Transactions: []domain.Transaction{
					{Type: domain.Tx275, Status: domain.TxStatusAccepted, RelatedID: "tx-837"},
					{Type: domain.Tx275, Status: domain.TxStatusAccepted, RelatedID: "tx-837"},
				},
			})
		case "/transactions":
			return jsonResponse(http.StatusCreated, domain.Envelope{})
		default:
			t.Fatalf("unexpected downstream path %s", r.URL.Path)
			return nil, nil
		}
	})}
	handler := newIntakeTestMux(intakeApp{payerURL: "http://payer-core", client: client})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="275">
  <Sender id="provider-vitesse-temple" />
  <Receiver id="Adventure Society" />
  <AttachmentPacket packetId="packet-claim-1">
    <Attachment>
      <ClaimId>claim-1</ClaimId>
      <ProviderId>provider-vitesse-temple</ProviderId>
      <AttachmentType>OZ</AttachmentType>
      <AttachmentControlNumber>ATTACH-PKT-1</AttachmentControlNumber>
      <ReportTypeCode>B4</ReportTypeCode>
      <TransmissionCode>EL</TransmissionCode>
      <ContentType>text/plain</ContentType>
      <Description>First resurrection note</Description>
      <Content>First note.</Content>
    </Attachment>
    <Attachment>
      <ClaimId>claim-1</ClaimId>
      <ProviderId>provider-vitesse-temple</ProviderId>
      <AttachmentType>OZ</AttachmentType>
      <AttachmentControlNumber>ATTACH-PKT-2</AttachmentControlNumber>
      <ReportTypeCode>B4</ReportTypeCode>
      <TransmissionCode>EL</TransmissionCode>
      <ContentType>text/plain</ContentType>
      <Description>Second resurrection note</Description>
      <DocumentReferenceUrl>https://docs.example.test/claim-1/second-note.txt</DocumentReferenceUrl>
    </Attachment>
  </AttachmentPacket>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, []string{"/claims/claim-1/attachments", "/transactions"}, downstreamPaths)
	assert.Equal(t, "packet-claim-1", packetRequest.PacketID)
	require.Len(t, packetRequest.Attachments, 2)
	assert.Equal(t, 1, packetRequest.Attachments[0].PacketSequence)
	assert.Equal(t, 2, packetRequest.Attachments[1].PacketSequence)
	assert.Equal(t, "packet-claim-1", packetRequest.Attachments[1].PacketID)
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
			name: "275 attachment", wantMethod: http.MethodPost, wantPath: "/claims/claim-1/attachments",
			body: `<AshnX12Transaction type="275"><Attachment><ClaimId>claim-1</ClaimId><ProviderId>provider-vitesse-temple</ProviderId><AttachmentType>OZ</AttachmentType><AttachmentControlNumber>ATTACH-1</AttachmentControlNumber><ReportTypeCode>B4</ReportTypeCode><TransmissionCode>EL</TransmissionCode><ContentType>text/plain</ContentType><Description>notes</Description><Content>content</Content></Attachment></AshnX12Transaction>`,
		},
		{
			name: "275 prior authorization attachment", wantMethod: http.MethodPost, wantPath: "/auth-requests/tx-278/attachments",
			body: `<AshnX12Transaction type="275"><Attachment><AuthorizationTransactionId>tx-278</AuthorizationTransactionId><ProviderId>provider-vitesse-temple</ProviderId><AttachmentType>OZ</AttachmentType><AttachmentControlNumber>ATTACH-AUTH-1</AttachmentControlNumber><ReportTypeCode>B4</ReportTypeCode><TransmissionCode>EL</TransmissionCode><ContentType>text/plain</ContentType><Description>notes</Description><Content>content</Content></Attachment></AshnX12Transaction>`,
		},
		{
			name: "275 external document reference", wantMethod: http.MethodPost, wantPath: "/claims/claim-1/attachments",
			body: `<AshnX12Transaction type="275"><Attachment><ClaimId>claim-1</ClaimId><ProviderId>provider-vitesse-temple</ProviderId><AttachmentType>OZ</AttachmentType><AttachmentControlNumber>ATTACH-REF-1</AttachmentControlNumber><ReportTypeCode>B4</ReportTypeCode><TransmissionCode>EL</TransmissionCode><ContentType>text/plain</ContentType><Description>notes</Description><DocumentReferenceUrl>https://docs.example.test/doc.txt</DocumentReferenceUrl></Attachment></AshnX12Transaction>`,
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

func TestInboundXMLRejectsUnsupportedAndInvalidPayloads(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "unknown type", body: `<AshnX12Transaction type="999"></AshnX12Transaction>`, want: "unsupported transaction type"},
		{name: "missing attachment", body: `<AshnX12Transaction type="275"></AshnX12Transaction>`, want: "missing attachment"},
		{name: "missing attachment content or reference", body: `<AshnX12Transaction type="275"><Attachment><ClaimId>claim-1</ClaimId><ProviderId>provider-vitesse-temple</ProviderId><AttachmentType>OZ</AttachmentType><AttachmentControlNumber>ATTACH-1</AttachmentControlNumber><ReportTypeCode>B4</ReportTypeCode><TransmissionCode>EL</TransmissionCode><ContentType>text/plain</ContentType><Description>notes</Description></Attachment></AshnX12Transaction>`, want: "missing Content or DocumentReferenceUrl"},
		{name: "ambiguous attachment target", body: `<AshnX12Transaction type="275"><Attachment><ClaimId>claim-1</ClaimId><AuthorizationTransactionId>tx-278</AuthorizationTransactionId><ProviderId>provider-vitesse-temple</ProviderId><AttachmentType>OZ</AttachmentType><AttachmentControlNumber>ATTACH-1</AttachmentControlNumber><ReportTypeCode>B4</ReportTypeCode><TransmissionCode>EL</TransmissionCode><ContentType>text/plain</ContentType><Description>notes</Description><Content>content</Content></Attachment></AshnX12Transaction>`, want: "attachment requires exactly one of ClaimId or AuthorizationTransactionId"},
		{name: "invalid service type", body: `<AshnX12Transaction type="278"><PriorAuthorization><AdventurerId>adv-1</AdventurerId><ProviderId>provider-vitesse-temple</ProviderId><ServiceType>vacation</ServiceType><IncidentSeverity>Diamond</IncidentSeverity></PriorAuthorization></AshnX12Transaction>`, want: "invalid field ServiceType"},
		{name: "invalid payment amount", body: `<AshnX12Transaction type="835"><Payment><ClaimId>claim-1</ClaimId><PaymentAmountCents>-1</PaymentAmountCents></Payment></AshnX12Transaction>`, want: "invalid field PaymentAmountCents"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound, err := parseInboundXML([]byte(tt.body))
			require.NoError(t, err)
			_, _, _, err = inbound.toPayerRequest()
			require.Error(t, err)
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

func TestInboundRawX12ParsesSupportedTransactionTypes(t *testing.T) {
	claim, err := parseInboundRawX12([]byte(raw837Fixture()))
	require.NoError(t, err)
	assert.Equal(t, "837", claim.Type)
	assert.Equal(t, "provider-vitesse-temple", claim.Sender.ID)
	require.NotNil(t, claim.Claim)
	assert.Equal(t, "adv-raw-1", claim.Claim.AdventurerID)
	assert.Equal(t, "125000", claim.Claim.AmountCents)
	require.Len(t, claim.Claim.Diagnoses, 2)
	assert.Equal(t, "ABK", claim.Claim.Diagnoses[0].Qualifier)
	assert.Equal(t, "S062X9A", claim.Claim.Diagnoses[0].Code)
	assert.True(t, claim.Claim.Diagnoses[0].Primary)
	require.Len(t, claim.Claim.ServiceLines, 2)
	assert.Equal(t, "ASHN1", claim.Claim.ServiceLines[0].ProcedureCode)
	assert.Equal(t, "95000", claim.Claim.ServiceLines[0].AmountCents)
	assert.Equal(t, 2, claim.Claim.ServiceLines[1].LineNumber)
	assert.Equal(t, "ASHN2", claim.Claim.ServiceLines[1].ProcedureCode)

	attachment, err := parseInboundRawX12([]byte(raw275Fixture()))
	require.NoError(t, err)
	assert.Equal(t, "275", attachment.Type)
	require.NotNil(t, attachment.Attachment)
	assert.Equal(t, "claim-raw-1", attachment.Attachment.ClaimID)
	assert.Equal(t, "ATTACH-RAW-1", attachment.Attachment.AttachmentControlNumber)
}

func TestInboundRawX12RejectsUnsupportedAndInvalidPayloads(t *testing.T) {
	_, err := parseInboundRawX12([]byte(`ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000001*0*T*:~GS*HC*provider-vitesse-temple*Adventure Society*20260708*1200*000000001*X*005010X999A1~ST*999*000000001~SE*2*000000001~`))
	require.Error(t, err)
	assert.Equal(t, "raw X12 transaction type 999 not implemented", err.Error())

	_, err = parseInboundRawX12([]byte(`ST*837*0001~NM1*IL*1*adv-raw-1****MI*adv-raw-1~`))
	require.Error(t, err)
	assert.Equal(t, "missing CLM claim segment", err.Error())
}

func acknowledgedTypeFromPayload(t *testing.T, payload json.RawMessage) domain.TransactionType {
	t.Helper()
	var body struct {
		AcknowledgedType domain.TransactionType `json:"acknowledgedType"`
	}
	require.NoError(t, json.Unmarshal(payload, &body))
	return body.AcknowledgedType
}

func TestValidateTradingPartnerAllowsMissingSenderAndRejectsReceiverMismatch(t *testing.T) {
	app := intakeApp{tradingPartners: seedTradingPartners()}
	inbound := inboundTransaction{Type: "834", Sender: party{ID: "partner-greenstone"}, Receiver: party{ID: "Wrong Receiver"}}

	_, err := app.validateTradingPartner(inbound)

	require.Error(t, err)
	assert.Equal(t, "trading partner receiver mismatch", err.Error())
}

func TestAcceptXMLRejectsUnsupportedContentType(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`<AshnX12Transaction type="837" />`))
	request.Header.Set("Content-Type", "application/octet-stream")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusUnsupportedMediaType, response.Code)
	assert.Equal(t, "unsupported content type", decodeEnvelope(t, response).Error)
}

func TestAcceptTransactionRejectsMalformedJSON(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/transactions", strings.NewReader(`{"type":"837"`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "invalid json", decodeEnvelope(t, response).Error)
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

func TestAcceptXMLRejectsInvalidTransactionSpecificFields(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "invalid rank",
			want: "invalid field Rank",
			body: `<AshnX12Transaction type="834"><Sender id="partner-greenstone" /><Receiver id="Adventure Society" /><Enrollment><Name>Farros</Name><Rank>Mythic</Rank><Guild>Grim Foundations</Guild><Region>Greenstone</Region></Enrollment></AshnX12Transaction>`,
		},
		{
			name: "invalid severity",
			want: "invalid field IncidentSeverity",
			body: `<AshnX12Transaction type="837"><Sender id="provider-vitesse-temple" /><Receiver id="Adventure Society" /><Claim><AdventurerId>adv-1</AdventurerId><ProviderId>provider-vitesse-temple</ProviderId><IncidentSeverity>Cosmic</IncidentSeverity><AmountCents>125000</AmountCents></Claim></AshnX12Transaction>`,
		},
		{
			name: "sender mismatch",
			want: "sender must match ProviderId",
			body: `<AshnX12Transaction type="270"><Sender id="provider-vitesse-temple" /><Receiver id="Adventure Society" /><EligibilityInquiry><AdventurerId>adv-1</AdventurerId><ProviderId>provider-rimaros-hospital</ProviderId></EligibilityInquiry></AshnX12Transaction>`,
		},
		{
			name: "oversized claim amount",
			want: "invalid field AmountCents",
			body: `<AshnX12Transaction type="837"><Sender id="provider-vitesse-temple" /><Receiver id="Adventure Society" /><Claim><AdventurerId>adv-1</AdventurerId><ProviderId>provider-vitesse-temple</ProviderId><IncidentSeverity>Awakened</IncidentSeverity><AmountCents>900000000</AmountCents></Claim></AshnX12Transaction>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newIntakeTestMux(intakeApp{})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/xml")
			handler.ServeHTTP(response, request)

			assert.Equal(t, http.StatusBadRequest, response.Code)
			assert.Equal(t, tt.want, decodeEnvelope(t, response).Error)
		})
	}
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

func TestAcceptXMLRejectsUnknownTradingPartner(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="837">
  <Sender id="unknown-provider" />
  <Receiver id="Adventure Society" />
  <Claim>
    <AdventurerId>adv-1</AdventurerId>
    <ProviderId>unknown-provider</ProviderId>
    <IncidentSeverity>Awakened</IncidentSeverity>
    <AmountCents>125000</AmountCents>
  </Claim>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "unknown trading partner", decodeEnvelope(t, response).Error)
}

func TestAcceptXMLRejectsDisallowedPartnerTransaction(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="837">
  <Sender id="partner-greenstone" />
  <Receiver id="Adventure Society" />
  <Claim>
    <AdventurerId>adv-1</AdventurerId>
    <ProviderId>partner-greenstone</ProviderId>
    <IncidentSeverity>Awakened</IncidentSeverity>
    <AmountCents>125000</AmountCents>
  </Claim>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "transaction type 837 not allowed for trading partner", decodeEnvelope(t, response).Error)
}

func TestValidateTradingPartnerProfileAppliesAttachmentRules(t *testing.T) {
	partners := seedTradingPartners()
	inbound := inboundTransaction{
		Type: string(domain.Tx275),
		Attachment: &xmlAttachment{
			AttachmentType:          "PN",
			AttachmentControlNumber: "RIM-275-1",
			ReportTypeCode:          "03",
			TransmissionCode:        "EL",
			ContentType:             "application/pdf",
			Content:                 "%PDF-1.7",
		},
	}

	err := validateTradingPartnerProfile(partners["tp-vitesse-temple"], inbound)
	require.Error(t, err)
	assert.Equal(t, "attachment type PN is not allowed for trading partner tp-vitesse-temple; allowed: OZ", err.Error())

	require.NoError(t, validateTradingPartnerProfile(partners["tp-rimaros-hospital"], inbound))
}

func TestValidateTradingPartnerProfileRejectsPriorAuthOutsideProfile(t *testing.T) {
	partner := seedTradingPartners()["tp-vitesse-temple"]
	inbound := inboundTransaction{
		Type: string(domain.Tx278),
		PriorAuthorization: &xmlPriorAuth{
			ServiceType:      "dragon-riding",
			IncidentSeverity: "Awakened",
		},
	}

	err := validateTradingPartnerProfile(partner, inbound)
	require.Error(t, err)
	assert.Equal(t, "service type dragon-riding is not allowed for trading partner tp-vitesse-temple; allowed: resurrection, restoration, curse-removal, trauma-care", err.Error())
}

func TestValidateTradingPartnerProfileAppliesClaimRules(t *testing.T) {
	partners := seedTradingPartners()
	inbound := inboundTransaction{
		Type: string(domain.Tx837),
		Claim: &xmlClaim{
			Diagnoses: []xmlClaimDiagnosis{
				{Qualifier: "ABK", Code: "M542", Primary: true},
			},
			ServiceLines: []xmlClaimServiceLine{
				{ProcedureCode: "RIM100", AmountCents: "10000"},
			},
		},
	}

	err := validateTradingPartnerProfile(partners["tp-vitesse-temple"], inbound)
	require.Error(t, err)
	assert.Equal(t, "diagnosis code M542 is not allowed for trading partner tp-vitesse-temple; allowed: S610, T509, S062X9A", err.Error())

	require.NoError(t, validateTradingPartnerProfile(partners["tp-rimaros-hospital"], inbound))
}

func TestValidateTradingPartnerProfileRejectsClaimProcedureOutsideProfile(t *testing.T) {
	partner := seedTradingPartners()["tp-vitesse-temple"]
	inbound := inboundTransaction{
		Type: string(domain.Tx837),
		Claim: &xmlClaim{
			Diagnoses: []xmlClaimDiagnosis{
				{Qualifier: "ABK", Code: "T509", Primary: true},
			},
			ServiceLines: []xmlClaimServiceLine{
				{ProcedureCode: "RIM100", AmountCents: "10000"},
			},
		},
	}

	err := validateTradingPartnerProfile(partner, inbound)
	require.Error(t, err)
	assert.Equal(t, "procedure code RIM100 is not allowed for trading partner tp-vitesse-temple; allowed: ASHN", err.Error())
}

func TestAcceptXMLRejectsPartnerProfileViolationBeforeForwarding(t *testing.T) {
	forwarded := false
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/transactions" {
			forwarded = true
		}
		return jsonResponse(http.StatusCreated, domain.Envelope{Lore: "unexpected"})
	})}
	handler := newIntakeTestMux(intakeApp{client: client, payerURL: "http://payer-core"})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="275">
  <Sender id="provider-vitesse-temple" />
  <Receiver id="Adventure Society" />
  <Attachment>
    <ClaimId>claim-1</ClaimId>
    <ProviderId>provider-vitesse-temple</ProviderId>
    <AttachmentType>PN</AttachmentType>
    <AttachmentControlNumber>RIM-275-1</AttachmentControlNumber>
    <ReportTypeCode>03</ReportTypeCode>
    <TransmissionCode>EL</TransmissionCode>
    <ContentType>application/pdf</ContentType>
    <Description>Operative note</Description>
    <Content>%PDF-1.7</Content>
  </Attachment>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "attachment type PN is not allowed for trading partner tp-vitesse-temple; allowed: OZ", decodeEnvelope(t, response).Error)
	assert.False(t, forwarded)
}

func TestAcceptXMLRejectsClaimProfileViolationBeforeForwarding(t *testing.T) {
	forwarded := false
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/transactions" {
			forwarded = true
		}
		return jsonResponse(http.StatusCreated, domain.Envelope{Lore: "unexpected"})
	})}
	handler := newIntakeTestMux(intakeApp{client: client, payerURL: "http://payer-core"})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", strings.NewReader(`
<AshnX12Transaction type="837">
  <Sender id="provider-vitesse-temple" />
  <Receiver id="Adventure Society" />
  <Claim>
    <AdventurerId>adv-1</AdventurerId>
    <ProviderId>provider-vitesse-temple</ProviderId>
    <IncidentSeverity>Awakened</IncidentSeverity>
    <AmountCents>10000</AmountCents>
    <Diagnosis qualifier="ABK" primary="true"><Code>M542</Code></Diagnosis>
    <ServiceLine lineNumber="1"><ProcedureCode>ASHN1</ProcedureCode><AmountCents>10000</AmountCents></ServiceLine>
  </Claim>
</AshnX12Transaction>`))
	request.Header.Set("Content-Type", "application/xml")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "diagnosis code M542 is not allowed for trading partner tp-vitesse-temple; allowed: S610, T509, S062X9A", decodeEnvelope(t, response).Error)
	assert.False(t, forwarded)
}

func TestListTradingPartnersReturnsSeedProfiles(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/x12/trading-partners", nil)
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	var partners []domain.TradingPartner
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, response).Data, &partners))
	require.NotEmpty(t, partners)
	senderIDs := []string{}
	for _, partner := range partners {
		senderIDs = append(senderIDs, partner.SenderID)
	}
	assert.Contains(t, senderIDs, "partner-greenstone")
	assert.Equal(t, []string{"OZ"}, seedTradingPartners()["tp-vitesse-temple"].ValidationProfile.AttachmentTypes)
	assert.Equal(t, []string{"S610", "T509", "S062X9A"}, seedTradingPartners()["tp-vitesse-temple"].ValidationProfile.DiagnosisCodes)
	assert.Equal(t, []string{"ASHN", "RIM"}, seedTradingPartners()["tp-rimaros-hospital"].ValidationProfile.ProcedureCodePrefixes)
}

func TestSaveAndDeleteTradingPartnerInMemory(t *testing.T) {
	partners := seedTradingPartners()
	handler := newIntakeTestMux(intakeApp{tradingPartners: partners})

	body := `{"name":"Crystal Tower Partner","senderId":"provider-crystal-tower","receiverId":"Adventure Society","allowedTransactionTypes":["270","275","837"],"routeTarget":"payer-core","status":"active"}`
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/x12/trading-partners", strings.NewReader(body))
	createRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(createResponse, createRequest)

	assert.Equal(t, http.StatusCreated, createResponse.Code)
	var created domain.TradingPartner
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, createResponse).Data, &created))
	assert.Equal(t, "tp-provider-crystal-tower", created.ID)
	assert.Equal(t, []string{"270", "275", "837"}, created.AllowedTransactionTypes)
	assert.Contains(t, partners, created.ID)

	updateResponse := httptest.NewRecorder()
	updateRequest := httptest.NewRequest(http.MethodPut, "/x12/trading-partners/"+created.ID, strings.NewReader(`{"name":"Crystal Tower Partner","senderId":"provider-crystal-tower","receiverId":"Adventure Society","allowedTransactionTypes":["270"],"routeTarget":"payer-core","status":"inactive"}`))
	updateRequest.SetPathValue("id", created.ID)
	updateRequest.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(updateResponse, updateRequest)

	assert.Equal(t, http.StatusOK, updateResponse.Code)
	assert.Equal(t, "inactive", partners[created.ID].Status)
	assert.Equal(t, []string{"270"}, partners[created.ID].AllowedTransactionTypes)

	deleteResponse := httptest.NewRecorder()
	deleteRequest := httptest.NewRequest(http.MethodDelete, "/x12/trading-partners/"+created.ID, nil)
	deleteRequest.SetPathValue("id", created.ID)
	handler.ServeHTTP(deleteResponse, deleteRequest)

	assert.Equal(t, http.StatusOK, deleteResponse.Code)
	assert.NotContains(t, partners, created.ID)
}

func TestSaveTradingPartnerValidatesRequiredFields(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{tradingPartners: seedTradingPartners()})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/trading-partners", strings.NewReader(`{"name":"Bad Partner"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "missing trading partner sender", decodeEnvelope(t, response).Error)
}

func TestSaveAndDeleteTradingPartnerPersistsToDatabase(t *testing.T) {
	db, mock, cleanup := newIntakeMockDB(t)
	defer cleanup()
	app := intakeApp{db: db, tradingPartners: map[string]domain.TradingPartner{}}
	partner := domain.TradingPartner{
		ID: "tp-1", Name: "Partner One", SenderID: "sender-1", ReceiverID: "Adventure Society",
		AllowedTransactionTypes: []string{"270", "837"}, RouteTarget: "payer-core", Status: "active",
	}
	mock.ExpectExec("INSERT INTO trading_partners").
		WithArgs("tp-1", "Partner One", "sender-1", "Adventure Society", "270,837", "{}", "payer-core", "active").
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, app.persistTradingPartner(partner))

	mock.ExpectExec("DELETE FROM trading_partners").
		WithArgs("tp-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, app.removeTradingPartner("tp-1"))
	require.NoError(t, mock.ExpectationsWereMet())
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

func TestRejectionMetricsSummarizePartnerTrends(t *testing.T) {
	db, mock, cleanup := newIntakeMockDB(t)
	defer cleanup()
	app := intakeApp{db: db}
	first := time.Date(2026, 7, 8, 13, 0, 0, 0, time.UTC)
	second := first.Add(24 * time.Hour)

	mock.ExpectQuery("SELECT id, COALESCE\\(partner_id").
		WithArgs("rejected", 101, 0).
		WillReturnRows(messageRows().
			AddRow("msg-1", "tp-vitesse-temple", "application/xml", "837", "<xml />", "rejected", "diagnosis code M542 is not allowed", 400, second).
			AddRow("msg-2", "tp-vitesse-temple", "application/xml", "837", "<xml />", "rejected", "diagnosis code BAD is not allowed", 400, first).
			AddRow("msg-3", "tp-rimaros", "application/xml", "275", "<xml />", "rejected", "attachment type ZZ is not allowed", 400, first))

	metrics, err := app.queryRejectionMetrics(messageFilters{})

	require.NoError(t, err)
	assert.Equal(t, 3, metrics.Total)
	assert.Equal(t, []domain.InboundRejectionCount{{Label: "tp-vitesse-temple", Count: 2, Query: "tp-vitesse-temple", PartnerID: "tp-vitesse-temple"}, {Label: "tp-rimaros", Count: 1, Query: "tp-rimaros", PartnerID: "tp-rimaros"}}, metrics.ByPartner)
	assert.Equal(t, []domain.InboundRejectionCount{{Label: "837", Count: 2, Type: "837"}, {Label: "275", Count: 1, Type: "275"}}, metrics.ByType)
	assert.Equal(t, []domain.InboundRejectionCount{{Label: "Diagnosis code profile", Count: 2, Query: "diagnosis code"}, {Label: "Attachment type profile", Count: 1, Query: "attachment type"}}, metrics.ByReason)
	assert.Equal(t, []domain.InboundRejectionTrend{{Date: "2026-07-08", Count: 2}, {Date: "2026-07-09", Count: 1}}, metrics.Trend)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRejectionMetricsRouteWithoutDatabaseReturnsEmptyMetrics(t *testing.T) {
	handler := newIntakeTestMux(intakeApp{})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/x12/messages/rejections", nil)
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeEnvelope(t, response)
	var metrics domain.InboundRejectionMetrics
	require.NoError(t, json.Unmarshal(envelope.Data, &metrics))
	assert.Equal(t, 0, metrics.Total)
	assert.Contains(t, envelope.Lore, "not connected")
}

func TestMessageArchiveQueriesExportsAndReplaysMissingMessages(t *testing.T) {
	db, mock, cleanup := newIntakeMockDB(t)
	defer cleanup()
	app := intakeApp{db: db}
	now := time.Now()

	mock.ExpectQuery("SELECT id, COALESCE\\(partner_id").
		WithArgs("accepted", "834", "%Farros%", 2, 0).
		WillReturnRows(messageRows().
			AddRow("msg-1", "partner-greenstone", "application/xml", "834", "<xml>Farros</xml>", "accepted", "", 201, now).
			AddRow("msg-2", "partner-greenstone", "application/xml", "834", "<xml>Farros 2</xml>", "accepted", "", 201, now))
	messages, page, err := app.queryMessages(pageRequest{Limit: 1, Offset: 0}, messageFilters{Status: "accepted", Type: "834", Q: "Farros"})
	require.NoError(t, err)
	assert.Len(t, messages, 1)
	assert.True(t, page.HasMore)

	mock.ExpectQuery("SELECT id, COALESCE\\(partner_id").
		WithArgs("msg-1").
		WillReturnRows(messageRows().AddRow("msg-1", "partner-greenstone", "application/xml", "834", "<xml>Farros</xml>", "accepted", "", 201, now))
	found, ok := app.findMessage("msg-1")
	require.True(t, ok)
	assert.Equal(t, "msg-1", found.ID)

	mock.ExpectQuery("SELECT id, COALESCE\\(partner_id").
		WithArgs("msg-1").
		WillReturnRows(messageRows().AddRow("msg-1", "partner-greenstone", "application/xml", "834", "<xml>Farros</xml>", "accepted", "", 201, now))
	xmlResponse := httptest.NewRecorder()
	xmlRequest := httptest.NewRequest(http.MethodGet, "/x12/messages/msg-1/export", nil)
	xmlRequest.SetPathValue("id", "msg-1")
	app.exportMessage(xmlResponse, xmlRequest)
	assert.Equal(t, http.StatusOK, xmlResponse.Code)
	assert.Contains(t, xmlResponse.Header().Get("Content-Type"), "application/xml")
	assert.Equal(t, "<xml>Farros</xml>", xmlResponse.Body.String())

	mock.ExpectQuery("SELECT id, COALESCE\\(partner_id").
		WithArgs("msg-1").
		WillReturnRows(messageRows().AddRow("msg-1", "partner-greenstone", "application/xml", "834", "<xml>Farros</xml>", "accepted", "", 201, now))
	jsonResponse := httptest.NewRecorder()
	jsonRequest := httptest.NewRequest(http.MethodGet, "/x12/messages/msg-1/export?format=json", nil)
	jsonRequest.SetPathValue("id", "msg-1")
	app.exportMessage(jsonResponse, jsonRequest)
	assert.Equal(t, http.StatusOK, jsonResponse.Code)
	assert.Contains(t, jsonResponse.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, jsonResponse.Body.String(), `"id": "msg-1"`)

	missingExport := httptest.NewRecorder()
	missingExportRequest := httptest.NewRequest(http.MethodGet, "/x12/messages/missing/export", nil)
	missingExportRequest.SetPathValue("id", "missing")
	intakeApp{}.exportMessage(missingExport, missingExportRequest)
	assert.Equal(t, http.StatusNotFound, missingExport.Code)

	missingReplay := httptest.NewRecorder()
	missingReplayRequest := httptest.NewRequest(http.MethodPost, "/x12/messages/missing/replay", nil)
	missingReplayRequest.SetPathValue("id", "missing")
	intakeApp{}.replayMessage(missingReplay, missingReplayRequest)
	assert.Equal(t, http.StatusNotFound, missingReplay.Code)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReplayMessageReprocessesStoredXML(t *testing.T) {
	db, mock, cleanup := newIntakeMockDB(t)
	defer cleanup()
	now := time.Now()
	downstreamPaths := []string{}
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		downstreamPaths = append(downstreamPaths, r.URL.Path)
		return jsonResponse(http.StatusCreated, domain.Envelope{Lore: "replayed"})
	})}
	app := intakeApp{db: db, payerURL: "http://payer-core", client: client}

	mock.ExpectQuery("SELECT id, COALESCE\\(partner_id").
		WithArgs("msg-1").
		WillReturnRows(messageRows().AddRow("msg-1", "partner-greenstone", "application/xml", "834", `
<AshnX12Transaction type="834">
  <Sender id="partner-greenstone" />
  <Receiver id="Adventure Society" />
  <Enrollment>
    <Name>Replay Farros</Name>
    <Rank>Iron</Rank>
    <Guild>Grim Foundations</Guild>
    <Region>Greenstone</Region>
  </Enrollment>
</AshnX12Transaction>`, "accepted", "", 201, now))
	mock.ExpectExec("INSERT INTO inbound_messages").
		WithArgs(sqlmock.AnyArg(), "tp-greenstone-guild", "application/xml", "834", sqlmock.AnyArg(), "accepted", "", 201).
		WillReturnResult(sqlmock.NewResult(1, 1))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/x12/messages/msg-1/replay", nil)
	request.SetPathValue("id", "msg-1")
	app.replayMessage(response, request)

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, []string{"/enrollments", "/transactions"}, downstreamPaths)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestForwardHandlesRequestCreationDownstreamAndRejectedResponses(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", nil)
	response := httptest.NewRecorder()
	status, message := intakeApp{payerURL: "://bad"}.forward(response, request, http.MethodPost, "/claims", map[string]string{"ok": "true"})
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "request creation failed", message)

	response = httptest.NewRecorder()
	status, message = intakeApp{payerURL: "http://payer", client: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, assert.AnError
	})}}.forward(response, request, http.MethodPost, "/claims", map[string]string{"ok": "true"})
	assert.Equal(t, http.StatusBadGateway, status)
	assert.Equal(t, "payer-core unavailable", message)

	response = httptest.NewRecorder()
	status, message = intakeApp{payerURL: "http://payer", client: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadRequest, domain.ErrorEnvelope{Error: "bad request"})
	})}}.forward(response, request, http.MethodPost, "/claims", map[string]string{"ok": "true"})
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "payer-core rejected intake-derived request", message)
}

func TestRecord999HandlesRejectedPersistenceResponse(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "/transactions", r.URL.Path)
		return jsonResponse(http.StatusBadRequest, domain.ErrorEnvelope{Error: "rejected"})
	})}

	assert.NotPanics(t, func() {
		intakeApp{payerURL: "http://payer", client: client}.record999(httptest.NewRequest(http.MethodPost, "/x12/xml", nil), "related-1", "834", "partner-1", false, "bad")
	})
}

func TestAuditInboundMessagePersistsWhenDatabaseExists(t *testing.T) {
	db, mock, cleanup := newIntakeMockDB(t)
	defer cleanup()
	app := intakeApp{db: db}
	mock.ExpectExec("INSERT INTO inbound_messages").
		WithArgs(sqlmock.AnyArg(), "partner-1", "application/xml", "834", "<xml />", "accepted", "", 201).
		WillReturnResult(sqlmock.NewResult(1, 1))

	id := app.auditInboundMessage("application/xml", "partner-1", "834", "<xml />", "accepted", "", 201)

	assert.NotEmpty(t, id)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListMessagesDatabaseErrorReturnsEnvelope(t *testing.T) {
	db, mock, cleanup := newIntakeMockDB(t)
	defer cleanup()
	app := intakeApp{db: db}
	mock.ExpectQuery("SELECT id, COALESCE\\(partner_id").
		WillReturnError(assert.AnError)

	response := httptest.NewRecorder()
	app.listMessages(response, httptest.NewRequest(http.MethodGet, "/x12/messages", nil))

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.Equal(t, "message list failed", decodeEnvelope(t, response).Error)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLoadTradingPartnersReadsDatabaseAndSplitsCSV(t *testing.T) {
	db, mock, cleanup := newIntakeMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, name, sender_id, receiver_id, allowed_transaction_types").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "sender_id", "receiver_id", "allowed_transaction_types", "validation_profile", "route_target", "status"}).
			AddRow("tp-1", "Partner One", "sender-1", "Adventure Society", "834, 837, , 999", `{"attachmentTypes":["OZ"],"maxEmbeddedContentBytes":2048,"diagnosisCodes":["T509"],"procedureCodePrefixes":["ASHN"]}`, "payer-core", "active"))

	partners := loadTradingPartners(db)

	require.Len(t, partners, 1)
	assert.Equal(t, []string{"834", "837", "999"}, partners["tp-1"].AllowedTransactionTypes)
	assert.Equal(t, []string{"OZ"}, partners["tp-1"].ValidationProfile.AttachmentTypes)
	assert.Equal(t, []string{"T509"}, partners["tp-1"].ValidationProfile.DiagnosisCodes)
	assert.Equal(t, []string{"ASHN"}, partners["tp-1"].ValidationProfile.ProcedureCodePrefixes)
	assert.Equal(t, 2048, partners["tp-1"].ValidationProfile.MaxEmbeddedContentBytes)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, []string{"270", "837"}, splitCSV("270, ,837"))
}

func TestLoadTradingPartnersFallsBackOnDatabaseErrorAndOpenDBNoEnv(t *testing.T) {
	db, mock, cleanup := newIntakeMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, name, sender_id, receiver_id, allowed_transaction_types").
		WillReturnError(assert.AnError)
	assert.Len(t, loadTradingPartners(db), 3)
	require.NoError(t, mock.ExpectationsWereMet())

	emptyDB, emptyMock, emptyCleanup := newIntakeMockDB(t)
	defer emptyCleanup()
	emptyMock.ExpectQuery("SELECT id, name, sender_id, receiver_id, allowed_transaction_types").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "sender_id", "receiver_id", "allowed_transaction_types", "validation_profile", "route_target", "status"}))
	assert.Len(t, loadTradingPartners(emptyDB), 3)
	require.NoError(t, emptyMock.ExpectationsWereMet())

	t.Setenv("DATABASE_URL", "")
	assert.Nil(t, openDB())

	assert.Nil(t, openDBWith("dsn", func(_, _ string) (*sql.DB, error) {
		return nil, assert.AnError
	}))

	pingDB, pingMock, pingCleanup := newIntakeMockDBWithPing(t)
	defer pingCleanup()
	pingMock.ExpectPing().WillReturnError(assert.AnError)
	assert.Nil(t, openDBWith("dsn", func(_, _ string) (*sql.DB, error) {
		return pingDB, nil
	}))

	okDB, okMock, okCleanup := newIntakeMockDBWithPing(t)
	defer okCleanup()
	okMock.ExpectPing()
	assert.NotNil(t, openDBWith("dsn", func(driverName, dsn string) (*sql.DB, error) {
		assert.Equal(t, "postgres", driverName)
		assert.Equal(t, "dsn", dsn)
		return okDB, nil
	}))
	require.NoError(t, okMock.ExpectationsWereMet())
}

func TestFindMessageHandlesMissingRowsAndRecord999Guards(t *testing.T) {
	db, mock, cleanup := newIntakeMockDB(t)
	defer cleanup()
	app := intakeApp{db: db, payerURL: "://bad"}
	mock.ExpectQuery("SELECT id, COALESCE\\(partner_id").
		WithArgs("missing").
		WillReturnRows(messageRows())

	_, ok := app.findMessage("missing")
	assert.False(t, ok)
	request := httptest.NewRequest(http.MethodPost, "/x12/xml", nil)
	app.record999(request, "", "834", "", true, "")
	app.record999(request, "related-1", "834", "", true, "")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEDIHelpersParseFiltersPaginationAndValidation(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/x12/messages?limit=999&offset=-1&status=accepted&type=834&q=Farros", nil)
	page := parsePage(request, 25)
	filters := parseMessageFilters(request)
	assert.Equal(t, 100, page.Limit)
	assert.Equal(t, 0, page.Offset)
	assert.Equal(t, messageFilters{Status: "accepted", Type: "834", Q: "Farros"}, filters)

	items, pageInfo := trimFetchedPage([]int{1, 2, 3}, pageRequest{Limit: 2, Offset: 10})
	assert.Equal(t, []int{1, 2}, items)
	assert.True(t, pageInfo.HasMore)

	clauses, args := []string{}, []any{}
	addTextFilter(&clauses, &args, "status", "accepted")
	addSearchFilter(&clauses, &args, "Farros", "id", "raw_payload")
	assert.Equal(t, "SELECT * FROM inbound_messages WHERE "+strings.Join(clauses, " AND "), appendWhere("SELECT * FROM inbound_messages", clauses))
	assert.Len(t, args, 2)

	assert.True(t, validRegion("Greenstone"))
	assert.False(t, validRegion("Moon"))
	assert.True(t, validServiceType("curse-removal"))
	assert.False(t, validServiceType("vacation"))
	assert.True(t, isXMLContent("application/vnd.ashn+x12+xml; charset=utf-8"))
	assert.True(t, isJSONContent("application/vnd.ashn+x12+json"))
	assert.False(t, isJSONContent("text/plain"))
	parsed, err := parsePositiveInt64("AmountCents", "42")
	require.NoError(t, err)
	assert.Equal(t, int64(42), parsed)
	_, err = parsePositiveInt64("AmountCents", "0")
	assert.Error(t, err)
}

func TestEDIHealthEnvAndLogMiddleware(t *testing.T) {
	t.Setenv("EDI_TEST_ENV", "configured")
	assert.Equal(t, "configured", env("EDI_TEST_ENV", "fallback"))
	assert.Equal(t, "fallback", env("EDI_MISSING_ENV", "fallback"))

	response := httptest.NewRecorder()
	health(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "edi-intake")

	called := false
	handler := logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	logged := httptest.NewRecorder()
	handler.ServeHTTP(logged, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.True(t, called)
	assert.Equal(t, http.StatusNoContent, logged.Code)
}

func TestEDIOpenAPIIncludesIntakeRoutes(t *testing.T) {
	spec := ediOpenAPI()

	info := spec["info"].(map[string]string)
	assert.Equal(t, "ASHN EDI Intake", info["title"])
	paths := spec["paths"].(map[string]any)
	assert.Contains(t, paths, "/x12/transactions")
	assert.Contains(t, paths, "/x12/xml")
	assert.Contains(t, paths, "/x12/raw")
	assert.Contains(t, paths, "/x12/messages/rejections")
	assert.Contains(t, paths, "/x12/messages/{id}/replay")
	assert.Contains(t, paths, "/x12/trading-partners")
}

func newIntakeTestMux(app intakeApp) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /x12/transactions", app.acceptTransaction)
	mux.HandleFunc("POST /x12/xml", app.acceptXML)
	mux.HandleFunc("POST /x12/raw", app.acceptTransaction)
	mux.HandleFunc("GET /x12/messages", app.listMessages)
	mux.HandleFunc("GET /x12/messages/rejections", app.rejectionMetrics)
	mux.HandleFunc("GET /x12/messages/{id}/export", app.exportMessage)
	mux.HandleFunc("POST /x12/messages/{id}/replay", app.replayMessage)
	mux.HandleFunc("GET /x12/trading-partners", app.listTradingPartners)
	mux.HandleFunc("POST /x12/trading-partners", app.saveTradingPartner)
	mux.HandleFunc("PUT /x12/trading-partners/{id}", app.saveTradingPartner)
	mux.HandleFunc("DELETE /x12/trading-partners/{id}", app.deleteTradingPartner)
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

func newIntakeMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}

func newIntakeMockDBWithPing(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}

func messageRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "partner_id", "content_type", "transaction_type", "raw_payload", "status", "error", "downstream_status", "created_at"})
}

func raw837Fixture() string {
	return strings.Join([]string{
		"ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000001*0*T*:~",
		"GS*HC*provider-vitesse-temple*Adventure Society*20260708*1200*000000001*X*005010X837P~",
		"ST*837*000000001~",
		"BHT*0019*00*000000001*20260708*1200*CH~",
		"HL*1**20*1~",
		"NM1*41*2*provider-vitesse-temple*****46*provider-vitesse-temple~",
		"NM1*85*2*provider-vitesse-temple*****XX*provider-vitesse-temple~",
		"HL*2*1*22*0~",
		"NM1*IL*1*adv-raw-1****MI*adv-raw-1~",
		"CLM*claim-raw-1*1250.00***11:B:1*Y*A*Y*I~",
		"HI*ABK:S062X9A*ABF:T509~",
		"SV1*HC:ASHN1*950.00*UN*1***1~",
		"SV1*HC:ASHN2*300.00*UN*1***2~",
		"SE*13*000000001~",
		"GE*1*000000001~",
		"IEA*1*000000001~",
	}, "\n")
}

func raw275Fixture() string {
	return strings.Join([]string{
		"ISA*00*          *00*          *ZZ*provider-vitesse-temple*ZZ*Adventure Society*260708*1200*^*00501*000000002*0*T*:~",
		"GS*HC*provider-vitesse-temple*Adventure Society*20260708*1200*000000002*X*005010X275A1~",
		"ST*275*000000002~",
		"BHT*0019*00*000000002*20260708*1200*CH~",
		"TRN*1*tx-raw-275*provider-vitesse-temple~",
		"HL*1**20*1~",
		"NM1*1P*2*provider-vitesse-temple*****XX*provider-vitesse-temple~",
		"HL*2*1*22*0~",
		"NM1*IL*1*adv-raw-1****MI*adv-raw-1~",
		"REF*1K*claim-raw-1~",
		"REF*6R*ATTACH-RAW-1~",
		"REF*F8*packet-raw-1-OF-1~",
		"PWK*B4*EL***AC*ATTACH-RAW-1~",
		"LQ*AT*OZ~",
		"K3*Content-Type: text/plain~",
		"NTE*ADD*Raw resurrection notes~",
		"BIN*38*Patient survived raw X12 dragonfire.~",
		"SE*18*000000002~",
		"GE*1*000000002~",
		"IEA*1*000000002~",
	}, "\n")
}
