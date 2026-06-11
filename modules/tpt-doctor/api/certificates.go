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

// CertificateType enumerates the medical certificates a GP can issue.
type CertificateType string

const (
	CertTypeSickLeave    CertificateType = "sick-leave"      // Employee sick leave / Work and Income
	CertTypeFitToWork    CertificateType = "fit-to-work"     // Return-to-work fitness
	CertTypeACC          CertificateType = "acc"             // ACC45/ACC6 medical certificate
	CertTypeImmunisation CertificateType = "immunisation"    // Vaccination proof
	CertTypePreEmploy    CertificateType = "pre-employment"  // Pre-employment medical
	CertTypeDeath        CertificateType = "death"           // Death certificate (MCCD)
)

// Certificate is a medical certificate issued during or following an encounter.
type Certificate struct {
	ID          string          `json:"id"`
	Type        CertificateType `json:"type"`
	PatientID   string          `json:"patientId"`
	PatientNHI  string          `json:"patientNhi"`
	IssuingHPI  string          `json:"issuingHpi"`
	EncounterID string          `json:"encounterId,omitempty"`
	FromDate    string          `json:"fromDate,omitempty"` // YYYY-MM-DD — start of incapacity / fitness
	ToDate      string          `json:"toDate,omitempty"`   // YYYY-MM-DD — end of incapacity
	Diagnosis   string          `json:"diagnosis,omitempty"` // Free text or ICD-10-AM code
	Notes       string          `json:"notes,omitempty"`
	IssuedAt    time.Time       `json:"issuedAt"`
	TenantID    string          `json:"tenantId"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// certificateCreateRequest is the body for POST /api/v1/certificates.
type certificateCreateRequest struct {
	Type        CertificateType `json:"type"`
	PatientID   string          `json:"patientId"`
	PatientNHI  string          `json:"patientNhi"`
	IssuingHPI  string          `json:"issuingHpi"`
	EncounterID string          `json:"encounterId,omitempty"`
	FromDate    string          `json:"fromDate,omitempty"`
	ToDate      string          `json:"toDate,omitempty"`
	Diagnosis   string          `json:"diagnosis,omitempty"`
	Notes       string          `json:"notes,omitempty"`
}

// CertificatesHandler handles all /api/v1/certificates routes.
type CertificatesHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/certificates.
// Supports query params: patient, type.
func (h *CertificatesHandler) List(w http.ResponseWriter, r *http.Request) {
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
	typeFilter := q.Get("type")

	certs, err := h.listCertificates(ctx, tenantID.String(), patientFilter, typeFilter)
	if err != nil {
		h.logger.Error("list certificates", slog.Any("error", err), slog.String("tenant", tenantID.String()))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list certificates"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "DocumentReference",
		ResourceID:   "list",
		TenantID:     tenantID,
		Details:      map[string]any{"patient": patientFilter, "type": typeFilter},
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"certificates": certs,
		"total":        len(certs),
	})
}

// Create handles POST /api/v1/certificates.
// Issues a new medical certificate, recording the issuing practitioner's HPI CPN.
func (h *CertificatesHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req certificateCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if err := validateCertificateCreate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	// HPCA requirement: validate the issuing practitioner holds a current APC.
	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.IssuingHPI)
	if err != nil {
		h.logger.Error("HPI APC validation for certificate", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate practitioner APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "INVALID_APC",
			Message: "issuing practitioner does not have a current Annual Practising Certificate",
			Details: apcStatus,
		})
		return
	}

	cert, err := h.insertCertificate(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert certificate", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to issue certificate"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "DocumentReference",
		ResourceID:   cert.ID,
		TenantID:     tenantID,
		Details:      map[string]any{"cert_type": string(req.Type)},
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusCreated, cert)
}

// Get handles GET /api/v1/certificates/{id}.
func (h *CertificatesHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "certificate ID is required"})
		return
	}

	cert, err := h.getCertByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "certificate not found"})
			return
		}
		h.logger.Error("get certificate", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve certificate"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "DocumentReference",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, cert)
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

var validCertTypes = map[CertificateType]bool{
	CertTypeSickLeave:    true,
	CertTypeFitToWork:    true,
	CertTypeACC:          true,
	CertTypeImmunisation: true,
	CertTypePreEmploy:    true,
	CertTypeDeath:        true,
}

func validateCertificateCreate(req *certificateCreateRequest) error {
	if req.Type == "" {
		return fmt.Errorf("type is required")
	}
	if !validCertTypes[req.Type] {
		return fmt.Errorf("invalid certificate type %q", req.Type)
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		return fmt.Errorf("either patientId or patientNhi is required")
	}
	if req.IssuingHPI == "" {
		return fmt.Errorf("issuingHpi is required")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data access
// ---------------------------------------------------------------------------

func (h *CertificatesHandler) listCertificates(
	ctx context.Context,
	tenantID, patientFilter, typeFilter string,
) ([]Certificate, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, type, patient_id, patient_nhi, issuing_hpi,
		        encounter_id, from_date, to_date, diagnosis, notes,
		        issued_at, tenant_id, created_at, updated_at
		 FROM certificates
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter = '' OR patient_id = @patient_filter)
		   AND (@type_filter    = '' OR type       = @type_filter)
		 ORDER BY issued_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":      tenantID,
			"patient_filter": patientFilter,
			"type_filter":    typeFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query certificates: %w", err)
	}
	defer rows.Close()

	var results []Certificate
	for rows.Next() {
		var c Certificate
		if err := rows.Scan(
			&c.ID, &c.Type, &c.PatientID, &c.PatientNHI, &c.IssuingHPI,
			&c.EncounterID, &c.FromDate, &c.ToDate, &c.Diagnosis, &c.Notes,
			&c.IssuedAt, &c.TenantID, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan certificate: %w", err)
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

func (h *CertificatesHandler) getCertByID(ctx context.Context, id, tenantID string) (Certificate, error) {
	var c Certificate
	err := h.pool.QueryRow(ctx,
		`SELECT id, type, patient_id, patient_nhi, issuing_hpi,
		        encounter_id, from_date, to_date, diagnosis, notes,
		        issued_at, tenant_id, created_at, updated_at
		 FROM certificates
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&c.ID, &c.Type, &c.PatientID, &c.PatientNHI, &c.IssuingHPI,
		&c.EncounterID, &c.FromDate, &c.ToDate, &c.Diagnosis, &c.Notes,
		&c.IssuedAt, &c.TenantID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Certificate{}, errNotFound
		}
		return Certificate{}, fmt.Errorf("get certificate: %w", err)
	}
	return c, nil
}

