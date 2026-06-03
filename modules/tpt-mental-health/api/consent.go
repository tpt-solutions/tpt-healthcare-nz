// Package api — enhanced mental health consent model.
//
// Mental health records are extra-sensitive under the Health Information Privacy
// Code (HIPC). The standard core/consent package handles general health data.
// This handler manages the elevated mh_consents table, which supports:
//
//   - access:         individual access to their own MH records
//   - disclosure:     disclosure to a named third party (requires conditions + role)
//   - research:       use in anonymised / approved research
//   - family-sharing: sharing with nominated family/whānau under HIPC Rule 11
//   - cto-related:    disclosures required by or related to a compulsory order
//
// All consent records carry extra_sensitive = true and are checked before any
// third-party disclosure is made from this service.
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
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MHConsentType classifies the purpose of a mental health consent record.
type MHConsentType string

const (
	// MHConsentAccess covers the patient's right to access their own MH records.
	MHConsentAccess MHConsentType = "access"
	// MHConsentDisclosure covers disclosure to a named third party.
	MHConsentDisclosure MHConsentType = "disclosure"
	// MHConsentResearch covers anonymised research use.
	MHConsentResearch MHConsentType = "research"
	// MHConsentFamilySharing covers sharing with nominated family/whānau.
	MHConsentFamilySharing MHConsentType = "family-sharing"
	// MHConsentCTORelated covers disclosures required by a compulsory order.
	MHConsentCTORelated MHConsentType = "cto-related"
)

