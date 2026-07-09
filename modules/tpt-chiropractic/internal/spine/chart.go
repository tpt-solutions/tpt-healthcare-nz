// Package spine provides spinal segment charting for chiropractic assessments.
package spine

// SpinalChart represents the full spinal assessment chart for a patient.
type SpinalChart struct {
	PatientNHI  string          `json:"patientNhi"`
	Entries     []VertebraEntry `json:"entries"`
	ChartDate   int64           `json:"chartDate"`
	ClinicianID string          `json:"clinicianId"`
	PracticeID  string          `json:"practiceId"`
	VisitID     string          `json:"visitId,omitempty"`
	CreatedAt   int64           `json:"createdAt"`
	UpdatedAt   int64           `json:"updatedAt"`
}

// VertebraEntry documents the assessment of a single spinal segment.
type VertebraEntry struct {
	Segment      string `json:"segment"`      // e.g. "C1", "C2", "T4", "L3", "S1"
	Region       string `json:"region"`       // cervical, thoracic, lumbar, sacral
	Fixation     bool   `json:"fixation"`     // segmental fixation present
	Subluxation  bool   `json:"subluxation"`  // vertebral subluxation complex
	Misalignment string `json:"misalignment"` // direction of misalignment
	Mobility     string `json:"mobility"`     // hypomobile, hypermobile, normal
	Tenderness   string `json:"tenderness"`   // none, mild, moderate, severe
	MuscleTone   string `json:"muscleTone"`   // hypertonic, hypotonic, normal
	XRayFindings string `json:"xRayFindings,omitempty"`
	Adjustment   string `json:"adjustment"` // adjustment technique used
	Note         string `json:"note,omitempty"`
	UpdatedAt    int64  `json:"updatedAt"`
}