func (h *CertificatesHandler) insertCertificate(ctx context.Context, req certificateCreateRequest, tenantID string) (Certificate, error) {
	var c Certificate
	err := h.pool.QueryRow(ctx,
		`INSERT INTO certificates
		   (type, patient_id, patient_nhi, issuing_hpi, encounter_id,
		    from_date, to_date, diagnosis, notes, issued_at, tenant_id)
		 VALUES
		   (@type, @patient_id, @patient_nhi, @issuing_hpi, @encounter_id,
		    @from_date, @to_date, @diagnosis, @notes, now(), @tenant_id)
		 RETURNING id, type, patient_id, patient_nhi, issuing_hpi,
		           encounter_id, from_date, to_date, diagnosis, notes,
		           issued_at, tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"type":        req.Type,
			"patient_id":  req.PatientID,
			"patient_nhi": req.PatientNHI,
			"issuing_hpi": req.IssuingHPI,
			"encounter_id": req.EncounterID,
			"from_date":   req.FromDate,
			"to_date":     req.ToDate,
			"diagnosis":   req.Diagnosis,
			"notes":       req.Notes,
			"tenant_id":   tenantID,
		},
	).Scan(
		&c.ID, &c.Type, &c.PatientID, &c.PatientNHI, &c.IssuingHPI,
		&c.EncounterID, &c.FromDate, &c.ToDate, &c.Diagnosis, &c.Notes,
		&c.IssuedAt, &c.TenantID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return Certificate{}, fmt.Errorf("insert certificate: %w", err)
	}
	return c, nil
}
