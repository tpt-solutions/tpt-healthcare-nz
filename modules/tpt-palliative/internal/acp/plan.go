// Package acp provides domain models for Advance Care Planning.
package acp

import "time"

// PlanStatus represents where the ACP is in its lifecycle.
type PlanStatus string

const (
	StatusDraft       PlanStatus = "draft"
	StatusProposed    PlanStatus = "proposed"
	StatusActive      PlanStatus = "active"
	StatusSuspended   PlanStatus = "suspended"
	StatusCompleted   PlanStatus = "completed"
	StatusWithdrawn   PlanStatus = "withdrawn"
)

// TreatmentIntentLevel for escalation-of-care decisions.
type TreatmentIntentLevel string

const (
	IntentCurative       TreatmentIntentLevel = "curative"
	IntentPalliative     TreatmentIntentLevel = "palliative"
	IntentSymptomControl TreatmentIntentLevel = "symptom_control"
	IntentComfortOnly    TreatmentIntentLevel = "comfort_only"
)

// Plan is the top-level Advance Care Plan document.
type Plan struct {
	ID                  string               `json:"id"`
	PatientNHI          string               `json:"patientNhi"`
	Status              PlanStatus           `json:"status"`
	TreatmentIntent     TreatmentIntentLevel `json:"treatmentIntent"`
	DNACPR              bool                 `json:"dnacpr"`
	DNACPRDocumentedAt  *time.Time           `json:"dnacprDocumentedAt,omitempty"`
	DNACPRSignedBy      string               `json:"dnacprSignedBy,omitempty"`
	ResuscitationDiscussedWithPatient bool   `json:"resuscitationDiscussedWithPatient"`
	ResuscitationDiscussedWithFamily   bool   `json:"resuscitationDiscussedWithFamily"`
	PreferredPlaceOfCare   string           `json:"preferredPlaceOfCare,omitempty"`
	PreferredPlaceOfDeath  string          `json:"preferredPlaceOfDeath,omitempty"`
	OrganDonationWishes    *bool            `json:"organDonationWishes,omitempty"`
	SpiritualWishes        string          `json:"spiritualWishes,omitempty"`
	CulturalWishes         string          `json:"culturalWishes,omitempty"`
	LegalProxy             *LegalProxy       `json:"legalProxy,omitempty"`
	Decisions              []CareDecision    `json:"decisions,omitempty"`
	ReviewDate             time.Time         `json:"reviewDate"`
	CreatedAt              time.Time         `json:"createdAt"`
	UpdatedAt              time.Time         `json:"updatedAt"`
}

// LegalProxy is an Enduring Power of Attorney (EPA) or welfare guardian.
type LegalProxy struct {
	Name         string `json:"name"`
	Relationship string `json:"relationship"`
	Phone        string `json:"phone,omitempty"`
	Email        string `json:"email,omitempty"`
	EPAReference string `json:"epaReference,omitempty"`
	IsActive     bool   `json:"isActive"`
}

// CareDecision captures a specific treatment preference.
type CareDecision struct {
	ID          string    `json:"id"`
	Treatment   string    `json:"treatment"` // e.g. "mechanical_ventilation", "cpr", "antibiotics", "artificial_nutrition"
	Decision    string    `json:"decision"`  // yes, no, maybe, time_limited
	Reason      string    `json:"reason,omitempty"`
	TimeLimitedUntil *time.Time `json:"timeLimitedUntil,omitempty"`
	ClinicalRecommendation string `json:"clinicalRecommendation,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}
