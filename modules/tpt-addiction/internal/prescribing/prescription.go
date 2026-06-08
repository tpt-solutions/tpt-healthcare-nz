// Package prescribing provides OST (Opioid Substitution Therapy) prescription models.
package prescribing

import "time"

// OSTDrug represents the prescribed medication.
type OSTDrug string

const (
	DrugMethadone       OSTDrug = "methadone"
	DrugBuprenorphine   OSTDrug = "buprenorphine"
	DrugSuboxone        OSTDrug = "buprenorphine_naloxone" // Suboxone / Bunavail
)

// OSTPrescription records an OST medication prescription with dose and supervision rules.
type OSTPrescription struct {
	ID              string    `json:"id"`
	PatientNHI      string    `json:"patientNhi"`
	ProgrammeID     string    `json:"programmeId"`
	PrescriberID    string    `json:"prescriberId"`     // HPI number
	PracticeID      string    `json:"practiceId"`
	Drug            OSTDrug   `json:"drug"`
	DoseMg          float64   `json:"doseMg"`           // current prescribed dose
	Formulation     string    `json:"formulation"`      // liquid, tablet, sublingual
	Frequency       string    `json:"frequency"`        // daily, alternate_day, three_times_weekly
	Supervised      bool      `json:"supervised"`       // true if pharmacy-supervised
	TakeHomeDays    int       `json:"takeHomeDays"`     // 0-7
	StartDate       time.Time `json:"startDate"`
	EndDate         *time.Time `json:"endDate,omitempty"`
	Status          string    `json:"status"`          // active, paused, discontinued, completed
	Indication      string    `json:"indication"`      // opioid_dependence, pain, palliative
	ClinicalNotes   string    `json:"clinicalNotes,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// DoseAdjustment records an authorised change in prescribed dose.
type DoseAdjustment struct {
	ID              string    `json:"id"`
	PrescriptionID  string    `json:"prescriptionId"`
	AdjustedBy      string    `json:"adjustedBy"`      // prescriber HPI
	AdjustedAt      time.Time `json:"adjustedAt"`
	PreviousDoseMg  float64   `json:"previousDoseMg"`
	NewDoseMg       float64   `json:"newDoseMg"`
	Reason          string    `json:"reason"`          // induction, reduction, clinical_response, adverse_event
	ClinicalNotes   string    `json:"clinicalNotes,omitempty"`
	WitnessedBy     string    `json:"witnessedBy,omitempty"` // pharmacist confirmation for controlled drug
	CreatedAt       time.Time `json:"createdAt"`
}
