package x12parser

import (
	"fmt"
	"strconv"
	"strings"

	"ashn/packages/domain"
)

type ParsedTransaction struct {
	Type               string
	Sender             Party
	Receiver           Party
	Enrollment         *Enrollment
	EligibilityInquiry *Eligibility
	PriorAuthorization *PriorAuthorization
	Attachment         *Attachment
	Claim              *Claim
	ClaimStatusRequest *ClaimStatus
	Payment            *Payment
	PremiumPayment     *PremiumPayment
}

type Party struct {
	ID string
}

type Enrollment struct {
	Name   string
	Rank   string
	Guild  string
	Region string
}

type Eligibility struct {
	AdventurerID string
	ProviderID   string
	ServiceType  string
}

type PriorAuthorization struct {
	AdventurerID     string
	ProviderID       string
	ServiceType      string
	IncidentSeverity string
	DentalService    DentalService
}

type DentalService struct {
	CDTCode     string
	ToothNumber string
	Surface     string
	Quadrant    string
	Orthodontic bool
}

type Claim struct {
	AdventurerID               string
	ProviderID                 string
	IncidentSeverity           string
	AmountCents                string
	AuthorizationTransactionID string
	ServiceLines               []ClaimServiceLine
	Diagnoses                  []ClaimDiagnosis
	AttachmentControls         []AttachmentControl
}

type ClaimServiceLine struct {
	LineNumber    int
	ProcedureCode string
	Description   string
	Units         int
	AmountCents   string
	CDTCode       string
	ToothNumber   string
	Surface       string
	Quadrant      string
	Orthodontic   bool
}

type ClaimDiagnosis struct {
	Qualifier   string
	Code        string
	Description string
	Primary     bool
}

type AttachmentControl struct {
	ReportTypeCode          string
	TransmissionCode        string
	AttachmentControlNumber string
}

type Attachment struct {
	PacketID                   string
	PacketSequence             int
	PacketCount                int
	ClaimID                    string
	AuthorizationTransactionID string
	ProviderID                 string
	AttachmentPurpose          string
	AttachmentTraceID          string
	AttachmentFormatCode       string
	AttachmentObjectType       string
	AttachmentEncoding         string
	AttachmentServiceDate      string
	AttachmentType             string
	AttachmentControlNumber    string
	ReportTypeCode             string
	TransmissionCode           string
	ContentType                string
	FileName                   string
	Description                string
	Content                    string
	DocumentReferenceID        string
	DocumentReferenceURL       string
}

type ClaimStatus struct {
	ClaimID string
}

type Payment struct {
	ClaimID            string
	PaymentAmountCents string
}

type PremiumPayment struct {
	AdventurerID string
	AmountCents  string
}