// MHConsent represents a single mental health consent record.
type MHConsent struct {
	ID              string        `json:"id"`
	PatientID       string        `json:"patientId"`
	PatientNHI      string        `json:"patientNhi"`
	TenantID        string        `json:"tenantId"`
	ConsentType     MHConsentType `json:"consentType"`
	Granted         bool          `json:"granted"`
	Purpose         string        `json:"purpose"`
	GrantedBy       string        `json:"grantedBy"` // practitioner HPI or patient NHI
	GrantedAt       time.Time     `json:"grantedAt"`
	ExpiresAt       *time.Time    `json:"expiresAt,omitempty"`
	RevokedAt       *time.Time    `json:"revokedAt,omitempty"`
	ThirdPartyID    string        `json:"thirdPartyId,omitempty"`
	ThirdPartyRole  string        `json:"thirdPartyRole,omitempty"` // "family","employer","insurer","court"
	Conditions      string        `json:"conditions,omitempty"`
	EvidenceRef     string        `json:"evidenceRef,omitempty"`
	ExtraSensitive  bool          `json:"extraSensitive"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

// mhConsentGrantRequest is the body for POST /api/v1/consents.
type mhConsentGrantRequest struct {
	PatientID      string        `json:"patientId"`
	PatientNHI     string        `json:"patientNhi"`
	ConsentType    MHConsentType `json:"consentType"`
	Granted        bool          `json:"granted"`
	Purpose        string        `json:"purpose"`
	GrantedBy      string        `json:"grantedBy"`
	ExpiresAt      *time.Time    `json:"expiresAt,omitempty"`
	ThirdPartyID   string        `json:"thirdPartyId,omitempty"`
	ThirdPartyRole string        `json:"thirdPartyRole,omitempty"`
	Conditions     string        `json:"conditions,omitempty"`
	EvidenceRef    string        `json:"evidenceRef,omitempty"`
}

// MHConsentHandler handles all /api/v1/consents routes for mental health.
type MHConsentHandler struct {
	pool       *pgxpool.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/consents.
// Query params: patient (internal ID), nhi, type.
func (h *MHConsentHandler) List(w http.ResponseWriter, r *http.Request) {
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

	// Consent list requires MH clinician role — never filtered by access consent itself.
	if !hasMHRole(principal) {
		writeJSON(w, http.StatusForbidden, apiError{Code: "FORBIDDEN", Message: "mental-health-clinician role is required"})
		return
	}

	q := r.URL.Query()
	patientFilter := q.Get("patient")
	nhiFilter := q.Get("nhi")
	typeFilter := q.Get("type")

	consents, err := h.listConsents(ctx, tenantID, patientFilter, nhiFilter, typeFilter)
	if err != nil {
		h.logger.Error("list consents", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list consents"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MentalHealthConsent",
		ResourceID:   "list",
	})

	writeJSON(w, http.StatusOK, map[string]any{"consents": consents, "total": len(consents)})
}

// Get handles GET /api/v1/consents/{id}.
func (h *MHConsentHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	if !hasMHRole(principal) {
		writeJSON(w, http.StatusForbidden, apiError{Code: "FORBIDDEN", Message: "mental-health-clinician role is required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "consent ID is required"})
		return
	}

	consent, err := h.getConsentByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "consent not found"})
			return
		}
		h.logger.Error("get consent", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve consent"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MentalHealthConsent",
		ResourceID:   id,
		PatientNHI:   consent.PatientNHI,
	})

	writeJSON(w, http.StatusOK, consent)
}

// Grant handles POST /api/v1/consents.
// Records a new consent grant or refusal for the named purpose and type.
func (h *MHConsentHandler) Grant(w http.ResponseWriter, r *http.Request) {
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

	if !hasMHRole(principal) {
		writeJSON(w, http.StatusForbidden, apiError{Code: "FORBIDDEN", Message: "mental-health-clinician role is required to record consent"})
		return
	}

	var req mhConsentGrantRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patientId or patientNhi is required"})
		return
	}
	if !validMHConsentType(req.ConsentType) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_TYPE", Message: fmt.Sprintf("unknown consent type %q", req.ConsentType)})
		return
	}
	if req.Purpose == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PURPOSE", Message: "purpose is required"})
		return
	}
	if req.GrantedBy == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_GRANTED_BY", Message: "grantedBy is required"})
		return
	}

	// Disclosure consent requires a named third party and their role.
	if req.ConsentType == MHConsentDisclosure {
		if req.ThirdPartyID == "" {
			writeJSON(w, http.StatusBadRequest, apiError{
				Code:    "MISSING_THIRD_PARTY",
				Message: "thirdPartyId is required for disclosure consent",
			})
			return
		}
		if req.ThirdPartyRole == "" {
			writeJSON(w, http.StatusBadRequest, apiError{
				Code:    "MISSING_THIRD_PARTY_ROLE",
				Message: "thirdPartyRole is required for disclosure consent",
			})
			return
		}
	}

	consent, err := h.insertConsent(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert consent", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to record consent"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "MentalHealthConsent",
		ResourceID:   consent.ID,
		PatientNHI:   req.PatientNHI,
		Details: map[string]any{
			"type":    string(req.ConsentType),
			"granted": req.Granted,
		},
	})

	writeJSON(w, http.StatusCreated, consent)
}

// Revoke handles POST /api/v1/consents/{id}/revoke.
// Marks the consent as revoked at the current time.
func (h *MHConsentHandler) Revoke(w http.ResponseWriter, r *http.Request) {
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

	if !hasMHRole(principal) {
		writeJSON(w, http.StatusForbidden, apiError{Code: "FORBIDDEN", Message: "mental-health-clinician role is required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "consent ID is required"})
		return
	}

	existing, err := h.getConsentByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "consent not found"})
			return
		}
		h.logger.Error("get consent for revoke", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve consent"})
		return
	}

	if existing.RevokedAt != nil {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_REVOKED", Message: "consent is already revoked"})
		return
	}

	revoked, err := h.revokeConsent(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "consent not found"})
			return
		}
		h.logger.Error("revoke consent", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "REVOKE_ERROR", Message: "failed to revoke consent"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "MentalHealthConsent",
		ResourceID:   id,
		PatientNHI:   revoked.PatientNHI,
		Details:      map[string]any{"action": "revoke"},
	})

	writeJSON(w, http.StatusOK, revoked)
}

// ---------------------------------------------------------------------------
// Database helpers
// ---------------------------------------------------------------------------

func (h *MHConsentHandler) listConsents(
	ctx context.Context,
	tenantID uuid.UUID,
	patientFilter, nhiFilter, typeFilter string,
) ([]MHConsent, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id,
		        consent_type, granted, purpose, granted_by, granted_at,
		        expires_at, revoked_at, third_party_id, third_party_role,
		        conditions, evidence_ref, extra_sensitive, created_at, updated_at
		 FROM mh_consents
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR patient_nhi = $3)
		   AND ($4 = '' OR consent_type = $4)
		 ORDER BY granted_at DESC
		 LIMIT 200`,
		tenantID, patientFilter, nhiFilter, typeFilter,
	)
	if err != nil {
		return nil, fmt.Errorf("query consents: %w", err)
	}
	defer rows.Close()

	var results []MHConsent
	for rows.Next() {
		c, err := scanConsent(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

func (h *MHConsentHandler) getConsentByID(ctx context.Context, id string, tenantID uuid.UUID) (MHConsent, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id,
		        consent_type, granted, purpose, granted_by, granted_at,
		        expires_at, revoked_at, third_party_id, third_party_role,
		        conditions, evidence_ref, extra_sensitive, created_at, updated_at
		 FROM mh_consents
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	c, err := scanConsentRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MHConsent{}, errNotFound
		}
		return MHConsent{}, fmt.Errorf("get consent by id: %w", err)
	}
	return c, nil
}

