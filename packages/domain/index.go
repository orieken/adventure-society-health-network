package domain

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

type Rank string

const (
	RankIron    Rank = "Iron"
	RankBronze  Rank = "Bronze"
	RankSilver  Rank = "Silver"
	RankGold    Rank = "Gold"
	RankDiamond Rank = "Diamond"
)

type Region string

const (
	RegionGreenstone Region = "Greenstone"
	RegionYaresh     Region = "Yaresh"
	RegionRimaros    Region = "Rimaros"
	RegionVitesse    Region = "Vitesse"
)

type ProviderType string

const (
	ProviderTypeClinic  ProviderType = "Clinic"
	ProviderTypeTemple  ProviderType = "Temple"
	ProviderTypeOutpost ProviderType = "Outpost"
)

type CoverageStatus string

const (
	CoverageActive    CoverageStatus = "Active"
	CoverageInactive  CoverageStatus = "Inactive"
	CoveragePending   CoverageStatus = "Pending"
	CoverageSuspended CoverageStatus = "Suspended"
)

type TransactionType string

const (
	Tx834   TransactionType = "834"
	Tx820   TransactionType = "820"
	Tx270   TransactionType = "270"
	Tx271   TransactionType = "271"
	Tx275   TransactionType = "275"
	Tx278   TransactionType = "278"
	Tx837   TransactionType = "837"
	Tx835   TransactionType = "835"
	Tx276   TransactionType = "276"
	Tx277   TransactionType = "277"
	Tx269   TransactionType = "269"
	Tx999   TransactionType = "999"
	Tx277CA TransactionType = "277CA"
)

type TransactionStatus string

const (
	TxStatusCreated    TransactionStatus = "Created"
	TxStatusDispatched TransactionStatus = "Dispatched"
	TxStatusAccepted   TransactionStatus = "Accepted"
	TxStatusPending    TransactionStatus = "Pending"
	TxStatusApproved   TransactionStatus = "Approved"
	TxStatusDenied     TransactionStatus = "Denied"
	TxStatusPaid       TransactionStatus = "Paid"
	TxStatusFailed     TransactionStatus = "Failed"
)

type IncidentSeverity string

const (
	SeverityNormal   IncidentSeverity = "Normal"
	SeverityAwakened IncidentSeverity = "Awakened"
	SeverityDiamond  IncidentSeverity = "Diamond"
)

type ClaimStatus string

const (
	ClaimSubmitted            ClaimStatus = "Submitted"
	ClaimPending              ClaimStatus = "Pending"
	ClaimPendingDocumentation ClaimStatus = "Pending Documentation"
	ClaimApproved             ClaimStatus = "Approved"
	ClaimDenied               ClaimStatus = "Denied"
	ClaimPaid                 ClaimStatus = "Paid"
)

type Adventurer struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Rank           Rank           `json:"rank"`
	Guild          string         `json:"guild"`
	Region         Region         `json:"region"`
	CoverageStatus CoverageStatus `json:"coverageStatus"`
}

type Provider struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	ProviderType ProviderType `json:"providerType"`
	TierRank     Rank         `json:"tierRank"`
	Region       Region       `json:"region"`
}

type TradingPartner struct {
	ID                      string                   `json:"id"`
	Name                    string                   `json:"name"`
	SenderID                string                   `json:"senderId"`
	ReceiverID              string                   `json:"receiverId"`
	AllowedTransactionTypes []string                 `json:"allowedTransactionTypes"`
	RouteTarget             string                   `json:"routeTarget"`
	Status                  string                   `json:"status"`
	ValidationProfile       PartnerValidationProfile `json:"validationProfile,omitempty"`
}

type PartnerValidationProfile struct {
	AttachmentTypes         []string `json:"attachmentTypes,omitempty"`
	ReportTypeCodes         []string `json:"reportTypeCodes,omitempty"`
	TransmissionCodes       []string `json:"transmissionCodes,omitempty"`
	ContentTypes            []string `json:"contentTypes,omitempty"`
	ControlNumberPrefixes   []string `json:"controlNumberPrefixes,omitempty"`
	MaxEmbeddedContentBytes int      `json:"maxEmbeddedContentBytes,omitempty"`
	ServiceTypes            []string `json:"serviceTypes,omitempty"`
	IncidentSeverities      []string `json:"incidentSeverities,omitempty"`
	DiagnosisQualifiers     []string `json:"diagnosisQualifiers,omitempty"`
	DiagnosisCodes          []string `json:"diagnosisCodes,omitempty"`
	ProcedureCodePrefixes   []string `json:"procedureCodePrefixes,omitempty"`
	ProcedureCodes          []string `json:"procedureCodes,omitempty"`
}

