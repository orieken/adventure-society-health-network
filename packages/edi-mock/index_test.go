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

func TestGenerate999UsesAcknowledgedTransactionType(t *testing.T) {
	tx := Generate999("inbound-1", domain.Tx270, "edi-intake", "provider-vitesse-temple", false, "missing field ProviderId")

	assert.Contains(t, tx.RawX12, "AK2*270*")
	assert.Contains(t, tx.RawX12, "IK5*R")
	assert.True(t, strings.Contains(tx.RawX12, "AK9*R*1*1*0"))
}
