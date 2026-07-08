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
	}

	tx := Generate837(claim)

	assert.Contains(t, tx.RawX12, "NM1*85*2*provider-vitesse-temple")
	assert.Contains(t, tx.RawX12, "NM1*IL*1*adv-1")
	assert.Contains(t, tx.RawX12, "CLM*claim-1*1250.00")
	assert.Contains(t, tx.RawX12, "HI*ABK:T509")
	assert.Contains(t, tx.RawX12, "SV1*HC:ASHN1*1250.00")
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
	}

	tx := Generate835(claim, 100000)

	assert.Contains(t, tx.RawX12, "BPR*I*850.00")
	assert.Contains(t, tx.RawX12, "CLP*claim-1*1*1250.00*850.00*150.00")
	assert.Contains(t, tx.RawX12, "CAS*CO*45*250.00")
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
	assert.Contains(t, tx.RawX12, "REF*1K*claim-1")
	assert.Contains(t, tx.RawX12, "REF*6R*ATTACH-1")
	assert.Contains(t, tx.RawX12, "PWK*B4*EL***AC*ATTACH-1")
	assert.Contains(t, tx.RawX12, "LQ*AT*OZ")
	assert.Contains(t, tx.RawX12, "K3*Content-Type: text/plain")
	assert.Contains(t, tx.RawX12, "BIN*")
	assert.Contains(t, tx.RawX12, "Patient stabilized after dragonfire incident.")
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
	assert.Contains(t, premium.RawX12, "ASHN placeholder transaction")

	eligibilityRequest := Generate270(adventurer, provider)
	assert.Equal(t, domain.Tx270, eligibilityRequest.Type)
	assert.Contains(t, eligibilityRequest.RawX12, "EQ*30")

	eligibleResponse := Generate271(adventurer, true)
	assert.Equal(t, domain.TxStatusAccepted, eligibleResponse.Status)
	assert.Contains(t, eligibleResponse.RawX12, "EB*1")

	ineligibleResponse := Generate271(adventurer, false)
	assert.Equal(t, domain.TxStatusDenied, ineligibleResponse.Status)
	assert.Contains(t, ineligibleResponse.RawX12, "EB*6")

	authRequest := Generate278Request(adventurer, provider, "resurrection")
	assert.Equal(t, domain.Tx278, authRequest.Type)
	assert.Contains(t, authRequest.RawX12, "UM*AR*I*2")

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
