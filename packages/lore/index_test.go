package lore

import (
	"testing"

	"ashn/packages/domain"

	"github.com/stretchr/testify/assert"
)

func TestRankDescriptionCoversKnownAndUnknownRanks(t *testing.T) {
	assert.Contains(t, RankDescription(domain.RankIron), "Roadside Clinic")
	assert.Contains(t, RankDescription(domain.RankBronze), "Town Healer")
	assert.Contains(t, RankDescription(domain.RankSilver), "Regional Clinic")
	assert.Contains(t, RankDescription(domain.RankGold), "City Hospital")
	assert.Contains(t, RankDescription(domain.RankDiamond), "Temple of the Healer")
	assert.Contains(t, RankDescription(domain.Rank("Mythic")), "Unranked")
}

func TestSeverityDescriptionCoversKnownAndUnknownSeverities(t *testing.T) {
	assert.Contains(t, SeverityDescription(domain.SeverityNormal), "outpatient 837P")
	assert.Contains(t, SeverityDescription(domain.SeverityAwakened), "inpatient 837I")
	assert.Contains(t, SeverityDescription(domain.SeverityDiamond), "prior auth 278")
	assert.Contains(t, SeverityDescription(domain.IncidentSeverity("Cosmic")), "unknown")
}

func TestThemeTransactionUsesDefaultsAndTransactionSpecificText(t *testing.T) {
	tests := map[domain.TransactionType]string{
		domain.Tx834: "Society registration accepted",
		domain.Tx820: "Guild dues payment recorded",
		domain.Tx270: "Eligibility verification requested",
		domain.Tx271: "Eligibility response issued",
		domain.Tx275: "Patient information attachment sent",
		domain.Tx278: "Prior authorization requested",
		domain.Tx837: "Incident claim submitted",
		domain.Tx835: "Adventure Society remittance issued",
		domain.Tx276: "Claim status inquiry sent",
		domain.Tx277: "Claim status response issued",
		domain.Tx269: "Coordination of benefits checked",
	}

	for txType, want := range tests {
		assert.Contains(t, ThemeTransaction(txType, "Farros", "Adventure Society"), want)
		assert.Contains(t, ThemeTransaction(txType, "Farros", "Adventure Society"), "Farros")
	}

	assert.Contains(t, ThemeTransaction(domain.TransactionType("999")), "Unknown sender")
	assert.Contains(t, ThemeTransaction(domain.Tx834, ""), "Unknown sender")
}
