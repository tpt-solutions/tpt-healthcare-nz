package api

import (
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/medsafe"
)

// PrescriptionStatus mirrors the FHIR MedicationRequest status value set.
type PrescriptionStatus string

const (
	PrescriptionStatusActive    PrescriptionStatus = "active"
	PrescriptionStatusOnHold    PrescriptionStatus = "on-hold"
	PrescriptionStatusCancelled PrescriptionStatus = "cancelled"
	PrescriptionStatusCompleted PrescriptionStatus = "completed"
	PrescriptionStatusDraft     PrescriptionStatus = "draft"
	PrescriptionStatusStopped   PrescriptionStatus = "stopped"
)

// Dosage represents a medication dosage instruction.
type Dosage struct {
	Text             string  `json:"text"`
	Route            string  `json:"route,omitempty"`        // e.g. "oral", "topical"
	DoseValue        float64 `json:"doseValue"`              // numeric dose
	DoseUnit         string  `json:"doseUnit"`               // e.g. "mg", "mL"
	Frequency        string  `json:"frequency"`              // e.g. "twice daily"
	DurationDays     int     `json:"durationDays,omitempty"` // 0 = ongoing
	MaxDailyDose     float64 `json:"maxDailyDose,omitempty"` // safety cap
	MaxDailyDoseUnit string  `json:"maxDailyDoseUnit,omitempty"`
}

// Prescription is the domain model for an e-prescription (FHIR MedicationRequest).
type Prescription struct {
	ID                  string             `json:"id"`
	PatientID           string             `json:"patientId"`
	PatientNHI          string             `json:"patientNhi"`
	PractitionerHPI     string             `json:"practitionerHpi"`
	EncounterID         string             `json:"encounterId,omitempty"`
	NZULMCode           string             `json:"nzulmCode"` // NZMT product code
	MedicationName      string             `json:"medicationName"`
	Status              PrescriptionStatus `json:"status"`
	Dosage              Dosage             `json:"dosage"`
	PHARMACSubsidised   bool               `json:"pharmácSubsidised"`
	SubsidyCode         string             `json:"subsidyCode,omitempty"`
	InteractionWarnings []string           `json:"interactionWarnings,omitempty"`
	// InteractionCheckSkipped is true when the drug interaction check could not
	// be performed (e.g. active-medication lookup failed). The prescriber must
	// manually verify interactions before dispensing.
	InteractionCheckSkipped bool       `json:"interactionCheckSkipped,omitempty"`
	SubsidyCheckSkipped     bool       `json:"subsidyCheckSkipped,omitempty"`
	Repeats                 int        `json:"repeats"`
	RepeatsRemaining        int        `json:"repeatsRemaining"`
	TenantID                string     `json:"tenantId"`
	IssuedAt                time.Time  `json:"issuedAt"`
	ExpiresAt               *time.Time `json:"expiresAt,omitempty"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

// prescriptionCreateRequest is the body for POST /api/v1/prescriptions.
type prescriptionCreateRequest struct {
	PatientID       string `json:"patientId"`
	PatientNHI      string `json:"patientNhi"`
	PractitionerHPI string `json:"practitionerHpi"`
	EncounterID     string `json:"encounterId,omitempty"`
	NZULMCode       string `json:"nzulmCode"`
	Dosage          Dosage `json:"dosage"`
	Repeats         int    `json:"repeats"`
}

// prescriptionUpdateRequest is the body for PUT /api/v1/prescriptions/{id}.
type prescriptionUpdateRequest struct {
	Status  *PrescriptionStatus `json:"status,omitempty"`
	Dosage  *Dosage             `json:"dosage,omitempty"`
	Repeats *int                `json:"repeats,omitempty"`
}

// printablePrescription is the response for POST /api/v1/prescriptions/{id}/print.
type printablePrescription struct {
	PrescriptionID   string    `json:"prescriptionId"`
	PatientName      string    `json:"patientName"`
	PatientNHI       string    `json:"patientNhi"`
	PatientDOB       string    `json:"patientDob"`
	PractitionerName string    `json:"practitionerName"`
	PractitionerHPI  string    `json:"practitionerHpi"`
	MedicationName   string    `json:"medicationName"`
	NZULMCode        string    `json:"nzulmCode"`
	DosageText       string    `json:"dosageText"`
	Repeats          int       `json:"repeats"`
	SubsidyCode      string    `json:"subsidyCode,omitempty"`
	IssuedAt         time.Time `json:"issuedAt"`
	PrintedAt        time.Time `json:"printedAt"`
}

// prescriptionADERequest is the body for POST /api/v1/prescriptions/{id}/ade.
// The prescriber fills this in when they suspect the prescribed medicine caused
// an adverse event in the patient. Medicines Act 1981 s45 obliges reporters
// to notify Medsafe via the CARM system.
type prescriptionADERequest struct {
	EventDate        time.Time           `json:"eventDate"`
	EventDescription string              `json:"eventDescription"`
	Seriousness      medsafe.Seriousness `json:"seriousness"`
	Outcome          string              `json:"outcome,omitempty"`
	PatientAge       int                 `json:"patientAge,omitempty"`
	PatientSex       string              `json:"patientSex,omitempty"`
	RelevantHistory  string              `json:"relevantHistory,omitempty"`
}

// prescriptionDispatchRequest is the body for POST /api/v1/prescriptions/{id}/dispatch.
type prescriptionDispatchRequest struct {
	PharmacyHPI string `json:"pharmacyHpi"`
	Quantity    int    `json:"quantity,omitempty"` // defaults to 1 when zero or absent
	IsUrgent    bool   `json:"isUrgent,omitempty"`
}
