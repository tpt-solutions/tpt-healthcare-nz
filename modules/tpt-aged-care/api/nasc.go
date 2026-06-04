package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// NASCReferralStatus tracks the lifecycle of a referral to the NASC organisation.
type NASCReferralStatus string

const (
	ReferralPending    NASCReferralStatus = "pending"
	ReferralAccepted   NASCReferralStatus = "accepted"
	ReferralAssessing  NASCReferralStatus = "assessing"
	ReferralCompleted  NASCReferralStatus = "completed"
	ReferralDeclined   NASCReferralStatus = "declined"
	ReferralWithdrawn  NASCReferralStatus = "withdrawn"
)

// SupportNeedsLevel is the output tier assigned by the NASC after assessment.
// Levels determine the maximum weekly funded hours and service types available.
type SupportNeedsLevel string

const (
	NeedsLevelLow      SupportNeedsLevel = "low"
	NeedsLevelModerate SupportNeedsLevel = "moderate"
	NeedsLevelHigh     SupportNeedsLevel = "high"
	NeedsLevelComplex  SupportNeedsLevel = "complex"
)

// ServicePlanStatus tracks the lifecycle of a funded service plan.
type ServicePlanStatus string

const (
	PlanActive    ServicePlanStatus = "active"
	PlanExpiring  ServicePlanStatus = "expiring"
	PlanExpired   ServicePlanStatus = "expired"
	PlanSuspended ServicePlanStatus = "suspended"
	PlanClosed    ServicePlanStatus = "closed"
)

// FundedService is a single service line in an NASC service plan.
type FundedService struct {
	ServiceType   string  `json:"serviceType"`   // e.g., "personal-care", "domestic", "day-programme"
	HoursPerWeek  float64 `json:"hoursPerWeek"`
	ProviderID    string  `json:"providerId,omitempty"`
	ProviderName  string  `json:"providerName,omitempty"`
	StartDate     string  `json:"startDate"` // YYYY-MM-DD
	EndDate       string  `json:"endDate,omitempty"`
}

