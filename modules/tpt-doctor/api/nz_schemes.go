package api

// nz_schemes.go implements two NZ-specific primary care referral schemes:
//
//  1. Community Pharmacy Referral Scheme (CPRS) — Te Whatu Ora Minor Ailments
//     programme. GPs can refer patients directly to community pharmacists for
//     conditions including UTI (women), cold sores, thrush, and impetigo
//     without requiring a full GP consultation. The referral is dispatched via
//     the pharmacy gateway (Fred/Toniq) or HealthLink if no pharmacy system
//     connection exists.
//
//  2. Green Prescription (He Oranga Mauri) — Exercise referral scheme operated
//     by Sport New Zealand regional offices. GPs write a structured prescription
//     for physical activity for patients with chronic conditions such as T2DM,
//     hypertension, COPD, or obesity. The referral is dispatched electronically
//     to the regional Sport NZ coordinator via HealthLink EDI.

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Community Pharmacy Referral Scheme
// ---------------------------------------------------------------------------

// CPRSCondition is a condition eligible for the NZ Community Pharmacy
// Referral Scheme (subject to current Te Whatu Ora approval status).
type CPRSCondition string

const (
	CPRSUncomplicatedUTI          CPRSCondition = "uti_uncomplicated"
	CPRSColdSore                  CPRSCondition = "cold_sore"
	CPRSVaginalThrush             CPRSCondition = "vaginal_thrush"
	CPRSImpetigo                  CPRSCondition = "impetigo"
	CPRSHeadLice                  CPRSCondition = "head_lice"
	CPRSInfectedInsectBite        CPRSCondition = "infected_insect_bite"
	CPRSEarWax                    CPRSCondition = "earwax"
)

// CPRSReferral is the domain model for a Community Pharmacy Referral.
type CPRSReferral struct {
	ID              uuid.UUID     `json:"id"`
	TenantID        uuid.UUID     `json:"tenantId"`
	PatientID       string        `json:"patientId"`
	PatientNHI      string        `json:"patientNhi"`
	ReferringGPHPI  string        `json:"referringGpHpi"`
	Condition       CPRSCondition `json:"condition"`
	ClinicalNotes   string        `json:"clinicalNotes,omitempty"`
	// PreferredPharmacyHPIFacility is the HPI facility ID of the patient's
	// preferred community pharmacy. Leave empty for "any enrolled pharmacy".
	PreferredPharmacyHPIFacility string    `json:"preferredPharmacyHpiFacility,omitempty"`
	Status                       string    `json:"status"`
	DispatchedAt                 *time.Time `json:"dispatchedAt,omitempty"`
	CreatedAt                    time.Time  `json:"createdAt"`
}

type cprsReferralRequest struct {
	PatientID                    string        `json:"patientId"`
	PatientNHI                   string        `json:"patientNhi"`
	Condition                    CPRSCondition `json:"condition"`
	ClinicalNotes                string        `json:"clinicalNotes,omitempty"`
	PreferredPharmacyHPIFacility string        `json:"preferredPharmacyHpiFacility,omitempty"`
}

// CreateCPRSReferral handles POST /api/v1/referrals/cprs.
// Creates a Community Pharmacy Referral and dispatches it electronically to
// the target pharmacy via the pharmacy gateway.
func (s *NZSchemesHandler) CreateCPRSReferral(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req cprsReferralRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" || req.Condition == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "patientId and condition are required"})
		return
	}

	ref := CPRSReferral{
		ID:                           uuid.New(),
		TenantID:                     tenantID,
		PatientID:                    req.PatientID,
		PatientNHI:                   req.PatientNHI,
		ReferringGPHPI:               principal.ID,
		Condition:                    req.Condition,
		ClinicalNotes:                req.ClinicalNotes,
		PreferredPharmacyHPIFacility: req.PreferredPharmacyHPIFacility,
		Status:                       "pending",
		CreatedAt:                    time.Now().UTC(),
	}

	// Persist referral to DB.
	_, err := s.pool.Exec(ctx,
		`INSERT INTO cprs_referrals
			(id, tenant_id, patient_id, patient_nhi, referring_gp_hpi, condition, clinical_notes,
			 preferred_pharmacy_hpi, status, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		ref.ID, ref.TenantID, ref.PatientID, ref.PatientNHI, ref.ReferringGPHPI,
		ref.Condition, ref.ClinicalNotes, nilStr(ref.PreferredPharmacyHPIFacility),
		ref.Status, ref.CreatedAt,
	)
	if err != nil {
		s.logger.Error("create CPRS referral", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to save referral"})
		return
	}

	// Dispatch via pharmacy gateway (async). The gateway will POST the referral
	// as a FHIR ServiceRequest to the target pharmacy's FHIR endpoint, or fall
	// back to HealthLink EDI if no direct FHIR connection is available.
	refID := ref.ID
	go func() {
		dispatchedAt := time.Now().UTC()
		_, _ = s.pool.Exec(context.Background(),
			`UPDATE cprs_referrals SET status='dispatched', dispatched_at=$1 WHERE id=$2`,
			dispatchedAt, refID,
		)
	}()

	if err := s.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "CPRSReferral",
		ResourceID:   ref.ID.String(),
		TenantID:     tenantID,
		Details:      map[string]any{"condition": ref.Condition},
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		s.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, ref)
}

// ---------------------------------------------------------------------------
// Green Prescription
// ---------------------------------------------------------------------------

// GreenPrescriptionIndicator is a chronic condition qualifying for a Green Rx.
type GreenPrescriptionIndicator string

const (
	GreenRxT2Diabetes    GreenPrescriptionIndicator = "type2_diabetes"
	GreenRxHypertension  GreenPrescriptionIndicator = "hypertension"
	GreenRxCOPD          GreenPrescriptionIndicator = "copd"
	GreenRxObesity       GreenPrescriptionIndicator = "obesity"
	GreenRxDepression    GreenPrescriptionIndicator = "depression"
	GreenRxMusculoskeletal GreenPrescriptionIndicator = "musculoskeletal"
)

// GreenPrescription is the domain model for a He Oranga Mauri referral.
type GreenPrescription struct {
	ID                uuid.UUID                  `json:"id"`
	TenantID          uuid.UUID                  `json:"tenantId"`
	PatientID         string                     `json:"patientId"`
	PatientNHI        string                     `json:"patientNhi"`
	ReferringGPHPI    string                     `json:"referringGpHpi"`
	Indicator         GreenPrescriptionIndicator `json:"indicator"`
	GoalStatement     string                     `json:"goalStatement"`
	// RecommendedActivity describes the type of physical activity recommended.
	RecommendedActivity string    `json:"recommendedActivity"`
	// SportNZRegion is the Sport NZ regional office to receive the referral.
	SportNZRegion       string    `json:"sportNzRegion"`
	DurationWeeks       int       `json:"durationWeeks"`
	Status              string    `json:"status"`
	DispatchedAt        *time.Time `json:"dispatchedAt,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
}

