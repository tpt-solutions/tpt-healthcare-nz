package api

import (
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// Bedside Medication Verification (Five Rights) + S8 Controlled Drug Register
// ---------------------------------------------------------------------------

// VerificationStatus tracks the outcome of bedside verification.
type VerificationStatus string

const (
	VerificationPending    VerificationStatus = "pending"
	VerificationMatched    VerificationStatus = "matched"
	VerificationMismatch   VerificationStatus = "mismatch"
	VerificationOverride   VerificationStatus = "override"
	VerificationRefused    VerificationStatus = "refused"
)

// BedsideVerification records the five-rights check at point of administration.
type BedsideVerification struct {
	ID                string             `json:"id"`
	AdmissionID       string             `json:"admissionId"`
	MedicationID      string             `json:"medicationId"`
	PatientNHI        string             `json:"patientNhi"`
	AdministeredBy    string             `json:"administeredBy"`
	VerifiedAt        time.Time          `json:"verifiedAt"`
	// Five Rights
	RightPatient      VerificationStatus `json:"rightPatient"`
	RightDrug         VerificationStatus `json:"rightDrug"`
	RightDose         VerificationStatus `json:"rightDose"`
	RightRoute        VerificationStatus `json:"rightRoute"`
	RightTime         VerificationStatus `json:"rightTime"`
	// Barcode scan data
	PatientBarcode    string             `json:"patientBarcode"`
	MedicationBarcode string             `json:"medicationBarcode"`
	OverrideReason    string             `json:"overrideReason,omitempty"`
	WitnessedBy       string             `json:"witnessedBy,omitempty"`
	Status            string             `json:"status"` // completed, overridden, refused
}

// IsFiveRightsOK returns true if all five rights matched.
func (v *BedsideVerification) IsFiveRightsOK() bool {
	return v.RightPatient == VerificationMatched &&
		v.RightDrug == VerificationMatched &&
		v.RightDose == VerificationMatched &&
		v.RightRoute == VerificationMatched &&
		v.RightTime == VerificationMatched
}

// S8DrugCategory classifies controlled substances (NZ Misuse of Drugs Act 1975).
type S8DrugCategory string

const (
	S8ClassA S8DrugCategory = "class_a" // e.g. heroin, morphine, methadone
	S8ClassB S8DrugCategory = "class_b" // e.g. codeine, buprenorphine
	S8ClassC S8DrugCategory = "class_c" // e.g. benzodiazepines
)

// S8RegisterEntry records each administration of a Schedule 8 controlled drug.
type S8RegisterEntry struct {
	ID                string           `json:"id"`
	AdmissionID       string           `json:"admissionId"`
	PatientNHI        string           `json:"patientNhi"`
	DrugName          string           `json:"drugName"`
	DrugCategory      S8DrugCategory   `json:"drugCategory"`
	Formulation       string           `json:"formulation"`    // liquid, tablet, injection
	DoseMg            float64          `json:"doseMg"`
	Strength          string           `json:"strength"`       // e.g. "10mg/mL"
	QuantityUsed      float64          `json:"quantityUsed"`
	QuantityWastage   float64          `json:"quantityWastage"`
	WitnessedBy       string           `json:"witnessedBy"`    // second nurse witness HPI
	AdministeredBy    string           `json:"administeredBy"`
	AdministeredAt    time.Time        `json:"administeredAt"`
	RunningBalance    float64          `json:"runningBalance"` // stock count
	Notes             string           `json:"notes,omitempty"`
	CreatedAt         time.Time        `json:"createdAt"`
}

// S8StockCount records periodic controlled drug stock reconciliation.
type S8StockCount struct {
	ID              string           `json:"id"`
	DrugName        string           `json:"drugName"`
	DrugCategory    S8DrugCategory   `json:"drugCategory"`
	CountDate       time.Time        `json:"countDate"`
	CountedBy       string           `json:"countedBy"`
	ExpectedBalance float64          `json:"expectedBalance"`
	ActualBalance   float64          `json:"actualBalance"`
	Discrepancy     float64          `json:"discrepancy"`
	DiscrepancyReason string         `json:"discrepancyReason,omitempty"`
	WitnessedBy     string           `json:"witnessedBy"`
	CreatedAt       time.Time        `json:"createdAt"`
}

// NewBedsideVerification creates a new verification record.
func NewBedsideVerification(admissionID, medicationID, patientNHI, administeredBy string) *BedsideVerification {
	return &BedsideVerification{
		AdmissionID:    admissionID,
		MedicationID:   medicationID,
		PatientNHI:     patientNHI,
		AdministeredBy: administeredBy,
		VerifiedAt:     time.Now(),
		RightPatient:   VerificationPending,
		RightDrug:      VerificationPending,
		RightDose:      VerificationPending,
		RightRoute:     VerificationPending,
		RightTime:      VerificationPending,
		Status:         "pending",
	}
}

// Validate checks required fields for bedside verification.
func (v *BedsideVerification) Validate() error {
	if v.AdmissionID == "" {
		return fmt.Errorf("verification: admission ID is required")
	}
	if v.MedicationID == "" {
		return fmt.Errorf("verification: medication ID is required")
	}
	if v.PatientNHI == "" {
		return fmt.Errorf("verification: patient NHI is required")
	}
	return nil
}

// NewS8RegisterEntry creates a new S8 controlled drug record.
func NewS8RegisterEntry(admissionID, patientNHI, drugName string, category S8DrugCategory, doseMg float64, administeredBy, witnessedBy string) *S8RegisterEntry {
	return &S8RegisterEntry{
		AdmissionID:    admissionID,
		PatientNHI:     patientNHI,
		DrugName:       drugName,
		DrugCategory:   category,
		DoseMg:         doseMg,
		AdministeredBy: administeredBy,
		WitnessedBy:    witnessedBy,
		AdministeredAt: time.Now(),
		CreatedAt:      time.Now(),
	}
}

// Validate checks required fields for an S8 register entry.
func (e *S8RegisterEntry) Validate() error {
	if e.AdmissionID == "" {
		return fmt.Errorf("s8_register: admission ID is required")
	}
	if e.PatientNHI == "" {
		return fmt.Errorf("s8_register: patient NHI is required")
	}
	if e.DrugName == "" {
		return fmt.Errorf("s8_register: drug name is required")
	}
	if e.DoseMg <= 0 {
		return fmt.Errorf("s8_register: dose must be positive")
	}
	if e.WitnessedBy == "" {
		return fmt.Errorf("s8_register: witness is required for Schedule 8 drugs")
	}
	return nil
}