func Parse(body []byte) (ParsedTransaction, error) {
	segments := parseRawX12Segments(string(body))
	if len(segments) == 0 {
		return ParsedTransaction{}, fmt.Errorf("missing X12 segments")
	}
	segmentMap := map[string][][]string{}
	for _, segment := range segments {
		if len(segment) == 0 {
			continue
		}
		segmentMap[segment[0]] = append(segmentMap[segment[0]], segment)
	}
	st := firstRawSegment(segmentMap, "ST")
	if len(st) < 2 || strings.TrimSpace(st[1]) == "" {
		return ParsedTransaction{}, fmt.Errorf("missing ST transaction set")
	}
	parsed := ParsedTransaction{
		Type:     strings.TrimSpace(st[1]),
		Sender:   Party{ID: rawSenderID(segmentMap)},
		Receiver: Party{ID: rawReceiverID(segmentMap)},
	}
	switch domain.TransactionType(parsed.Type) {
	case domain.Tx834:
		enrollment, err := raw834Enrollment(segmentMap)
		if err != nil {
			return parsed, err
		}
		parsed.Enrollment = &enrollment
	case domain.Tx820:
		premium, err := raw820PremiumPayment(segmentMap)
		if err != nil {
			return parsed, err
		}
		parsed.PremiumPayment = &premium
	case domain.Tx270:
		eligibility, err := raw270Eligibility(segmentMap, parsed.Sender.ID)
		if err != nil {
			return parsed, err
		}
		parsed.EligibilityInquiry = &eligibility
	case domain.Tx276:
		claimStatus, err := raw276ClaimStatus(segmentMap)
		if err != nil {
			return parsed, err
		}
		parsed.ClaimStatusRequest = &claimStatus
	case domain.Tx278:
		priorAuth, err := raw278PriorAuthorization(segmentMap, parsed.Sender.ID)
		if err != nil {
			return parsed, err
		}
		parsed.PriorAuthorization = &priorAuth
	case domain.Tx837:
		claim, err := raw837Claim(segmentMap, parsed.Sender.ID)
		if err != nil {
			return parsed, err
		}
		parsed.Claim = &claim
	case domain.Tx275:
		attachment, err := raw275Attachment(segmentMap, parsed.Sender.ID)
		if err != nil {
			return parsed, err
		}
		parsed.Attachment = &attachment
	case domain.Tx835:
		payment, err := raw835Payment(segmentMap)
		if err != nil {
			return parsed, err
		}
		parsed.Payment = &payment
	default:
		return parsed, fmt.Errorf("raw X12 transaction type %s not implemented", parsed.Type)
	}
	return parsed, nil
}

func raw834Enrollment(segmentMap map[string][][]string) (Enrollment, error) {
	enrollment := Enrollment{
		Name:   firstNonEmpty(rawNM1Name(segmentMap, "IL"), rawK3Value(segmentMap, "Name")),
		Rank:   rawK3Value(segmentMap, "Rank"),
		Guild:  rawK3Value(segmentMap, "Guild"),
		Region: rawK3Value(segmentMap, "Region"),
	}
	if enrollment.Name == "" {
		return Enrollment{}, fmt.Errorf("missing subscriber NM1 segment")
	}
	if enrollment.Rank == "" || enrollment.Guild == "" || enrollment.Region == "" {
		return Enrollment{}, fmt.Errorf("missing enrollment K3 metadata")
	}
	return enrollment, nil
}

func raw820PremiumPayment(segmentMap map[string][][]string) (PremiumPayment, error) {
	premium := PremiumPayment{
		AdventurerID: rawNM1ID(segmentMap, "IL"),
		AmountCents:  raw820AmountCents(segmentMap),
	}
	if premium.AdventurerID == "" {
		return PremiumPayment{}, fmt.Errorf("missing subscriber NM1 segment")
	}
	if premium.AmountCents == "" {
		return PremiumPayment{}, fmt.Errorf("invalid premium amount")
	}
	return premium, nil
}

func raw270Eligibility(segmentMap map[string][][]string, senderID string) (Eligibility, error) {
	eligibility := Eligibility{
		AdventurerID: rawNM1ID(segmentMap, "IL"),
		ProviderID:   firstNonEmpty(rawNM1ID(segmentMap, "1P"), rawNM1ID(segmentMap, "85"), senderID),
	}
	if rawServiceType(segmentMap) == "35" {
		eligibility.ServiceType = "dental"
	}
	if eligibility.AdventurerID == "" {
		return Eligibility{}, fmt.Errorf("missing subscriber NM1 segment")
	}
	if eligibility.ProviderID == "" {
		return Eligibility{}, fmt.Errorf("missing provider NM1 segment")
	}
	return eligibility, nil
}

func raw276ClaimStatus(segmentMap map[string][][]string) (ClaimStatus, error) {
	claimID := rawClaimReference(segmentMap)
	if claimID == "" {
		return ClaimStatus{}, fmt.Errorf("missing claim REF segment")
	}
	return ClaimStatus{ClaimID: claimID}, nil
}

