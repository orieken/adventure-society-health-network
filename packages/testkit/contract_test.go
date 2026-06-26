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
}

func TestClaimStatusTransactionPair(t *testing.T) {
	request := edimock.Generate276("claim-1")
	response := edimock.Generate277("claim-1", domain.ClaimPending)
	assert.Equal(t, domain.Tx276, request.Type)
	assert.Equal(t, domain.TxStatusDispatched, request.Status)
	assert.Equal(t, domain.Tx277, response.Type)
	assert.Equal(t, domain.TxStatusAccepted, response.Status)
}
