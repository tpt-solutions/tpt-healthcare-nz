package api

import (
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
)

// patientRecord is the internal representation stored in the database.
// PHI fields (nhiEncrypted, name, dob) are AES-256-GCM encrypted at rest.
type patientRecord struct {
	ID           string    `json:"id"`
	NHIEncrypted []byte    `json:"-"`
	NHI          string    `json:"nhi,omitempty"` // plaintext, only populated after decryption
	TenantID     string    `json:"tenantId"`
	FHIRResource []byte    `json:"-"` // encrypted FHIR Patient JSON
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// patientResponse is the API response for a patient resource.
type patientResponse struct {
	ID        string      `json:"id"`
	NHI       string      `json:"nhi"`
	TenantID  string      `json:"tenantId"`
	Patient   *r5.Patient `json:"patient"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// patientCreateRequest is the request body for POST /api/v1/patients.
type patientCreateRequest struct {
	NHI     string      `json:"nhi"`
	Patient *r5.Patient `json:"patient"`
}

// patientUpdateRequest is the request body for PUT /api/v1/patients/{id}.
type patientUpdateRequest struct {
	Patient *r5.Patient `json:"patient"`
}

// enrolmentRequest is the request body for POST and PUT /api/v1/patients/{id}/enrolment.
type enrolmentRequest struct {
	// PractitionerHPI is the individual practitioner's HPI Common Person Number (CPN).
	PractitionerHPI string `json:"practitionerHpi"`
	// PracticeHPI is the HPI facility OrgID of the enrolling practice.
	// Required for UpdateEnrolment; ignored by CreateEnrolment which derives the
	// practice from the authenticated tenant.
	PracticeHPI string `json:"practiceHpi"`
	FundingCode string `json:"fundingCode"`
	StartDate   string `json:"startDate"` // YYYY-MM-DD
}

// transferRequest is the body for POST /api/v1/patients/{id}/enrolment/transfer.
type transferRequest struct {
	ToPractitionerHPI string `json:"toPractitionerHpi"`
	FundingCode       string `json:"fundingCode,omitempty"`
	TransferDate      string `json:"transferDate"` // YYYY-MM-DD
	Reason            string `json:"reason,omitempty"`
}