func raw278PriorAuthorization(segmentMap map[string][][]string, senderID string) (PriorAuthorization, error) {
	priorAuth := PriorAuthorization{
		AdventurerID:     rawNM1ID(segmentMap, "IL"),
		ProviderID:       firstNonEmpty(rawNM1ID(segmentMap, "1P"), senderID),
		ServiceType:      raw278ServiceType(segmentMap),
		IncidentSeverity: rawSeverity(segmentMap),
	}
	if priorAuth.AdventurerID == "" {
		return PriorAuthorization{}, fmt.Errorf("missing subscriber NM1 segment")
	}
	if priorAuth.ProviderID == "" {
		return PriorAuthorization{}, fmt.Errorf("missing provider NM1 segment")
	}
	if priorAuth.ServiceType == "" {
		return PriorAuthorization{}, fmt.Errorf("missing UM service type")
	}
	return priorAuth, nil
}

func raw837Claim(segmentMap map[string][][]string, senderID string) (Claim, error) {
	clm := firstRawSegment(segmentMap, "CLM")
	if len(clm) < 3 {
		return Claim{}, fmt.Errorf("missing CLM claim segment")
	}
	claim := Claim{
		ProviderID:         firstNonEmpty(rawNM1ID(segmentMap, "85"), rawNM1ID(segmentMap, "41"), senderID),
		AdventurerID:       rawNM1ID(segmentMap, "IL"),
		IncidentSeverity:   rawSeverity(segmentMap),
		AmountCents:        rawAmountCents(clm[2]),
		ServiceLines:       raw837ServiceLines(segmentMap),
		Diagnoses:          raw837Diagnoses(segmentMap),
		AttachmentControls: raw837AttachmentControls(segmentMap),
	}
	if claim.AdventurerID == "" {
		return Claim{}, fmt.Errorf("missing subscriber NM1 segment")
	}
	if claim.ProviderID == "" {
		return Claim{}, fmt.Errorf("missing provider NM1 segment")
	}
	if claim.AmountCents == "" {
		return Claim{}, fmt.Errorf("invalid CLM amount")
	}
	if claim.IncidentSeverity == "" {
		claim.IncidentSeverity = string(domain.SeverityNormal)
	}
	return claim, nil
}

func raw275Attachment(segmentMap map[string][][]string, senderID string) (Attachment, error) {
	attachment := Attachment{
		ProviderID:       firstNonEmpty(rawNM1ID(segmentMap, "1P"), senderID),
		ContentType:      "text/plain",
		Description:      "Raw X12 patient information attachment",
		TransmissionCode: "EL",
	}
	if bgn := firstRawSegment(segmentMap, "BGN"); len(bgn) > 2 {
		attachment.AttachmentPurpose = attachmentPurposeFromBGN01(bgn[1])
		attachment.AttachmentTraceID = strings.TrimSpace(bgn[2])
	}
	for _, ref := range segmentMap["REF"] {
		if len(ref) < 3 {
			continue
		}
		switch strings.TrimSpace(ref[1]) {
		case "1K":
			attachment.ClaimID = strings.TrimSpace(ref[2])
		case "G1":
			attachment.AuthorizationTransactionID = strings.TrimSpace(ref[2])
		case "6R":
			if attachment.AttachmentControlNumber == "" {
				attachment.AttachmentControlNumber = strings.TrimSpace(ref[2])
			}
		case "F8":
			attachment.PacketID, attachment.PacketSequence, attachment.PacketCount = rawPacketReference(ref[2])
		}
	}
	if pwk := firstRawSegment(segmentMap, "PWK"); len(pwk) > 2 {
		attachment.ReportTypeCode = strings.TrimSpace(pwk[1])
		attachment.TransmissionCode = strings.TrimSpace(pwk[2])
		if len(pwk) > 6 && attachment.AttachmentControlNumber == "" {
			attachment.AttachmentControlNumber = strings.TrimSpace(pwk[6])
		}
	}
	if dtp := firstRawSegment(segmentMap, "DTP"); len(dtp) > 3 && strings.TrimSpace(dtp[1]) == "472" {
		attachment.AttachmentServiceDate = rawDate(dtp[3])
	}
	if cat := firstRawSegment(segmentMap, "CAT"); len(cat) > 2 {
		if attachment.ReportTypeCode == "" {
			attachment.ReportTypeCode = strings.TrimSpace(cat[1])
		}
		attachment.AttachmentFormatCode = strings.TrimSpace(cat[2])
	}
	if ooi := firstRawSegment(segmentMap, "OOI"); len(ooi) > 1 {
		attachment.AttachmentObjectType = strings.TrimSpace(ooi[1])
	}
	if bds := firstRawSegment(segmentMap, "BDS"); len(bds) > 1 {
		attachment.AttachmentEncoding = strings.TrimSpace(bds[1])
		if len(bds) > 3 && attachment.ContentType == "" {
			applyRawK3(&attachment, bds[3])
		}
	}
	if lq := firstRawSegment(segmentMap, "LQ"); len(lq) > 2 {
		attachment.AttachmentType = strings.TrimSpace(lq[2])
	}
	if nte := firstRawSegment(segmentMap, "NTE"); len(nte) > 2 {
		attachment.Description = strings.TrimSpace(nte[2])
	}
	if k3 := firstRawSegment(segmentMap, "K3"); len(k3) > 1 {
		applyRawK3(&attachment, k3[1])
	}
	if bin := firstRawSegment(segmentMap, "BIN"); len(bin) > 2 {
		attachment.Content = strings.TrimSpace(bin[2])
	}
	if attachment.DocumentReferenceURL == "" && attachment.Content == "" {
		attachment.Content = "Raw X12 attachment metadata only."
	}
	if attachment.ProviderID == "" {
		return Attachment{}, fmt.Errorf("missing provider NM1 segment")
	}
	return attachment, nil
}

