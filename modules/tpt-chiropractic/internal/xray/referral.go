// Package xray provides X-ray referral management for chiropractic.
package xray

type Referral struct {
	ID          string `json:"id"`
	PatientNHI  string `json:"patientNhi"`
	ClinicianID string `json:"clinicianId"`
	PracticeID  string `json:"practiceId"`
	Region      string `json:"region"`  // cervical, thoracic, lumbar, full_spine
	Views       string `json:"views"`   // AP, lateral, obliques, flexion_extension
	Urgency     string `json:"urgency"` // routine, urgent, emergency
	Indication  string `json:"indication"`
	Findings    string `json:"findings,omitempty"`
	Radiologist string `json:"radiologist,omitempty"`
	ReportURL   string `json:"reportUrl,omitempty"`
	Status      string `json:"status"` // ordered, completed, reported
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}
