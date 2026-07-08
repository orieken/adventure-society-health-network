package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"ashn/packages/domain"
	edimock "ashn/packages/edi-mock"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEnvelope struct {
	Data         json.RawMessage      `json:"data"`
	Lore         string               `json:"lore"`
	Transaction  *domain.Transaction  `json:"transaction"`
	Transactions []domain.Transaction `json:"transactions"`
	Page         *domain.PageInfo     `json:"page"`
	Error        string               `json:"error"`
}

func TestEnrollCreatesActiveAdventurerAnd834(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	response := serveJSON(t, mux, http.MethodPost, "/enrollments", domain.EnrollmentRequest{
		Name:   "Farros",
		Rank:   domain.RankIron,
		Guild:  "Grim Foundations",
		Region: domain.RegionGreenstone,
	})

	assert.Equal(t, http.StatusCreated, response.Code)
	envelope := decodeEnvelope(t, response)
	require.NotNil(t, envelope.Transaction)
	assert.Equal(t, domain.Tx834, envelope.Transaction.Type)
	assert.Equal(t, domain.TxStatusAccepted, envelope.Transaction.Status)
	assert.NotEmpty(t, envelope.Lore)

	var adventurer domain.Adventurer
	require.NoError(t, json.Unmarshal(envelope.Data, &adventurer))
	assert.Equal(t, domain.CoverageActive, adventurer.CoverageStatus)
	assert.Equal(t, adventurer, app.adventurers[adventurer.ID])
}

func TestEligibilityReturns270And271Pair(t *testing.T) {
	app := newTestStore()
	adventurer := domain.Adventurer{ID: "adv-1", Name: "Farros", Rank: domain.RankIron, Guild: "Grim Foundations", Region: domain.RegionGreenstone, CoverageStatus: domain.CoverageActive}
	app.adventurers[adventurer.ID] = adventurer
	mux := newPayerTestMux(app)

	response := serveJSON(t, mux, http.MethodPost, "/eligibility/query", domain.EligibilityRequest{
		AdventurerID: adventurer.ID,
		ProviderID:   "provider-vitesse-temple",
	})

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeEnvelope(t, response)
	require.Len(t, envelope.Transactions, 2)
	assert.Equal(t, domain.Tx270, envelope.Transactions[0].Type)
	assert.Equal(t, domain.TxStatusDispatched, envelope.Transactions[0].Status)
	assert.Equal(t, domain.Tx271, envelope.Transactions[1].Type)
	assert.Equal(t, domain.TxStatusAccepted, envelope.Transactions[1].Status)
	assert.Len(t, app.transactions, 2)
}

func TestGetAdventurerReturnsDetailAndNotFound(t *testing.T) {
	app := newTestStore()
	app.adventurers["adv-1"] = domain.Adventurer{ID: "adv-1", Name: "Farros", Rank: domain.RankIron}
	mux := newPayerTestMux(app)

	found := httptest.NewRecorder()
	mux.ServeHTTP(found, httptest.NewRequest(http.MethodGet, "/adventurers/adv-1", nil))
	assert.Equal(t, http.StatusOK, found.Code)
	var adventurer domain.Adventurer
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, found).Data, &adventurer))
	assert.Equal(t, "Farros", adventurer.Name)

	missing := httptest.NewRecorder()
	mux.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/adventurers/missing", nil))
	assert.Equal(t, http.StatusNotFound, missing.Code)
	assert.Equal(t, "adventurer not found", decodeEnvelope(t, missing).Error)
}

func TestAuthRequestQueuesPending278(t *testing.T) {
	app := newTestStore()
	app.adventurers["adv-1"] = domain.Adventurer{ID: "adv-1", Name: "Farros", CoverageStatus: domain.CoverageActive}
	mux := newPayerTestMux(app)

	response := serveJSON(t, mux, http.MethodPost, "/auth-requests", domain.PriorAuthRequest{
		AdventurerID:     "adv-1",
		ProviderID:       "provider-vitesse-temple",
		ServiceType:      "resurrection",
		IncidentSeverity: domain.SeverityDiamond,
	})

	assert.Equal(t, http.StatusAccepted, response.Code)
	envelope := decodeEnvelope(t, response)
	require.NotNil(t, envelope.Transaction)
	assert.Equal(t, domain.Tx278, envelope.Transaction.Type)
	assert.Equal(t, domain.TxStatusPending, envelope.Transaction.Status)
	assert.Contains(t, app.transactions, envelope.Transaction.ID)
}

func TestDecideAuthorizationUpdates278Status(t *testing.T) {
	app := newTestStore()
	tx := edimock.Generate278Request(
		domain.Adventurer{ID: "adv-1", Name: "Farros"},
		domain.Provider{ID: "provider-vitesse-temple", Name: "Temple"},
		"resurrection",
	)
	tx.ID = "tx-278"
	tx.RawX12 = strings.ReplaceAll(tx.RawX12, "ST*278*", "ST*278*tx-278")
	app.transactions["tx-278"] = tx
	mux := newPayerTestMux(app)

	response := serveJSON(t, mux, http.MethodPost, "/auth-requests/tx-278/decision", domain.AuthorizationDecisionRequest{
		Decision: "Approved",
		Reason:   "medical necessity confirmed",
	})

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeEnvelope(t, response)
	require.NotNil(t, envelope.Transaction)
	assert.Equal(t, domain.TxStatusApproved, envelope.Transaction.Status)
	assert.Contains(t, envelope.Transaction.RawX12, "HCR*A1")
	assert.Equal(t, domain.TxStatusApproved, app.transactions["tx-278"].Status)

	invalid := serveJSON(t, mux, http.MethodPost, "/auth-requests/tx-278/decision", domain.AuthorizationDecisionRequest{Decision: "Maybe"})
	assert.Equal(t, http.StatusBadRequest, invalid.Code)

	app.transactions["tx-837"] = domain.Transaction{ID: "tx-837", Type: domain.Tx837, Status: domain.TxStatusAccepted}
	wrongType := serveJSON(t, mux, http.MethodPost, "/auth-requests/tx-837/decision", domain.AuthorizationDecisionRequest{Decision: "Denied"})
	assert.Equal(t, http.StatusBadRequest, wrongType.Code)
}