func raw835Payment(segmentMap map[string][][]string) (Payment, error) {
	payment := Payment{}
	if clp := firstRawSegment(segmentMap, "CLP"); len(clp) > 4 {
		payment.ClaimID = strings.TrimSpace(clp[1])
		payment.PaymentAmountCents = rawAmountCents(clp[4])
	}
	if payment.PaymentAmountCents == "" {
		payment.PaymentAmountCents = raw835BPRAmountCents(segmentMap)
	}
	if payment.ClaimID == "" {
		return Payment{}, fmt.Errorf("missing CLP claim segment")
	}
	if payment.PaymentAmountCents == "" {
		return Payment{}, fmt.Errorf("invalid payment amount")
	}
	return payment, nil
}

func parseRawX12Segments(raw string) [][]string {
	parts := strings.Split(raw, "~")
	segments := make([][]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ReplaceAll(part, "\n", ""))
		if part == "" {
			continue
		}
		elements := strings.Split(part, "*")
		for index := range elements {
			elements[index] = strings.TrimSpace(elements[index])
		}
		segments = append(segments, elements)
	}
	return segments
}

func firstRawSegment(segmentMap map[string][][]string, id string) []string {
	segments := segmentMap[id]
	if len(segments) == 0 {
		return nil
	}
	return segments[0]
}

func rawSenderID(segmentMap map[string][][]string) string {
	if gs := firstRawSegment(segmentMap, "GS"); len(gs) > 2 {
		return strings.TrimSpace(gs[2])
	}
	if isa := firstRawSegment(segmentMap, "ISA"); len(isa) > 6 {
		return strings.TrimSpace(isa[6])
	}
	return ""
}

func rawReceiverID(segmentMap map[string][][]string) string {
	if gs := firstRawSegment(segmentMap, "GS"); len(gs) > 3 {
		return strings.TrimSpace(gs[3])
	}
	if isa := firstRawSegment(segmentMap, "ISA"); len(isa) > 8 {
		return strings.TrimSpace(isa[8])
	}
	return ""
}

