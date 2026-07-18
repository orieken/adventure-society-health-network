package edimock

import (
	"strings"
	"testing"

	"ashn/packages/domain"

	"github.com/stretchr/testify/assert"
)

func TestGenerate837IncludesCompanionGuideInspiredSegments(t *testing.T) {
	claim := domain.Claim{
		ID:               "claim-1",
		AdventurerID:     "adv-1",
		ProviderID:       "provider-vitesse-temple",
		IncidentSeverity: domain.SeverityAwakened,
		AmountCents:      125000,
		Status:           domain.ClaimSubmitted,
		Diagnoses: []domain.ClaimDiagnosis{
			{Qualifier: "ABK", Code: "T509", Primary: true},
			{Qualifier: "ABF", Code: "S610"},
		},
		ServiceLines: []domain.ClaimServiceLine{
			{LineNumber: 1, ProcedureCode: "ASHN1", Description: "Resurrection stabilization", Units: 1, AmountCents: 95000},
			{LineNumber: 2, ProcedureCode: "ASHN2", Description: "Dragonfire trauma supplies", Units: 1, AmountCents: 30000},
		},
	}

	tx := Generate837(claim)

	assert.Contains(t, tx.RawX12, "NM1*85*2*provider-vitesse-temple")
	assert.Contains(t, tx.RawX12, "NM1*IL*1*adv-1")
	assert.Contains(t, tx.RawX12, "CLM*claim-1*1250.00")
	assert.Contains(t, tx.RawX12, "HI*ABK:T509*ABF:S610")
	assert.Contains(t, tx.RawX12, "SV1*HC:ASHN1*950.00*UN*1***1")
	assert.Contains(t, tx.RawX12, "SV1*HC:ASHN2*300.00*UN*1***2")
}

func TestGenerate837DIncludesDentalServiceSegments(t *testing.T) {
	claim := domain.Claim{
		ID:               "claim-dental-1",
		AdventurerID:     "adv-1",
		ProviderID:       "provider-vitesse-temple",
		IncidentSeverity: domain.SeverityNormal,
		AmountCents:      85000,
		Status:           domain.ClaimSubmitted,
		Diagnoses: []domain.ClaimDiagnosis{
			{Qualifier: "ABK", Code: "K021", Primary: true},
		},
		ServiceLines: []domain.ClaimServiceLine{
			{LineNumber: 1, ProcedureCode: "D7240", CDTCode: "D7240", Description: "Removal of impacted tooth", Units: 1, AmountCents: 85000, ToothNumber: "14", Surface: "MO", Quadrant: "UR", Orthodontic: true},
		},
	}

	tx := Generate837(claim)

	assert.Equal(t, domain.Tx837D, tx.Type)
	assert.Contains(t, tx.RawX12, "ST*837D")
	assert.Contains(t, tx.RawX12, "CLM*claim-dental-1*850.00")
	assert.Contains(t, tx.RawX12, "HI*ABK:K021")
	assert.Contains(t, tx.RawX12, "SV3*AD:D7240*850.00*UN*1***1")
	assert.Contains(t, tx.RawX12, "TOO*JP*14")
	assert.Contains(t, tx.RawX12, "REF*D9*SURFACE-MO")
	assert.Contains(t, tx.RawX12, "REF*D9*QUADRANT-UR")
	assert.Contains(t, tx.RawX12, "CRC*ZZ*Y*ORTHO")
	assert.Contains(t, string(tx.Payload), `"x12":"837D Dental Claim"`)
}

func TestGenerate835IncludesPaymentAndAdjustmentSegments(t *testing.T) {
	claim := domain.Claim{
		ID:                         "claim-1",
		ProviderID:                 "provider-vitesse-temple",
		AmountCents:                125000,
		AllowedAmountCents:         100000,
		PaidAmountCents:            85000,
		PatientResponsibilityCents: 15000,
		AdjustmentAmountCents:      25000,
		AdjustmentReason:           "ASHN contractual allowance",
		Status:                     domain.ClaimApproved,
		ServiceLines: []domain.ClaimServiceLine{
			{LineNumber: 1, ProcedureCode: "ASHN1", AmountCents: 95000, PaidAmountCents: 64600, AdjustmentAmountCents: 19000},
			{LineNumber: 2, ProcedureCode: "ASHN2", AmountCents: 30000, PaidAmountCents: 20400, AdjustmentAmountCents: 6000},
		},
	}

	tx := Generate835(claim, 100000)

	assert.Contains(t, tx.RawX12, "BPR*I*850.00")
	assert.Contains(t, tx.RawX12, "CLP*claim-1*1*1250.00*850.00*150.00")
	assert.Contains(t, tx.RawX12, "CAS*CO*45*250.00")
	assert.Contains(t, tx.RawX12, "SVC*HC:ASHN1*950.00*646.00")
	assert.Contains(t, tx.RawX12, "SVC*HC:ASHN2*300.00*204.00")
	assert.Contains(t, tx.RawX12, "REF*6R*2")
}

