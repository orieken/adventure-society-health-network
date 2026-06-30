package edimock

import (
	"fmt"
	"strings"
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
	tx := domain.Transaction{
		ID: domain.NewID(), Type: txType, Status: status, SenderID: senderID, ReceiverID: receiverID,
		Payload: domain.Payload(payload), CreatedAt: time.Now().UTC(),
	}
	tx.RawX12 = rawX12(tx)
	return tx
}

func rawX12(tx domain.Transaction) string {
	control := controlNumber(tx.ID)
	segments := []string{
		fmt.Sprintf("ISA*00*          *00*          *ZZ*%-15s*ZZ*%-15s*%s*%s*^*00501*%09s*0*T*:~",
			fixed(tx.SenderID, 15), fixed(tx.ReceiverID, 15), tx.CreatedAt.Format("060102"), tx.CreatedAt.Format("1504"), control),
		fmt.Sprintf("GS*HC*%s*%s*%s*%s*%s*X*005010X%s~", element(tx.SenderID), element(tx.ReceiverID), tx.CreatedAt.Format("20060102"), tx.CreatedAt.Format("1504"), control, implementationGuide(tx.Type)),
		fmt.Sprintf("ST*%s*%s~", tx.Type, control),
		fmt.Sprintf("BHT*0019*00*%s*%s*%s*CH~", control, tx.CreatedAt.Format("20060102"), tx.CreatedAt.Format("1504")),
	}
	segments = append(segments, transactionSegments(tx)...)
	segments = append(segments,
		fmt.Sprintf("SE*%d*%s~", len(segments)+3, control),
		fmt.Sprintf("GE*1*%s~", control),
		fmt.Sprintf("IEA*1*%09s~", control),
	)
	return strings.Join(segments, "\n")
}

func transactionSegments(tx domain.Transaction) []string {
	switch tx.Type {
	case domain.Tx834:
		return []string{
			"BGN*00*" + element(tx.ID) + "*" + tx.CreatedAt.Format("20060102") + "~",
			"INS*Y*18*030*XN*A***FT~",
			"NM1*IL*1*" + element(tx.SenderID) + "****MI*" + element(tx.SenderID) + "~",
		}
	case domain.Tx270:
		return []string{
			"HL*1**20*1~",
			"NM1*PR*2*" + element(tx.ReceiverID) + "*****PI*" + element(tx.ReceiverID) + "~",
			"NM1*1P*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"EQ*30~",
		}
	case domain.Tx271:
		return []string{
			"HL*1**20*1~",
			"NM1*PR*2*" + element(tx.SenderID) + "*****PI*" + element(tx.SenderID) + "~",
			"NM1*IL*1*" + element(tx.ReceiverID) + "****MI*" + element(tx.ReceiverID) + "~",
			"EB*" + eligibilityCode(tx.Status) + "**30~",
		}
	case domain.Tx278:
		return []string{
			"HL*1**20*1~",
			"NM1*1P*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"UM*AR*I*2~",
			"HCR*" + authCode(tx.Status) + "~",
		}
	case domain.Tx837:
		return []string{
			"HL*1**20*1~",
			"NM1*41*2*" + element(tx.SenderID) + "*****46*" + element(tx.SenderID) + "~",
			"CLM*" + element(tx.ID) + "***11:B:1*Y*A*Y*I~",
			"HI*ABK:ASHN~",
		}
	case domain.Tx835:
		return []string{
			"BPR*I*0*C*CHK************" + tx.CreatedAt.Format("20060102") + "~",
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"CLP*" + element(tx.ID) + "*1*0*0**MC*" + element(tx.ID) + "~",
		}
	case domain.Tx276:
		return []string{
			"HL*1**20*1~",
			"NM1*1P*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"TRN*1*" + element(tx.ID) + "~",
			"REF*1K*" + element(tx.ID) + "~",
		}
	case domain.Tx277:
		return []string{
			"HL*1**20*1~",
			"NM1*PR*2*" + element(tx.SenderID) + "*****PI*" + element(tx.SenderID) + "~",
			"TRN*2*" + element(tx.ID) + "~",
			"STC*A1:" + statusCode(tx.Status) + "~",
		}
	default:
		return []string{"NTE*ADD*ASHN placeholder transaction~"}
	}
}

func implementationGuide(txType domain.TransactionType) string {
	switch txType {
	case domain.Tx834:
		return "220A1"
	case domain.Tx270, domain.Tx271:
		return "270A1"
	case domain.Tx276, domain.Tx277:
		return "276A1"
	case domain.Tx278:
		return "278A1"
	case domain.Tx835:
		return "835A1"
	default:
		return "837P"
	}
}

func eligibilityCode(status domain.TransactionStatus) string {
	if status == domain.TxStatusAccepted {
		return "1"
	}
	return "6"
}

func authCode(status domain.TransactionStatus) string {
	if status == domain.TxStatusApproved {
		return "A1"
	}
	if status == domain.TxStatusDenied {
		return "A3"
	}
	return "A4"
}

func statusCode(status domain.TransactionStatus) string {
	if status == domain.TxStatusAccepted || status == domain.TxStatusPaid {
		return "20"
	}
	if status == domain.TxStatusDenied || status == domain.TxStatusFailed {
		return "21"
	}
	return "19"
}

func controlNumber(id string) string {
	clean := strings.NewReplacer("-", "", "_", "").Replace(id)
	if len(clean) > 9 {
		clean = clean[:9]
	}
	for len(clean) < 9 {
		clean = "0" + clean
	}
	return clean
}

func fixed(value string, length int) string {
	value = element(value)
	if len(value) > length {
		return value[:length]
	}
	return value
}

func element(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer("*", "-", "~", "-", "\n", " ", "\r", " ")
	if value == "" {
		return "UNKNOWN"
	}
	return replacer.Replace(value)
}