// NASCReferral represents a referral sent to the NASC for needs assessment.
type NASCReferral struct {
	ID              string             `json:"id"`
	PatientID       string             `json:"patientId"`
	PatientNHI      string             `json:"patientNhi"`
	ReferrerHPI     string             `json:"referrerHpi"`
	TenantID        string             `json:"tenantId"`
	Status          NASCReferralStatus `json:"status"`
	ReferralReason  string             `json:"referralReason"`
	UrgencyFlag     bool               `json:"urgencyFlag"`
	// NASCOrgCode is the DHB-region NASC organisation code (MoH-assigned).
	NASCOrgCode     string            `json:"nascOrgCode"`
	InterRAIRefID   string            `json:"interraiRefId,omitempty"`
	CompletedAt     *time.Time        `json:"completedAt,omitempty"`
	DeclineReason   string            `json:"declineReason,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

// NASCServicePlan represents the funded support plan produced after NASC assessment.
type NASCServicePlan struct {
	ID              string            `json:"id"`
	PatientID       string            `json:"patientId"`
	PatientNHI      string            `json:"patientNhi"`
	TenantID        string            `json:"tenantId"`
	ReferralID      string            `json:"referralId"`
	Status          ServicePlanStatus `json:"status"`
	NeedsLevel      SupportNeedsLevel `json:"needsLevel"`
	Services        []FundedService   `json:"services"`
	// GoalsNotes is AES-256-GCM encrypted at rest.
	GoalsNotes      string            `json:"goalsNotes,omitempty"`
	PlanStartDate   string            `json:"planStartDate"` // YYYY-MM-DD
	PlanEndDate     string            `json:"planEndDate,omitempty"`
	NextReviewDate  string            `json:"nextReviewDate,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

// Internal DB records.
type nascReferralRecord struct {
	ID            string
	PatientID     string
	PatientNHI    string
	ReferrerHPI   string
	TenantID      string
	Status        NASCReferralStatus
	Reason        string
	UrgencyFlag   bool
	NASCOrgCode   string
	InterRAIRefID string
	CompletedAt   *time.Time
	DeclineReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type nascServicePlanRecord struct {
	ID            string
	PatientID     string
	PatientNHI    string
	TenantID      string
	ReferralID    string
	Status        ServicePlanStatus
	NeedsLevel    SupportNeedsLevel
	Services      []FundedService
	GoalsEnc      []byte
	PlanStartDate string
	PlanEndDate   string
	NextReview    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NASCHandler handles all /api/v1/nasc/* routes.
type NASCHandler struct {
	pool       dbPool
	enc        *encryption.Encryptor
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ---------------------------------------------------------------------------
// Referral handlers
// ---------------------------------------------------------------------------

// ListReferrals handles GET /api/v1/nasc/referrals.
func (h *NASCHandler) ListReferrals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	q := r.URL.Query()
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, referrer_hpi, tenant_id,
		        status, referral_reason, urgency_flag, nasc_org_code,
		        interrai_ref_id, completed_at, decline_reason, created_at, updated_at
		 FROM aged_care_nasc_referrals
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR status = $3)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		tenantID, q.Get("patient"), q.Get("status"),
	)
	if err != nil {
		h.logger.Error("list NASC referrals", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list referrals"})
		return
	}
	defer rows.Close()

	var results []NASCReferral
	for rows.Next() {
		rec, err := scanReferral(rows)
		if err != nil {
			h.logger.Error("scan NASC referral", slog.Any("error", err))
			continue
		}
		results = append(results, referralToResponse(rec))
	}
	if err := rows.Err(); err != nil {
		h.logger.Error("iterate NASC referrals", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to iterate referrals"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "NASCReferral",
		ResourceID:   "list",
	})

	writeJSON(w, http.StatusOK, map[string]any{"referrals": results, "total": len(results)})
}

// GetReferral handles GET /api/v1/nasc/referrals/{id}.
func (h *NASCHandler) GetReferral(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	rec, err := h.getReferralByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "referral not found"})
			return
		}
		h.logger.Error("get NASC referral", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve referral"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "NASCReferral",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
	})

	writeJSON(w, http.StatusOK, referralToResponse(rec))
}

// CreateReferral handles POST /api/v1/nasc/referrals.
func (h *NASCHandler) CreateReferral(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req struct {
		PatientID      string `json:"patientId"`
		PatientNHI     string `json:"patientNhi"`
		ReferrerHPI    string `json:"referrerHpi"`
		ReferralReason string `json:"referralReason"`
		UrgencyFlag    bool   `json:"urgencyFlag"`
		NASCOrgCode    string `json:"nascOrgCode"`
		InterRAIRefID  string `json:"interraiRefId,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" && req.PatientID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patientId or patientNhi is required"})
		return
	}
	if req.ReferrerHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_REFERRER", Message: "referrerHpi is required"})
		return
	}
	if req.NASCOrgCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NASC_ORG", Message: "nascOrgCode is required"})
		return
	}

	apcValid, err := h.hpiClient.ValidateAPC(ctx, req.ReferrerHPI)
	if err != nil {
		h.logger.Error("HPI APC check", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not verify practitioner APC"})
		return
	}
	if !apcValid {
		writeJSON(w, http.StatusForbidden, apiError{Code: "INVALID_APC", Message: "referrer does not hold a current Annual Practising Certificate"})
		return
	}

	var interraiRef *string
	if req.InterRAIRefID != "" {
		interraiRef = &req.InterRAIRefID
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO aged_care_nasc_referrals
		   (patient_id, patient_nhi, referrer_hpi, tenant_id, status,
		    referral_reason, urgency_flag, nasc_org_code, interrai_ref_id)
		 VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8)
		 RETURNING id, patient_id, patient_nhi, referrer_hpi, tenant_id,
		           status, referral_reason, urgency_flag, nasc_org_code,
		           interrai_ref_id, completed_at, decline_reason, created_at, updated_at`,
		req.PatientID, req.PatientNHI, req.ReferrerHPI, tenantID,
		req.ReferralReason, req.UrgencyFlag, req.NASCOrgCode, interraiRef,
	)
	rec, err := scanReferral(row)
	if err != nil {
		h.logger.Error("insert NASC referral", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create referral"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "NASCReferral",
		ResourceID:   rec.ID,
		PatientNHI:   req.PatientNHI,
		Details:      map[string]any{"nascOrgCode": req.NASCOrgCode, "urgent": req.UrgencyFlag},
	})

	writeJSON(w, http.StatusCreated, referralToResponse(rec))
}

// UpdateReferral handles PUT /api/v1/nasc/referrals/{id}.
func (h *NASCHandler) UpdateReferral(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	var req struct {
		Status        NASCReferralStatus `json:"status,omitempty"`
		DeclineReason string             `json:"declineReason,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_nasc_referrals
		 SET status         = COALESCE(NULLIF($1::text, ''), status::text)::nasc_referral_status,
		     decline_reason = COALESCE(NULLIF($2, ''), decline_reason),
		     updated_at     = now()
		 WHERE id = $3 AND tenant_id = $4
		 RETURNING id, patient_id, patient_nhi, referrer_hpi, tenant_id,
		           status, referral_reason, urgency_flag, nasc_org_code,
		           interrai_ref_id, completed_at, decline_reason, created_at, updated_at`,
		string(req.Status), req.DeclineReason, id, tenantID,
	)
	rec, err := scanReferral(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "referral not found"})
			return
		}
		h.logger.Error("update NASC referral", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update referral"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "NASCReferral",
		ResourceID:   id,
	})

	writeJSON(w, http.StatusOK, referralToResponse(rec))
}

// CompleteReferral handles POST /api/v1/nasc/referrals/{id}/complete.
func (h *NASCHandler) CompleteReferral(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	now := time.Now().UTC()
	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_nasc_referrals
		 SET status = 'completed', completed_at = $1, updated_at = $1
		 WHERE id = $2 AND tenant_id = $3
		 RETURNING id, patient_id, patient_nhi, referrer_hpi, tenant_id,
		           status, referral_reason, urgency_flag, nasc_org_code,
		           interrai_ref_id, completed_at, decline_reason, created_at, updated_at`,
		now, id, tenantID,
	)
	rec, err := scanReferral(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "referral not found"})
			return
		}
		h.logger.Error("complete NASC referral", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "COMPLETE_ERROR", Message: "failed to complete referral"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "complete",
		ResourceType: "NASCReferral",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
	})

	writeJSON(w, http.StatusOK, referralToResponse(rec))
}

// ---------------------------------------------------------------------------
// Service plan handlers
// ---------------------------------------------------------------------------

// ListServicePlans handles GET /api/v1/nasc/service-plans.
func (h *NASCHandler) ListServicePlans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	q := r.URL.Query()
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, referral_id,
		        status, needs_level, services, goals_notes,
		        plan_start_date, plan_end_date, next_review_date, created_at, updated_at
		 FROM aged_care_nasc_service_plans
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR status = $3)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		tenantID, q.Get("patient"), q.Get("status"),
	)
	if err != nil {
		h.logger.Error("list service plans", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list service plans"})
		return
	}
	defer rows.Close()

	var results []NASCServicePlan
	for rows.Next() {
		rec, err := scanServicePlan(rows)
		if err != nil {
			h.logger.Error("scan service plan", slog.Any("error", err))
			continue
		}
		sp, err := h.decryptPlan(rec)
		if err != nil {
			h.logger.Error("decrypt service plan", slog.Any("error", err))
			continue
		}
		results = append(results, sp)
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "NASCServicePlan",
		ResourceID:   "list",
	})

	writeJSON(w, http.StatusOK, map[string]any{"plans": results, "total": len(results)})
}

// GetServicePlan handles GET /api/v1/nasc/service-plans/{id}.
func (h *NASCHandler) GetServicePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	rec, err := h.getPlanByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "service plan not found"})
			return
		}
		h.logger.Error("get service plan", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve service plan"})
		return
	}

	sp, err := h.decryptPlan(rec)
	if err != nil {
		h.logger.Error("decrypt service plan", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt service plan"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "NASCServicePlan",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
	})

	writeJSON(w, http.StatusOK, sp)
}

// CreateServicePlan handles POST /api/v1/nasc/service-plans.
func (h *NASCHandler) CreateServicePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req struct {
		PatientID     string            `json:"patientId"`
		PatientNHI    string            `json:"patientNhi"`
		ReferralID    string            `json:"referralId"`
		NeedsLevel    SupportNeedsLevel `json:"needsLevel"`
		Services      []FundedService   `json:"services"`
		GoalsNotes    string            `json:"goalsNotes,omitempty"`
		PlanStartDate string            `json:"planStartDate"`
		PlanEndDate   string            `json:"planEndDate,omitempty"`
		NextReview    string            `json:"nextReviewDate,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" && req.PatientID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patientId or patientNhi is required"})
		return
	}
	if req.PlanStartDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_START_DATE", Message: "planStartDate is required"})
		return
	}
	if !validNeedsLevel(req.NeedsLevel) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NEEDS_LEVEL", Message: fmt.Sprintf("unknown needs level %q", req.NeedsLevel)})
		return
	}

	goalsEnc, err := h.enc.Encrypt([]byte(req.GoalsNotes))
	if err != nil {
		h.logger.Error("encrypt goals notes", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt notes"})
		return
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO aged_care_nasc_service_plans
		   (patient_id, patient_nhi, tenant_id, referral_id, status, needs_level,
		    services, goals_notes, plan_start_date, plan_end_date, next_review_date)
		 VALUES ($1, $2, $3, $4, 'active', $5, $6, $7, $8, $9, $10)
		 RETURNING id, patient_id, patient_nhi, tenant_id, referral_id,
		           status, needs_level, services, goals_notes,
		           plan_start_date, plan_end_date, next_review_date, created_at, updated_at`,
		req.PatientID, req.PatientNHI, tenantID, req.ReferralID,
		string(req.NeedsLevel), req.Services, goalsEnc,
		req.PlanStartDate, req.PlanEndDate, req.NextReview,
	)
	rec, err := scanServicePlan(row)
	if err != nil {
		h.logger.Error("insert service plan", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create service plan"})
		return
	}

	sp, err := h.decryptPlan(rec)
	if err != nil {
		h.logger.Error("decrypt after insert", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt service plan"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "NASCServicePlan",
		ResourceID:   rec.ID,
		PatientNHI:   req.PatientNHI,
		Details:      map[string]any{"needsLevel": string(req.NeedsLevel)},
	})

	writeJSON(w, http.StatusCreated, sp)
}

// UpdateServicePlan handles PUT /api/v1/nasc/service-plans/{id}.
func (h *NASCHandler) UpdateServicePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	var req struct {
		Status      ServicePlanStatus `json:"status,omitempty"`
		NeedsLevel  SupportNeedsLevel `json:"needsLevel,omitempty"`
		Services    []FundedService   `json:"services,omitempty"`
		GoalsNotes  string            `json:"goalsNotes,omitempty"`
		PlanEndDate string            `json:"planEndDate,omitempty"`
		NextReview  string            `json:"nextReviewDate,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	rec, err := h.getPlanByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "service plan not found"})
			return
		}
		h.logger.Error("get plan for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve service plan"})
		return
	}

	if req.Status != "" {
		rec.Status = req.Status
	}
	if req.NeedsLevel != "" {
		rec.NeedsLevel = req.NeedsLevel
	}
	if len(req.Services) > 0 {
		rec.Services = req.Services
	}
	if req.PlanEndDate != "" {
		rec.PlanEndDate = req.PlanEndDate
	}
	if req.NextReview != "" {
		rec.NextReview = req.NextReview
	}

	goalsEnc := rec.GoalsEnc
	if req.GoalsNotes != "" {
		goalsEnc, err = h.enc.Encrypt([]byte(req.GoalsNotes))
		if err != nil {
			h.logger.Error("encrypt goals notes", slog.Any("error", err))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt notes"})
			return
		}
	}

	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_nasc_service_plans
		 SET status = $1, needs_level = $2, services = $3, goals_notes = $4,
		     plan_end_date = $5, next_review_date = $6, updated_at = now()
		 WHERE id = $7 AND tenant_id = $8
		 RETURNING id, patient_id, patient_nhi, tenant_id, referral_id,
		           status, needs_level, services, goals_notes,
		           plan_start_date, plan_end_date, next_review_date, created_at, updated_at`,
		string(rec.Status), string(rec.NeedsLevel), rec.Services, goalsEnc,
		rec.PlanEndDate, rec.NextReview, id, tenantID,
	)
	updated, err := scanServicePlan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "service plan not found"})
			return
		}
		h.logger.Error("update service plan", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update service plan"})
		return
	}

	sp, err := h.decryptPlan(updated)
	if err != nil {
		h.logger.Error("decrypt after update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt service plan"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "NASCServicePlan",
		ResourceID:   id,
	})

	writeJSON(w, http.StatusOK, sp)
}

// ReviewServicePlan handles POST /api/v1/nasc/service-plans/{id}/review.
// Records that a mandatory review was conducted and sets the next review date.
func (h *NASCHandler) ReviewServicePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	var req struct {
		NextReviewDate string `json:"nextReviewDate"`
		ReviewNotes    string `json:"reviewNotes,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.NextReviewDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NEXT_REVIEW", Message: "nextReviewDate is required"})
		return
	}

	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_nasc_service_plans
		 SET next_review_date = $1, updated_at = now()
		 WHERE id = $2 AND tenant_id = $3
		 RETURNING id, patient_id, patient_nhi, tenant_id, referral_id,
		           status, needs_level, services, goals_notes,
		           plan_start_date, plan_end_date, next_review_date, created_at, updated_at`,
		req.NextReviewDate, id, tenantID,
	)
	rec, err := scanServicePlan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "service plan not found"})
			return
		}
		h.logger.Error("review service plan", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "REVIEW_ERROR", Message: "failed to record plan review"})
		return
	}

	sp, err := h.decryptPlan(rec)
	if err != nil {
		h.logger.Error("decrypt after review", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt service plan"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "review",
		ResourceType: "NASCServicePlan",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
		Details:      map[string]any{"nextReviewDate": req.NextReviewDate},
	})

	writeJSON(w, http.StatusOK, sp)
}

// ---------------------------------------------------------------------------
// DB helpers
// ---------------------------------------------------------------------------

func (h *NASCHandler) getReferralByID(ctx context.Context, id string, tenantID uuid.UUID) (nascReferralRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, referrer_hpi, tenant_id,
		        status, referral_reason, urgency_flag, nasc_org_code,
		        interrai_ref_id, completed_at, decline_reason, created_at, updated_at
		 FROM aged_care_nasc_referrals
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanReferral(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nascReferralRecord{}, errNotFound
		}
		return nascReferralRecord{}, fmt.Errorf("get NASC referral by id: %w", err)
	}
	return rec, nil
}

func (h *NASCHandler) getPlanByID(ctx context.Context, id string, tenantID uuid.UUID) (nascServicePlanRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, referral_id,
		        status, needs_level, services, goals_notes,
		        plan_start_date, plan_end_date, next_review_date, created_at, updated_at
		 FROM aged_care_nasc_service_plans
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanServicePlan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nascServicePlanRecord{}, errNotFound
		}
		return nascServicePlanRecord{}, fmt.Errorf("get service plan by id: %w", err)
	}
	return rec, nil
}

func (h *NASCHandler) decryptPlan(rec nascServicePlanRecord) (NASCServicePlan, error) {
	var notes string
	if len(rec.GoalsEnc) > 0 {
		plain, err := h.enc.Decrypt(rec.GoalsEnc)
		if err != nil {
			return NASCServicePlan{}, fmt.Errorf("decrypt goals notes: %w", err)
		}
		notes = string(plain)
	}
	return NASCServicePlan{
		ID:            rec.ID,
		PatientID:     rec.PatientID,
		PatientNHI:    rec.PatientNHI,
		TenantID:      rec.TenantID,
		ReferralID:    rec.ReferralID,
		Status:        rec.Status,
		NeedsLevel:    rec.NeedsLevel,
		Services:      rec.Services,
		GoalsNotes:    notes,
		PlanStartDate: rec.PlanStartDate,
		PlanEndDate:   rec.PlanEndDate,
		NextReviewDate: rec.NextReview,
		CreatedAt:     rec.CreatedAt,
		UpdatedAt:     rec.UpdatedAt,
	}, nil
}

func scanReferral(s rowScanner) (nascReferralRecord, error) {
	var rec nascReferralRecord
	var status, interraiRef string
	if err := s.Scan(
		&rec.ID, &rec.PatientID, &rec.PatientNHI, &rec.ReferrerHPI, &rec.TenantID,
		&status, &rec.Reason, &rec.UrgencyFlag, &rec.NASCOrgCode,
		&interraiRef, &rec.CompletedAt, &rec.DeclineReason, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return nascReferralRecord{}, err
	}
	rec.Status = NASCReferralStatus(status)
	rec.InterRAIRefID = interraiRef
	return rec, nil
}

func scanServicePlan(s rowScanner) (nascServicePlanRecord, error) {
	var rec nascServicePlanRecord
	var status, needsLevel string
	if err := s.Scan(
		&rec.ID, &rec.PatientID, &rec.PatientNHI, &rec.TenantID, &rec.ReferralID,
		&status, &needsLevel, &rec.Services, &rec.GoalsEnc,
		&rec.PlanStartDate, &rec.PlanEndDate, &rec.NextReview,
		&rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return nascServicePlanRecord{}, err
	}
	rec.Status = ServicePlanStatus(status)
	rec.NeedsLevel = SupportNeedsLevel(needsLevel)
	return rec, nil
}

func referralToResponse(rec nascReferralRecord) NASCReferral {
	return NASCReferral{
		ID:            rec.ID,
		PatientID:     rec.PatientID,
		PatientNHI:    rec.PatientNHI,
		ReferrerHPI:   rec.ReferrerHPI,
		TenantID:      rec.TenantID,
		Status:        rec.Status,
		ReferralReason: rec.Reason,
		UrgencyFlag:   rec.UrgencyFlag,
		NASCOrgCode:   rec.NASCOrgCode,
		InterRAIRefID: rec.InterRAIRefID,
		CompletedAt:   rec.CompletedAt,
		DeclineReason: rec.DeclineReason,
		CreatedAt:     rec.CreatedAt,
		UpdatedAt:     rec.UpdatedAt,
	}
}

func validNeedsLevel(l SupportNeedsLevel) bool {
	switch l {
	case NeedsLevelLow, NeedsLevelModerate, NeedsLevelHigh, NeedsLevelComplex:
		return true
	}
	return false
}
