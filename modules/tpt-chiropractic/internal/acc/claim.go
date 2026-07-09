// Package acc provides ACC chiropractic claim types.
package acc

import "time"

type ClaimStatus string

const (
	StatusDraft     ClaimStatus = "draft"
	StatusSubmitted ClaimStatus = "submitted"
	StatusAccepted  ClaimStatus = "accepted"
	StatusDeclined  ClaimStatus = "declined"
)

type Claim struct {
	ID             string      `json:"id"`
	PatientNHI     string      `json:"patientNhi"`
	ProviderHPI    string      `json:"providerHpi"`
	PracticeID     string      `json:"practiceId"`
	AccidentDate   time.Time   `json:"accidentDate"`
	AccidentDesc   string      `json:"accidentDesc"`
	Diagnosis      string      `json:"diagnosis"`
	Region         string      `json:"region"`
	VisitCount     int         `json:"visitCount"`
	TotalFee       int         `json:"totalFee"`
	Status         ClaimStatus `json:"status"`
	ACCClaimNumber string      `json:"accClaimNumber,omitempty"`
	Notes          string      `json:"notes,omitempty"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}