func TestGenerate835RepresentsDeniedClaimWithoutPayment(t *testing.T) {
	claim := domain.Claim{
		ID:                    "claim-denied",
		ProviderID:            "provider-vitesse-temple",
		AmountCents:           250000,
		AdjustmentAmountCents: 250000,
		AdjustmentReason:      "Non-covered catastrophic encounter",
		DenialReason:          "Prior authorization or benefit exception required",
		Status:                domain.ClaimDenied,
	}

	tx := Generate835(claim, 100000)

	assert.Equal(t, domain.Tx835, tx.Type)
	assert.Contains(t, tx.RawX12, "BPR*I*0.00")
	assert.Contains(t, tx.RawX12, "CLP*claim-denied*4*2500.00*0.00*0.00")
	assert.Contains(t, tx.RawX12, "CAS*CO*45*2500.00")
	assert.Contains(t, string(tx.Payload), `"denialReason":"Prior authorization or benefit exception required"`)
}

func TestGenerate275IncludesAttachmentSegmentsAndRelationship(t *testing.T) {
	claim := domain.Claim{
		ID:            "claim-1",
		AdventurerID:  "adv-1",
		ProviderID:    "provider-vitesse-temple",
		TransactionID: "tx-837",
		Status:        domain.ClaimSubmitted,
	}

	tx := Generate275(claim, domain.AttachmentRequest{
		PacketID:                "packet-claim-1",
		PacketSequence:          1,
		PacketCount:             2,
		AttachmentFormatCode:    "TXT",
		AttachmentObjectType:    "DOC",
		AttachmentEncoding:      "ASC",
		AttachmentServiceDate:   "2026-07-18",
		AttachmentType:          "OZ",
		AttachmentControlNumber: "ATTACH-1",
		ReportTypeCode:          "B4",
		TransmissionCode:        "EL",
		ContentType:             "text/plain",
		Description:             "Resurrection medical necessity notes",
		Content:                 "Patient stabilized after dragonfire incident.",
	}, "")

	assert.Equal(t, domain.Tx275, tx.Type)
	assert.Equal(t, domain.TxStatusAccepted, tx.Status)
	assert.Equal(t, "tx-837", tx.RelatedID)
	assert.Contains(t, tx.RawX12, "ST*275")
	assert.Contains(t, tx.RawX12, "BGN*02*tx-837")
	assert.Contains(t, tx.RawX12, "REF*1K*claim-1")
	assert.Contains(t, tx.RawX12, "REF*6R*ATTACH-1")
	assert.Contains(t, tx.RawX12, "REF*F8*packet-claim-1-1-OF-2")
	assert.Contains(t, tx.RawX12, "DTP*472*D8*20260718")
	assert.Contains(t, tx.RawX12, "LX*1")
	assert.Contains(t, tx.RawX12, "PWK*B4*EL***AC*ATTACH-1")
	assert.Contains(t, tx.RawX12, "CAT*B4*TXT")
	assert.Contains(t, tx.RawX12, "OOI*DOC*ATTACH-1")
	assert.Contains(t, tx.RawX12, "BDS*ASC**Content-Type: text/plain")
	assert.Contains(t, tx.RawX12, "LQ*AT*OZ")
	assert.Contains(t, tx.RawX12, "K3*Content-Type: text/plain")
	assert.Contains(t, tx.RawX12, "BIN*")
	assert.Contains(t, tx.RawX12, "Patient stabilized after dragonfire incident.")
}

