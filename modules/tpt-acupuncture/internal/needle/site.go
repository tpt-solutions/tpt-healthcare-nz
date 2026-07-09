// Package needle provides acupuncture needle site documentation and treatment record types.
package needle

// NeedleSession represents a single acupuncture treatment session with needle site documentation.
type NeedleSession struct {
	ID            string        `json:"id"`
	PatientNHI    string        `json:"patientNhi"`
	VisitID       string        `json:"visitId"`
	ClinicianID   string        `json:"clinicianId"`
	PracticeID    string        `json:"practiceId"`
	Points        []NeedlePoint `json:"points"`
	DeQiSensation bool          `json:"deQiSensation"` // presence of De Qi sensation
	ElectroStim   bool          `json:"electroStim"`   // electro-acupuncture used
	RetentionMin  int           `json:"retentionMin"`  // needle retention time in minutes
	Assessment    string        `json:"assessment"`
	Notes         string        `json:"notes,omitempty"`
	CreatedAt     int64         `json:"createdAt"`
	UpdatedAt     int64         `json:"updatedAt"`
}

// NeedlePoint documents a single acupuncture point used during a session.
type NeedlePoint struct {
	PointCode   string `json:"pointCode"`   // e.g. "LI4", "ST36" (meridian standard nomenclature)
	Meridian    string `json:"meridian"`    // e.g. "large_intestine", "stomach"
	Side        string `json:"side"`        // left, right, bilateral
	Depth       string `json:"depth"`       // depth of insertion
	Method      string `json:"method"`      // manual, electrical
	Stimulation string `json:"stimulation"` // reducing, tonifying, neutral
	Reaction    string `json:"reaction"`    // patient reaction / sensation
}

// TreatmentRecord represents an acupuncture treatment record.
type TreatmentRecord struct {
	ID           string `json:"id"`
	PatientNHI   string `json:"patientNhi"`
	VisitID      string `json:"visitId"`
	ClinicianID  string `json:"clinicianId"`
	PracticeID   string `json:"practiceId"`
	Diagnosis    string `json:"diagnosis"`
	TCMDiagnosis string `json:"tcmDiagnosis"`       // TCM pattern differentiation
	Principle    string `json:"principle"`          // treatment principle
	PointsUsed   int    `json:"pointsUsed"`         // number of points used
	NeedleCount  int    `json:"needleCount"`        // total needles used
	DurationMin  int    `json:"durationMin"`        // session duration
	HerbalRx     string `json:"herbalRx,omitempty"` // concurrent herbal prescription
	MoxaUsed     bool   `json:"moxaUsed"`           // moxibustion used
	CupsUsed     bool   `json:"cupsUsed"`           // cupping used
	Outcome      string `json:"outcome"`            // treatment outcome
	Notes        string `json:"notes,omitempty"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}
