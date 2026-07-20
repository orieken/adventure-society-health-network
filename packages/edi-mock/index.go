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

func Generate270(adventurer domain.Adventurer, provider domain.Provider, serviceTypes ...string) domain.Transaction {
	serviceType := eligibilityServiceType(serviceTypes...)
	return transaction(domain.Tx270, domain.TxStatusDispatched, provider.ID, "Adventure Society", map[string]any{
		"x12": "270 Eligibility Inquiry", "adventurerId": adventurer.ID, "providerId": provider.ID, "serviceType": serviceType,
		"lore": lore.ThemeTransaction(domain.Tx270, adventurer.Name, provider.Name),
	})
}

func Generate271(adventurer domain.Adventurer, eligibility bool, serviceTypes ...string) domain.Transaction {
	serviceType := eligibilityServiceType(serviceTypes...)
	status := domain.TxStatusDenied
	if eligibility {
		status = domain.TxStatusAccepted
	}
	payload := map[string]any{
		"x12": "271 Eligibility Response", "adventurerId": adventurer.ID, "eligible": eligibility,
		"coverageStatus": adventurer.CoverageStatus, "serviceType": serviceType, "lore": lore.ThemeTransaction(domain.Tx271, adventurer.Name, "Adventure Society"),
	}
	if isDentalEligibility(serviceType) {
		payload["dentalEligibility"] = DentalEligibility(adventurer, eligibility)
	}
	return transaction(domain.Tx271, status, "Adventure Society", adventurer.ID, payload)
}

func Generate269(request domain.BenefitCoordinationRequest) domain.Transaction {
	serviceType := eligibilityServiceType(request.ServiceType)
	return transaction(domain.Tx269, domain.TxStatusAccepted, request.ProviderID, "Adventure Society", map[string]any{
		"x12":              "269 Health Care Benefit Coordination",
		"adventurerId":     request.AdventurerID,
		"providerId":       request.ProviderID,
		"primaryPayerId":   request.PrimaryPayerID,
		"secondaryPayerId": request.SecondaryPayerID,
		"serviceType":      serviceType,
		"coordination":     "primary payer remains Adventure Society; secondary payer captured for COB review",
		"lore":             lore.ThemeTransaction(domain.Tx269, request.PrimaryPayerID, request.SecondaryPayerID),
	})
}

func DentalEligibility(adventurer domain.Adventurer, eligible bool) domain.DentalEligibilityDetail {
	remainingMaximumCents := int64(0)
	if eligible {
		remainingMaximumCents = 125000
		if adventurer.Rank == domain.RankGold || adventurer.Rank == domain.RankDiamond {
			remainingMaximumCents = 150000
		}
	}
	waitingPeriodMonths := 6
	if adventurer.Rank == domain.RankGold || adventurer.Rank == domain.RankDiamond {
		waitingPeriodMonths = 0
	}
	return domain.DentalEligibilityDetail{
		ServiceType:               "dental",
		AnnualMaximumCents:        150000,
		RemainingMaximumCents:     remainingMaximumCents,
		PreventiveCoveragePercent: 100,
		BasicCoveragePercent:      80,
		MajorCoveragePercent:      50,
		WaitingPeriodMonths:       waitingPeriodMonths,
		FrequencyLimit:            "2 cleanings per plan year; 1 panoramic image per 36 months",
	}
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
		"packetId": attachment.PacketID, "packetSequence": attachment.PacketSequence,
		"packetCount":             attachment.PacketCount,
		"attachmentPurpose":       attachmentPurpose(attachment.AttachmentPurpose, "unsolicited"),
		"attachmentTraceId":       attachmentTraceID(attachment.AttachmentTraceID, relatedID),
		"attachmentFormatCode":    attachmentFormatCode(attachment.AttachmentFormatCode, attachment.ContentType),
		"attachmentObjectType":    attachmentObjectType(attachment.AttachmentObjectType),
		"attachmentEncoding":      attachmentEncoding(attachment.AttachmentEncoding, attachment.Content),
		"attachmentServiceDate":   attachment.AttachmentServiceDate,
		"attachmentControlNumber": attachment.AttachmentControlNumber, "description": attachment.Description,
		"reportTypeCode": attachment.ReportTypeCode, "transmissionCode": attachment.TransmissionCode,
		"contentType": attachment.ContentType, "fileName": attachment.FileName, "documentReferenceId": attachment.DocumentReferenceID,
		"documentReferenceUrl": attachment.DocumentReferenceURL,
		"content":              attachment.Content, "attachmentReviewStatus": "Received",
		"lore": lore.ThemeTransaction(domain.Tx275, claim.ProviderID, "Adventure Society"),
	})
	tx.RelatedID = relatedID
	tx.RawX12 = rawX12(tx)
	return tx
}

