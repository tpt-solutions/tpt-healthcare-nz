package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// ReferralPriority mirrors the FHIR ServiceRequest priority value set.
type ReferralPriority string

const (
	ReferralPriorityRoutine ReferralPriority = "routine"
	ReferralPriorityUrgent  ReferralPriority = "urgent"
	ReferralPriorityASAP    ReferralPriority = "asap"
	ReferralPriorityStat    ReferralPriority = "stat"
)

// ReferralStatus mirrors the FHIR ServiceRequest status value set.
type ReferralStatus string

const (
	ReferralStatusDraft     ReferralStatus = "draft"
	ReferralStatusActive    ReferralStatus = "active"
	ReferralStatusCompleted ReferralStatus = "completed"
	ReferralStatusRevoked   ReferralStatus = "revoked"
)

// Referral is the domain model for a specialist referral (FHIR ServiceRequest).
type Referral struct {
	ID            string           `json:"id"`
	PatientID     string           `json:"patientId"`
	PatientNHI    string           `json:"patientNhi"`
	ReferringHPI  string           `json:"referringHpi"`
	SpecialtyCode string           `json:"specialtyCode"`
	ServiceType   string           `json:"serviceType"`
	Priority      ReferralPriority `json:"priority"`
	Reason        string           `json:"reason"`
	ClinicalNotes string           `json:"clinicalNotes,omitempty"`
	EncounterID   string           `json:"encounterId,omitempty"`
	Status        ReferralStatus   `json:"status"`
	TenantID      string           `json:"tenantId"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
	SentAt        *time.Time       `json:"sentAt,omitempty"`
}

// referralCreateRequest is the body for POST /api/v1/referrals.
type referralCreateRequest struct {
	PatientID     string           `json:"patientId"`
	PatientNHI    string           `json:"patientNhi"`
	ReferringHPI  string           `json:"referringHpi"`
	SpecialtyCode string           `json:"specialtyCode"`
	ServiceType   string           `json:"serviceType"`
	Priority      ReferralPriority `json:"priority"`
	Reason        string           `json:"reason"`
	ClinicalNotes string           `json:"clinicalNotes,omitempty"`
	EncounterID   string           `json:"encounterId,omitempty"`
}

// referralUpdateRequest is the body for PUT /api/v1/referrals/{id}.
type referralUpdateRequest struct {
	Priority      ReferralPriority `json:"priority,omitempty"`
	Reason        string           `json:"reason,omitempty"`
	ClinicalNotes string           `json:"clinicalNotes,omitempty"`
	Status        ReferralStatus   `json:"status,omitempty"`
}

// ReferralsHandler handles all /api/v1/referrals routes.
type ReferralsHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/referrals.
// Supports query parameters: patient (internal ID), status, provider (HPI CPN).
func (h *ReferralsHandler) List(w http.ResponseWriter, r *http.Request) {
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

	q := r.URL.Query()
	patientFilter := q.Get("patient")
	statusFilter := q.Get("status")
	providerFilter := q.Get("provider")

	referrals, err := h.listReferrals(ctx, tenantID, patientFilter, statusFilter, providerFilter)
	if err != nil {
		h.logger.Error("list referrals", slog.Any("error", err), slog.String("tenant", tenantID))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list referrals"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "ServiceRequest",
		ResourceID:   "list",
		TenantID:     tenantID,
		Metadata:     map[string]string{"patient": patientFilter, "status": statusFilter},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"referrals": referrals,
		"total":     len(referrals),
	})
}

// Create handles POST /api/v1/referrals.
// Creates a new specialist referral (FHIR ServiceRequest) in draft status.
func (h *ReferralsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req referralCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if err := validateReferralCreate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	// HPCA requirement: validate the referring practitioner holds a current APC.
	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.ReferringHPI)
	if err != nil {
		h.logger.Error("HPI APC validation for referral", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate practitioner APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "INVALID_APC",
			Message: "referring practitioner does not have a current Annual Practising Certificate",
			Details: apcStatus,
		})
		return
	}

	referral, err := h.insertReferral(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert referral", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create referral"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "ServiceRequest",
		ResourceID:   referral.ID,
		TenantID:     tenantID,
		Metadata:     map[string]string{"specialty": req.SpecialtyCode, "priority": string(req.Priority)},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, referral)
}

// Get handles GET /api/v1/referrals/{id}.
func (h *ReferralsHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "referral ID is required"})
		return
	}

	referral, err := h.getReferralByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "referral not found"})
			return
		}
		h.logger.Error("get referral", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve referral"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "ServiceRequest",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, referral)
}

// Update handles PUT /api/v1/referrals/{id}.
// Allows updating priority, reason, clinical notes, and status before sending.
func (h *ReferralsHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "referral ID is required"})
		return
	}

	var req referralUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	referral, err := h.getReferralByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "referral not found"})
			return
		}
		h.logger.Error("get referral for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve referral"})
		return
	}

	if referral.Status == ReferralStatusCompleted || referral.Status == ReferralStatusRevoked {
		writeJSON(w, http.StatusConflict, apiError{
			Code:    "IMMUTABLE_STATUS",
			Message: fmt.Sprintf("referral in %s status cannot be modified", referral.Status),
		})
		return
	}

	updated, err := h.updateReferral(ctx, id, req, tenantID)
	if err != nil {
		h.logger.Error("update referral", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update referral"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "ServiceRequest",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "update"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// Send handles POST /api/v1/referrals/{id}/send.
// Transitions a draft referral to active, recording the sent timestamp.
// In production, this would dispatch the referral to the receiving provider's inbox.
func (h *ReferralsHandler) Send(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "referral ID is required"})
		return
	}

	referral, err := h.getReferralByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "referral not found"})
			return
		}
		h.logger.Error("get referral for send", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve referral"})
		return
	}

	if referral.Status != ReferralStatusDraft {
		writeJSON(w, http.StatusConflict, apiError{
			Code:    "ALREADY_SENT",
			Message: fmt.Sprintf("referral is already in %s status", referral.Status),
		})
		return
	}

	now := time.Now().UTC()
	sent, err := h.markReferralSent(ctx, id, now, tenantID)
	if err != nil {
		h.logger.Error("mark referral sent", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SEND_ERROR", Message: "failed to send referral"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "ServiceRequest",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "send", "specialty": referral.SpecialtyCode},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, sent)
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func validateReferralCreate(req *referralCreateRequest) error {
	if req.PatientID == "" && req.PatientNHI == "" {
		return fmt.Errorf("either patientId or patientNhi is required")
	}
	if req.ReferringHPI == "" {
		return fmt.Errorf("referringHpi is required")
	}
	if req.SpecialtyCode == "" {
		return fmt.Errorf("specialtyCode is required")
	}
	if req.Reason == "" {
		return fmt.Errorf("reason is required")
	}
	if req.Priority == "" {
		req.Priority = ReferralPriorityRoutine
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data access
// ---------------------------------------------------------------------------

func (h *ReferralsHandler) listReferrals(
	ctx context.Context,
	tenantID, patientFilter, statusFilter, providerFilter string,
) ([]Referral, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, referring_hpi,
		        specialty_code, service_type, priority, reason,
		        clinical_notes, encounter_id, status,
		        tenant_id, created_at, updated_at, sent_at
		 FROM referrals
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter  = '' OR patient_id    = @patient_filter)
		   AND (@status_filter   = '' OR status        = @status_filter)
		   AND (@provider_filter = '' OR referring_hpi = @provider_filter)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":       tenantID,
			"patient_filter":  patientFilter,
			"status_filter":   statusFilter,
			"provider_filter": providerFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query referrals: %w", err)
	}
	defer rows.Close()

	var results []Referral
	for rows.Next() {
		var ref Referral
		if err := rows.Scan(
			&ref.ID, &ref.PatientID, &ref.PatientNHI, &ref.ReferringHPI,
			&ref.SpecialtyCode, &ref.ServiceType, &ref.Priority, &ref.Reason,
			&ref.ClinicalNotes, &ref.EncounterID, &ref.Status,
			&ref.TenantID, &ref.CreatedAt, &ref.UpdatedAt, &ref.SentAt,
		); err != nil {
			return nil, fmt.Errorf("scan referral: %w", err)
		}
		results = append(results, ref)
	}
	return results, rows.Err()
}

func (h *ReferralsHandler) getReferralByID(ctx context.Context, id, tenantID string) (Referral, error) {
	var ref Referral
	err := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, referring_hpi,
		        specialty_code, service_type, priority, reason,
		        clinical_notes, encounter_id, status,
		        tenant_id, created_at, updated_at, sent_at
		 FROM referrals
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&ref.ID, &ref.PatientID, &ref.PatientNHI, &ref.ReferringHPI,
		&ref.SpecialtyCode, &ref.ServiceType, &ref.Priority, &ref.Reason,
		&ref.ClinicalNotes, &ref.EncounterID, &ref.Status,
		&ref.TenantID, &ref.CreatedAt, &ref.UpdatedAt, &ref.SentAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Referral{}, errNotFound
		}
		return Referral{}, fmt.Errorf("get referral: %w", err)
	}
	return ref, nil
}

func (h *ReferralsHandler) insertReferral(ctx context.Context, req referralCreateRequest, tenantID string) (Referral, error) {
	var ref Referral
	err := h.pool.QueryRow(ctx,
		`INSERT INTO referrals
		   (patient_id, patient_nhi, referring_hpi, specialty_code, service_type,
		    priority, reason, clinical_notes, encounter_id, status, tenant_id)
		 VALUES
		   (@patient_id, @patient_nhi, @referring_hpi, @specialty_code, @service_type,
		    @priority, @reason, @clinical_notes, @encounter_id, @status, @tenant_id)
		 RETURNING id, patient_id, patient_nhi, referring_hpi,
		           specialty_code, service_type, priority, reason,
		           clinical_notes, encounter_id, status,
		           tenant_id, created_at, updated_at, sent_at`,
		db.NamedArgs{
			"patient_id":     req.PatientID,
			"patient_nhi":    req.PatientNHI,
			"referring_hpi":  req.ReferringHPI,
			"specialty_code": req.SpecialtyCode,
			"service_type":   req.ServiceType,
			"priority":       req.Priority,
			"reason":         req.Reason,
			"clinical_notes": req.ClinicalNotes,
			"encounter_id":   req.EncounterID,
			"status":         ReferralStatusDraft,
			"tenant_id":      tenantID,
		},
	).Scan(
		&ref.ID, &ref.PatientID, &ref.PatientNHI, &ref.ReferringHPI,
		&ref.SpecialtyCode, &ref.ServiceType, &ref.Priority, &ref.Reason,
		&ref.ClinicalNotes, &ref.EncounterID, &ref.Status,
		&ref.TenantID, &ref.CreatedAt, &ref.UpdatedAt, &ref.SentAt,
	)
	if err != nil {
		return Referral{}, fmt.Errorf("insert referral: %w", err)
	}
	return ref, nil
}

func (h *ReferralsHandler) updateReferral(ctx context.Context, id string, req referralUpdateRequest, tenantID string) (Referral, error) {
	var ref Referral
	err := h.pool.QueryRow(ctx,
		`UPDATE referrals
		 SET priority       = COALESCE(NULLIF(@priority, ''), priority),
		     reason         = COALESCE(NULLIF(@reason, ''), reason),
		     clinical_notes = COALESCE(NULLIF(@clinical_notes, ''), clinical_notes),
		     status         = COALESCE(NULLIF(@status, ''), status),
		     updated_at     = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, referring_hpi,
		           specialty_code, service_type, priority, reason,
		           clinical_notes, encounter_id, status,
		           tenant_id, created_at, updated_at, sent_at`,
		db.NamedArgs{
			"priority":       string(req.Priority),
			"reason":         req.Reason,
			"clinical_notes": req.ClinicalNotes,
			"status":         string(req.Status),
			"id":             id,
			"tenant_id":      tenantID,
		},
	).Scan(
		&ref.ID, &ref.PatientID, &ref.PatientNHI, &ref.ReferringHPI,
		&ref.SpecialtyCode, &ref.ServiceType, &ref.Priority, &ref.Reason,
		&ref.ClinicalNotes, &ref.EncounterID, &ref.Status,
		&ref.TenantID, &ref.CreatedAt, &ref.UpdatedAt, &ref.SentAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Referral{}, errNotFound
		}
		return Referral{}, fmt.Errorf("update referral: %w", err)
	}
	return ref, nil
}

func (h *ReferralsHandler) markReferralSent(ctx context.Context, id string, sentAt time.Time, tenantID string) (Referral, error) {
	var ref Referral
	err := h.pool.QueryRow(ctx,
		`UPDATE referrals
		 SET status     = @status,
		     sent_at    = @sent_at,
		     updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, referring_hpi,
		           specialty_code, service_type, priority, reason,
		           clinical_notes, encounter_id, status,
		           tenant_id, created_at, updated_at, sent_at`,
		db.NamedArgs{
			"status":    ReferralStatusActive,
			"sent_at":   sentAt,
			"id":        id,
			"tenant_id": tenantID,
		},
	).Scan(
		&ref.ID, &ref.PatientID, &ref.PatientNHI, &ref.ReferringHPI,
		&ref.SpecialtyCode, &ref.ServiceType, &ref.Priority, &ref.Reason,
		&ref.ClinicalNotes, &ref.EncounterID, &ref.Status,
		&ref.TenantID, &ref.CreatedAt, &ref.UpdatedAt, &ref.SentAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Referral{}, errNotFound
		}
		return Referral{}, fmt.Errorf("mark referral sent: %w", err)
	}
	return ref, nil
}