func (h *MHConsentHandler) insertConsent(ctx context.Context, req mhConsentGrantRequest, tenantID uuid.UUID) (MHConsent, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO mh_consents
		   (patient_id, patient_nhi, tenant_id, consent_type, granted,
		    purpose, granted_by, granted_at, expires_at,
		    third_party_id, third_party_role, conditions, evidence_ref, extra_sensitive)
		 VALUES
		   ($1, $2, $3, $4, $5, $6, $7, now(), $8, $9, $10, $11, $12, TRUE)
		 RETURNING id, patient_id, patient_nhi, tenant_id,
		           consent_type, granted, purpose, granted_by, granted_at,
		           expires_at, revoked_at, third_party_id, third_party_role,
		           conditions, evidence_ref, extra_sensitive, created_at, updated_at`,
		req.PatientID, req.PatientNHI, tenantID,
		string(req.ConsentType), req.Granted,
		req.Purpose, req.GrantedBy, req.ExpiresAt,
		req.ThirdPartyID, req.ThirdPartyRole, req.Conditions, req.EvidenceRef,
	)
	return scanConsentRow(row)
}

func (h *MHConsentHandler) revokeConsent(ctx context.Context, id string, tenantID uuid.UUID) (MHConsent, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE mh_consents
		 SET revoked_at = now(),
		     updated_at = now()
		 WHERE id = $1 AND tenant_id = $2 AND revoked_at IS NULL
		 RETURNING id, patient_id, patient_nhi, tenant_id,
		           consent_type, granted, purpose, granted_by, granted_at,
		           expires_at, revoked_at, third_party_id, third_party_role,
		           conditions, evidence_ref, extra_sensitive, created_at, updated_at`,
		id, tenantID,
	)
	c, err := scanConsentRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MHConsent{}, errNotFound
		}
		return MHConsent{}, fmt.Errorf("revoke consent: %w", err)
	}
	return c, nil
}

func scanConsent(s rowScanner) (MHConsent, error) {
	return scanConsentRow(s)
}

func scanConsentRow(s rowScanner) (MHConsent, error) {
	var c MHConsent
	var ct string
	if err := s.Scan(
		&c.ID, &c.PatientID, &c.PatientNHI, &c.TenantID,
		&ct, &c.Granted, &c.Purpose, &c.GrantedBy, &c.GrantedAt,
		&c.ExpiresAt, &c.RevokedAt, &c.ThirdPartyID, &c.ThirdPartyRole,
		&c.Conditions, &c.EvidenceRef, &c.ExtraSensitive, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return MHConsent{}, err
	}
	c.ConsentType = MHConsentType(ct)
	return c, nil
}

// hasMHRole reports whether the principal holds a role that permits managing
// mental health consents.
func hasMHRole(p *auth.Principal) bool {
	for _, role := range p.Roles {
		if role == "mental-health-clinician" || role == "admin" {
			return true
		}
	}
	return false
}

func validMHConsentType(t MHConsentType) bool {
	switch t {
	case MHConsentAccess, MHConsentDisclosure, MHConsentResearch,
		MHConsentFamilySharing, MHConsentCTORelated:
		return true
	}
	return false
}

// CheckDisclosureConsent reports whether an active disclosure consent exists
// for the given patient, tenant, and third-party recipient.
// Called by any handler that needs to disclose MH information outside the
// care team (e.g. sharing records with family or insurers).
func CheckDisclosureConsent(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, patientNHI, thirdPartyID string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM mh_consents
		     WHERE tenant_id      = $1
		       AND patient_nhi    = $2
		       AND consent_type   = 'disclosure'
		       AND third_party_id = $3
		       AND granted        = TRUE
		       AND revoked_at     IS NULL
		       AND (expires_at IS NULL OR expires_at > now())
		 )`,
		tenantID, patientNHI, thirdPartyID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check disclosure consent: %w", err)
	}
	return exists, nil
}
