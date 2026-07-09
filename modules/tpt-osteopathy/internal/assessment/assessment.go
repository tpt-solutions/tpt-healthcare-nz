// Package assessment provides osteopathic assessment types.
package assessment

// Assessment represents an osteopathic clinical assessment.
type Assessment struct {
	ID                      string             `json:"id"`
	PatientNHI              string             `json:"patientNhi"`
	ClinicianID             string             `json:"clinicianId"`
	PracticeID              string             `json:"practiceId"`
	VisitID                 string             `json:"visitId,omitempty"`
	AssessmentDate          int64              `json:"assessmentDate"`
	ChiefComplaint          string             `json:"chiefComplaint"`
	HistoryOfPresentIllness string             `json:"historyOfPresentIllness"`
	PastMedicalHistory      string             `json:"pastMedicalHistory,omitempty"`
	Medications             []string           `json:"medications,omitempty"`
	PosturalAnalysis        PosturalFindings   `json:"posturalAnalysis"`
	GaitAnalysis            string             `json:"gaitAnalysis,omitempty"`
	RangeOfMotion           []MotionFinding    `json:"rangeOfMotion,omitempty"`
	PalpationFindings       []PalpationFinding `json:"palpationFindings,omitempty"`
	NeurologicalTests       string             `json:"neurologicalTests,omitempty"`
	OrthopedicTests         string             `json:"orthopedicTests,omitempty"`
	Diagnosis               string             `json:"diagnosis"`
	TreatmentPlan           string             `json:"treatmentPlan"`
	Prognosis               string             `json:"prognosis,omitempty"`
	Notes                   string             `json:"notes,omitempty"`
	CreatedAt               int64              `json:"createdAt"`
	UpdatedAt               int64              `json:"updatedAt"`
}

// PosturalFindings documents postural deviations observed during assessment.
type PosturalFindings struct {
	AnteriorView         string `json:"anteriorView,omitempty"`
	PosteriorView        string `json:"posteriorView,omitempty"`
	LateralView          string `json:"lateralView,omitempty"`
	Scoliosis            bool   `json:"scoliosis"`
	LegLengthDiscrepancy bool   `json:"legLengthDiscrepancy"`
	Notes                string `json:"notes,omitempty"`
}

// MotionFinding documents range of motion for a specific region.
type MotionFinding struct {
	Region     string `json:"region"`   // cervical, thoracic, lumbar, hip, knee, etc.
	Movement   string `json:"movement"` // flexion, extension, rotation, lateral_flexion
	Active     string `json:"active"`   // degrees or qualitative description
	Passive    string `json:"passive"`
	EndFeel    string `json:"endFeel"` // soft, firm, hard, empty
	Restricted bool   `json:"restricted"`
}

// PalpationFinding documents tissue quality found during palpatory examination.
type PalpationFinding struct {
	Location      string `json:"location"`
	Tenderness    string `json:"tenderness"`    // none, mild, moderate, severe
	MuscleTone    string `json:"muscleTone"`    // normal, hypertonic, hypotonic
	TissueQuality string `json:"tissueQuality"` // normal, oedematous, fibrotic
	Temperature   string `json:"temperature"`   // normal, warm, cool
	Restriction   bool   `json:"restriction"`
}
