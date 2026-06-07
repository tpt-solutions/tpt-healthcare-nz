// Package acc implements ACC claim lodgement and management for allied health professions
// (physiotherapy, occupational therapy, speech-language therapy, podiatry).
package acc

import (
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/nhi"
)

// ClaimType categorises the type of ACC claim.
type ClaimType string

const (
	ClaimTypePhysiotherapy     ClaimType = "physiotherapy"
	ClaimTypeOccupationalTherapy ClaimType = "occupational_therapy"
	ClaimTypeSpeechLanguage    ClaimType = "speech_language_therapy"
	ClaimTypePodiatry          ClaimType = "podiatry"
)

// ClaimStatus tracks the lifecycle of an ACC claim.
type ClaimStatus string

const (
	ClaimStatusDraft       ClaimStatus = "draft"
	ClaimStatusSubmitted   ClaimStatus = "submitted"
	ClaimStatusAccepted    ClaimStatus = "accepted"
	ClaimStatusDeclined    ClaimStatus = "declined"
	ClaimStatusUnderReview ClaimStatus = "under_review"
	ClaimStatusClosed      ClaimStatus = "closed"
	ClaimStatusExpired     ClaimStatus = "expired"
)

// TreatmentStatus tracks treatment progress within a claim.
type TreatmentStatus string

const (
	TreatmentStatusPlanned   TreatmentStatus = "planned"
	TreatmentStatusActive    TreatmentStatus = "active"
	TreatmentStatusCompleted TreatmentStatus = "completed"
	TreatmentStatusSuspended TreatmentStatus = "suspended"
	TreatmentStatusDeclined  TreatmentStatus = "declined"
)

// Claim represents an ACC claim for allied health treatment.
type Claim struct {
	ID              string        `json:"id"`
	PatientNHI      string        `json:"patientNhi"`
	ClinicianID     string        `json:"clinicianId"`
	PracticeID      string        `json:"practiceId"`
	ClaimType       ClaimType     `json:"claimType"`
	ACCNumber       string        `json:"accNumber"`
	InjuryDate      int64         `json:"injuryDate"`
	ClaimDate       int64         `json:"claimDate"`
	Status          ClaimStatus   `json:"status"`
	Diagnosis       string        `json:"diagnosis"`
	ICD10Code       string        `json:"icd10Code,omitempty"`
	BodyRegion      string        `json:"bodyRegion"`
	InjuryMechanism string        `json:"injuryMechanism"`
	Referrer        string        `json:"referrer,omitempty"` // GP, specialist, self
	ApprovedSessions int          `json:"approvedSessions"`
	UsedSessions    int           `json:"usedSessions"`
	StartDate       int64         `json:"startDate"`
	ExpiryDate      int64         `json:"expiryDate"`
	LastTreatmentDate int64       `json:"lastTreatmentDate,omitempty"`
	NextReviewDate  int64         `json:"nextReviewDate,omitempty"`
	ClinicalNotes   string        `json:"clinicalNotes,omitempty"`
	CreatedAt       int64         `json:"createdAt"`
	UpdatedAt       int64         `json:"updatedAt"`
}

