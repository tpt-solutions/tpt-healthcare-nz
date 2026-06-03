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
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// SpecimenStatus values mirror the NZ pathology workflow.
type SpecimenStatus string

const (
	SpecimenCollected   SpecimenStatus = "collected"
	SpecimenInTransit   SpecimenStatus = "in-transit"
	SpecimenReceived    SpecimenStatus = "received"
	SpecimenProcessing  SpecimenStatus = "processing"
	SpecimenReported    SpecimenStatus = "reported"
	SpecimenDiscarded   SpecimenStatus = "discarded"
)

// Specimen is the domain model for a pathology_specimens row.
type Specimen struct {
	ID              string         `json:"id"`
	TenantID        string         `json:"tenantId"`
	PatientNHI      string         `json:"patientNhi,omitempty"`
	AccessionNumber string         `json:"accessionNumber"`
	CollectionSite  string         `json:"collectionSite,omitempty"`
	CollectedAt     *time.Time     `json:"collectedAt,omitempty"`
	ReceivedAt      *time.Time     `json:"receivedAt,omitempty"`
	Status          SpecimenStatus `json:"status"`
	SpecimenType    string         `json:"specimenType,omitempty"`
	ContainerType   string         `json:"containerType,omitempty"`
	CollectedBy     string         `json:"collectedBy,omitempty"`
	OrderingHPI     string         `json:"orderingHpi,omitempty"`
	NZLLabOrder     string         `json:"nzlLabOrder,omitempty"`
	NZLFundingCode  string         `json:"nzlFundingCode,omitempty"`
	NZLUrgency      string         `json:"nzlUrgency,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

// specimenCreateRequest is the body for POST /api/v1/specimens.
type specimenCreateRequest struct {
	PatientNHI      string `json:"patientNhi"`
	AccessionNumber string `json:"accessionNumber"`
	CollectionSite  string `json:"collectionSite,omitempty"`
	CollectedAt     string `json:"collectedAt,omitempty"` // RFC3339
	SpecimenType    string `json:"specimenType,omitempty"`
	ContainerType   string `json:"containerType,omitempty"`
	CollectedBy     string `json:"collectedBy,omitempty"`
	OrderingHPI     string `json:"orderingHpi,omitempty"`
	NZLLabOrder     string `json:"nzlLabOrder,omitempty"`
	NZLFundingCode  string `json:"nzlFundingCode,omitempty"`
	NZLUrgency      string `json:"nzlUrgency,omitempty"`
}

// specimenStatusRequest is the body for PUT /api/v1/specimens/{id}/status.
type specimenStatusRequest struct {
	Status SpecimenStatus `json:"status"`
}

// SpecimensHandler handles /api/v1/specimens routes.
type SpecimensHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/specimens.
// Supports query params: patient (NHI), status, accession.
func (h *SpecimensHandler) List(w http.ResponseWriter, r *http.Request) {
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
	specimens, err := h.listSpecimens(ctx, tenantID, q.Get("patient"), q.Get("status"), q.Get("accession"))
	if err != nil {
		h.logger.Error("list specimens", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list specimens"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Specimen",
		ResourceID:   "list",
		TenantID:     tenantID,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"specimens": specimens,
		"total":     len(specimens),
	})
}

// Get handles GET /api/v1/specimens/{id}.
func (h *SpecimensHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "specimen ID is required"})
		return
	}

	spec, err := h.getSpecimen(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "specimen not found"})
			return
		}
		h.logger.Error("get specimen", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve specimen"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Specimen",
		ResourceID:   id,
		TenantID:     tenantID,
	})

	writeJSON(w, http.StatusOK, spec)
}

// Create handles POST /api/v1/specimens.
// Used when a specimen is manually registered (e.g. walk-in collection).
func (h *SpecimensHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req specimenCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.AccessionNumber == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "accessionNumber is required"})
		return
	}

	var collectedAt *time.Time
	if req.CollectedAt != "" {
		t, err := time.Parse(time.RFC3339, req.CollectedAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_DATE", Message: "collectedAt must be RFC3339"})
			return
		}
		collectedAt = &t
	}

	spec, err := h.insertSpecimen(ctx, req, collectedAt, tenantID)
	if err != nil {
		h.logger.Error("insert specimen", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create specimen"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Specimen",
		ResourceID:   spec.ID,
		TenantID:     tenantID,
		Metadata:     map[string]string{"accession": req.AccessionNumber},
	})

	writeJSON(w, http.StatusCreated, spec)
}

// UpdateStatus handles PUT /api/v1/specimens/{id}/status.
// Transitions a specimen through the NZ lab workflow states.
func (h *SpecimensHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "specimen ID is required"})
		return
	}

	var req specimenStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !validSpecimenStatus(req.Status) {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code:    "INVALID_STATUS",
			Message: "status must be one of: collected, in-transit, received, processing, reported, discarded",
		})
		return
	}

	spec, err := h.updateSpecimenStatus(ctx, id, req.Status, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "specimen not found"})
			return
		}
		h.logger.Error("update specimen status", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update specimen status"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Specimen",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"new_status": string(req.Status)},
	})

	writeJSON(w, http.StatusOK, spec)
}

// ---------------------------------------------------------------------------
// Data access
// ---------------------------------------------------------------------------

func (h *SpecimensHandler) listSpecimens(
	ctx context.Context,
	tenantID, patientFilter, statusFilter, accessionFilter string,
) ([]Specimen, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, patient_nhi, accession_number, collection_site,
		        collected_at, received_at, status, specimen_type, container_type,
		        collected_by, ordering_hpi, nzl_lab_order, nzl_funding_code,
		        nzl_urgency, created_at, updated_at
		 FROM pathology_specimens
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter   = '' OR patient_nhi      = @patient_filter)
		   AND (@status_filter    = '' OR status           = @status_filter)
		   AND (@accession_filter = '' OR accession_number = @accession_filter)
		 ORDER BY collected_at DESC NULLS LAST, created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":        tenantID,
			"patient_filter":   patientFilter,
			"status_filter":    statusFilter,
			"accession_filter": accessionFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query specimens: %w", err)
	}
	defer rows.Close()

	var results []Specimen
	for rows.Next() {
		var s Specimen
		if err := rows.Scan(
			&s.ID, &s.TenantID, &s.PatientNHI, &s.AccessionNumber, &s.CollectionSite,
			&s.CollectedAt, &s.ReceivedAt, &s.Status, &s.SpecimenType, &s.ContainerType,
			&s.CollectedBy, &s.OrderingHPI, &s.NZLLabOrder, &s.NZLFundingCode,
			&s.NZLUrgency, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan specimen: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

func (h *SpecimensHandler) getSpecimen(ctx context.Context, id, tenantID string) (Specimen, error) {
	var s Specimen
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, patient_nhi, accession_number, collection_site,
		        collected_at, received_at, status, specimen_type, container_type,
		        collected_by, ordering_hpi, nzl_lab_order, nzl_funding_code,
		        nzl_urgency, created_at, updated_at
		 FROM pathology_specimens
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&s.ID, &s.TenantID, &s.PatientNHI, &s.AccessionNumber, &s.CollectionSite,
		&s.CollectedAt, &s.ReceivedAt, &s.Status, &s.SpecimenType, &s.ContainerType,
		&s.CollectedBy, &s.OrderingHPI, &s.NZLLabOrder, &s.NZLFundingCode,
		&s.NZLUrgency, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Specimen{}, errNotFound
		}
		return Specimen{}, fmt.Errorf("get specimen: %w", err)
	}
	return s, nil
}

func (h *SpecimensHandler) insertSpecimen(ctx context.Context, req specimenCreateRequest, collectedAt *time.Time, tenantID string) (Specimen, error) {
	var s Specimen
	err := h.pool.QueryRow(ctx,
		`INSERT INTO pathology_specimens
		   (tenant_id, patient_nhi, accession_number, collection_site, collected_at,
		    status, specimen_type, container_type, collected_by, ordering_hpi,
		    nzl_lab_order, nzl_funding_code, nzl_urgency)
		 VALUES
		   (@tenant_id, @patient_nhi, @accession_number, @collection_site, @collected_at,
		    'collected', @specimen_type, @container_type, @collected_by, @ordering_hpi,
		    @nzl_lab_order, @nzl_funding_code, @nzl_urgency)
		 RETURNING id, tenant_id, patient_nhi, accession_number, collection_site,
		           collected_at, received_at, status, specimen_type, container_type,
		           collected_by, ordering_hpi, nzl_lab_order, nzl_funding_code,
		           nzl_urgency, created_at, updated_at`,
		db.NamedArgs{
			"tenant_id":        tenantID,
			"patient_nhi":      req.PatientNHI,
			"accession_number": req.AccessionNumber,
			"collection_site":  req.CollectionSite,
			"collected_at":     collectedAt,
			"specimen_type":    req.SpecimenType,
			"container_type":   req.ContainerType,
			"collected_by":     req.CollectedBy,
			"ordering_hpi":     req.OrderingHPI,
			"nzl_lab_order":    req.NZLLabOrder,
			"nzl_funding_code": req.NZLFundingCode,
			"nzl_urgency":      req.NZLUrgency,
		},
	).Scan(
		&s.ID, &s.TenantID, &s.PatientNHI, &s.AccessionNumber, &s.CollectionSite,
		&s.CollectedAt, &s.ReceivedAt, &s.Status, &s.SpecimenType, &s.ContainerType,
		&s.CollectedBy, &s.OrderingHPI, &s.NZLLabOrder, &s.NZLFundingCode,
		&s.NZLUrgency, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return Specimen{}, fmt.Errorf("insert specimen: %w", err)
	}
	return s, nil
}

func (h *SpecimensHandler) updateSpecimenStatus(ctx context.Context, id string, status SpecimenStatus, tenantID string) (Specimen, error) {
	var s Specimen

	// Set received_at when the lab receives the specimen.
	var setReceived string
	if status == SpecimenReceived {
		setReceived = ", received_at = now()"
	}

	err := h.pool.QueryRow(ctx,
		`UPDATE pathology_specimens
		 SET status = @status, updated_at = now()`+setReceived+`
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, tenant_id, patient_nhi, accession_number, collection_site,
		           collected_at, received_at, status, specimen_type, container_type,
		           collected_by, ordering_hpi, nzl_lab_order, nzl_funding_code,
		           nzl_urgency, created_at, updated_at`,
		db.NamedArgs{
			"status":    string(status),
			"id":        id,
			"tenant_id": tenantID,
		},
	).Scan(
		&s.ID, &s.TenantID, &s.PatientNHI, &s.AccessionNumber, &s.CollectionSite,
		&s.CollectedAt, &s.ReceivedAt, &s.Status, &s.SpecimenType, &s.ContainerType,
		&s.CollectedBy, &s.OrderingHPI, &s.NZLLabOrder, &s.NZLFundingCode,
		&s.NZLUrgency, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Specimen{}, errNotFound
		}
		return Specimen{}, fmt.Errorf("update specimen status: %w", err)
	}
	return s, nil
}

func validSpecimenStatus(s SpecimenStatus) bool {
	switch s {
	case SpecimenCollected, SpecimenInTransit, SpecimenReceived,
		SpecimenProcessing, SpecimenReported, SpecimenDiscarded:
		return true
	}
	return false
}
