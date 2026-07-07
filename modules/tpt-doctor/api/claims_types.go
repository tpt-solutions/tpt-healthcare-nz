package api

import "time"

// ClaimDestination selects which regulatory scheme receives the submitted claim.
type ClaimDestination string

const (
	DestinationACC      ClaimDestination = "acc"
	DestinationWorkSafe ClaimDestination = "worksafe"
)

// ClaimStatus mirrors the ACC/WorkSafe claim lifecycle.
type ClaimStatus string

const (
	ClaimStatusDraft     ClaimStatus = "draft"
	ClaimStatusSubmitted ClaimStatus = "submitted"
	ClaimStatusAccepted  ClaimStatus = "accepted"
	ClaimStatusRejected  ClaimStatus = "rejected"
	ClaimStatusPaid      ClaimStatus = "paid"
	ClaimStatusCancelled ClaimStatus = "cancelled"
	ClaimStatusPending   ClaimStatus = "pending"
)

// ACCFormType enumerates the supported ACC claim form types.
type ACCFormType string

const (
	ACCFormACC45 ACCFormType = "ACC45" // Injury claims
	ACCFormACC6  ACCFormType = "ACC6"  // Treatment injury
)

// Claim is the domain model for a claim generated from a clinical encounter.
// Destination controls whether the claim routes to ACC or WorkSafe NZ.
type Claim struct {
	ID                string           `json:"id"`
	EncounterID       string           `json:"encounterId"`
	PatientID         string           `json:"patientId"`
	PatientNHI        string           `json:"patientNhi"`
	PractitionerHPI   string           `json:"practitionerHpi"`
	FormType          ACCFormType      `json:"formType"`
	FormNumber        string           `json:"formNumber,omitempty"`
	DiagnosisCodes    []string         `json:"diagnosisCodes"`
	InjuryDate        time.Time        `json:"injuryDate"`
	InjuryDescription string           `json:"injuryDescription"`
	Status            ClaimStatus      `json:"status"`
	Destination       ClaimDestination `json:"destination"`
	ACCClaimNumber    string           `json:"accClaimNumber,omitempty"`
	WorkSafeRefNumber string           `json:"workSafeRefNumber,omitempty"`
	EmployerNZBN      string           `json:"employerNzbn,omitempty"`
	InjuryMechanism   string           `json:"injuryMechanism,omitempty"`
	RejectionReason   string           `json:"rejectionReason,omitempty"`
	PaidAmount        *float64         `json:"paidAmount,omitempty"`
	TenantID          string           `json:"tenantId"`
	CreatedAt         time.Time        `json:"createdAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`
	SubmittedAt       *time.Time       `json:"submittedAt,omitempty"`
}

// claimCreateRequest is the body for POST /api/v1/claims.
type claimCreateRequest struct {
	EncounterID       string      `json:"encounterId"`
	PatientID         string      `json:"patientId"`
	PatientNHI        string      `json:"patientNhi"`
	PractitionerHPI   string      `json:"practitionerHpi"`
	FormType          ACCFormType `json:"formType"`
	DiagnosisCodes    []string    `json:"diagnosisCodes"`
	InjuryDate        time.Time   `json:"injuryDate"`
	InjuryDescription string      `json:"injuryDescription"`
	// Destination selects ACC or WorkSafe NZ. Defaults to "acc" when omitted.
	Destination ClaimDestination `json:"destination"`
	// EmployerNZBN is the NZBN of the employing organisation; only relevant for WorkSafe claims.
	EmployerNZBN string `json:"employerNzbn,omitempty"`
	// InjuryMechanism classifies the mechanism of workplace injury; only relevant for WorkSafe claims.
	InjuryMechanism string `json:"injuryMechanism,omitempty"`
}

// claimStatusResponse is the response for GET /api/v1/claims/{id}/status.
type claimStatusResponse struct {
	ClaimID           string      `json:"claimId"`
	Status            ClaimStatus `json:"status"`
	ACCClaimNumber    string      `json:"accClaimNumber,omitempty"`
	WorkSafeRefNumber string      `json:"workSafeRefNumber,omitempty"`
	RejectionReason   string      `json:"rejectionReason,omitempty"`
	PaidAmount        *float64    `json:"paidAmount,omitempty"`
	LastCheckedAt     time.Time   `json:"lastCheckedAt"`
}