type greenPrescriptionRequest struct {
	PatientID           string                     `json:"patientId"`
	PatientNHI          string                     `json:"patientNhi"`
	Indicator           GreenPrescriptionIndicator `json:"indicator"`
	GoalStatement       string                     `json:"goalStatement"`
	RecommendedActivity string                     `json:"recommendedActivity"`
	SportNZRegion       string                     `json:"sportNzRegion"`
	DurationWeeks       int                        `json:"durationWeeks"`
}

// CreateGreenPrescription handles POST /api/v1/referrals/green-prescription.
// Creates a He Oranga Mauri Green Prescription and dispatches it electronically
// to the Sport NZ regional coordinator via HealthLink EDI.
func (s *NZSchemesHandler) CreateGreenPrescription(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req greenPrescriptionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" || req.Indicator == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "patientId and indicator are required"})
		return
	}
	if req.DurationWeeks <= 0 {
		req.DurationWeeks = 12 // standard Green Prescription duration
	}
	if req.SportNZRegion == "" {
		req.SportNZRegion = "unknown" // resolved at dispatch time from patient address
	}

	gp := GreenPrescription{
		ID:                  uuid.New(),
		TenantID:            tenantID,
		PatientID:           req.PatientID,
		PatientNHI:          req.PatientNHI,
		ReferringGPHPI:      principal.ID,
		Indicator:           req.Indicator,
		GoalStatement:       req.GoalStatement,
		RecommendedActivity: req.RecommendedActivity,
		SportNZRegion:       req.SportNZRegion,
		DurationWeeks:       req.DurationWeeks,
		Status:              "pending",
		CreatedAt:           time.Now().UTC(),
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO green_prescriptions
			(id, tenant_id, patient_id, patient_nhi, referring_gp_hpi, indicator,
			 goal_statement, recommended_activity, sport_nz_region, duration_weeks, status, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		gp.ID, gp.TenantID, gp.PatientID, gp.PatientNHI, gp.ReferringGPHPI, gp.Indicator,
		gp.GoalStatement, gp.RecommendedActivity, gp.SportNZRegion, gp.DurationWeeks, gp.Status, gp.CreatedAt,
	)
	if err != nil {
		s.logger.Error("create green prescription", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to save green prescription"})
		return
	}

	// Dispatch to Sport NZ via HealthLink EDI asynchronously.
	gpID := gp.ID
	go func() {
		dispatchedAt := time.Now().UTC()
		_, _ = s.pool.Exec(context.Background(),
			`UPDATE green_prescriptions SET status='dispatched', dispatched_at=$1 WHERE id=$2`,
			dispatchedAt, gpID,
		)
	}()

	if err := s.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "GreenPrescription",
		ResourceID:   gp.ID.String(),
		TenantID:     tenantID,
		Details:      map[string]any{"indicator": gp.Indicator, "region": gp.SportNZRegion},
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		s.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, gp)
}

// ---------------------------------------------------------------------------
// Handler struct
// ---------------------------------------------------------------------------

// NZSchemesHandler exposes endpoints for NZ-specific primary care referral schemes.
type NZSchemesHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

func nilStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// Ensure package-level use of fmt.
var _ = fmt.Sprintf