func Generate275ForAuthorization(auth domain.Transaction, attachment domain.AttachmentRequest) domain.Transaction {
	adventurerID := payloadString(auth, "adventurerId", auth.ID)
	providerID := payloadString(auth, "providerId", auth.SenderID)
	tx := transaction(domain.Tx275, domain.TxStatusAccepted, providerID, "Adventure Society", map[string]any{
		"x12": "275 Patient Information", "authorizationTransactionId": auth.ID, "providerId": providerID,
		"adventurerId": adventurerID, "attachmentType": attachment.AttachmentType,
		"packetId": attachment.PacketID, "packetSequence": attachment.PacketSequence,
		"packetCount":             attachment.PacketCount,
		"attachmentPurpose":       attachmentPurpose(attachment.AttachmentPurpose, "solicited"),
		"attachmentTraceId":       attachmentTraceID(attachment.AttachmentTraceID, auth.ID),
		"attachmentFormatCode":    attachmentFormatCode(attachment.AttachmentFormatCode, attachment.ContentType),
		"attachmentObjectType":    attachmentObjectType(attachment.AttachmentObjectType),
		"attachmentEncoding":      attachmentEncoding(attachment.AttachmentEncoding, attachment.Content),
		"attachmentServiceDate":   attachment.AttachmentServiceDate,
		"attachmentControlNumber": attachment.AttachmentControlNumber, "description": attachment.Description,
		"reportTypeCode": attachment.ReportTypeCode, "transmissionCode": attachment.TransmissionCode,
		"contentType": attachment.ContentType, "fileName": attachment.FileName, "documentReferenceId": attachment.DocumentReferenceID,
		"documentReferenceUrl": attachment.DocumentReferenceURL,
		"content":              attachment.Content, "attachmentReviewStatus": "Received",
		"lore": lore.ThemeTransaction(domain.Tx275, providerID, "Adventure Society"),
	})
	tx.RelatedID = auth.ID
	tx.RawX12 = rawX12(tx)
	return tx
}

func Generate278Request(adventurer domain.Adventurer, provider domain.Provider, serviceType string) domain.Transaction {
	return Generate278RequestWithDental(adventurer, provider, serviceType, nil)
}

func Generate278RequestWithDental(adventurer domain.Adventurer, provider domain.Provider, serviceType string, dentalService *domain.DentalServiceDetail) domain.Transaction {
	payload := map[string]any{
		"x12":          "278 Prior Authorization Request",
		"adventurerId": adventurer.ID,
		"providerId":   provider.ID,
		"serviceType":  serviceType,
		"lore":         lore.ThemeTransaction(domain.Tx278, adventurer.Name, provider.Name),
	}
	if dentalService != nil {
		payload["dentalService"] = dentalService
	}
	return transaction(domain.Tx278, domain.TxStatusPending, provider.ID, "Adventure Society", payload)
}

func WithStatus(tx domain.Transaction, status domain.TransactionStatus) domain.Transaction {
	tx.Status = status
	tx.RawX12 = rawX12(tx)
	return tx
}

func Generate837(claim domain.Claim) domain.Transaction {
	transactionType := domain.Tx837
	label := "837 Health Care Claim"
	if claimHasDentalServiceLines(claim) {
		transactionType = domain.Tx837D
		label = "837D Dental Claim"
	}
	return transaction(transactionType, domain.TxStatusAccepted, claim.ProviderID, "Adventure Society", map[string]any{
		"x12": label, "claim": claim, "severityDescription": lore.SeverityDescription(claim.IncidentSeverity),
		"lore": lore.ThemeTransaction(transactionType, claim.AdventurerID, claim.ProviderID),
	})
}

