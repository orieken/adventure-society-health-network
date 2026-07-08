package edimock

import (
	"encoding/json"
	"fmt"
	"strconv"
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

func Generate275(claim domain.Claim, attachment domain.AttachmentRequest, relatedID string) domain.Transaction {
	if relatedID == "" {
		relatedID = claim.TransactionID
	}
	if relatedID == "" {
		relatedID = claim.ID
	}
	tx := transaction(domain.Tx275, domain.TxStatusAccepted, claim.ProviderID, "Adventure Society", map[string]any{
		"x12": "275 Patient Information", "claimId": claim.ID, "providerId": claim.ProviderID,
		"adventurerId": claim.AdventurerID, "attachmentType": attachment.AttachmentType,
		"attachmentControlNumber": attachment.AttachmentControlNumber, "description": attachment.Description,
		"reportTypeCode": attachment.ReportTypeCode, "transmissionCode": attachment.TransmissionCode,
		"contentType": attachment.ContentType,
		"content":     attachment.Content, "lore": lore.ThemeTransaction(domain.Tx275, claim.ProviderID, "Adventure Society"),
	})
	tx.RelatedID = relatedID
	tx.RawX12 = rawX12(tx)
	return tx
}

func Generate278Request(adventurer domain.Adventurer, provider domain.Provider, serviceType string) domain.Transaction {
	return transaction(domain.Tx278, domain.TxStatusPending, provider.ID, "Adventure Society", map[string]any{
		"x12": "278 Prior Authorization Request", "adventurerId": adventurer.ID, "providerId": provider.ID,
		"serviceType": serviceType, "lore": lore.ThemeTransaction(domain.Tx278, adventurer.Name, provider.Name),
	})
}

func WithStatus(tx domain.Transaction, status domain.TransactionStatus) domain.Transaction {
	tx.Status = status
	tx.RawX12 = rawX12(tx)
	return tx
}

func Generate837(claim domain.Claim) domain.Transaction {
	return transaction(domain.Tx837, domain.TxStatusAccepted, claim.ProviderID, "Adventure Society", map[string]any{
		"x12": "837 Health Care Claim", "claim": claim, "severityDescription": lore.SeverityDescription(claim.IncidentSeverity),
		"lore": lore.ThemeTransaction(domain.Tx837, claim.AdventurerID, claim.ProviderID),
	})
}

func Generate835(claim domain.Claim, paymentAmountCents int64) domain.Transaction {
	if claim.PaidAmountCents > 0 || claim.Status == domain.ClaimDenied {
		paymentAmountCents = claim.PaidAmountCents
	}
	return transaction(domain.Tx835, domain.TxStatusPaid, "Adventure Society", claim.ProviderID, map[string]any{
		"x12": "835 Claim Payment / Remittance Advice", "claimId": claim.ID,
		"billedAmountCents": claim.AmountCents, "allowedAmountCents": claim.AllowedAmountCents,
		"paymentAmountCents": paymentAmountCents, "patientResponsibilityCents": claim.PatientResponsibilityCents,
		"adjustmentAmountCents": claim.AdjustmentAmountCents, "adjustmentReason": claim.AdjustmentReason,
		"denialReason": claim.DenialReason, "claimStatus": claim.Status,
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

func Generate999(relatedID string, acknowledgedType domain.TransactionType, senderID, receiverID string, accepted bool, errorText string) domain.Transaction {
	status := domain.TxStatusAccepted
	outcome := "accepted"
	if !accepted {
		status = domain.TxStatusFailed
		outcome = "rejected"
	}
	tx := transaction(domain.Tx999, status, senderID, receiverID, map[string]any{
		"x12": "999 Implementation Acknowledgment", "relatedId": relatedID, "acknowledgedType": acknowledgedType,
		"accepted": accepted, "outcome": outcome, "error": errorText,
	})
	tx.RelatedID = relatedID
	tx.RawX12 = rawX12(tx)
	return tx
}

func Generate277CA(claim domain.Claim, relatedID string, accepted bool) domain.Transaction {
	status := domain.TxStatusAccepted
	outcome := "accepted"
	if !accepted {
		status = domain.TxStatusFailed
		outcome = "rejected"
	}
	tx := transaction(domain.Tx277CA, status, "Adventure Society", claim.ProviderID, map[string]any{
		"x12": "277CA Health Care Claim Acknowledgment", "claimId": claim.ID, "relatedId": relatedID,
		"accepted": accepted, "outcome": outcome, "claimStatus": claim.Status,
	})
	tx.RelatedID = relatedID
	tx.RawX12 = rawX12(tx)
	return tx
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
			"REF*38*" + element(tx.ReceiverID) + "~",
			"INS*Y*18*030*XN*A***FT~",
			"NM1*IL*1*" + element(tx.SenderID) + "****MI*" + element(tx.SenderID) + "~",
			"DMG*D8*19800101*U~",
			"HD*030**HLT~",
			"DTP*348*D8*" + tx.CreatedAt.Format("20060102") + "~",
		}
	case domain.Tx270:
		return []string{
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*PR*2*" + element(tx.ReceiverID) + "*****PI*" + element(tx.ReceiverID) + "~",
			"HL*2*1*21*1~",
			"NM1*1P*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"HL*3*2*22*0~",
			"NM1*IL*1*" + element(payloadString(tx, "adventurerId", "adventurer")) + "****MI*" + element(payloadString(tx, "adventurerId", tx.ID)) + "~",
			"DTP*291*D8*" + tx.CreatedAt.Format("20060102") + "~",
			"EQ*30~",
		}
	case domain.Tx271:
		return []string{
			"TRN*2*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*PR*2*" + element(tx.SenderID) + "*****PI*" + element(tx.SenderID) + "~",
			"HL*2*1*22*0~",
			"NM1*IL*1*" + element(tx.ReceiverID) + "****MI*" + element(tx.ReceiverID) + "~",
			"EB*" + eligibilityCode(tx.Status) + "**30~",
			"DTP*291*D8*" + tx.CreatedAt.Format("20060102") + "~",
		}
	case domain.Tx275:
		attachment := attachmentInfo(tx)
		return []string{
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*1P*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"HL*2*1*22*0~",
			"NM1*IL*1*" + element(attachment.AdventurerID) + "****MI*" + element(attachment.AdventurerID) + "~",
			"REF*1K*" + element(attachment.ClaimID) + "~",
			"REF*6R*" + element(attachment.ControlNumber) + "~",
			"PWK*" + element(attachment.ReportTypeCode) + "*" + element(attachment.TransmissionCode) + "***AC*" + element(attachment.ControlNumber) + "~",
			"LQ*AT*" + element(attachment.AttachmentType) + "~",
			"K3*Content-Type: " + element(attachment.ContentType) + "~",
			"NTE*ADD*" + element(attachment.Description) + "~",
			"BIN*" + strconv.Itoa(len(attachment.Content)) + "*" + element(attachment.Content) + "~",
		}
	case domain.Tx278:
		return []string{
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*1P*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"HL*2*1*22*0~",
			"NM1*IL*1*" + element(payloadString(tx, "adventurerId", "adventurer")) + "****MI*" + element(payloadString(tx, "adventurerId", tx.ID)) + "~",
			"UM*AR*I*2***" + element(payloadString(tx, "serviceType", "service")) + "~",
			"DTP*472*D8*" + tx.CreatedAt.Format("20060102") + "~",
			"HCR*" + authCode(tx.Status) + "~",
		}
	case domain.Tx837:
		claim := claimInfo(tx)
		return []string{
			"HL*1**20*1~",
			"NM1*41*2*" + element(tx.SenderID) + "*****46*" + element(tx.SenderID) + "~",
			"PER*IC*ASHN CLAIM OFFICE*TE*5550100~",
			"NM1*85*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"HL*2*1*22*0~",
			"NM1*IL*1*" + element(claim.AdventurerID) + "****MI*" + element(claim.AdventurerID) + "~",
			"CLM*" + element(claim.ID) + "*" + cents(claim.AmountCents) + "***11:B:1*Y*A*Y*I~",
			"DTP*472*D8*" + tx.CreatedAt.Format("20060102") + "~",
			"HI*ABK:" + diagnosisCode(claim.Severity) + "~",
			"SV1*HC:ASHN1*" + cents(claim.AmountCents) + "*UN*1***1~",
		}
	case domain.Tx835:
		remit := remittanceAmounts(tx)
		return []string{
			"BPR*I*" + cents(remit.Paid) + "*C*CHK************" + tx.CreatedAt.Format("20060102") + "~",
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"CLP*" + element(remit.ClaimID) + "*" + remit.ClaimStatusCode + "*" + cents(remit.Billed) + "*" + cents(remit.Paid) + "*" + cents(remit.PatientResponsibility) + "*MC*" + element(tx.ID) + "~",
			"CAS*CO*45*" + cents(remit.Adjustment) + "~",
		}
	case domain.Tx276:
		return []string{
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*PR*2*" + element(tx.ReceiverID) + "*****PI*" + element(tx.ReceiverID) + "~",
			"HL*2*1*21*1~",
			"NM1*1P*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"HL*3*2*22*0~",
			"NM1*IL*1*" + element(payloadString(tx, "claimId", tx.ID)) + "****MI*" + element(payloadString(tx, "claimId", tx.ID)) + "~",
			"REF*1K*" + element(payloadString(tx, "claimId", tx.ID)) + "~",
		}
	case domain.Tx277:
		claimID := payloadString(tx, "claimId", tx.ID)
		return []string{
			"TRN*2*" + element(claimID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*PR*2*" + element(tx.SenderID) + "*****PI*" + element(tx.SenderID) + "~",
			"HL*2*1*22*0~",
			"NM1*IL*1*" + element(claimID) + "****MI*" + element(claimID) + "~",
			"REF*1K*" + element(claimID) + "~",
			"STC*A1:" + statusCode(tx.Status) + "~",
		}
	case domain.Tx999:
		acknowledgedType := acknowledgedTransactionType(tx)
		return []string{
			"AK1*HC*" + controlNumber(tx.RelatedID) + "~",
			"AK2*" + element(string(acknowledgedType)) + "*" + controlNumber(tx.RelatedID) + "~",
			"IK5*" + implementationAckCode(tx.Status) + "~",
			"AK9*" + implementationAckCode(tx.Status) + "*1*1*" + ackAcceptedCount(tx.Status) + "~",
		}
	case domain.Tx277CA:
		return []string{
			"TRN*1*" + controlNumber(tx.RelatedID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*PR*2*" + element(tx.SenderID) + "*****PI*" + element(tx.SenderID) + "~",
			"HL*2*1*22*0~",
			"NM1*IL*1*" + element(payloadString(tx, "claimId", tx.RelatedID)) + "****MI*" + element(payloadString(tx, "claimId", tx.RelatedID)) + "~",
			"TRN*2*" + controlNumber(tx.RelatedID) + "~",
			"STC*A1:" + statusCode(tx.Status) + "*" + tx.CreatedAt.Format("20060102") + "~",
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
	case domain.Tx275:
		return "275A1"
	case domain.Tx276, domain.Tx277:
		return "276A1"
	case domain.Tx278:
		return "278A1"
	case domain.Tx835:
		return "835A1"
	case domain.Tx999:
		return "999A1"
	case domain.Tx277CA:
		return "277A1"
	default:
		return "837P"
	}
}

func acknowledgedTransactionType(tx domain.Transaction) domain.TransactionType {
	var payload map[string]any
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		return domain.Tx837
	}
	acknowledgedType, ok := payload["acknowledgedType"]
	if !ok {
		return domain.Tx837
	}
	return domain.TransactionType(fmt.Sprint(acknowledgedType))
}

type remittance struct {
	ClaimID               string
	ClaimStatusCode       string
	Billed                int64
	Paid                  int64
	PatientResponsibility int64
	Adjustment            int64
}

type x12ClaimInfo struct {
	ID           string
	AdventurerID string
	ProviderID   string
	Severity     domain.IncidentSeverity
	AmountCents  int64
}

type x12AttachmentInfo struct {
	ClaimID          string
	ProviderID       string
	AdventurerID     string
	AttachmentType   string
	ControlNumber    string
	ReportTypeCode   string
	TransmissionCode string
	ContentType      string
	Description      string
	Content          string
}

func attachmentInfo(tx domain.Transaction) x12AttachmentInfo {
	var payload map[string]any
	info := x12AttachmentInfo{
		ClaimID:          tx.RelatedID,
		ProviderID:       tx.SenderID,
		AdventurerID:     "adventurer",
		AttachmentType:   "OZ",
		ControlNumber:    controlNumber(tx.ID),
		ReportTypeCode:   "B4",
		TransmissionCode: "EL",
		ContentType:      "text/plain",
		Description:      "ASHN patient information attachment",
		Content:          "supporting documentation",
	}
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		return info
	}
	info.ClaimID = stringValue(payload, "claimId", info.ClaimID)
	info.ProviderID = stringValue(payload, "providerId", info.ProviderID)
	info.AdventurerID = stringValue(payload, "adventurerId", info.AdventurerID)
	info.AttachmentType = stringValue(payload, "attachmentType", info.AttachmentType)
	info.ControlNumber = stringValue(payload, "attachmentControlNumber", info.ControlNumber)
	info.ReportTypeCode = stringValue(payload, "reportTypeCode", info.ReportTypeCode)
	info.TransmissionCode = stringValue(payload, "transmissionCode", info.TransmissionCode)
	info.ContentType = stringValue(payload, "contentType", info.ContentType)
	info.Description = stringValue(payload, "description", info.Description)
	info.Content = stringValue(payload, "content", info.Content)
	return info
}

func claimInfo(tx domain.Transaction) x12ClaimInfo {
	var payload struct {
		Claim domain.Claim `json:"claim"`
	}
	info := x12ClaimInfo{ID: tx.ID, AdventurerID: "adventurer", ProviderID: tx.SenderID, Severity: domain.SeverityNormal}
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		return info
	}
	if payload.Claim.ID != "" {
		info.ID = payload.Claim.ID
	}
	if payload.Claim.AdventurerID != "" {
		info.AdventurerID = payload.Claim.AdventurerID
	}
	if payload.Claim.ProviderID != "" {
		info.ProviderID = payload.Claim.ProviderID
	}
	if payload.Claim.IncidentSeverity != "" {
		info.Severity = payload.Claim.IncidentSeverity
	}
	info.AmountCents = payload.Claim.AmountCents
	return info
}

func remittanceAmounts(tx domain.Transaction) remittance {
	var payload map[string]any
	remit := remittance{ClaimID: tx.ID, ClaimStatusCode: "1"}
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		return remit
	}
	remit.ClaimID = stringValue(payload, "claimId", tx.ID)
	remit.Billed = int64Value(payload, "billedAmountCents")
	remit.Paid = int64Value(payload, "paymentAmountCents")
	remit.PatientResponsibility = int64Value(payload, "patientResponsibilityCents")
	remit.Adjustment = int64Value(payload, "adjustmentAmountCents")
	if stringValue(payload, "denialReason", "") != "" {
		remit.ClaimStatusCode = "4"
	}
	return remit
}

func payloadString(tx domain.Transaction, key string, fallback string) string {
	var payload map[string]any
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		return fallback
	}
	return stringValue(payload, key, fallback)
}

func stringValue(payload map[string]any, key string, fallback string) string {
	value, ok := payload[key]
	if !ok {
		return fallback
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" {
		return fallback
	}
	return text
}

func diagnosisCode(severity domain.IncidentSeverity) string {
	switch severity {
	case domain.SeverityNormal:
		return "S610"
	case domain.SeverityAwakened:
		return "T509"
	case domain.SeverityDiamond:
		return "S062X9A"
	default:
		return "ASHN"
	}
}

func int64Value(payload map[string]any, key string) int64 {
	value, ok := payload[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		parsed, _ := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		return parsed
	}
}

func cents(value int64) string {
	return fmt.Sprintf("%.2f", float64(value)/100)
}

func implementationAckCode(status domain.TransactionStatus) string {
	if status == domain.TxStatusAccepted {
		return "A"
	}
	return "R"
}

func ackAcceptedCount(status domain.TransactionStatus) string {
	if status == domain.TxStatusAccepted {
		return "1"
	}
	return "0"
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