// TreatmentSession represents a single treatment session under an ACC claim.
type TreatmentSession struct {
	ID              string            `json:"id"`
	ClaimID         string            `json:"claimId"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	SessionDate     int64             `json:"sessionDate"`
	SessionNumber   int               `json:"sessionNumber"`
	DurationMinutes int               `json:"durationMinutes"`
	ChargeCode      string            `json:"chargeCode"` // ACC charge code
	ChargeAmount    float64           `json:"chargeAmount"`
	TreatmentType   string            `json:"treatmentType"`
	BodyRegion      string            `json:"bodyRegion"`
	Subjective      string            `json:"subjective"`
	Objective       string            `json:"objective"`
	Assessment      string            `json:"assessment"`
	Plan            string            `json:"plan"`
	OutcomeMeasures []OutcomeMeasure  `json:"outcomeMeasures"`
	Status          TreatmentStatus   `json:"status"`
	SubmittedAt     int64             `json:"submittedAt,omitempty"`
	PaidAt          int64             `json:"paidAt,omitempty"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// OutcomeMeasure represents a standardised outcome measure for ACC reporting.
type OutcomeMeasure struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`           // e.g., "NDI", "ODI", "DASH", "LEFS", "COPM", "FIM"
	Domain        string  `json:"domain"`
	Score         float64 `json:"score"`
	MaxScore      float64 `json:"maxScore"`
	Date          int64   `json:"date"`
	Interpretation string `json:"interpretation,omitempty"`
	CreatedAt     int64   `json:"createdAt"`
}

// ReviewReport represents a clinical review report for ACC.
type ReviewReport struct {
	ID              string            `json:"id"`
	ClaimID         string            `json:"claimId"`
	PatientNHI      string            `json:"patientNhi"`
	ClinicianID     string            `json:"clinicianId"`
	ReportDate      int64             `json:"reportDate"`
	ReportType      ReviewType        `json:"reportType"`
	SessionsSinceLastReview int       `json:"sessionsSinceLastReview"`
	ProgressSummary string            `json:"progressSummary"`
	CurrentStatus   string            `json:"currentStatus"`
	GoalsAchieved   []string          `json:"goalsAchieved"`
	GoalsOngoing    []string          `json:"goalsOngoing"`
	GoalsNotAchieved []string         `json:"goalsNotAchieved"`
	OutcomeMeasures []OutcomeMeasure  `json:"outcomeMeasures"`
	Recommendation  ReviewRecommendation `json:"recommendation"`
	AdditionalSessionsRequested int   `json:"additionalSessionsRequested"`
	ProposedEndDate int64             `json:"proposedEndDate,omitempty"`
	Status          ReviewStatus      `json:"status"`
	SubmittedAt     int64             `json:"submittedAt,omitempty"`
	CreatedAt       int64             `json:"createdAt"`
	UpdatedAt       int64             `json:"updatedAt"`
}

// ReviewType categorises the type of review.
type ReviewType string

const (
	ReviewTypeInitial     ReviewType = "initial"
	ReviewTypeProgress    ReviewType = "progress"
	ReviewTypeDischarge   ReviewType = "discharge"
	ReviewTypeExtension   ReviewType = "extension"
	ReviewTypeReassessment ReviewType = "reassessment"
)

// ReviewRecommendation indicates the clinician's recommendation.
type ReviewRecommendation string

const (
	RecommendContinue     ReviewRecommendation = "continue"
	RecommendExtend       ReviewRecommendation = "extend"
	RecommendDischarge    ReviewRecommendation = "discharge"
	RecommendRefer        ReviewRecommendation = "refer"
	RecommendInvestigate  ReviewRecommendation = "investigate"
)

// ReviewStatus tracks review report status.
type ReviewStatus string

const (
	ReviewStatusDraft       ReviewStatus = "draft"
	ReviewStatusSubmitted   ReviewStatus = "submitted"
	ReviewStatusAccepted    ReviewStatus = "accepted"
	ReviewStatusDeclined    ReviewStatus = "declined"
	ReviewStatusMoreInfo    ReviewStatus = "more_info_required"
)

// ChargeCode represents an ACC charge code for allied health.
type ChargeCode struct {
	Code        string  `json:"code"`
	Description string  `json:"description"`
	Profession  string  `json:"profession"` // physio, ot, speech, podiatry
	Unit        string  `json:"unit"`       // session, 15min, 30min, 45min, 60min
	Rate        float64 `json:"rate"`
	EffectiveFrom int64 `json:"effectiveFrom"`
	EffectiveTo   int64 `json:"effectiveTo,omitempty"`
	Active      bool    `json:"active"`
}

// NewClaim creates a new ACC claim with defaults.
func NewClaim() *Claim {
	now := time.Now().UnixMilli()
	return &Claim{
		Status:       ClaimStatusDraft,
		ClaimDate:    now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Validate checks required fields for an ACC claim.
func (c *Claim) Validate() error {
	if c.PatientNHI == "" {
		return fmt.Errorf("acc: patient NHI is required")
	}
	if !nhi.ValidateNHI(c.PatientNHI) {
		return fmt.Errorf("acc: invalid patient NHI: %s", c.PatientNHI)
	}
	if c.ClinicianID == "" {
		return fmt.Errorf("acc: clinician ID is required")
	}
	if c.ClaimType == "" {
		return fmt.Errorf("acc: claim type is required")
	}
	if c.ACCNumber == "" {
		return fmt.Errorf("acc: ACC claim number is required")
	}
	if c.InjuryDate == 0 {
		return fmt.Errorf("acc: injury date is required")
	}
	if c.Diagnosis == "" {
		return fmt.Errorf("acc: diagnosis is required")
	}
	if c.BodyRegion == "" {
		return fmt.Errorf("acc: body region is required")
	}
	if c.StartDate == 0 {
		return fmt.Errorf("acc: start date is required")
	}
	if c.ExpiryDate == 0 {
		return fmt.Errorf("acc: expiry date is required")
	}
	return nil
}

// CanAddSession checks if another session can be added to this claim.
// Returns false if the claim is not accepted, has no remaining approved sessions,
// or has passed its expiry date.
func (c *Claim) CanAddSession() bool {
	if c.Status != ClaimStatusAccepted {
		return false
	}
	if c.UsedSessions >= c.ApprovedSessions {
		return false
	}
	if c.ExpiryDate > 0 && time.Now().UnixMilli() > c.ExpiryDate {
		return false
	}
	return true
}

// AddSession increments the used sessions count.
func (c *Claim) AddSession() {
	c.UsedSessions++
	c.LastTreatmentDate = time.Now().UnixMilli()
	c.UpdatedAt = time.Now().UnixMilli()
}

// NewTreatmentSession creates a new treatment session with defaults.
func NewTreatmentSession() *TreatmentSession {
	now := time.Now().UnixMilli()
	return &TreatmentSession{
		OutcomeMeasures: []OutcomeMeasure{},
		Status:          TreatmentStatusPlanned,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// Validate checks required fields for a treatment session.
func (s *TreatmentSession) Validate() error {
	if s.ClaimID == "" {
		return fmt.Errorf("acc: claim ID is required")
	}
	if s.PatientNHI == "" {
		return fmt.Errorf("acc: patient NHI is required")
	}
	if !nhi.ValidateNHI(s.PatientNHI) {
		return fmt.Errorf("acc: invalid patient NHI: %s", s.PatientNHI)
	}
	if s.ClinicianID == "" {
		return fmt.Errorf("acc: clinician ID is required")
	}
	if s.SessionDate == 0 {
		return fmt.Errorf("acc: session date is required")
	}
	if s.ChargeCode == "" {
		return fmt.Errorf("acc: charge code is required")
	}
	if GetChargeCodeByCode(s.ChargeCode) == nil {
		return fmt.Errorf("acc: unknown charge code: %s", s.ChargeCode)
	}
	if s.DurationMinutes <= 0 {
		return fmt.Errorf("acc: duration must be positive")
	}
	return nil
}

// NewReviewReport creates a new review report with defaults.
func NewReviewReport() *ReviewReport {
	now := time.Now().UnixMilli()
	return &ReviewReport{
		GoalsAchieved:       []string{},
		GoalsOngoing:        []string{},
		GoalsNotAchieved:    []string{},
		OutcomeMeasures:     []OutcomeMeasure{},
		Status:              ReviewStatusDraft,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

// Validate checks required fields for a review report.
func (r *ReviewReport) Validate() error {
	if r.ClaimID == "" {
		return fmt.Errorf("acc: claim ID is required")
	}
	if r.PatientNHI == "" {
		return fmt.Errorf("acc: patient NHI is required")
	}
	if !nhi.ValidateNHI(r.PatientNHI) {
		return fmt.Errorf("acc: invalid patient NHI: %s", r.PatientNHI)
	}
	if r.ClinicianID == "" {
		return fmt.Errorf("acc: clinician ID is required")
	}
	if r.ReportDate == 0 {
		return fmt.Errorf("acc: report date is required")
	}
	if r.ReportType == "" {
		return fmt.Errorf("acc: report type is required")
	}
	if r.Recommendation == "" {
		return fmt.Errorf("acc: recommendation is required")
	}
	return nil
}

// Standard ACC charge codes for allied health (NZ 2024 rates - indicative).
var StandardChargeCodes = []ChargeCode{
	// Physiotherapy
	{Code: "PHY001", Description: "Physiotherapy initial assessment (45 min)", Profession: "physiotherapy", Unit: "session", Rate: 85.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "PHY002", Description: "Physiotherapy follow-up treatment (30 min)", Profession: "physiotherapy", Unit: "session", Rate: 55.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "PHY003", Description: "Physiotherapy extended treatment (45 min)", Profession: "physiotherapy", Unit: "session", Rate: 75.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "PHY004", Description: "Physiotherapy group session (60 min)", Profession: "physiotherapy", Unit: "session", Rate: 35.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "PHY005", Description: "Physiotherapy hydrotherapy (45 min)", Profession: "physiotherapy", Unit: "session", Rate: 65.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "PHY006", Description: "Physiotherapy report writing (per 15 min)", Profession: "physiotherapy", Unit: "15min", Rate: 22.00, EffectiveFrom: 1704067200000, Active: true},

	// Occupational Therapy
	{Code: "OT001", Description: "OT initial assessment (60 min)", Profession: "occupational_therapy", Unit: "session", Rate: 95.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "OT002", Description: "OT follow-up treatment (45 min)", Profession: "occupational_therapy", Unit: "session", Rate: 70.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "OT003", Description: "OT home visit assessment (90 min)", Profession: "occupational_therapy", Unit: "session", Rate: 140.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "OT004", Description: "OT worksite assessment (120 min)", Profession: "occupational_therapy", Unit: "session", Rate: 180.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "OT005", Description: "OT equipment prescription (30 min)", Profession: "occupational_therapy", Unit: "session", Rate: 55.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "OT006", Description: "OT report writing (per 15 min)", Profession: "occupational_therapy", Unit: "15min", Rate: 25.00, EffectiveFrom: 1704067200000, Active: true},

	// Speech-Language Therapy
	{Code: "SLT001", Description: "SLT initial assessment (60 min)", Profession: "speech_language_therapy", Unit: "session", Rate: 100.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "SLT002", Description: "SLT follow-up therapy (45 min)", Profession: "speech_language_therapy", Unit: "session", Rate: 75.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "SLT003", Description: "SLT swallowing assessment (60 min)", Profession: "speech_language_therapy", Unit: "session", Rate: 110.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "SLT004", Description: "SLT AAC assessment (90 min)", Profession: "speech_language_therapy", Unit: "session", Rate: 150.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "SLT005", Description: "SLT report writing (per 15 min)", Profession: "speech_language_therapy", Unit: "15min", Rate: 28.00, EffectiveFrom: 1704067200000, Active: true},

	// Podiatry
	{Code: "POD001", Description: "Podiatry initial assessment (30 min)", Profession: "podiatry", Unit: "session", Rate: 65.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "POD002", Description: "Podiatry follow-up treatment (20 min)", Profession: "podiatry", Unit: "session", Rate: 45.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "POD003", Description: "Podiatry wound care (30 min)", Profession: "podiatry", Unit: "session", Rate: 70.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "POD004", Description: "Podiatry nail surgery (60 min)", Profession: "podiatry", Unit: "session", Rate: 180.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "POD005", Description: "Podiatry diabetic foot assessment (45 min)", Profession: "podiatry", Unit: "session", Rate: 85.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "POD006", Description: "Podiatry orthotic therapy (45 min)", Profession: "podiatry", Unit: "session", Rate: 95.00, EffectiveFrom: 1704067200000, Active: true},
	{Code: "POD007", Description: "Podiatry report writing (per 15 min)", Profession: "podiatry", Unit: "15min", Rate: 20.00, EffectiveFrom: 1704067200000, Active: true},
}

// GetChargeCodesByProfession returns charge codes for a specific profession.
func GetChargeCodesByProfession(profession string) []ChargeCode {
	var codes []ChargeCode
	for _, code := range StandardChargeCodes {
		if code.Profession == profession && code.Active {
			codes = append(codes, code)
		}
	}
	return codes
}

// GetChargeCodeByCode returns a charge code by its code.
func GetChargeCodeByCode(code string) *ChargeCode {
	for _, c := range StandardChargeCodes {
		if c.Code == code && c.Active {
			return &c
		}
	}
	return nil
}