func raw837ServiceLines(segmentMap map[string][][]string) []ClaimServiceLine {
	serviceLines := []ClaimServiceLine{}
	for index, sv1 := range segmentMap["SV1"] {
		if len(sv1) < 3 {
			continue
		}
		amountCents := rawAmountCents(sv1[2])
		if amountCents == "" {
			continue
		}
		line := ClaimServiceLine{
			LineNumber:    index + 1,
			ProcedureCode: rawProcedureCode(sv1[1]),
			Description:   "Raw X12 service line",
			Units:         rawServiceUnits(sv1),
			AmountCents:   amountCents,
		}
		if len(sv1) > 7 {
			if parsed, err := strconv.Atoi(strings.TrimSpace(sv1[7])); err == nil && parsed > 0 {
				line.LineNumber = parsed
			}
		}
		serviceLines = append(serviceLines, line)
	}
	return serviceLines
}

func raw837Diagnoses(segmentMap map[string][][]string) []ClaimDiagnosis {
	diagnoses := []ClaimDiagnosis{}
	for _, hi := range segmentMap["HI"] {
		for _, rawElement := range hi[1:] {
			parts := strings.SplitN(rawElement, ":", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
				continue
			}
			qualifier := strings.TrimSpace(parts[0])
			diagnoses = append(diagnoses, ClaimDiagnosis{
				Qualifier: qualifier,
				Code:      strings.TrimSpace(parts[1]),
				Primary:   strings.EqualFold(qualifier, "ABK") || len(diagnoses) == 0,
			})
		}
	}
	return diagnoses
}

func raw837AttachmentControls(segmentMap map[string][][]string) []AttachmentControl {
	controls := []AttachmentControl{}
	for _, pwk := range segmentMap["PWK"] {
		if len(pwk) < 7 {
			continue
		}
		control := strings.TrimSpace(pwk[6])
		if control == "" {
			continue
		}
		controls = append(controls, AttachmentControl{
			ReportTypeCode:          strings.TrimSpace(pwk[1]),
			TransmissionCode:        strings.TrimSpace(pwk[2]),
			AttachmentControlNumber: control,
		})
	}
	for _, ref := range segmentMap["REF"] {
		if len(ref) < 3 || strings.TrimSpace(ref[1]) != "6R" {
			continue
		}
		control := strings.TrimSpace(ref[2])
		if control == "" || hasAttachmentControl(controls, control) {
			continue
		}
		controls = append(controls, AttachmentControl{AttachmentControlNumber: control})
	}
	return controls
}

func hasAttachmentControl(controls []AttachmentControl, control string) bool {
	for _, existing := range controls {
		if strings.EqualFold(existing.AttachmentControlNumber, control) {
			return true
		}
	}
	return false
}

func rawNM1ID(segmentMap map[string][][]string, entityCode string) string {
	for _, nm1 := range segmentMap["NM1"] {
		if len(nm1) < 4 || !strings.EqualFold(nm1[1], entityCode) {
			continue
		}
		if len(nm1) > 9 && strings.TrimSpace(nm1[9]) != "" {
			return strings.TrimSpace(nm1[9])
		}
		return strings.TrimSpace(nm1[3])
	}
	return ""
}

func rawNM1Name(segmentMap map[string][][]string, entityCode string) string {
	for _, nm1 := range segmentMap["NM1"] {
		if len(nm1) >= 4 && strings.EqualFold(nm1[1], entityCode) {
			return strings.TrimSpace(nm1[3])
		}
	}
	return ""
}

func rawK3Value(segmentMap map[string][][]string, key string) string {
	prefix := strings.ToLower(strings.TrimSpace(key)) + ":"
	for _, k3 := range segmentMap["K3"] {
		if len(k3) < 2 {
			continue
		}
		value := strings.TrimSpace(k3[1])
		if strings.HasPrefix(strings.ToLower(value), prefix) {
			return strings.TrimSpace(value[len(prefix):])
		}
	}
	return ""
}

func rawClaimReference(segmentMap map[string][][]string) string {
	for _, ref := range segmentMap["REF"] {
		if len(ref) >= 3 && strings.EqualFold(strings.TrimSpace(ref[1]), "1K") {
			return strings.TrimSpace(ref[2])
		}
	}
	return ""
}

