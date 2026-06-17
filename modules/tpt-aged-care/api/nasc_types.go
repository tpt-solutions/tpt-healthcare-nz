package api

import "time"

// NASCReferralStatus tracks the lifecycle of a referral to the NASC organisation.
type NASCReferralStatus string

const (
	ReferralPending   NASCReferralStatus = "pending"
	ReferralAccepted  NASCReferralStatus = "accepted"
	ReferralAssessing NASCReferralStatus = "assessing"
	ReferralCompleted NASCReferralStatus = "completed"
	ReferralDeclined  NASCReferralStatus = "declined"
	ReferralWithdrawn NASCReferralStatus = "withdrawn"
)

// SupportNeedsLevel is the output tier assigned by the NASC after assessment.
// Levels determine the maximum weekly funded hours and service types available.
type SupportNeedsLevel string

const (
	NeedsLevelLow      SupportNeedsLevel = "low"
	NeedsLevelModerate SupportNeedsLevel = "moderate"
	NeedsLevelHigh     SupportNeedsLevel = "high"
	NeedsLevelComplex  SupportNeedsLevel = "complex"
)

// ServicePlanStatus tracks the lifecycle of a funded service plan.
type ServicePlanStatus string

const (
	PlanActive    ServicePlanStatus = "active"
	PlanExpiring  ServicePlanStatus = "expiring"
	PlanExpired   ServicePlanStatus = "expired"
	PlanSuspended ServicePlanStatus = "suspended"
	PlanClosed    ServicePlanStatus = "closed"
)

// FundedService is a single service line in an NASC service plan.
type FundedService struct {
	ServiceType  string  `json:"serviceType"` // e.g., "personal-care", "domestic", "day-programme"
	HoursPerWeek float64 `json:"hoursPerWeek"`
	ProviderID   string  `json:"providerId,omitempty"`
	ProviderName string  `json:"providerName,omitempty"`
	StartDate    string  `json:"startDate"` // YYYY-MM-DD
	EndDate      string  `json:"endDate,omitempty"`
}

// NASCReferral represents a referral sent to the NASC for needs assessment.
type NASCReferral struct {
	ID             string             `json:"id"`
	PatientID      string             `json:"patientId"`
	PatientNHI     string             `json:"patientNhi"`
	ReferrerHPI    string             `json:"referrerHpi"`
	TenantID       string             `json:"tenantId"`
	Status         NASCReferralStatus `json:"status"`
	ReferralReason string             `json:"referralReason"`
	UrgencyFlag    bool               `json:"urgencyFlag"`
	// NASCOrgCode is the DHB-region NASC organisation code (MoH-assigned).
	NASCOrgCode   string     `json:"nascOrgCode"`
	InterRAIRefID string     `json:"interraiRefId,omitempty"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
	DeclineReason string     `json:"declineReason,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// NASCServicePlan represents the funded support plan produced after NASC assessment.
type NASCServicePlan struct {
	ID            string            `json:"id"`
	PatientID     string            `json:"patientId"`
	PatientNHI    string            `json:"patientNhi"`
	TenantID      string            `json:"tenantId"`
	ReferralID    string            `json:"referralId"`
	Status        ServicePlanStatus `json:"status"`
	NeedsLevel    SupportNeedsLevel `json:"needsLevel"`
	Services      []FundedService   `json:"services"`
	// GoalsNotes is AES-256-GCM encrypted at rest.
	GoalsNotes     string    `json:"goalsNotes,omitempty"`
	PlanStartDate  string    `json:"planStartDate"` // YYYY-MM-DD
	PlanEndDate    string    `json:"planEndDate,omitempty"`
	NextReviewDate string    `json:"nextReviewDate,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type nascReferralRecord struct {
	ID            string
	PatientID     string
	PatientNHI    string
	ReferrerHPI   string
	TenantID      string
	Status        NASCReferralStatus
	Reason        string
	UrgencyFlag   bool
	NASCOrgCode   string
	InterRAIRefID string
	CompletedAt   *time.Time
	DeclineReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type nascServicePlanRecord struct {
	ID            string
	PatientID     string
	PatientNHI    string
	TenantID      string
	ReferralID    string
	Status        ServicePlanStatus
	NeedsLevel    SupportNeedsLevel
	Services      []FundedService
	GoalsEnc      []byte
	PlanStartDate string
	PlanEndDate   string
	NextReview    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
