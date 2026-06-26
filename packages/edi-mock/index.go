package edimock

import (
	"time"

	"ashn/packages/domain"
	"ashn/packages/lore"
)

func Generate834(adventurer domain.Adventurer, sponsor string) domain.Transaction {
	return transaction(domain.Tx834, domain.TxStatusAccepted, adventurer.ID, sponsor, map[string]any{
		"x12": "834 Benefit Enrollment and Maintenance", "adventurer": adventurer, "sponsor": sponsor,
		"lore": lore.ThemeTransaction(domain.Tx834, adventurer.Name, sponsor),
	})
}

func Generate820(adventurer domain.Adventurer, amountCents int64) domain.Transaction {
	return transaction(domain.Tx820, domain.TxStatusAccepted, adventurer.ID, "Adventure Society", map[string]any{
		"x12": "820 Premium Payment", "adventurerId": adventurer.ID, "amountCents": amountCents,
		"lore": lore.ThemeTransaction(domain.Tx820, adventurer.Name, "Adventure Society"),
	})
}

func Generate270(adventurer domain.Adventurer, provider domain.Provider) domain.Transaction {
	return transaction(domain.Tx270, domain.TxStatusDispatched, provider.ID, "Adventure Society", map[string]any{
		"x12": "270 Eligibility Inquiry", "adventurerId": adventurer.ID, "providerId": provider.ID,
		"lore": lore.ThemeTransaction(domain.Tx270, adventurer.Name, provider.Name),
	})
}

func Generate271(adventurer domain.Adventurer, eligibility bool) domain.Transaction {
	status := domain.TxStatusDenied
	if eligibility {
		status = domain.TxStatusAccepted
	}
	return transaction(domain.Tx271, status, "Adventure Society", adventurer.ID, map[string]any{
		"x12": "271 Eligibility Response", "adventurerId": adventurer.ID, "eligible": eligibility,
		"coverageStatus": adventurer.CoverageStatus, "lore": lore.ThemeTransaction(domain.Tx271, adventurer.Name, "Adventure Society"),
	})
}

func Generate278Request(adventurer domain.Adventurer, provider domain.Provider, serviceType string) domain.Transaction {
	return transaction(domain.Tx278, domain.TxStatusPending, provider.ID, "Adventure Society", map[string]any{
		"x12": "278 Prior Authorization Request", "adventurerId": adventurer.ID, "providerId": provider.ID,
		"serviceType": serviceType, "lore": lore.ThemeTransaction(domain.Tx278, adventurer.Name, provider.Name),
	})
}

func Generate837(claim domain.Claim) domain.Transaction {
	return transaction(domain.Tx837, domain.TxStatusAccepted, claim.ProviderID, "Adventure Society", map[string]any{
		"x12": "837 Health Care Claim", "claim": claim, "severityDescription": lore.SeverityDescription(claim.IncidentSeverity),
		"lore": lore.ThemeTransaction(domain.Tx837, claim.AdventurerID, claim.ProviderID),
	})
}

func Generate835(claim domain.Claim, paymentAmountCents int64) domain.Transaction {
	return transaction(domain.Tx835, domain.TxStatusPaid, "Adventure Society", claim.ProviderID, map[string]any{
		"x12": "835 Claim Payment / Remittance Advice", "claimId": claim.ID, "paymentAmountCents": paymentAmountCents,
		"lore": lore.ThemeTransaction(domain.Tx835, claim.ID, claim.ProviderID),
	})
}

func Generate276(claimID string) domain.Transaction {
	return transaction(domain.Tx276, domain.TxStatusDispatched, "provider", "Adventure Society", map[string]any{
		"x12": "276 Claim Status Request", "claimId": claimID,
		"lore": lore.ThemeTransaction(domain.Tx276, claimID, "Adventure Society"),
	})
}

func Generate277(claimID string, status domain.ClaimStatus) domain.Transaction {
	return transaction(domain.Tx277, domain.TxStatusAccepted, "Adventure Society", "provider", map[string]any{
		"x12": "277 Claim Status Response", "claimId": claimID, "claimStatus": status,
		"lore": lore.ThemeTransaction(domain.Tx277, claimID, "Adventure Society"),
	})
}

func transaction(txType domain.TransactionType, status domain.TransactionStatus, senderID, receiverID string, payload any) domain.Transaction {
	return domain.Transaction{
		ID: domain.NewID(), Type: txType, Status: status, SenderID: senderID, ReceiverID: receiverID,
		Payload: domain.Payload(payload), CreatedAt: time.Now().UTC(),
	}
}