func TestEligibilityMissingReferencesReturnErrors(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	missingAdventurer := serveJSON(t, mux, http.MethodPost, "/eligibility/query", domain.EligibilityRequest{
		AdventurerID: "missing",
		ProviderID:   "provider-vitesse-temple",
	})
	assert.Equal(t, http.StatusNotFound, missingAdventurer.Code)
	assert.Equal(t, "adventurer not found", decodeEnvelope(t, missingAdventurer).Error)

	app.adventurers["adv-1"] = domain.Adventurer{ID: "adv-1", Name: "Farros"}
	missingProvider := serveJSON(t, mux, http.MethodPost, "/eligibility/query", domain.EligibilityRequest{
		AdventurerID: "adv-1",
		ProviderID:   "missing",
	})
	assert.Equal(t, http.StatusNotFound, missingProvider.Code)
	assert.Equal(t, "provider not found", decodeEnvelope(t, missingProvider).Error)
}

func TestClaimPaymentUpdatesClaimAndEmits835(t *testing.T) {
	app := newTestStore()
	adventurer := domain.Adventurer{ID: "adv-1", Name: "Farros", Rank: domain.RankIron, Guild: "Grim Foundations", Region: domain.RegionGreenstone, CoverageStatus: domain.CoverageActive}
	app.adventurers[adventurer.ID] = adventurer
	mux := newPayerTestMux(app)

	claimResponse := serveJSON(t, mux, http.MethodPost, "/claims", domain.ClaimRequest{
		AdventurerID:     adventurer.ID,
		ProviderID:       "provider-vitesse-temple",
		IncidentSeverity: domain.SeverityAwakened,
		AmountCents:      125000,
	})
	require.Equal(t, http.StatusCreated, claimResponse.Code)
	claimEnvelope := decodeEnvelope(t, claimResponse)
	var claim domain.Claim
	require.NoError(t, json.Unmarshal(claimEnvelope.Data, &claim))
	require.Len(t, claimEnvelope.Transactions, 2)
	assert.Equal(t, domain.Tx837, claimEnvelope.Transactions[0].Type)
	assert.Equal(t, domain.Tx277CA, claimEnvelope.Transactions[1].Type)
	assert.Equal(t, claimEnvelope.Transactions[0].ID, claimEnvelope.Transactions[1].RelatedID)

	paymentResponse := serveJSON(t, mux, http.MethodPost, "/claims/"+claim.ID+"/payment", domain.PaymentRequest{PaymentAmountCents: 100000})

	assert.Equal(t, http.StatusOK, paymentResponse.Code)
	paymentEnvelope := decodeEnvelope(t, paymentResponse)
	require.NotNil(t, paymentEnvelope.Transaction)
	assert.Equal(t, domain.Tx835, paymentEnvelope.Transaction.Type)
	assert.Equal(t, domain.TxStatusPaid, paymentEnvelope.Transaction.Status)
	assert.Equal(t, domain.ClaimPaid, app.claims[claim.ID].Status)
}

func TestPayClaimMissingClaimReturnsError(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	response := serveJSON(t, mux, http.MethodPost, "/claims/missing/payment", domain.PaymentRequest{PaymentAmountCents: 100000})

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "claim not found", decodeEnvelope(t, response).Error)
}

func TestAttachClaimInformationEmits275Transaction(t *testing.T) {
	app := newTestStore()
	app.claims["claim-1"] = domain.Claim{
		ID:            "claim-1",
		AdventurerID:  "adv-1",
		ProviderID:    "provider-vitesse-temple",
		TransactionID: "tx-837",
		Status:        domain.ClaimSubmitted,
	}
	mux := newPayerTestMux(app)

	response := serveJSON(t, mux, http.MethodPost, "/claims/claim-1/attachments", domain.AttachmentRequest{
		AttachmentType:          "OZ",
		AttachmentControlNumber: "ATTACH-1",
		ReportTypeCode:          "B4",
		TransmissionCode:        "EL",
		ContentType:             "text/plain",
		Description:             "Resurrection notes",
		Content:                 "Patient survived a dragonfire incident.",
	})

	assert.Equal(t, http.StatusCreated, response.Code)
	envelope := decodeEnvelope(t, response)
	require.NotNil(t, envelope.Transaction)
	assert.Equal(t, domain.Tx275, envelope.Transaction.Type)
	assert.Equal(t, "tx-837", envelope.Transaction.RelatedID)
	assert.Contains(t, envelope.Transaction.RawX12, "ST*275")
	assert.Contains(t, envelope.Transaction.RawX12, "PWK*B4*EL***AC*ATTACH-1")
	assert.Contains(t, envelope.Transaction.RawX12, "LQ*AT*OZ")
}

func TestAttachClaimInformationClearsDocumentationHold(t *testing.T) {
	app := newTestStore()
	app.claims["claim-1"] = domain.Claim{
		ID:            "claim-1",
		AdventurerID:  "adv-1",
		ProviderID:    "provider-vitesse-temple",
		TransactionID: "tx-837",
		Status:        domain.ClaimPendingDocumentation,
	}
	mux := newPayerTestMux(app)

	response := serveJSON(t, mux, http.MethodPost, "/claims/claim-1/attachments", domain.AttachmentRequest{
		AttachmentType:          "OZ",
		AttachmentControlNumber: "ATTACH-1",
		ReportTypeCode:          "B4",
		TransmissionCode:        "EL",
		ContentType:             "text/plain",
		Description:             "Resurrection notes",
		Content:                 "Patient survived a dragonfire incident.",
	})

	assert.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, domain.ClaimPending, app.claims["claim-1"].Status)
	assert.Contains(t, string(decodeEnvelope(t, response).Data), string(domain.ClaimPending))
}

