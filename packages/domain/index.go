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
	ClaimSubmitted ClaimStatus = "Submitted"
	ClaimPending   ClaimStatus = "Pending"
	ClaimApproved  ClaimStatus = "Approved"
	ClaimDenied    ClaimStatus = "Denied"
	ClaimPaid      ClaimStatus = "Paid"
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

type InboundMessage struct {
	ID               string    `json:"id"`
	ContentType      string    `json:"contentType"`
	TransactionType  string    `json:"transactionType,omitempty"`
	RawPayload       string    `json:"rawPayload"`
	Status           string    `json:"status"`
	Error            string    `json:"error,omitempty"`
	DownstreamStatus int       `json:"downstreamStatus,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Claim struct {
	ID               string           `json:"id"`
	AdventurerID     string           `json:"adventurerId"`
	ProviderID       string           `json:"providerId"`
	IncidentSeverity IncidentSeverity `json:"incidentSeverity"`
	TransactionID    string           `json:"transactionId"`
	AmountCents      int64            `json:"amountCents"`
	Status           ClaimStatus      `json:"status"`
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

type ClaimRequest struct {
	AdventurerID     string           `json:"adventurerId"`
	ProviderID       string           `json:"providerId"`
	IncidentSeverity IncidentSeverity `json:"incidentSeverity"`
	AmountCents      int64            `json:"amountCents"`
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
