package api

import "time"

// OrderType classifies the compulsory order under MHCAA 1992.
type OrderType string

const (
	OrderCAO          OrderType = "CAO"
	OrderCTOInpatient OrderType = "CTO-inpatient"
	OrderCTOCommunity OrderType = "CTO-community"
	OrderSPO          OrderType = "SPO"
)

// OrderStatus tracks the MHCAA 1992 order lifecycle.
type OrderStatus string

const (
	OrderActive    OrderStatus = "active"
	OrderSuspended OrderStatus = "suspended"
	OrderExpired   OrderStatus = "expired"
	OrderRevoked   OrderStatus = "revoked"
	OrderAppealed  OrderStatus = "appealed"
)

// CompulsoryOrder represents a MHCAA 1992 compulsory order record.
type CompulsoryOrder struct {
	ID                string      `json:"id"`
	PatientID         string      `json:"patientId"`
	PatientNHI        string      `json:"patientNhi"`
	TenantID          string      `json:"tenantId"`
	EpisodeID         string      `json:"episodeId,omitempty"`
	OrderType         OrderType   `json:"orderType"`
	Status            OrderStatus `json:"status"`
	ResponsibleHPI    string      `json:"responsibleHpi"`
	SecondOpinionHPI  string      `json:"secondOpinionHpi,omitempty"`
	LegalAuthority    string      `json:"legalAuthority,omitempty"`
	Conditions        string      `json:"conditions,omitempty"` // decrypted
	IssuedDate        string      `json:"issuedDate"`           // YYYY-MM-DD
	ExpiryDate        string      `json:"expiryDate"`           // YYYY-MM-DD
	FirstReviewDate   string      `json:"firstReviewDate"`      // YYYY-MM-DD
	LastReviewDate    string      `json:"lastReviewDate,omitempty"`
	NextReviewDate    string      `json:"nextReviewDate"` // YYYY-MM-DD
	TribunalReference string      `json:"tribunalReference,omitempty"`
	ExtraSensitive    bool        `json:"extraSensitive"`
	CreatedAt         time.Time   `json:"createdAt"`
	UpdatedAt         time.Time   `json:"updatedAt"`
}

// orderCreateRequest is the body for POST /api/v1/orders.
type orderCreateRequest struct {
	PatientID        string    `json:"patientId"`
	PatientNHI       string    `json:"patientNhi"`
	EpisodeID        string    `json:"episodeId,omitempty"`
	OrderType        OrderType `json:"orderType"`
	ResponsibleHPI   string    `json:"responsibleHpi"`
	SecondOpinionHPI string    `json:"secondOpinionHpi,omitempty"`
	LegalAuthority   string    `json:"legalAuthority,omitempty"`
	Conditions       string    `json:"conditions,omitempty"`
	IssuedDate       string    `json:"issuedDate"`      // YYYY-MM-DD
	ExpiryDate       string    `json:"expiryDate"`      // YYYY-MM-DD
	FirstReviewDate  string    `json:"firstReviewDate"` // YYYY-MM-DD
	NextReviewDate   string    `json:"nextReviewDate"`  // YYYY-MM-DD
}

// orderUpdateRequest is the body for PUT /api/v1/orders/{id}.
type orderUpdateRequest struct {
	ResponsibleHPI    string      `json:"responsibleHpi,omitempty"`
	SecondOpinionHPI  string      `json:"secondOpinionHpi,omitempty"`
	LegalAuthority    string      `json:"legalAuthority,omitempty"`
	Conditions        string      `json:"conditions,omitempty"`
	ExpiryDate        string      `json:"expiryDate,omitempty"`
	NextReviewDate    string      `json:"nextReviewDate,omitempty"`
	TribunalReference string      `json:"tribunalReference,omitempty"`
	Status            OrderStatus `json:"status,omitempty"`
}

// reviewRequest is the body for POST /api/v1/orders/{id}/review.
// Records a mandatory legal review of an active compulsory order.
type reviewRequest struct {
	ReviewedAt     string `json:"reviewedAt"`     // YYYY-MM-DD
	NextReviewDate string `json:"nextReviewDate"` // YYYY-MM-DD
	Outcome        string `json:"outcome"`        // "continued", "varied", "discharged"
	Notes          string `json:"notes,omitempty"`
}

// revokeRequest is the body for POST /api/v1/orders/{id}/revoke.
type revokeRequest struct {
	Reason string `json:"reason"`
}

// orderRecord is the internal DB representation of a compulsory order.
type orderRecord struct {
	ID                string
	PatientID         string
	PatientNHI        string
	TenantID          string
	EpisodeID         string
	OrderType         string
	Status            string
	ResponsibleHPI    string
	SecondOpinionHPI  string
	LegalAuthority    string
	ConditionsEnc     []byte
	IssuedDate        string
	ExpiryDate        string
	FirstReviewDate   string
	LastReviewDate    string
	NextReviewDate    string
	RevocationEnc     []byte
	TribunalReference string
	ExtraSensitive    bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