func TestAttachClaimInformationValidatesClaimAndRequiredFields(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	missingClaim := serveJSON(t, mux, http.MethodPost, "/claims/missing/attachments", domain.AttachmentRequest{
		AttachmentType: "OZ", AttachmentControlNumber: "ATTACH-1", ReportTypeCode: "B4", TransmissionCode: "EL", ContentType: "text/plain", Description: "notes", Content: "content",
	})
	assert.Equal(t, http.StatusNotFound, missingClaim.Code)

	app.claims["claim-1"] = domain.Claim{ID: "claim-1", ProviderID: "provider-vitesse-temple"}
	invalid := serveJSON(t, mux, http.MethodPost, "/claims/claim-1/attachments", domain.AttachmentRequest{
		AttachmentType: "OZ",
	})
	assert.Equal(t, http.StatusBadRequest, invalid.Code)
	assert.Equal(t, "invalid attachment", decodeEnvelope(t, invalid).Error)

	disallowed := serveJSON(t, mux, http.MethodPost, "/claims/claim-1/attachments", domain.AttachmentRequest{
		AttachmentType:          "PN",
		AttachmentControlNumber: "BAD-1",
		ReportTypeCode:          "03",
		TransmissionCode:        "EL",
		ContentType:             "application/pdf",
		Description:             "notes",
		Content:                 "content",
	})
	assert.Equal(t, http.StatusBadRequest, disallowed.Code)
	assert.Contains(t, decodeEnvelope(t, disallowed).Lore, "attachment type PN is not allowed")
}

func TestGetClaimReturnsClaimDetail(t *testing.T) {
	app := newTestStore()
	app.claims["claim-1"] = domain.Claim{ID: "claim-1", AdventurerID: "adv-1", ProviderID: "provider-vitesse-temple", IncidentSeverity: domain.SeverityAwakened, AmountCents: 125000, Status: domain.ClaimSubmitted}
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/claims/claim-1", nil)
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeEnvelope(t, response)
	var claim domain.Claim
	require.NoError(t, json.Unmarshal(envelope.Data, &claim))
	assert.Equal(t, "claim-1", claim.ID)
	assert.Equal(t, domain.ClaimSubmitted, claim.Status)
	assert.NotEmpty(t, envelope.Lore)
}

func TestGetTransactionReturnsTransactionDetail(t *testing.T) {
	app := newTestStore()
	app.transactions["tx-1"] = domain.Transaction{ID: "tx-1", Type: domain.Tx837, Status: domain.TxStatusAccepted, CreatedAt: time.Now()}
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/transactions/tx-1", nil)
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeEnvelope(t, response)
	require.NotNil(t, envelope.Transaction)
	assert.Equal(t, "tx-1", envelope.Transaction.ID)
	assert.Equal(t, domain.Tx837, envelope.Transaction.Type)
}

func TestGetTransactionMissingReturnsError(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/transactions/missing", nil))

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "transaction not found", decodeEnvelope(t, response).Error)
}

func TestExportAndReplayTransaction(t *testing.T) {
	app := newTestStore()
	tx := domain.Transaction{ID: "tx-1", Type: domain.Tx837, Status: domain.TxStatusAccepted, SenderID: "provider-vitesse-temple", ReceiverID: "Adventure Society", Payload: domain.Payload(map[string]string{"claimId": "claim-1"}), RawX12: "ST*837*tx-1~", CreatedAt: time.Now()}
	app.transactions[tx.ID] = tx
	mux := newPayerTestMux(app)

	exportResponse := httptest.NewRecorder()
	mux.ServeHTTP(exportResponse, httptest.NewRequest(http.MethodGet, "/transactions/tx-1/export?format=x12", nil))
	assert.Equal(t, http.StatusOK, exportResponse.Code)
	assert.Equal(t, "ST*837*tx-1~", exportResponse.Body.String())
	assert.Contains(t, exportResponse.Header().Get("Content-Disposition"), ".x12")

	replayResponse := httptest.NewRecorder()
	mux.ServeHTTP(replayResponse, httptest.NewRequest(http.MethodPost, "/transactions/tx-1/replay", nil))
	assert.Equal(t, http.StatusCreated, replayResponse.Code)
	envelope := decodeEnvelope(t, replayResponse)
	require.NotNil(t, envelope.Transaction)
	assert.NotEqual(t, "tx-1", envelope.Transaction.ID)
	assert.Equal(t, "tx-1", envelope.Transaction.RelatedID)
	assert.Contains(t, app.transactions, envelope.Transaction.ID)
}

func TestExportTransactionSupportsXMLJSONAndMissingTransaction(t *testing.T) {
	app := newTestStore()
	tx := domain.Transaction{
		ID: "tx-1", Type: domain.Tx837, Status: domain.TxStatusAccepted, SenderID: `provider&"one"`,
		ReceiverID: "Adventure Society", RelatedID: "related-1", Payload: domain.Payload(map[string]string{"claimId": "claim-1"}),
		RawX12: "ST*837*tx-1~", CreatedAt: time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
	}
	app.transactions[tx.ID] = tx
	mux := newPayerTestMux(app)

	xmlResponse := httptest.NewRecorder()
	mux.ServeHTTP(xmlResponse, httptest.NewRequest(http.MethodGet, "/transactions/tx-1/export?format=xml", nil))
	assert.Equal(t, http.StatusOK, xmlResponse.Code)
	assert.Contains(t, xmlResponse.Header().Get("Content-Type"), "application/xml")
	assert.Contains(t, xmlResponse.Body.String(), `provider&amp;&quot;one&quot;`)
	assert.Contains(t, xmlResponse.Body.String(), "<RawX12><![CDATA[ST*837*tx-1~]]></RawX12>")

	jsonResponse := httptest.NewRecorder()
	mux.ServeHTTP(jsonResponse, httptest.NewRequest(http.MethodGet, "/transactions/tx-1/export", nil))
	assert.Equal(t, http.StatusOK, jsonResponse.Code)
	assert.Contains(t, jsonResponse.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, jsonResponse.Body.String(), `"id": "tx-1"`)

	missingResponse := httptest.NewRecorder()
	mux.ServeHTTP(missingResponse, httptest.NewRequest(http.MethodGet, "/transactions/missing/export", nil))
	assert.Equal(t, http.StatusNotFound, missingResponse.Code)
}