type Transaction struct {
	ID         string            `json:"id"`
	Type       TransactionType   `json:"type"`
	Status     TransactionStatus `json:"status"`
	SenderID   string            `json:"senderId"`
	ReceiverID string            `json:"receiverId"`
	Payload    json.RawMessage   `json:"payload"`
	RawX12     string            `json:"rawX12,omitempty"`
	RelatedID  string            `json:"relatedId,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
}

type DocumentReference struct {
	TransactionID              string `json:"transactionId"`
	ClaimID                    string `json:"claimId,omitempty"`
	AuthorizationTransactionID string `json:"authorizationTransactionId,omitempty"`
	AttachmentType             string `json:"attachmentType,omitempty"`
	AttachmentControlNumber    string `json:"attachmentControlNumber,omitempty"`
	ReportTypeCode             string `json:"reportTypeCode,omitempty"`
	ContentType                string `json:"contentType,omitempty"`
	Description                string `json:"description,omitempty"`
	DocumentReferenceID        string `json:"documentReferenceId,omitempty"`
	DocumentReferenceURL       string `json:"documentReferenceUrl,omitempty"`
	EmbeddedContentAvailable   bool   `json:"embeddedContentAvailable"`
	RetrievalMode              string `json:"retrievalMode"`
	RetrievalStatus            string `json:"retrievalStatus"`
	RetrievalInstructions      string `json:"retrievalInstructions"`
}

type InboundMessage struct {
	ID               string    `json:"id"`
	PartnerID        string    `json:"partnerId,omitempty"`
	ContentType      string    `json:"contentType"`
	TransactionType  string    `json:"transactionType,omitempty"`
	RawPayload       string    `json:"rawPayload"`
	Status           string    `json:"status"`
	Error            string    `json:"error,omitempty"`
	DownstreamStatus int       `json:"downstreamStatus,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Claim struct {
	ID                         string             `json:"id"`
	AdventurerID               string             `json:"adventurerId"`
	ProviderID                 string             `json:"providerId"`
	IncidentSeverity           IncidentSeverity   `json:"incidentSeverity"`
	TransactionID              string             `json:"transactionId"`
	AuthorizationTransactionID string             `json:"authorizationTransactionId,omitempty"`
	AuthorizationStatus        string             `json:"authorizationStatus,omitempty"`
	AuthorizationReason        string             `json:"authorizationReason,omitempty"`
	AmountCents                int64              `json:"amountCents"`
	AllowedAmountCents         int64              `json:"allowedAmountCents,omitempty"`
	PaidAmountCents            int64              `json:"paidAmountCents,omitempty"`
	PatientResponsibilityCents int64              `json:"patientResponsibilityCents,omitempty"`
	AdjustmentAmountCents      int64              `json:"adjustmentAmountCents,omitempty"`
	AdjustmentReason           string             `json:"adjustmentReason,omitempty"`
	DenialReason               string             `json:"denialReason,omitempty"`
	Status                     ClaimStatus        `json:"status"`
	ServiceLines               []ClaimServiceLine `json:"serviceLines,omitempty"`
	Diagnoses                  []ClaimDiagnosis   `json:"diagnoses,omitempty"`
}

type ClaimServiceLine struct {
	LineNumber                 int    `json:"lineNumber"`
	ProcedureCode              string `json:"procedureCode"`
	Description                string `json:"description"`
	Units                      int    `json:"units"`
	AmountCents                int64  `json:"amountCents"`
	AllowedAmountCents         int64  `json:"allowedAmountCents,omitempty"`
	PaidAmountCents            int64  `json:"paidAmountCents,omitempty"`
	PatientResponsibilityCents int64  `json:"patientResponsibilityCents,omitempty"`
	AdjustmentAmountCents      int64  `json:"adjustmentAmountCents,omitempty"`
	AdjustmentReason           string `json:"adjustmentReason,omitempty"`
	DenialReason               string `json:"denialReason,omitempty"`
}

