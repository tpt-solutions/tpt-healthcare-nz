// Package pain provides domain models for pain assessment and WHO analgesic ladder protocols.
package pain

import "time"

// SeverityLevel is a pain severity classification.
type SeverityLevel string

const (
	SeverityMild     SeverityLevel = "mild"      // 1-3
	SeverityModerate SeverityLevel = "moderate"  // 4-6
	SeveritySevere   SeverityLevel = "severe"    // 7-10
)

// PainType categorises the pain aetiology.
type PainType string

const (
	TypeNociceptive  PainType = "nociceptive"
	TypeNeuropathic  PainType = "neuropathic"
	TypeVisceral     PainType = "visceral"
	TypeBreakthrough PainType = "breakthrough"
	TypeMixed        PainType = "mixed"
)

// ProtocolStep is a WHO analgesic ladder step (or beyond).
type ProtocolStep string

const (
	StepOneNonOpioid   ProtocolStep = "step_1_non_opioid"    // paracetamol, NSAIDs
	StepTwoWeakOpioid  ProtocolStep = "step_2_weak_opioid"   // codeine, tramadol + non-opioid
	StepThreeStrong    ProtocolStep = "step_3_strong_opioid"   // morphine, oxycodone, fentanyl + adjuvant
	StepFourInterventional ProtocolStep = "step_4_interventional" // nerve blocks, spinal pumps, ketamine
)

// Assessment is a structured pain assessment record.
type Assessment struct {
	ID             string        `json:"id"`
	PatientNHI     string        `json:"patientNhi"`
	AssessmentDate time.Time     `json:"assessmentDate"`
	AssessorID     string        `json:"assessorId"`
	PainScore      int           `json:"painScore"` // 0-10 numeric rating scale
	Severity       SeverityLevel `json:"severity"`
	PainType       PainType      `json:"painType"`
	Location       string        `json:"location,omitempty"`
	Quality        string        `json:"quality,omitempty"` // sharp, burning, aching, etc.
	Exacerbating   string        `json:"exacerbating,omitempty"`
	Relieving      string        `json:"relieving,omitempty"`
	ImpactSleep    int           `json:"impactSleep"` // 0-10
	ImpactMobility int           `json:"impactMobility"`
	ImpactMood     int           `json:"impactMood"`
	BreakthroughEpisodes int     `json:"breakthroughEpisodes"` // in last 24h
	Notes          string        `json:"notes,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
}

// ProtocolRecord documents a pain protocol assigned to a patient.
type ProtocolRecord struct {
	ID               string       `json:"id"`
	PatientNHI       string       `json:"patientNhi"`
	Step             ProtocolStep `json:"step"`
	StartDate        time.Time    `json:"startDate"`
	EndDate          *time.Time   `json:"endDate,omitempty"`
	CurrentRegimen   []Medication `json:"currentRegimen,omitempty"`
	Adjuvants        []Medication `json:"adjuvants,omitempty"`
	BreakthroughPlan BreakthroughPlan `json:"breakthroughPlan,omitempty"`
	ReviewFrequencyDays int       `json:"reviewFrequencyDays"`
	NextReviewDate   time.Time    `json:"nextReviewDate"`
	PrescribedBy     string       `json:"prescribedBy"`
	Goals            []string     `json:"goals,omitempty"` // e.g. "pain <=3/10", "sleep through night"
	OutcomeScore     *int         `json:"outcomeScore,omitempty"` // follow-up pain score
	OutcomeDate      *time.Time   `json:"outcomeDate,omitempty"`
	CreatedAt        time.Time    `json:"createdAt"`
	UpdatedAt        time.Time    `json:"updatedAt"`
}

// Medication is a drug entry in a regimen.
type Medication struct {
	NZMTCode      string  `json:"nzmtCode,omitempty"`
	Name          string  `json:"name"`
	DoseMg        float64 `json:"doseMg"`
	Route         string  `json:"route"` // oral, s/c, transdermal, buccal, etc.
	Frequency     string  `json:"frequency"` // e.g. "q4h", "bd", "tds"
	PRN           bool    `json:"prn"`
	MaxDose24hMg  *float64 `json:"maxDose24hMg,omitempty"`
}

// BreakthroughPlan is the rescue medication plan for breakthrough pain.
type BreakthroughPlan struct {
	RescueMedication Medication `json:"rescueMedication"`
	MaxDosesPerDay   int        `json:"maxDosesPerDay"`
	Instructions     string     `json:"instructions,omitempty"`
	IfIneffective    string     `json:"ifIneffective,omitempty"` // escalation path
}

// DeliriumRiskFlags for opioid toxicity monitoring.
type DeliriumRiskFlags struct {
	PatientHasDeliriumRisk bool     `json:"patientHasDeliriumRisk"`
	RiskFactors            []string `json:"riskFactors,omitempty"` // renal_impairment, elderly, dehydration, polypharmacy
	MonitoringPlan         string   `json:"monitoringPlan,omitempty"`
}