func TestReplayMissingTransactionReturnsError(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/transactions/missing/replay", nil))

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "transaction not found", decodeEnvelope(t, response).Error)
}

func TestRecordTransactionAssignsMissingFields(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	response := serveJSON(t, mux, http.MethodPost, "/transactions", domain.Transaction{
		Type: domain.Tx999, Status: domain.TxStatusAccepted, SenderID: "edi", ReceiverID: "provider",
	})

	assert.Equal(t, http.StatusCreated, response.Code)
	envelope := decodeEnvelope(t, response)
	require.NotNil(t, envelope.Transaction)
	assert.NotEmpty(t, envelope.Transaction.ID)
	assert.False(t, envelope.Transaction.CreatedAt.IsZero())
	assert.Contains(t, app.transactions, envelope.Transaction.ID)
}

func TestMissingClaimReturnsErrorEnvelope(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/claims/missing/status", nil)
	mux.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNotFound, response.Code)
	envelope := decodeEnvelope(t, response)
	assert.Equal(t, "claim not found", envelope.Error)
	assert.NotEmpty(t, envelope.Lore)
}

func TestClaimStatusReturns276And277Pair(t *testing.T) {
	app := newTestStore()
	app.claims["claim-1"] = domain.Claim{ID: "claim-1", Status: domain.ClaimPaid}
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/claims/claim-1/status", nil))

	assert.Equal(t, http.StatusOK, response.Code)
	envelope := decodeEnvelope(t, response)
	require.Len(t, envelope.Transactions, 2)
	assert.Equal(t, domain.Tx276, envelope.Transactions[0].Type)
	assert.Equal(t, domain.Tx277, envelope.Transactions[1].Type)
}

func TestRequestClaimDocumentationMarksClaimAndEmits277(t *testing.T) {
	app := newTestStore()
	app.claims["claim-1"] = domain.Claim{ID: "claim-1", AdventurerID: "adv-1", ProviderID: "provider-vitesse-temple", TransactionID: "tx-837", Status: domain.ClaimPending}
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/claims/claim-1/documentation-request", nil))

	assert.Equal(t, http.StatusAccepted, response.Code)
	envelope := decodeEnvelope(t, response)
	require.NotNil(t, envelope.Transaction)
	assert.Equal(t, domain.Tx277, envelope.Transaction.Type)
	assert.Equal(t, "tx-837", envelope.Transaction.RelatedID)
	assert.Equal(t, domain.ClaimPendingDocumentation, app.claims["claim-1"].Status)
	assert.Contains(t, string(envelope.Transaction.Payload), "documentationRequest")
}

func TestRequestClaimDocumentationMissingClaim(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/claims/missing/documentation-request", nil))

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Equal(t, "claim not found", decodeEnvelope(t, response).Error)
}