func rawServiceType(segmentMap map[string][][]string) string {
	if eq := firstRawSegment(segmentMap, "EQ"); len(eq) > 1 {
		return strings.TrimSpace(eq[1])
	}
	return ""
}

func raw278ServiceType(segmentMap map[string][][]string) string {
	if um := firstRawSegment(segmentMap, "UM"); len(um) > 6 && strings.TrimSpace(um[6]) != "" {
		return strings.TrimSpace(um[6])
	}
	if sv1 := firstRawSegment(segmentMap, "SV1"); len(sv1) > 1 {
		return rawProcedureCode(sv1[1])
	}
	return ""
}

func rawProcedureCode(composite string) string {
	parts := strings.Split(composite, ":")
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(composite)
}

func rawServiceUnits(segment []string) int {
	if len(segment) > 4 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(segment[4])); err == nil && parsed > 0 {
			return parsed
		}
	}
	return 1
}

func rawSeverity(segmentMap map[string][][]string) string {
	for _, hi := range segmentMap["HI"] {
		for _, element := range hi[1:] {
			parts := strings.SplitN(element, ":", 2)
			if len(parts) != 2 {
				continue
			}
			switch strings.TrimSpace(parts[1]) {
			case "S610":
				return string(domain.SeverityNormal)
			case "T509":
				return string(domain.SeverityAwakened)
			case "S062X9A":
				return string(domain.SeverityDiamond)
			}
		}
	}
	return string(domain.SeverityNormal)
}

func rawAmountCents(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return ""
	}
	if strings.Contains(normalized, ".") {
		amount, err := strconv.ParseFloat(normalized, 64)
		if err != nil || amount <= 0 {
			return ""
		}
		return strconv.FormatInt(int64(amount*100+0.5), 10)
	}
	amount, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil || amount <= 0 {
		return ""
	}
	return strconv.FormatInt(amount*100, 10)
}

func rawDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 8 {
		return value[:4] + "-" + value[4:6] + "-" + value[6:]
	}
	return value
}

func raw820AmountCents(segmentMap map[string][][]string) string {
	if rmr := firstRawSegment(segmentMap, "RMR"); len(rmr) > 4 && strings.TrimSpace(rmr[4]) != "" {
		return rawAmountCents(rmr[4])
	}
	if bpr := firstRawSegment(segmentMap, "BPR"); len(bpr) > 2 {
		return rawAmountCents(bpr[2])
	}
	return ""
}

func raw835BPRAmountCents(segmentMap map[string][][]string) string {
	if bpr := firstRawSegment(segmentMap, "BPR"); len(bpr) > 2 {
		return rawAmountCents(bpr[2])
	}
	return ""
}

func rawPacketReference(value string) (string, int, int) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "-")
	if len(parts) >= 4 && strings.EqualFold(parts[len(parts)-2], "OF") {
		sequence, _ := strconv.Atoi(parts[len(parts)-3])
		count, _ := strconv.Atoi(parts[len(parts)-1])
		return strings.Join(parts[:len(parts)-3], "-"), sequence, count
	}
	return value, 0, 0
}

func applyRawK3(attachment *Attachment, value string) {
	value = strings.TrimSpace(value)
	switch {
	case strings.HasPrefix(value, "Document-Reference:"):
		reference := strings.TrimSpace(strings.TrimPrefix(value, "Document-Reference:"))
		if strings.HasPrefix(reference, "https://") || strings.HasPrefix(reference, "s3://") || strings.HasPrefix(reference, "gs://") {
			attachment.DocumentReferenceURL = reference
		} else {
			attachment.DocumentReferenceID = reference
		}
	case strings.HasPrefix(value, "Content-Type:"):
		attachment.ContentType = strings.TrimSpace(strings.TrimPrefix(value, "Content-Type:"))
	}
}

func attachmentPurposeFromBGN01(value string) string {
	switch strings.TrimSpace(value) {
	case "11":
		return "solicited"
	case "02":
		return "unsolicited"
	default:
		return strings.TrimSpace(value)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
