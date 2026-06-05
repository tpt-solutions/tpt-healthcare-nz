// Package acc provides ACC acupuncture claim types for injury-related acupuncture
// treatment in New Zealand. ACC covers acupuncture for specified injury conditions
// under the ACC treatment provider schedule.
package acc

import (
	"time"
)

// ClaimStatus tracks the lifecycle of an ACC acupuncture claim.
type ClaimStatus string

const (
	ClaimDraft     ClaimStatus = "draft"
	ClaimSubmitted ClaimStatus = "submitted"
	ClaimAccepted  ClaimStatus = "accepted"
	ClaimDeclined  ClaimStatus = "declined"
)

// Claim represents an ACC acupuncture claim.
type Claim struct {
	ID              string       `json:"id"`
	PatientNHI      string       `json:"patientNhi"`
	ProviderHPI     string       `json:"providerHpi"`
	PracticeID      string       `json:"practiceId"`
	AccidentDate    time.Time    `json:"accidentDate"`
	AccidentDesc    string       `json:"accidentDesc"`
	InjuryType      string       `json:"injuryType"`      // ACC injury classification code
	Diagnosis       string       `json:"diagnosis"`
	BodyRegion      string       `json:"bodyRegion"`      // affected body region for acupuncture
	SessionCount    int          `json:"sessionCount"`     // number of acupuncture sessions
	TotalFee        int          `json:"totalFee"`         // NZ cents
	Status          ClaimStatus  `json:"status"`
	ACCClaimNumber  string       `json:"accClaimNumber,omitempty"`
	Notes           string       `json:"notes,omitempty"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

// ValidationError holds per-field validation failures.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationResult is the result of validating a claim.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Validate performs pre-submission validation of the ACC claim.
func (c *Claim) Validate() *ValidationResult {
	result := &ValidationResult{Valid: true}
	if c.PatientNHI == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "patientNhi", Message: "Patient NHI is required", Code: "MISSING_NHI"})
	}
	if c.ProviderHPI == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "providerHpi", Message: "Provider HPI is required", Code: "MISSING_HPI"})
	}
	if c.AccidentDate.IsZero() {
		result.Errors = append(result.Errors, ValidationError{Field: "accidentDate", Message: "Accident date is required", Code: "MISSING_ACCIDENT_DATE"})
	}
	if len(result.Errors) > 0 {
		result.Valid = false
	}
	return result
}