func Generate835(claim domain.Claim, paymentAmountCents int64) domain.Transaction {
	if claim.PaidAmountCents > 0 || claim.Status == domain.ClaimDenied {
		paymentAmountCents = claim.PaidAmountCents
	}
	label := "835 Claim Payment / Remittance Advice"
	if claimHasDentalServiceLines(claim) {
		label = "835 Dental Claim Payment / Remittance Advice"
	}
	return transaction(domain.Tx835, domain.TxStatusPaid, "Adventure Society", claim.ProviderID, map[string]any{
		"x12": label, "claimId": claim.ID,
		"billedAmountCents": claim.AmountCents, "allowedAmountCents": claim.AllowedAmountCents,
		"paymentAmountCents": paymentAmountCents, "patientResponsibilityCents": claim.PatientResponsibilityCents,
		"adjustmentAmountCents": claim.AdjustmentAmountCents, "adjustmentReason": claim.AdjustmentReason,
		"denialReason": claim.DenialReason, "claimStatus": claim.Status, "serviceLines": claim.ServiceLines,
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

func Generate824(relatedID string, rejectedType domain.TransactionType, senderID, receiverID, errorText string) domain.Transaction {
	tx := transaction(domain.Tx824, domain.TxStatusFailed, senderID, receiverID, map[string]any{
		"x12": "824 Application Advice", "relatedId": relatedID, "rejectedType": rejectedType,
		"accepted": false, "outcome": "application-rejected", "error": errorText,
		"lore": lore.ThemeTransaction(domain.Tx824, relatedID, receiverID),
	})
	tx.RelatedID = relatedID
	tx.RawX12 = rawX12(tx)
	return tx
}

func GenerateTA1(relatedID, senderID, receiverID, errorText string) domain.Transaction {
	tx := transaction(domain.TxTA1, domain.TxStatusFailed, senderID, receiverID, map[string]any{
		"x12": "TA1 Interchange Acknowledgment", "relatedId": relatedID,
		"accepted": false, "outcome": "interchange-rejected", "error": errorText,
		"lore": lore.ThemeTransaction(domain.TxTA1, relatedID, receiverID),
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
		fmt.Sprintf("GS*HC*%s*%s*%s*%s*%s*X*%s~", element(tx.SenderID), element(tx.ReceiverID), tx.CreatedAt.Format("20060102"), tx.CreatedAt.Format("1504"), control, implementationVersion(tx.Type)),
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
	case domain.Tx820:
		return []string{
			"BPR*C*" + cents(int64Value(payloadMap(tx), "amountCents")) + "*C*ACH************" + tx.CreatedAt.Format("20060102") + "~",
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"N1*PR*" + element(tx.ReceiverID) + "~",
			"N1*PE*" + element(tx.SenderID) + "~",
			"RMR*IK*" + element(tx.SenderID) + "**" + cents(int64Value(payloadMap(tx), "amountCents")) + "~",
		}
	case domain.Tx270:
		serviceType := payloadString(tx, "serviceType", "medical")
		eqCode := "30"
		if isDentalEligibility(serviceType) {
			eqCode = "35"
		}
		return []string{
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*PR*2*" + element(tx.ReceiverID) + "*****PI*" + element(tx.ReceiverID) + "~",
			"HL*2*1*21*1~",
			"NM1*1P*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"HL*3*2*22*0~",
			"NM1*IL*1*" + element(payloadString(tx, "adventurerId", "adventurer")) + "****MI*" + element(payloadString(tx, "adventurerId", tx.ID)) + "~",
			"DTP*291*D8*" + tx.CreatedAt.Format("20060102") + "~",
			"EQ*" + eqCode + "~",
		}
	case domain.Tx271:
		serviceType := payloadString(tx, "serviceType", "medical")
		ebCode := "30"
		if isDentalEligibility(serviceType) {
			ebCode = "35"
		}
		segments := []string{
			"TRN*2*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*PR*2*" + element(tx.SenderID) + "*****PI*" + element(tx.SenderID) + "~",
			"HL*2*1*22*0~",
			"NM1*IL*1*" + element(tx.ReceiverID) + "****MI*" + element(tx.ReceiverID) + "~",
			"EB*" + eligibilityCode(tx.Status) + "**" + ebCode + "~",
			"DTP*291*D8*" + tx.CreatedAt.Format("20060102") + "~",
		}
		if isDentalEligibility(serviceType) {
			segments = append(segments, dentalEligibilitySegments(tx)...)
		}
		return segments
	case domain.Tx269:
		serviceType := payloadString(tx, "serviceType", "medical")
		eqCode := "30"
		if isDentalEligibility(serviceType) {
			eqCode = "35"
		}
		return []string{
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*PR*2*" + element(payloadString(tx, "primaryPayerId", tx.ReceiverID)) + "*****PI*" + element(payloadString(tx, "primaryPayerId", tx.ReceiverID)) + "~",
			"NM1*PR*2*" + element(payloadString(tx, "secondaryPayerId", "secondary-payer")) + "*****PI*" + element(payloadString(tx, "secondaryPayerId", "secondary-payer")) + "~",
			"HL*2*1*21*1~",
			"NM1*1P*2*" + element(payloadString(tx, "providerId", tx.SenderID)) + "*****XX*" + element(payloadString(tx, "providerId", tx.SenderID)) + "~",
			"HL*3*2*22*0~",
			"NM1*IL*1*" + element(payloadString(tx, "adventurerId", tx.ID)) + "****MI*" + element(payloadString(tx, "adventurerId", tx.ID)) + "~",
			"REF*6P*" + element(payloadString(tx, "primaryPayerId", tx.ReceiverID)) + "~",
			"REF*2U*" + element(payloadString(tx, "secondaryPayerId", "secondary-payer")) + "~",
			"EQ*" + eqCode + "~",
		}
	case domain.Tx275:
		attachment := attachmentInfo(tx)
		segments := []string{
			"BGN*" + bgnPurposeCode(attachment.Purpose) + "*" + element(firstNonEmptyString(attachment.TraceID, tx.ID)) + "*" + tx.CreatedAt.Format("20060102") + "~",
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*1P*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"HL*2*1*22*0~",
			"NM1*IL*1*" + element(attachment.AdventurerID) + "****MI*" + element(attachment.AdventurerID) + "~",
			attachmentReferenceSegment(attachment),
			"REF*6R*" + element(attachment.ControlNumber) + "~",
			attachmentPacketSegment(attachment),
			"TRN*2*" + element(attachment.ControlNumber) + "*" + element(tx.SenderID) + "~",
			"DTP*472*D8*" + attachmentServiceDate(attachment, tx.CreatedAt) + "~",
			"LX*" + strconv.Itoa(attachmentLoopNumber(attachment)) + "~",
			"PWK*" + element(attachment.ReportTypeCode) + "*" + element(attachment.TransmissionCode) + "***AC*" + element(attachment.ControlNumber) + "~",
			"CAT*" + element(attachment.ReportTypeCode) + "*" + element(attachmentFormatCode(attachment.FormatCode, attachment.ContentType)) + "~",
			"OOI*" + element(attachmentObjectType(attachment.ObjectType)) + "*" + element(attachment.ControlNumber) + "~",
			"BDS*" + element(attachmentEncoding(attachment.Encoding, attachment.Content)) + "**" + element(attachmentContentDescriptor(attachment)) + "~",
			"LQ*AT*" + element(attachment.AttachmentType) + "~",
			documentReferenceSegment(attachment),
			"NTE*ADD*" + element(attachment.Description) + "~",
		}
		return append(segments, attachmentContentSegments(attachment)...)
	case domain.Tx278:
		segments := []string{
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"HL*1**20*1~",
			"NM1*1P*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"HL*2*1*22*0~",
			"NM1*IL*1*" + element(payloadString(tx, "adventurerId", "adventurer")) + "****MI*" + element(payloadString(tx, "adventurerId", tx.ID)) + "~",
			"UM*AR*I*2***" + element(payloadString(tx, "serviceType", "service")) + "~",
			"DTP*472*D8*" + tx.CreatedAt.Format("20060102") + "~",
			"HCR*" + authCode(tx.Status) + "~",
		}
		return append(segments, dental278Segments(tx)...)
	case domain.Tx837, domain.Tx837D:
		claim := claimInfo(tx)
		segments := []string{
			"HL*1**20*1~",
			"NM1*41*2*" + element(tx.SenderID) + "*****46*" + element(tx.SenderID) + "~",
			"PER*IC*ASHN CLAIM OFFICE*TE*5550100~",
			"NM1*85*2*" + element(tx.SenderID) + "*****XX*" + element(tx.SenderID) + "~",
			"HL*2*1*22*0~",
			"NM1*IL*1*" + element(claim.AdventurerID) + "****MI*" + element(claim.AdventurerID) + "~",
			"CLM*" + element(claim.ID) + "*" + cents(claim.AmountCents) + "***11:B:1*Y*A*Y*I~",
			"DTP*472*D8*" + tx.CreatedAt.Format("20060102") + "~",
		}
		segments = append(segments, diagnosisSegments(claim)...)
		segments = append(segments, claimAttachmentControlSegments(claim)...)
		return append(segments, serviceLineSegments(claim)...)
	case domain.Tx835:
		remit := remittanceAmounts(tx)
		segments := []string{
			"BPR*I*" + cents(remit.Paid) + "*C*CHK************" + tx.CreatedAt.Format("20060102") + "~",
			"TRN*1*" + element(tx.ID) + "*" + element(tx.SenderID) + "~",
			"CLP*" + element(remit.ClaimID) + "*" + remit.ClaimStatusCode + "*" + cents(remit.Billed) + "*" + cents(remit.Paid) + "*" + cents(remit.PatientResponsibility) + "*MC*" + element(tx.ID) + "~",
			"CAS*CO*45*" + cents(remit.Adjustment) + "~",
		}
		return append(segments, remittanceServiceLineSegments(remit)...)
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
	case domain.Tx824:
		return []string{
			"BGN*11*" + element(tx.ID) + "*" + tx.CreatedAt.Format("20060102") + "~",
			"OTI*TR*" + applicationAdviceReference(tx) + "*" + element(tx.RelatedID) + "~",
			"TED*007*" + element(payloadString(tx, "error", "application validation failed")) + "~",
			"NTE*ADD*Rejected " + element(string(rejectedTransactionType(tx))) + " application advice~",
		}
	case domain.TxTA1:
		return []string{
			"TA1*" + controlNumber(tx.RelatedID) + "*" + tx.CreatedAt.Format("060102") + "*" + tx.CreatedAt.Format("1504") + "*R*000~",
			"NTE*ADD*" + element(payloadString(tx, "error", "interchange rejected")) + "~",
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

func attachmentReferenceSegment(attachment x12AttachmentInfo) string {
	if attachment.ClaimID != "" {
		return "REF*1K*" + element(attachment.ClaimID) + "~"
	}
	return "REF*G1*" + element(attachment.AuthorizationTransactionID) + "~"
}

func attachmentPacketSegment(attachment x12AttachmentInfo) string {
	if attachment.PacketID == "" {
		return "REF*F8*" + element(attachment.ControlNumber) + "~"
	}
	detail := attachment.PacketID
	if attachment.PacketSequence > 0 && attachment.PacketCount > 0 {
		detail = fmt.Sprintf("%s-%d-OF-%d", detail, attachment.PacketSequence, attachment.PacketCount)
	}
	return "REF*F8*" + element(detail) + "~"
}

func documentReferenceSegment(attachment x12AttachmentInfo) string {
	if attachment.DocumentReferenceURL != "" {
		return "K3*Document-Reference: " + element(attachment.DocumentReferenceURL) + "~"
	}
	if attachment.DocumentReferenceID != "" {
		return "K3*Document-Reference: " + element(attachment.DocumentReferenceID) + "~"
	}
	return "K3*Content-Type: " + element(attachment.ContentType) + "~"
}

func attachmentContentSegments(attachment x12AttachmentInfo) []string {
	if attachment.Content == "" {
		return nil
	}
	return []string{"BIN*" + strconv.Itoa(len(attachment.Content)) + "*" + element(attachment.Content) + "~"}
}

func implementationGuide(txType domain.TransactionType) string {
	switch txType {
	case domain.Tx834:
		return "220A1"
	case domain.Tx820:
		return "218"
	case domain.Tx270, domain.Tx271:
		return "270A1"
	case domain.Tx269:
		return "269A1"
	case domain.Tx275:
		return "275A1"
	case domain.Tx276, domain.Tx277:
		return "276A1"
	case domain.Tx278:
		return "278A1"
	case domain.Tx835:
		return "835A1"
	case domain.Tx824:
		return "824A1"
	case domain.TxTA1:
		return "TA1"
	case domain.Tx999:
		return "999A1"
	case domain.Tx277CA:
		return "277A1"
	case domain.Tx837D:
		return "837D"
	default:
		return "837P"
	}
}

func implementationVersion(txType domain.TransactionType) string {
	if txType == domain.Tx275 {
		return "006020X314"
	}
	return "005010X" + implementationGuide(txType)
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

func rejectedTransactionType(tx domain.Transaction) domain.TransactionType {
	var payload map[string]any
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		return domain.Tx275
	}
	rejectedType, ok := payload["rejectedType"]
	if !ok {
		return domain.Tx275
	}
	return domain.TransactionType(fmt.Sprint(rejectedType))
}

func applicationAdviceReference(tx domain.Transaction) string {
	if tx.RelatedID == "" {
		return "TN"
	}
	return "TN"
}

type remittance struct {
	ClaimID               string
	ClaimStatusCode       string
	Billed                int64
	Paid                  int64
	PatientResponsibility int64
	Adjustment            int64
	ServiceLines          []domain.ClaimServiceLine
}

type x12ClaimInfo struct {
	ID                 string
	AdventurerID       string
	ProviderID         string
	Severity           domain.IncidentSeverity
	AmountCents        int64
	ServiceLines       []domain.ClaimServiceLine
	Diagnoses          []domain.ClaimDiagnosis
	AttachmentControls []domain.AttachmentControl
}

type x12AttachmentInfo struct {
	ClaimID                    string
	AuthorizationTransactionID string
	ProviderID                 string
	AdventurerID               string
	PacketID                   string
	PacketSequence             int
	PacketCount                int
	Purpose                    string
	TraceID                    string
	FormatCode                 string
	ObjectType                 string
	Encoding                   string
	ServiceDate                string
	AttachmentType             string
	ControlNumber              string
	ReportTypeCode             string
	TransmissionCode           string
	ContentType                string
	Description                string
	Content                    string
	DocumentReferenceID        string
	DocumentReferenceURL       string
}

func attachmentInfo(tx domain.Transaction) x12AttachmentInfo {
	var payload map[string]any
	info := x12AttachmentInfo{
		ProviderID:       tx.SenderID,
		AdventurerID:     "adventurer",
		AttachmentType:   "OZ",
		ControlNumber:    controlNumber(tx.ID),
		ReportTypeCode:   "B4",
		TransmissionCode: "EL",
		ContentType:      "text/plain",
		Description:      "ASHN patient information attachment",
	}
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		return info
	}
	info.ClaimID = stringValue(payload, "claimId", info.ClaimID)
	info.AuthorizationTransactionID = stringValue(payload, "authorizationTransactionId", tx.RelatedID)
	info.ProviderID = stringValue(payload, "providerId", info.ProviderID)
	info.AdventurerID = stringValue(payload, "adventurerId", info.AdventurerID)
	info.PacketID = stringValue(payload, "packetId", info.PacketID)
	info.PacketSequence = intValue(payload, "packetSequence", info.PacketSequence)
	info.PacketCount = intValue(payload, "packetCount", info.PacketCount)
	info.Purpose = stringValue(payload, "attachmentPurpose", info.Purpose)
	info.TraceID = stringValue(payload, "attachmentTraceId", info.TraceID)
	info.FormatCode = stringValue(payload, "attachmentFormatCode", info.FormatCode)
	info.ObjectType = stringValue(payload, "attachmentObjectType", info.ObjectType)
	info.Encoding = stringValue(payload, "attachmentEncoding", info.Encoding)
	info.ServiceDate = stringValue(payload, "attachmentServiceDate", info.ServiceDate)
	info.AttachmentType = stringValue(payload, "attachmentType", info.AttachmentType)
	info.ControlNumber = stringValue(payload, "attachmentControlNumber", info.ControlNumber)
	info.ReportTypeCode = stringValue(payload, "reportTypeCode", info.ReportTypeCode)
	info.TransmissionCode = stringValue(payload, "transmissionCode", info.TransmissionCode)
	info.ContentType = stringValue(payload, "contentType", info.ContentType)
	info.Description = stringValue(payload, "description", info.Description)
	info.Content = stringValue(payload, "content", info.Content)
	info.DocumentReferenceID = stringValue(payload, "documentReferenceId", info.DocumentReferenceID)
	info.DocumentReferenceURL = stringValue(payload, "documentReferenceUrl", info.DocumentReferenceURL)
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
	info.ServiceLines = payload.Claim.ServiceLines
	info.Diagnoses = payload.Claim.Diagnoses
	info.AttachmentControls = payload.Claim.AttachmentControls
	return info
}

func diagnosisSegments(claim x12ClaimInfo) []string {
	diagnoses := claim.Diagnoses
	if len(diagnoses) == 0 {
		diagnoses = []domain.ClaimDiagnosis{{Qualifier: "ABK", Code: diagnosisCode(claim.Severity), Primary: true}}
	}
	elements := make([]string, 0, len(diagnoses))
	for index, diagnosis := range diagnoses {
		code := strings.TrimSpace(diagnosis.Code)
		if code == "" {
			continue
		}
		qualifier := strings.ToUpper(strings.TrimSpace(diagnosis.Qualifier))
		if qualifier == "" {
			qualifier = "ABF"
		}
		if diagnosis.Primary || index == 0 {
			qualifier = "ABK"
		}
		elements = append(elements, qualifier+":"+element(code))
	}
	if len(elements) == 0 {
		elements = append(elements, "ABK:"+diagnosisCode(claim.Severity))
	}
	return []string{"HI*" + strings.Join(elements, "*") + "~"}
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
	remit.ServiceLines = serviceLinesValue(payload, "serviceLines")
	if stringValue(payload, "denialReason", "") != "" {
		remit.ClaimStatusCode = "4"
	}
	return remit
}

func serviceLineSegments(claim x12ClaimInfo) []string {
	lines := claim.ServiceLines
	if len(lines) == 0 {
		lines = []domain.ClaimServiceLine{{
			LineNumber:    1,
			ProcedureCode: "ASHN1",
			Units:         1,
			AmountCents:   claim.AmountCents,
		}}
	}
	segments := make([]string, 0, len(lines))
	for index, line := range lines {
		lineNumber := line.LineNumber
		if lineNumber <= 0 {
			lineNumber = index + 1
		}
		procedureCode := strings.TrimSpace(line.ProcedureCode)
		if procedureCode == "" {
			procedureCode = fmt.Sprintf("ASHN%d", lineNumber)
		}
		units := line.Units
		if units <= 0 {
			units = 1
		}
		if claimLineIsDental(line) {
			cdtCode := strings.TrimSpace(line.CDTCode)
			if cdtCode == "" {
				cdtCode = procedureCode
			}
			segments = append(segments, "SV3*AD:"+element(cdtCode)+"*"+cents(line.AmountCents)+"*UN*"+strconv.Itoa(units)+"***"+strconv.Itoa(lineNumber)+"~")
			if line.ToothNumber != "" {
				segments = append(segments, "TOO*JP*"+element(line.ToothNumber)+"~")
			}
			if line.Surface != "" {
				segments = append(segments, "REF*D9*SURFACE-"+element(line.Surface)+"~")
			}
			if line.Quadrant != "" {
				segments = append(segments, "REF*D9*QUADRANT-"+element(line.Quadrant)+"~")
			}
			if line.Orthodontic {
				segments = append(segments, "CRC*ZZ*Y*ORTHO~")
			}
			continue
		}
		segments = append(segments, "SV1*HC:"+element(procedureCode)+"*"+cents(line.AmountCents)+"*UN*"+strconv.Itoa(units)+"***"+strconv.Itoa(lineNumber)+"~")
	}
	return segments
}

func claimAttachmentControlSegments(claim x12ClaimInfo) []string {
	segments := []string{}
	for _, control := range claim.AttachmentControls {
		controlNumber := strings.TrimSpace(control.AttachmentControlNumber)
		if controlNumber == "" {
			continue
		}
		reportType := strings.TrimSpace(control.ReportTypeCode)
		if reportType == "" {
			reportType = "B4"
		}
		transmission := strings.TrimSpace(control.TransmissionCode)
		if transmission == "" {
			transmission = "EL"
		}
		segments = append(segments, "PWK*"+element(reportType)+"*"+element(transmission)+"****"+element(controlNumber)+"~")
	}
	return segments
}

func claimHasDentalServiceLines(claim domain.Claim) bool {
	for _, line := range claim.ServiceLines {
		if claimLineIsDental(line) {
			return true
		}
	}
	return false
}

func claimLineIsDental(line domain.ClaimServiceLine) bool {
	return strings.TrimSpace(line.CDTCode) != "" || strings.TrimSpace(line.ToothNumber) != "" || strings.TrimSpace(line.Surface) != "" || strings.TrimSpace(line.Quadrant) != "" || line.Orthodontic
}

func remittanceServiceLineSegments(remit remittance) []string {
	segments := make([]string, 0, len(remit.ServiceLines)*7)
	for index, line := range remit.ServiceLines {
		lineNumber := line.LineNumber
		if lineNumber <= 0 {
			lineNumber = index + 1
		}
		procedureCode := strings.TrimSpace(line.ProcedureCode)
		if procedureCode == "" {
			procedureCode = fmt.Sprintf("ASHN%d", lineNumber)
		}
		qualifier := "HC"
		if claimLineIsDental(line) {
			qualifier = "AD"
			if strings.TrimSpace(line.CDTCode) != "" {
				procedureCode = strings.TrimSpace(line.CDTCode)
			}
		}
		segments = append(segments, "SVC*"+qualifier+":"+element(procedureCode)+"*"+cents(line.AmountCents)+"*"+cents(line.PaidAmountCents)+"~")
		if line.AdjustmentAmountCents > 0 {
			segments = append(segments, "CAS*CO*45*"+cents(line.AdjustmentAmountCents)+"~")
		}
		if line.AllowedAmountCents > 0 {
			segments = append(segments, "AMT*AU*"+cents(line.AllowedAmountCents)+"~")
		}
		if line.PatientResponsibilityCents > 0 {
			segments = append(segments, "AMT*PR*"+cents(line.PatientResponsibilityCents)+"~")
		}
		segments = append(segments, "REF*6R*"+strconv.Itoa(lineNumber)+"~")
		if claimLineIsDental(line) {
			segments = append(segments, dentalRemittanceReferenceSegments(line)...)
		}
		if strings.TrimSpace(line.DenialReason) != "" {
			segments = append(segments, "LQ*HE*"+element(line.DenialReason)+"~")
		}
	}
	return segments
}

func dentalRemittanceReferenceSegments(line domain.ClaimServiceLine) []string {
	segments := []string{}
	if strings.TrimSpace(line.ToothNumber) != "" {
		segments = append(segments, "REF*XZ*TOOTH-"+element(line.ToothNumber)+"~")
	}
	if strings.TrimSpace(line.Surface) != "" {
		segments = append(segments, "REF*D9*SURFACE-"+element(line.Surface)+"~")
	}
	if strings.TrimSpace(line.Quadrant) != "" {
		segments = append(segments, "REF*D9*QUADRANT-"+element(line.Quadrant)+"~")
	}
	if line.Orthodontic {
		segments = append(segments, "REF*D9*ORTHODONTIC~")
	}
	return segments
}

func serviceLinesValue(payload map[string]any, key string) []domain.ClaimServiceLine {
	value, ok := payload[key]
	if !ok {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var lines []domain.ClaimServiceLine
	if err := json.Unmarshal(data, &lines); err != nil {
		return nil
	}
	return lines
}

func payloadString(tx domain.Transaction, key string, fallback string) string {
	return stringValue(payloadMap(tx), key, fallback)
}

func attachmentPurpose(purpose string, fallback string) string {
	purpose = strings.ToLower(strings.TrimSpace(purpose))
	switch purpose {
	case "02":
		return "unsolicited"
	case "11":
		return "solicited"
	case "solicited", "unsolicited":
		return purpose
	default:
		return fallback
	}
}

func attachmentTraceID(traceID string, fallback string) string {
	return firstNonEmptyString(traceID, fallback)
}

func bgnPurposeCode(purpose string) string {
	switch attachmentPurpose(purpose, "unsolicited") {
	case "solicited":
		return "11"
	default:
		return "02"
	}
}

func attachmentFormatCode(formatCode string, contentType string) string {
	formatCode = strings.ToUpper(strings.TrimSpace(formatCode))
	if formatCode != "" {
		return formatCode
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.Contains(contentType, "pdf"):
		return "PDF"
	case strings.Contains(contentType, "image"), strings.Contains(contentType, "tiff"):
		return "IMG"
	default:
		return "TXT"
	}
}

func attachmentObjectType(objectType string) string {
	objectType = strings.ToUpper(strings.TrimSpace(objectType))
	if objectType == "" {
		return "DOC"
	}
	return objectType
}

func attachmentEncoding(encoding string, content string) string {
	encoding = strings.ToUpper(strings.TrimSpace(encoding))
	if encoding == "ASC" || encoding == "B64" || encoding == "REF" {
		return encoding
	}
	if strings.TrimSpace(content) == "" {
		return "REF"
	}
	return "ASC"
}

func attachmentServiceDate(attachment x12AttachmentInfo, createdAt time.Time) string {
	date := strings.TrimSpace(attachment.ServiceDate)
	if len(date) == 10 && date[4] == '-' && date[7] == '-' {
		return strings.ReplaceAll(date, "-", "")
	}
	if len(date) == 8 {
		return date
	}
	return createdAt.Format("20060102")
}

func attachmentLoopNumber(attachment x12AttachmentInfo) int {
	if attachment.PacketSequence > 0 {
		return attachment.PacketSequence
	}
	return 1
}

func attachmentContentDescriptor(attachment x12AttachmentInfo) string {
	if strings.TrimSpace(attachment.DocumentReferenceURL) != "" {
		return "Document-Reference: " + strings.TrimSpace(attachment.DocumentReferenceURL)
	}
	if strings.TrimSpace(attachment.ContentType) != "" {
		return "Content-Type: " + strings.TrimSpace(attachment.ContentType)
	}
	return "ASHN attachment payload"
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func eligibilityServiceType(serviceTypes ...string) string {
	if len(serviceTypes) == 0 {
		return "medical"
	}
	serviceType := strings.ToLower(strings.TrimSpace(serviceTypes[0]))
	if serviceType == "" {
		return "medical"
	}
	return serviceType
}

func isDentalEligibility(serviceType string) bool {
	serviceType = strings.ToLower(strings.TrimSpace(serviceType))
	return serviceType == "dental" || serviceType == "dental-eligibility"
}

func dentalEligibilitySegments(tx domain.Transaction) []string {
	payload := payloadMap(tx)
	value, ok := payload["dentalEligibility"]
	if !ok {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var detail domain.DentalEligibilityDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil
	}
	return []string{
		"EB*B**35***23*" + cents(detail.AnnualMaximumCents) + "~",
		"EB*C**35***29*" + cents(detail.RemainingMaximumCents) + "~",
		"MSG*Preventive " + strconv.Itoa(detail.PreventiveCoveragePercent) + "% Basic " + strconv.Itoa(detail.BasicCoveragePercent) + "% Major " + strconv.Itoa(detail.MajorCoveragePercent) + "%~",
		"MSG*Waiting period " + strconv.Itoa(detail.WaitingPeriodMonths) + " months; " + element(detail.FrequencyLimit) + "~",
	}
}

func dental278Segments(tx domain.Transaction) []string {
	payload := payloadMap(tx)
	value, ok := payload["dentalService"]
	if !ok {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var detail domain.DentalServiceDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil
	}
	segments := []string{}
	if detail.CDTCode != "" {
		segments = append(segments, "SV1*AD:"+element(detail.CDTCode)+"*0.00*UN*1~")
	}
	if detail.ToothNumber != "" {
		segments = append(segments, "TOO*JP*"+element(detail.ToothNumber)+"~")
	}
	if detail.Surface != "" {
		segments = append(segments, "REF*D9*SURFACE-"+element(detail.Surface)+"~")
	}
	if detail.Quadrant != "" {
		segments = append(segments, "REF*D9*QUADRANT-"+element(detail.Quadrant)+"~")
	}
	if detail.Orthodontic {
		segments = append(segments, "CRC*ZZ*Y*ORTHO~")
	}
	return segments
}

func payloadMap(tx domain.Transaction) map[string]any {
	var payload map[string]any
	if err := json.Unmarshal(tx.Payload, &payload); err != nil {
		return map[string]any{}
	}
	return payload
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

func intValue(payload map[string]any, key string, fallback int) int {
	value, ok := payload[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
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