type ClaimDiagnosis struct {
	Qualifier   string `json:"qualifier"`
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
	Primary     bool   `json:"primary,omitempty"`
}

type EnrollmentRequest struct {
	Name   string `json:"name"`
	Rank   Rank   `json:"rank"`
	Guild  string `json:"guild"`
	Region Region `json:"region"`
}

type EligibilityRequest struct {
	AdventurerID string `json:"adventurerId"`
	ProviderID   string `json:"providerId"`
}

type PriorAuthRequest struct {
	AdventurerID     string           `json:"adventurerId"`
	ProviderID       string           `json:"providerId"`
	ServiceType      string           `json:"serviceType"`
	IncidentSeverity IncidentSeverity `json:"incidentSeverity"`
}

type AuthorizationDecisionRequest struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
}

type ClaimRequest struct {
	AdventurerID               string             `json:"adventurerId"`
	ProviderID                 string             `json:"providerId"`
	IncidentSeverity           IncidentSeverity   `json:"incidentSeverity"`
	AmountCents                int64              `json:"amountCents"`
	AuthorizationTransactionID string             `json:"authorizationTransactionId,omitempty"`
	ServiceLines               []ClaimServiceLine `json:"serviceLines,omitempty"`
	Diagnoses                  []ClaimDiagnosis   `json:"diagnoses,omitempty"`
}

type DocumentationChecklistItem struct {
	Code           string `json:"code"`
	Label          string `json:"label"`
	AttachmentType string `json:"attachmentType"`
	ReportTypeCode string `json:"reportTypeCode"`
	ContentType    string `json:"contentType"`
	Required       bool   `json:"required"`
}

type ClaimDocumentationRequest struct {
	Reason            string                       `json:"reason,omitempty"`
	DueDate           string                       `json:"dueDate,omitempty"`
	RequiredDocuments []DocumentationChecklistItem `json:"requiredDocuments,omitempty"`
}

type AttachmentRequest struct {
	PacketID                string `json:"packetId,omitempty"`
	PacketSequence          int    `json:"packetSequence,omitempty"`
	PacketCount             int    `json:"packetCount,omitempty"`
	AttachmentType          string `json:"attachmentType"`
	AttachmentControlNumber string `json:"attachmentControlNumber"`
	ReportTypeCode          string `json:"reportTypeCode"`
	TransmissionCode        string `json:"transmissionCode"`
	ContentType             string `json:"contentType"`
	Description             string `json:"description"`
	Content                 string `json:"content"`
	DocumentReferenceID     string `json:"documentReferenceId,omitempty"`
	DocumentReferenceURL    string `json:"documentReferenceUrl,omitempty"`
}

type AttachmentPacketRequest struct {
	PacketID    string              `json:"packetId,omitempty"`
	Attachments []AttachmentRequest `json:"attachments"`
}

type AttachmentReviewRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type PaymentRequest struct {
	PaymentAmountCents int64 `json:"paymentAmountCents"`
}

type PageInfo struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Count   int  `json:"count"`
	HasMore bool `json:"hasMore"`
}

type TransactionJob struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	EntityID   string    `json:"entityId"`
	Status     string    `json:"status"`
	Attempts   int       `json:"attempts"`
	RunAfter   time.Time `json:"runAfter"`
	LastError  string    `json:"lastError,omitempty"`
	DeadLetter bool      `json:"deadLetter"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Envelope struct {
	Data         any           `json:"data,omitempty"`
	Lore         string        `json:"lore,omitempty"`
	Transaction  *Transaction  `json:"transaction,omitempty"`
	Transactions []Transaction `json:"transactions,omitempty"`
	Page         *PageInfo     `json:"page,omitempty"`
}

type ErrorEnvelope struct {
	Error string `json:"error"`
	Lore  string `json:"lore"`
}

func NewID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return hex.EncodeToString(bytes[0:4]) + "-" +
		hex.EncodeToString(bytes[4:6]) + "-" +
		hex.EncodeToString(bytes[6:8]) + "-" +
		hex.EncodeToString(bytes[8:10]) + "-" +
		hex.EncodeToString(bytes[10:16])
}

func Payload(value any) json.RawMessage {
	bytes, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return bytes
}
