// Package methadone provides OST programme domain models and dose tracking.
package methadone

import "time"

// ProgrammePhase represents the patient's current treatment phase.
type ProgrammePhase string

const (
	PhaseInduction     ProgrammePhase = "induction"
	PhaseStabilisation ProgrammePhase = "stabilisation"
	PhaseMaintenance   ProgrammePhase = "maintenance"
	PhaseTapering      ProgrammePhase = "tapering"
	PhaseDischarged    ProgrammePhase = "discharged"
)

// Programme represents a patient's enrolment in an Opioid Substitution Therapy programme.
type Programme struct {
	ID               string         `json:"id"`
	PatientNHI       string         `json:"patientNhi"`
	ClinicianID      string         `json:"clinicianId"`
	PracticeID       string         `json:"practiceId"`
	StartDate        time.Time      `json:"startDate"`
	EndDate          *time.Time     `json:"endDate,omitempty"`
	Phase            ProgrammePhase `json:"phase"`
	SubstancePrimary string         `json:"substancePrimary"` // e.g. heroin, morphine, oxycodone
	SubstanceOther   string         `json:"substanceOther,omitempty"`
	InitialDoseMg    float64        `json:"initialDoseMg"` // mg, e.g. 30.0
	CurrentDoseMg    float64        `json:"currentDoseMg"` // mg at last known prescription
	TargetDoseMg     *float64       `json:"targetDoseMg,omitempty"`
	TakeHomeLevel    int            `json:"takeHomeLevel"`   // 1-5 (NZ MSSA levels)
	TakeHomeMaxDays  int            `json:"takeHomeMaxDays"` // derived from level
	Pregnancy        bool           `json:"pregnancy"`       // triggers daily dosing rule
	Comorbidities    []string       `json:"comorbidities,omitempty"`
	LastUrineDate    *time.Time     `json:"lastUrineDate,omitempty"`
	NextReviewDate   time.Time      `json:"nextReviewDate"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

// DoseRecord records a single administered (or missed/supervised) dose.
type DoseRecord struct {
	ID              string    `json:"id"`
	ProgrammeID     string    `json:"programmeId"`
	AdministeredAt  time.Time `json:"administeredAt"`
	DoseMg          float64   `json:"doseMg"`
	Formulation     string    `json:"formulation"`     // liquid, tablet, sublingual
	WitnessedBy     string    `json:"witnessedBy"`     // staff member HPI or name
	DispensedBy     string    `json:"dispensedBy"`     // dispenser HPI or name
	PharmacistCheck bool      `json:"pharmacistCheck"` // second pharmacist check for controlled drug
	Status          string    `json:"status"`          // administered, refused, missed, vomited
	Notes           string    `json:"notes,omitempty"`
	TakeHome        bool      `json:"takeHome"`
	CreatedAt       time.Time `json:"createdAt"`
}

// TakeHomeApproval is a grant or revocation of take-home doses.
type TakeHomeApproval struct {
	ID            string     `json:"id"`
	ProgrammeID   string     `json:"programmeId"`
	ApprovedBy    string     `json:"approvedBy"` // clinician HPI
	ApprovedAt    time.Time  `json:"approvedAt"`
	Level         int        `json:"level"`   // 1-5
	MaxDays       int        `json:"maxDays"` // e.g. 1, 2, 3, 5, 7
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	RevokedBy     string     `json:"revokedBy,omitempty"`
	RevokedReason string     `json:"revokedReason,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

// UrineScreen records a drug urinalysis result.
type UrineScreen struct {
	ID            string       `json:"id"`
	ProgrammeID   string       `json:"programmeId"`
	CollectedAt   time.Time    `json:"collectedAt"`
	CollectedBy   string       `json:"collectedBy"`
	LabName       string       `json:"labName,omitempty"`
	LabReference  string       `json:"labReference,omitempty"`
	Results       []DrugResult `json:"results"`
	MSSAResult    string       `json:"mssaResult"` // conforming, non_conforming, borderline
	ClinicalNotes string       `json:"clinicalNotes,omitempty"`
	CreatedAt     time.Time    `json:"createdAt"`
}

// DrugResult for a single analyte.
type DrugResult struct {
	DrugName string  `json:"drugName"` // cannabis, methadone, buprenorphine, amphetamines, benzodiazepines, opioids
	Detected bool    `json:"detected"`
	Level    *string `json:"level,omitempty"` // positive, high, trace, etc.
	Expected bool    `json:"expected"`        // true if the prescribed drug
}

// TakeHomeDays returns the max take-home days for a given NZ MSSA level.
func TakeHomeDays(level int) int {
	switch level {
	case 1:
		return 0
	case 2:
		return 1
	case 3:
		return 3
	case 4:
		return 5
	case 5:
		return 7
	default:
		return 0
	}
}