func TestGenerate275ForAuthorizationUsesAuthReference(t *testing.T) {
	auth := Generate278Request(
		domain.Adventurer{ID: "adv-1", Name: "Farros"},
		domain.Provider{ID: "provider-vitesse-temple", Name: "Temple"},
		"resurrection",
	)
	auth.ID = "tx-278"

	tx := Generate275ForAuthorization(auth, domain.AttachmentRequest{
		AttachmentType:          "OZ",
		AttachmentControlNumber: "ATTACH-AUTH-1",
		ReportTypeCode:          "B4",
		TransmissionCode:        "EL",
		ContentType:             "text/plain",
		Description:             "Medical necessity notes",
		Content:                 "Encounter notes",
	})

	assert.Equal(t, domain.Tx275, tx.Type)
	assert.Equal(t, "tx-278", tx.RelatedID)
	assert.Contains(t, tx.RawX12, "BGN*11*tx-278")
	assert.Contains(t, tx.RawX12, "REF*G1*tx-278")
	assert.Contains(t, string(tx.Payload), `"attachmentPurpose":"solicited"`)
	assert.NotContains(t, tx.RawX12, "REF*1K*tx-278")
}

func TestGenerate275CanReferenceExternalDocumentWithoutEmbeddedContent(t *testing.T) {
	claim := domain.Claim{ID: "claim-1", AdventurerID: "adv-1", ProviderID: "provider-vitesse-temple", TransactionID: "tx-837"}

	tx := Generate275(claim, domain.AttachmentRequest{
		AttachmentType:          "OZ",
		AttachmentControlNumber: "ATTACH-REF-1",
		ReportTypeCode:          "B4",
		TransmissionCode:        "EL",
		ContentType:             "application/pdf",
		Description:             "External operative notes",
		DocumentReferenceID:     "doc-ashn-001",
		DocumentReferenceURL:    "https://docs.example.test/ashn/doc-ashn-001.pdf",
	}, "")

	assert.Contains(t, tx.RawX12, "K3*Document-Reference: https://docs.example.test/ashn/doc-ashn-001.pdf")
	assert.NotContains(t, tx.RawX12, "BIN*")
	assert.Contains(t, string(tx.Payload), `"documentReferenceId":"doc-ashn-001"`)
}

func TestGenerate999UsesAcknowledgedTransactionType(t *testing.T) {
	tx := Generate999("inbound-1", domain.Tx270, "edi-intake", "provider-vitesse-temple", false, "missing field ProviderId")

	assert.Contains(t, tx.RawX12, "AK2*270*")
	assert.Contains(t, tx.RawX12, "IK5*R")
	assert.True(t, strings.Contains(tx.RawX12, "AK9*R*1*1*0"))
}

