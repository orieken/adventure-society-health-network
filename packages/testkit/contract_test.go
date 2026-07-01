package testkit

import (
	"testing"

	"ashn/packages/domain"
	edimock "ashn/packages/edi-mock"

	"github.com/stretchr/testify/assert"
)

func TestEnrollmentTransactionContract(t *testing.T) {
	adventurer := domain.Adventurer{ID: domain.NewID(), Name: "Farros", Rank: domain.RankIron, Guild: "Grim Foundations", Region: domain.RegionGreenstone, CoverageStatus: domain.CoverageActive}
	tx := edimock.Generate834(adventurer, "Adventure Society")
	assert.Equal(t, domain.Tx834, tx.Type)
	assert.Equal(t, domain.TxStatusAccepted, tx.Status)
	assert.NotEmpty(t, tx.Payload)
	assert.Contains(t, tx.RawX12, "ISA*")
	assert.Contains(t, tx.RawX12, "ST*834")
	assert.Contains(t, tx.RawX12, "INS*Y")
}

func TestClaimStatusTransactionPair(t *testing.T) {
	request := edimock.Generate276("claim-1")
	response := edimock.Generate277("claim-1", domain.ClaimPending)
	assert.Equal(t, domain.Tx276, request.Type)
	assert.Equal(t, domain.TxStatusDispatched, request.Status)
	assert.Contains(t, request.RawX12, "ST*276")
	assert.Contains(t, request.RawX12, "REF*1K")
	assert.Equal(t, domain.Tx277, response.Type)
	assert.Equal(t, domain.TxStatusAccepted, response.Status)
	assert.Contains(t, response.RawX12, "ST*277")
	assert.Contains(t, response.RawX12, "STC*A1")
}

func TestAcknowledgmentTransactions(t *testing.T) {
	ack := edimock.Generate999("message-1", domain.Tx837, "edi-intake", "partner", true, "")
	assert.Equal(t, domain.Tx999, ack.Type)
	assert.Equal(t, domain.TxStatusAccepted, ack.Status)
	assert.Equal(t, "message-1", ack.RelatedID)
	assert.Contains(t, ack.RawX12, "ST*999")
	assert.Contains(t, ack.RawX12, "AK9*A")

	claim := domain.Claim{ID: "claim-1", ProviderID: "provider-vitesse-temple", Status: domain.ClaimSubmitted}
	claimAck := edimock.Generate277CA(claim, "tx-837", true)
	assert.Equal(t, domain.Tx277CA, claimAck.Type)
	assert.Equal(t, domain.TxStatusAccepted, claimAck.Status)
	assert.Equal(t, "tx-837", claimAck.RelatedID)
	assert.Contains(t, claimAck.RawX12, "ST*277CA")
	assert.Contains(t, claimAck.RawX12, "STC*A1")
}
