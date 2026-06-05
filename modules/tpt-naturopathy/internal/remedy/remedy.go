// Package remedy provides natural remedy tracking.
package remedy

type Remedy struct {
	ID          string `json:"id"`
	PatientNHI  string `json:"patientNhi"`
	Name        string `json:"name"`
	Type        string `json:"type"`        // herbal, homeopathic, flower_essence, tissue_salt, other
	Preparation string `json:"preparation"` // tincture, infusion, decoction, powder, tablet
	Dosage      string `json:"dosage"`
	Frequency   string `json:"frequency"`
	Duration    string `json:"duration"`
	Indication  string `json:"indication"`
	ClinicianID string `json:"clinicianId"`
	PracticeID  string `json:"practiceId"`
	Active      bool   `json:"active"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}