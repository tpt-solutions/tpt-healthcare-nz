package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

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

	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.ReferrerHPI)
	if err != nil {
		h.logger.Error("HPI APC check", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not verify practitioner APC"})
		return
	}
	if !apcStatus.Valid {
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