func TestRequestClaimDocumentationPersistsWithDatabase(t *testing.T) {
	db, mock, cleanup := newPayerMockDB(t)
	defer cleanup()
	app := &store{db: db, claims: map[string]domain.Claim{}, transactions: map[string]domain.Transaction{}}
	mux := newPayerTestMux(app)

	mock.ExpectQuery("SELECT id, adventurer_id, provider_id, incident_severity").
		WithArgs("claim-1").
		WillReturnRows(claimRows().AddRow("claim-1", "adv-1", "provider-vitesse-temple", domain.SeverityAwakened, "tx-837", int64(125000), int64(0), int64(0), int64(0), int64(0), "", "", domain.ClaimPending))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE claims SET status = $1 WHERE id = $2`)).
		WithArgs(string(domain.ClaimPendingDocumentation), "claim-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO transactions").
		WillReturnResult(sqlmock.NewResult(0, 1))

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/claims/claim-1/documentation-request", nil))

	assert.Equal(t, http.StatusAccepted, response.Code)
	assert.Equal(t, domain.ClaimPendingDocumentation, app.claims["claim-1"].Status)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPayerRejectsInvalidJSON(t *testing.T) {
	app := newTestStore()
	mux := newPayerTestMux(app)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/enrollments", bytes.NewReader([]byte("{"))))

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, "invalid json", decodeEnvelope(t, response).Error)
}

func TestListEndpointsReturnPersistedMemory(t *testing.T) {
	app := newTestStore()
	app.adventurers["adv-1"] = domain.Adventurer{ID: "adv-1", Name: "Farros", Rank: domain.RankIron, Guild: "Grim Foundations", Region: domain.RegionGreenstone, CoverageStatus: domain.CoverageActive}
	app.claims["claim-1"] = domain.Claim{ID: "claim-1", AdventurerID: "adv-1", ProviderID: "provider-vitesse-temple", IncidentSeverity: domain.SeverityAwakened, AmountCents: 125000, Status: domain.ClaimSubmitted}
	app.transactions["tx-old"] = domain.Transaction{ID: "tx-old", Type: domain.Tx834, Status: domain.TxStatusAccepted, CreatedAt: time.Now().Add(-time.Hour)}
	app.transactions["tx-new"] = domain.Transaction{ID: "tx-new", Type: domain.Tx837, Status: domain.TxStatusAccepted, CreatedAt: time.Now()}
	mux := newPayerTestMux(app)

	adventurersResponse := httptest.NewRecorder()
	mux.ServeHTTP(adventurersResponse, httptest.NewRequest(http.MethodGet, "/adventurers?limit=10", nil))
	assert.Equal(t, http.StatusOK, adventurersResponse.Code)
	var adventurers []domain.Adventurer
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, adventurersResponse).Data, &adventurers))
	assert.Len(t, adventurers, 1)

	claimsResponse := httptest.NewRecorder()
	mux.ServeHTTP(claimsResponse, httptest.NewRequest(http.MethodGet, "/claims?limit=10", nil))
	assert.Equal(t, http.StatusOK, claimsResponse.Code)
	var claims []domain.Claim
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, claimsResponse).Data, &claims))
	assert.Len(t, claims, 1)

	transactionsResponse := httptest.NewRecorder()
	mux.ServeHTTP(transactionsResponse, httptest.NewRequest(http.MethodGet, "/transactions?limit=1", nil))
	assert.Equal(t, http.StatusOK, transactionsResponse.Code)
	var transactions []domain.Transaction
	require.NoError(t, json.Unmarshal(decodeEnvelope(t, transactionsResponse).Data, &transactions))
	require.Len(t, transactions, 1)
	assert.Equal(t, "tx-new", transactions[0].ID)
	assert.Equal(t, 1, decodeEnvelope(t, transactionsResponse).Page.Count)
}

func TestListEndpointsApplyFiltersAndPagination(t *testing.T) {
	app := newTestStore()
	now := time.Now()
	app.adventurers["adv-1"] = domain.Adventurer{ID: "adv-1", Name: "Farros", Rank: domain.RankIron, Guild: "Grim Foundations", Region: domain.RegionGreenstone, CoverageStatus: domain.CoverageActive}
	app.adventurers["adv-2"] = domain.Adventurer{ID: "adv-2", Name: "Aldrion", Rank: domain.RankGold, Guild: "Cloud Palace", Region: domain.RegionVitesse, CoverageStatus: domain.CoveragePending}
	app.claims["claim-paid"] = domain.Claim{ID: "claim-paid", AdventurerID: "adv-1", ProviderID: "provider-vitesse-temple", IncidentSeverity: domain.SeverityAwakened, AmountCents: 125000, Status: domain.ClaimPaid}
	app.claims["claim-open"] = domain.Claim{ID: "claim-open", AdventurerID: "adv-2", ProviderID: "provider-greenstone-clinic", IncidentSeverity: domain.SeverityNormal, AmountCents: 50000, Status: domain.ClaimSubmitted}
	app.transactions["tx-837-new"] = domain.Transaction{ID: "tx-837-new", Type: domain.Tx837, Status: domain.TxStatusAccepted, SenderID: "adv-1", ReceiverID: "provider-vitesse-temple", Payload: domain.Payload(map[string]string{"claim": "paid"}), CreatedAt: now}
	app.transactions["tx-837-old"] = domain.Transaction{ID: "tx-837-old", Type: domain.Tx837, Status: domain.TxStatusAccepted, SenderID: "adv-1", ReceiverID: "provider-vitesse-temple", Payload: domain.Payload(map[string]string{"claim": "old"}), CreatedAt: now.Add(-time.Minute)}
	app.transactions["tx-834"] = domain.Transaction{ID: "tx-834", Type: domain.Tx834, Status: domain.TxStatusAccepted, SenderID: "adv-2", ReceiverID: societyID, CreatedAt: now.Add(-2 * time.Minute)}
	mux := newPayerTestMux(app)

	adventurersResponse := httptest.NewRecorder()
	mux.ServeHTTP(adventurersResponse, httptest.NewRequest(http.MethodGet, "/adventurers?limit=10&q=farros&rank=Iron&region=Greenstone&coverageStatus=Active", nil))
	require.Equal(t, http.StatusOK, adventurersResponse.Code)
	adventurersEnvelope := decodeEnvelope(t, adventurersResponse)
	var adventurers []domain.Adventurer
	require.NoError(t, json.Unmarshal(adventurersEnvelope.Data, &adventurers))
	require.Len(t, adventurers, 1)
	assert.Equal(t, "adv-1", adventurers[0].ID)
	require.NotNil(t, adventurersEnvelope.Page)
	assert.False(t, adventurersEnvelope.Page.HasMore)

	claimsResponse := httptest.NewRecorder()
	mux.ServeHTTP(claimsResponse, httptest.NewRequest(http.MethodGet, "/claims?limit=10&status=Paid&providerId=provider-vitesse-temple&adventurerId=adv-1&severity=Awakened&q=paid", nil))
	require.Equal(t, http.StatusOK, claimsResponse.Code)
	claimsEnvelope := decodeEnvelope(t, claimsResponse)
	var claims []domain.Claim
	require.NoError(t, json.Unmarshal(claimsEnvelope.Data, &claims))
	require.Len(t, claims, 1)
	assert.Equal(t, "claim-paid", claims[0].ID)

	transactionsResponse := httptest.NewRecorder()
	mux.ServeHTTP(transactionsResponse, httptest.NewRequest(http.MethodGet, "/transactions?limit=1&offset=1&type=837&status=Accepted&q=provider-vitesse", nil))
	require.Equal(t, http.StatusOK, transactionsResponse.Code)
	transactionsEnvelope := decodeEnvelope(t, transactionsResponse)
	var transactions []domain.Transaction
	require.NoError(t, json.Unmarshal(transactionsEnvelope.Data, &transactions))
	require.Len(t, transactions, 1)
	assert.Equal(t, "tx-837-old", transactions[0].ID)
	require.NotNil(t, transactionsEnvelope.Page)
	assert.Equal(t, 1, transactionsEnvelope.Page.Offset)
	assert.False(t, transactionsEnvelope.Page.HasMore)
}

func TestPayerOpenAPIIncludesWorkflowRoutes(t *testing.T) {
	spec := payerOpenAPI()

	info := spec["info"].(map[string]string)
	assert.Equal(t, "ASHN Payer Core", info["title"])
	paths := spec["paths"].(map[string]any)
	assert.Contains(t, paths, "/enrollments")
	assert.Contains(t, paths, "/claims/{id}/payment")
	assert.Contains(t, paths, "/transactions/{id}/replay")
}

func TestPayerHealthAndPaginationHelpers(t *testing.T) {
	healthResponse := httptest.NewRecorder()
	health(healthResponse, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, healthResponse.Code)
	assert.Contains(t, healthResponse.Body.String(), "payer-core")

	request := httptest.NewRequest(http.MethodGet, "/items?limit=999&offset=-10", nil)
	page := parsePage(request, 25)
	assert.Equal(t, 100, page.Limit)
	assert.Equal(t, 0, page.Offset)

	trimmed, pageInfo := trimFetchedPage([]int{1, 2, 3}, pageRequest{Limit: 2, Offset: 10})
	assert.Equal(t, []int{1, 2}, trimmed)
	assert.True(t, pageInfo.HasMore)

	assert.Equal(t, "SELECT * FROM claims", appendWhere("SELECT * FROM claims", nil))
	clauses, args := []string{}, []any{}
	addTextFilter(&clauses, &args, "status", "Paid")
	addSearchFilter(&clauses, &args, "farros", "id", "name")
	assert.Len(t, clauses, 2)
	assert.Len(t, args, 2)

	emptyPage, emptyPageInfo := paginate([]int{1}, pageRequest{Limit: 2, Offset: 10})
	assert.Empty(t, emptyPage)
	assert.Equal(t, 0, emptyPageInfo.Count)

	assert.False(t, sameFold(" Paid ", "Denied"))
	assert.False(t, containsAny("needle", "hay", "stack"))
}

func TestPayerFiltersExcludeNonMatches(t *testing.T) {
	assert.Empty(t, filterAdventurers([]domain.Adventurer{{ID: "adv-1", Rank: domain.RankGold}}, adventurerFilters{Rank: "Iron"}))
	assert.Empty(t, filterAdventurers([]domain.Adventurer{{ID: "adv-1", Region: domain.RegionVitesse}}, adventurerFilters{Region: "Greenstone"}))
	assert.Empty(t, filterAdventurers([]domain.Adventurer{{ID: "adv-1", CoverageStatus: domain.CoverageSuspended}}, adventurerFilters{CoverageStatus: "Active"}))
	assert.Empty(t, filterAdventurers([]domain.Adventurer{{ID: "adv-1", Name: "Farros"}}, adventurerFilters{Q: "Aldrion"}))

	assert.Empty(t, filterClaims([]domain.Claim{{ID: "claim-1", Status: domain.ClaimDenied}}, claimFilters{Status: "Paid"}))
	assert.Empty(t, filterClaims([]domain.Claim{{ID: "claim-1", ProviderID: "provider-1"}}, claimFilters{ProviderID: "provider-2"}))
	assert.Empty(t, filterClaims([]domain.Claim{{ID: "claim-1", AdventurerID: "adv-1"}}, claimFilters{AdventurerID: "adv-2"}))
	assert.Empty(t, filterClaims([]domain.Claim{{ID: "claim-1", IncidentSeverity: domain.SeverityNormal}}, claimFilters{Severity: "Diamond"}))
	assert.Empty(t, filterClaims([]domain.Claim{{ID: "claim-1"}}, claimFilters{Q: "missing"}))

	assert.Empty(t, filterTransactions([]domain.Transaction{{ID: "tx-1", Type: domain.Tx834}}, transactionFilters{Type: "837"}))
	assert.Empty(t, filterTransactions([]domain.Transaction{{ID: "tx-1", Status: domain.TxStatusDenied}}, transactionFilters{Status: "Accepted"}))
	assert.Empty(t, filterTransactions([]domain.Transaction{{ID: "tx-1"}}, transactionFilters{Q: "missing"}))
}

func TestPayerEnvLogMiddlewareEmbeddedWorkerAndMigration(t *testing.T) {
	t.Setenv("PAYER_TEST_ENV", "configured")
	assert.Equal(t, "configured", env("PAYER_TEST_ENV", "fallback"))
	assert.Equal(t, "fallback", env("PAYER_MISSING_ENV", "fallback"))

	called := false
	handler := logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.True(t, called)
	assert.Equal(t, http.StatusNoContent, response.Code)

	assert.NotPanics(t, func() { runEmbeddedWorker(nil) })

	migrationPath := filepath.Join(t.TempDir(), "migration.sql")
	require.NoError(t, os.WriteFile(migrationPath, []byte("SELECT 1;"), 0o600))
	t.Setenv("ASHN_MIGRATION_PATH", migrationPath)
	db, mock, cleanup := newPayerMockDB(t)
	defer cleanup()
	mock.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(0, 1))
	applyMigration(db)
	require.NoError(t, mock.ExpectationsWereMet())

	t.Setenv("DATABASE_URL", "")
	assert.Nil(t, openDB())

	assert.Nil(t, openDBWith("dsn", func(_, _ string) (*sql.DB, error) {
		return nil, assert.AnError
	}))

	pingDB, pingMock, pingCleanup := newPayerMockDBWithPing(t)
	defer pingCleanup()
	pingMock.ExpectPing().WillReturnError(assert.AnError)
	assert.Nil(t, openDBWith("dsn", func(_, _ string) (*sql.DB, error) {
		return pingDB, nil
	}))

	okDB, okMock, okCleanup := newPayerMockDBWithPing(t)
	defer okCleanup()
	okMock.ExpectPing()
	assert.NotNil(t, openDBWith("dsn", func(driverName, dsn string) (*sql.DB, error) {
		assert.Equal(t, "postgres", driverName)
		assert.Equal(t, "dsn", dsn)
		return okDB, nil
	}))
	require.NoError(t, okMock.ExpectationsWereMet())

	missingPath := filepath.Join(t.TempDir(), "missing.sql")
	t.Setenv("ASHN_MIGRATION_PATH", missingPath)
	assert.NotPanics(t, func() { applyMigration(nil) })
}

func TestPayerLoadersReadFromDatabase(t *testing.T) {
	db, mock, cleanup := newPayerMockDB(t)
	defer cleanup()
	now := time.Now()

	mock.ExpectQuery("SELECT id, name, provider_type, tier_rank, region FROM providers").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "provider_type", "tier_rank", "region"}).
			AddRow("provider-1", "Provider One", domain.ProviderTypeClinic, domain.RankGold, domain.RegionVitesse))
	providers := loadProviders(db)
	require.Len(t, providers, 1)
	assert.Equal(t, "Provider One", providers["provider-1"].Name)

	mock.ExpectQuery("SELECT id, name, rank, guild, region, coverage_status FROM adventurers").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rank", "guild", "region", "coverage_status"}).
			AddRow("adv-1", "Farros", domain.RankIron, "Grim Foundations", domain.RegionGreenstone, domain.CoverageActive))
	adventurers := loadAdventurers(db)
	require.Len(t, adventurers, 1)
	assert.Equal(t, "Farros", adventurers["adv-1"].Name)

	mock.ExpectQuery("SELECT id, adventurer_id, provider_id, incident_severity").
		WillReturnRows(claimRows().AddRow("claim-1", "adv-1", "provider-1", domain.SeverityAwakened, "tx-837", int64(100000), int64(80000), int64(68000), int64(12000), int64(20000), "allowance", "", domain.ClaimApproved))
	claims := loadClaims(db)
	require.Len(t, claims, 1)
	assert.Equal(t, domain.ClaimApproved, claims["claim-1"].Status)

	mock.ExpectQuery("SELECT id, type, status, sender_id, receiver_id, payload").
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "status", "sender_id", "receiver_id", "payload", "raw_x12", "related_id", "created_at"}).
			AddRow("tx-1", domain.Tx837, domain.TxStatusAccepted, "provider-1", societyID, []byte(`{"claimId":"claim-1"}`), "ST*837~", "", now))
	transactions := loadTransactions(db)
	require.Len(t, transactions, 1)
	assert.Equal(t, domain.Tx837, transactions["tx-1"].Type)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPayerLoadersFallbackOnDatabaseErrors(t *testing.T) {
	db, mock, cleanup := newPayerMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, name, provider_type, tier_rank, region FROM providers").
		WillReturnError(assert.AnError)
	assert.Len(t, loadProviders(db), 6)

	mock.ExpectQuery("SELECT id, name, rank, guild, region, coverage_status FROM adventurers").
		WillReturnError(assert.AnError)
	assert.Empty(t, loadAdventurers(db))

	mock.ExpectQuery("SELECT id, adventurer_id, provider_id, incident_severity").
		WillReturnError(assert.AnError)
	assert.Empty(t, loadClaims(db))

	mock.ExpectQuery("SELECT id, type, status, sender_id, receiver_id, payload").
		WillReturnError(assert.AnError)
	assert.Empty(t, loadTransactions(db))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPayerLoadProvidersFallsBackWhenTableEmpty(t *testing.T) {
	db, mock, cleanup := newPayerMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT id, name, provider_type, tier_rank, region FROM providers").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "provider_type", "tier_rank", "region"}))

	assert.Len(t, loadProviders(db), 6)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPayerDatabaseQueriesReturnPagedResults(t *testing.T) {
	db, mock, cleanup := newPayerMockDB(t)
	defer cleanup()
	app := &store{db: db}
	now := time.Now()

	mock.ExpectQuery("SELECT id, name, rank, guild, region, coverage_status FROM adventurers").
		WithArgs("Iron", "%Farros%", 3, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "rank", "guild", "region", "coverage_status"}).
			AddRow("adv-1", "Farros", domain.RankIron, "Grim", domain.RegionGreenstone, domain.CoverageActive).
			AddRow("adv-2", "Farros Two", domain.RankIron, "Grim", domain.RegionGreenstone, domain.CoverageActive).
			AddRow("adv-3", "Farros Three", domain.RankIron, "Grim", domain.RegionGreenstone, domain.CoverageActive))
	adventurers, adventurerPage, err := app.queryAdventurers(pageRequest{Limit: 2, Offset: 1}, adventurerFilters{Q: "Farros", Rank: "Iron"})
	require.NoError(t, err)
	assert.Len(t, adventurers, 2)
	assert.True(t, adventurerPage.HasMore)

	mock.ExpectQuery("SELECT id, adventurer_id, provider_id, incident_severity").
		WithArgs("Paid", "provider-1", "%claim%", 2, 0).
		WillReturnRows(claimRows().
			AddRow("claim-1", "adv-1", "provider-1", domain.SeverityAwakened, "tx-837", int64(100000), int64(80000), int64(68000), int64(12000), int64(20000), "allowance", "", domain.ClaimPaid))
	claims, claimPage, err := app.queryClaims(pageRequest{Limit: 1, Offset: 0}, claimFilters{Q: "claim", Status: "Paid", ProviderID: "provider-1"})
	require.NoError(t, err)
	assert.Len(t, claims, 1)
	assert.False(t, claimPage.HasMore)

	mock.ExpectQuery("SELECT id, type, status, sender_id, receiver_id, payload").
		WithArgs("837", "Accepted", "%provider%", 2, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "status", "sender_id", "receiver_id", "payload", "raw_x12", "related_id", "created_at"}).
			AddRow("tx-1", domain.Tx837, domain.TxStatusAccepted, "provider-1", societyID, []byte(`{"provider":"provider-1"}`), "ST*837~", "", now))
	transactions, transactionPage, err := app.queryTransactions(pageRequest{Limit: 1, Offset: 0}, transactionFilters{Q: "provider", Type: "837", Status: "Accepted"})
	require.NoError(t, err)
	assert.Len(t, transactions, 1)
	assert.False(t, transactionPage.HasMore)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPayerFindAndSaveDatabasePaths(t *testing.T) {
	db, mock, cleanup := newPayerMockDB(t)
	defer cleanup()
	app := &store{db: db, adventurers: map[string]domain.Adventurer{}, claims: map[string]domain.Claim{}, transactions: map[string]domain.Transaction{}}
	now := time.Now()

	mock.ExpectQuery("SELECT id, adventurer_id, provider_id, incident_severity").
		WithArgs("claim-1").
		WillReturnRows(claimRows().AddRow("claim-1", "adv-1", "provider-1", domain.SeverityAwakened, "tx-837", int64(100000), int64(80000), int64(68000), int64(12000), int64(20000), "allowance", "", domain.ClaimApproved))
	claim, ok := app.findClaim("claim-1")
	require.True(t, ok)
	assert.Equal(t, domain.ClaimApproved, claim.Status)

	mock.ExpectQuery("SELECT id, type, status, sender_id, receiver_id, payload").
		WithArgs("tx-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "status", "sender_id", "receiver_id", "payload", "raw_x12", "related_id", "created_at"}).
			AddRow("tx-1", domain.Tx837, domain.TxStatusAccepted, "provider-1", societyID, []byte(`{"claimId":"claim-1"}`), "ST*837~", "", now))
	tx, ok := app.findTransaction("tx-1")
	require.True(t, ok)
	assert.Equal(t, domain.Tx837, tx.Type)

	mock.ExpectExec("INSERT INTO adventurers").
		WithArgs("adv-2", "Aldrion", domain.RankGold, "Cloud Palace", domain.RegionVitesse, domain.CoverageActive).
		WillReturnResult(sqlmock.NewResult(1, 1))
	app.saveAdventurer(domain.Adventurer{ID: "adv-2", Name: "Aldrion", Rank: domain.RankGold, Guild: "Cloud Palace", Region: domain.RegionVitesse, CoverageStatus: domain.CoverageActive})

	mock.ExpectExec("INSERT INTO claims").
		WillReturnResult(sqlmock.NewResult(1, 1))
	app.saveClaim(domain.Claim{ID: "claim-2", AdventurerID: "adv-2", ProviderID: "provider-1", IncidentSeverity: domain.SeverityNormal, AmountCents: 50000, Status: domain.ClaimSubmitted})

	mock.ExpectExec("INSERT INTO transactions").
		WillReturnResult(sqlmock.NewResult(1, 1))
	app.saveTransaction(domain.Transaction{ID: "tx-2", Type: domain.Tx834, Status: domain.TxStatusAccepted, SenderID: "adv-2", ReceiverID: societyID, Payload: domain.Payload(map[string]string{"ok": "true"}), CreatedAt: now})

	mock.ExpectExec("INSERT INTO enrollments").
		WillReturnResult(sqlmock.NewResult(1, 1))
	app.saveEnrollment("adv-2", "tx-2", string(domain.TxStatusAccepted))

	mock.ExpectExec("INSERT INTO auth_requests").
		WillReturnResult(sqlmock.NewResult(1, 1))
	app.saveAuthRequest("adv-2", "provider-1", "tx-278", "resurrection", domain.SeverityDiamond, string(domain.TxStatusPending))

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO transaction_jobs (id, job_type, entity_id, status, attempts, run_after, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 0, $5, $6, $6)`)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	app.enqueueJob("job-kind", "entity-1", time.Second)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateClaimStatusDatabasePaths(t *testing.T) {
	db, mock, cleanup := newPayerMockDB(t)
	defer cleanup()
	app := &store{db: db, claims: map[string]domain.Claim{}, transactions: map[string]domain.Transaction{}}

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE claims SET status = $1 WHERE id = $2`)).
		WithArgs(string(domain.ClaimPendingDocumentation), "claim-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, app.updateClaimStatus(domain.Claim{ID: "claim-1", Status: domain.ClaimPendingDocumentation}))
	assert.Equal(t, domain.ClaimPendingDocumentation, app.claims["claim-1"].Status)

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE claims SET status = $1 WHERE id = $2`)).
		WithArgs(string(domain.ClaimPending), "claim-1").
		WillReturnError(assert.AnError)
	require.Error(t, app.updateClaimStatus(domain.Claim{ID: "claim-1", Status: domain.ClaimPending}))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPayerFindDatabaseMissesFallBackToMemory(t *testing.T) {
	db, mock, cleanup := newPayerMockDB(t)
	defer cleanup()
	app := &store{db: db, claims: map[string]domain.Claim{"memory-claim": {ID: "memory-claim", Status: domain.ClaimSubmitted}}, transactions: map[string]domain.Transaction{"memory-tx": {ID: "memory-tx", Type: domain.Tx834}}}

	mock.ExpectQuery("SELECT id, adventurer_id, provider_id, incident_severity").
		WithArgs("memory-claim").
		WillReturnRows(claimRows())
	claim, ok := app.findClaim("memory-claim")
	require.True(t, ok)
	assert.Equal(t, "memory-claim", claim.ID)

	mock.ExpectQuery("SELECT id, type, status, sender_id, receiver_id, payload").
		WithArgs("memory-tx").
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "status", "sender_id", "receiver_id", "payload", "raw_x12", "related_id", "created_at"}))
	tx, ok := app.findTransaction("memory-tx")
	require.True(t, ok)
	assert.Equal(t, "memory-tx", tx.ID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func newTestStore() *store {
	return &store{
		adventurers:  map[string]domain.Adventurer{},
		providers:    seedProviders(),
		claims:       map[string]domain.Claim{},
		transactions: map[string]domain.Transaction{},
	}
}

func newPayerTestMux(app *store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /enrollments", app.enroll)
	mux.HandleFunc("GET /adventurers", app.listAdventurers)
	mux.HandleFunc("GET /adventurers/{id}", app.getAdventurer)
	mux.HandleFunc("POST /eligibility/query", app.eligibility)
	mux.HandleFunc("POST /auth-requests", app.authRequest)
	mux.HandleFunc("POST /auth-requests/{id}/decision", app.decideAuthorization)
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
	mux.HandleFunc("POST /transactions/{id}/replay", app.replayTransaction)
	return mux
}

func serveJSON(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)
	return response
}

func decodeEnvelope(t *testing.T, response *httptest.ResponseRecorder) testEnvelope {
	t.Helper()
	var envelope testEnvelope
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	return envelope
}

func newPayerMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}

func newPayerMockDBWithPing(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	return db, mock, func() {
		_ = db.Close()
	}
}

func claimRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "adventurer_id", "provider_id", "incident_severity", "transaction_id", "amount_cents",
		"allowed_amount_cents", "paid_amount_cents", "patient_responsibility_cents", "adjustment_amount_cents",
		"adjustment_reason", "denial_reason", "status",
	})
}