func TestGenerateEnrollmentEligibilityAuthAndStatusTransactions(t *testing.T) {
	adventurer := domain.Adventurer{ID: "adv-1", Name: "Farros", CoverageStatus: domain.CoverageActive}
	provider := domain.Provider{ID: "provider-vitesse-temple", Name: "Temple of the Healer, Vitesse"}

	enrollment := Generate834(adventurer, "Adventure Society")
	assert.Equal(t, domain.Tx834, enrollment.Type)
	assert.Equal(t, domain.TxStatusAccepted, enrollment.Status)
	assert.Contains(t, enrollment.RawX12, "BGN*00*")

	premium := Generate820(adventurer, 5000)
	assert.Equal(t, domain.Tx820, premium.Type)
	assert.Contains(t, premium.RawX12, "005010X218")
	assert.Contains(t, premium.RawX12, "BPR*C*50.00")

	eligibilityRequest := Generate270(adventurer, provider)
	assert.Equal(t, domain.Tx270, eligibilityRequest.Type)
	assert.Contains(t, eligibilityRequest.RawX12, "EQ*30")

	eligibleResponse := Generate271(adventurer, true)
	assert.Equal(t, domain.TxStatusAccepted, eligibleResponse.Status)
	assert.Contains(t, eligibleResponse.RawX12, "EB*1")

	dentalEligibilityRequest := Generate270(adventurer, provider, "dental")
	assert.Contains(t, dentalEligibilityRequest.RawX12, "EQ*35")

	dentalEligibilityResponse := Generate271(adventurer, true, "dental")
	assert.Contains(t, dentalEligibilityResponse.RawX12, "EB*1**35")
	assert.Contains(t, dentalEligibilityResponse.RawX12, "EB*B**35***23*1500.00")
	assert.Contains(t, dentalEligibilityResponse.RawX12, "EB*C**35***29*1250.00")
	assert.Contains(t, dentalEligibilityResponse.RawX12, "MSG*Preventive 100% Basic 80% Major 50%")
	assert.Contains(t, string(dentalEligibilityResponse.Payload), `"dentalEligibility"`)

	ineligibleResponse := Generate271(adventurer, false)
	assert.Equal(t, domain.TxStatusDenied, ineligibleResponse.Status)
	assert.Contains(t, ineligibleResponse.RawX12, "EB*6")

	authRequest := Generate278Request(adventurer, provider, "resurrection")
	assert.Equal(t, domain.Tx278, authRequest.Type)
	assert.Contains(t, authRequest.RawX12, "UM*AR*I*2")

	dentalAuth := Generate278RequestWithDental(adventurer, provider, "dental-predetermination", &domain.DentalServiceDetail{
		CDTCode:     "D7240",
		ToothNumber: "14",
		Surface:     "MO",
		Quadrant:    "UR",
		Orthodontic: true,
	})
	assert.Contains(t, dentalAuth.RawX12, "UM*AR*I*2***dental-predetermination")
	assert.Contains(t, dentalAuth.RawX12, "SV1*AD:D7240")
	assert.Contains(t, dentalAuth.RawX12, "TOO*JP*14")

	claimStatusRequest := Generate276("claim-1")
	assert.Equal(t, domain.Tx276, claimStatusRequest.Type)
	assert.Contains(t, claimStatusRequest.RawX12, "REF*1K*claim-1")

	claimStatusResponse := Generate277("claim-1", domain.ClaimPaid)
	assert.Equal(t, domain.Tx277, claimStatusResponse.Type)
	assert.Contains(t, claimStatusResponse.RawX12, "STC*A1:20")
}

func TestGenerate277CAReflectsAcceptedAndRejectedOutcomes(t *testing.T) {
	claim := domain.Claim{ID: "claim-1", ProviderID: "provider-vitesse-temple", Status: domain.ClaimSubmitted}

	accepted := Generate277CA(claim, "tx-837", true)
	assert.Equal(t, domain.Tx277CA, accepted.Type)
	assert.Equal(t, domain.TxStatusAccepted, accepted.Status)
	assert.Equal(t, "tx-837", accepted.RelatedID)
	assert.Contains(t, accepted.RawX12, "STC*A1:20")

	rejected := Generate277CA(claim, "tx-837", false)
	assert.Equal(t, domain.TxStatusFailed, rejected.Status)
	assert.Contains(t, rejected.RawX12, "STC*A1:21")
}

func TestRawHelpersHandleFallbacksAndStatusCodes(t *testing.T) {
	assert.Equal(t, domain.Tx837, acknowledgedTransactionType(domain.Transaction{Payload: []byte(`not-json`)}))
	assert.Equal(t, "fallback", payloadString(domain.Transaction{Payload: []byte(`not-json`)}, "missing", "fallback"))
	assert.Equal(t, "fallback", stringValue(map[string]any{"empty": "  "}, "empty", "fallback"))
	assert.Equal(t, "S610", diagnosisCode(domain.SeverityNormal))
	assert.Equal(t, "S062X9A", diagnosisCode(domain.SeverityDiamond))
	assert.Equal(t, "ASHN", diagnosisCode(domain.IncidentSeverity("Cosmic")))
	assert.Equal(t, int64(42), int64Value(map[string]any{"value": int64(42)}, "value"))
	assert.Equal(t, int64(7), int64Value(map[string]any{"value": 7}, "value"))
	assert.Equal(t, int64(9), int64Value(map[string]any{"value": "9"}, "value"))
	assert.Equal(t, "A1", authCode(domain.TxStatusApproved))
	assert.Equal(t, "A3", authCode(domain.TxStatusDenied))
	assert.Equal(t, "A4", authCode(domain.TxStatusPending))
	assert.Equal(t, "21", statusCode(domain.TxStatusFailed))
	assert.Equal(t, "19", statusCode(domain.TxStatusPending))
	assert.Equal(t, "UNKNOWN", element(""))
	assert.Equal(t, "A-B-C D", element("A*B~C\nD"))
}
