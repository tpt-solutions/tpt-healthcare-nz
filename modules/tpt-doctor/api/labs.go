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

// LabOrderStatus mirrors the FHIR DiagnosticReport status value set.
type LabOrderStatus string

const (
	LabOrderStatusDraft      LabOrderStatus = "draft"
	LabOrderStatusRequested  LabOrderStatus = "requested"
	LabOrderStatusReceived   LabOrderStatus = "received"
	LabOrderStatusRegistered LabOrderStatus = "registered"
	LabOrderStatusPartial    LabOrderStatus = "partial"
	LabOrderStatusFinal      LabOrderStatus = "final"
	LabOrderStatusCancelled  LabOrderStatus = "cancelled"
)

// LabOrder is the domain model for a laboratory order and its results.
// It maps to a pair of FHIR resources: ServiceRequest (order) and
// DiagnosticReport (results).
type LabOrder struct {
	ID            string         `json:"id"`
	PatientID     string         `json:"patientId"`
	PatientNHI    string         `json:"patientNhi"`
	OrderingHPI   string         `json:"orderingHpi"`
	EncounterID   string         `json:"encounterId,omitempty"`
	Tests         []string       `json:"tests"`           // LOINC codes
	Priority      string         `json:"priority"`        // routine, urgent, stat
	ClinicalNotes string         `json:"clinicalNotes,omitempty"`
	Status        LabOrderStatus `json:"status"`
	FHIRReport    string         `json:"fhirReport,omitempty"` // FHIR DiagnosticReport JSON (encrypted at rest)
	ResultedAt    *time.Time     `json:"resultedAt,omitempty"`
	TenantID      string         `json:"tenantId"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

// labOrderCreateRequest is the body for POST /api/v1/labs.
type labOrderCreateRequest struct {
	PatientID     string   `json:"patientId"`
	PatientNHI    string   `json:"patientNhi"`
	OrderingHPI   string   `json:"orderingHpi"`
	EncounterID   string   `json:"encounterId,omitempty"`
	Tests         []string `json:"tests"`
	Priority      string   `json:"priority"`
	ClinicalNotes string   `json:"clinicalNotes,omitempty"`
}

// labResultRequest is the body for POST /api/v1/labs/{id}/result.
// It carries the incoming FHIR DiagnosticReport from the laboratory system.
type labResultRequest struct {
	FHIRReport string `json:"fhirReport"` // serialised FHIR DiagnosticReport JSON
}

// LabsHandler handles all /api/v1/labs routes.
type LabsHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/labs.
// Supports query params: patient, status, provider.
func (h *LabsHandler) List(w http.ResponseWriter, r *http.Request) {
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

	orders, err := h.listOrders(ctx, tenantID, patientFilter, statusFilter, providerFilter)
	if err != nil {
		h.logger.Error("list lab orders", slog.Any("error", err), slog.String("tenant", tenantID))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list lab orders"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "DiagnosticReport",
		ResourceID:   "list",
		TenantID:     tenantID,
		Metadata:     map[string]string{"patient": patientFilter, "status": statusFilter},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"labOrders": orders,
		"total":     len(orders),
	})
}

// Create handles POST /api/v1/labs.
// Creates a new lab order in requested status. Requires at least one LOINC test code.
func (h *LabsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req labOrderCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if err := validateLabOrderCreate(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	// HPCA requirement: validate the ordering practitioner holds a current APC.
	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.OrderingHPI)
	if err != nil {
		h.logger.Error("HPI APC validation for lab order", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate practitioner APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "INVALID_APC",
			Message: "ordering practitioner does not have a current Annual Practising Certificate",
			Details: apcStatus,
		})
		return
	}

	order, err := h.insertOrder(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert lab order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create lab order"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "ServiceRequest",
		ResourceID:   order.ID,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "lab-order"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, order)
}

// Get handles GET /api/v1/labs/{id}.
func (h *LabsHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "lab order ID is required"})
		return
	}

	order, err := h.getOrderByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "lab order not found"})
			return
		}
		h.logger.Error("get lab order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve lab order"})
		return
	}

	// Decrypt the FHIR report blob if it exists.
	if len(order.FHIRReport) > 0 {
		plain, err := h.enc.Decrypt([]byte(order.FHIRReport))
		if err != nil {
			h.logger.Error("decrypt FHIR report", slog.Any("error", err))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt lab result"})
			return
		}
		order.FHIRReport = string(plain)
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "DiagnosticReport",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, order)
}

// Result handles POST /api/v1/labs/{id}/result.
// Receives a FHIR DiagnosticReport from an external lab system (e.g. via HL7 MLLP
// or FHIR subscription) and stores it, transitioning the order to "final" status.
func (h *LabsHandler) Result(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "lab order ID is required"})
		return
	}

	var req labResultRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.FHIRReport == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_REPORT", Message: "fhirReport is required"})
		return
	}

	// Encrypt the DiagnosticReport JSON before persisting.
	encReport, err := h.enc.Encrypt([]byte(req.FHIRReport))
	if err != nil {
		h.logger.Error("encrypt FHIR report", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt lab result"})
		return
	}

	now := time.Now().UTC()
	updated, err := h.storeResult(ctx, id, encReport, now, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "lab order not found"})
			return
		}
		h.logger.Error("store lab result", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "STORE_ERROR", Message: "failed to store lab result"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "DiagnosticReport",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "result-received"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func validateLabOrderCreate(req *labOrderCreateRequest) error {
	if req.PatientID == "" && req.PatientNHI == "" {
		return fmt.Errorf("either patientId or patientNhi is required")
	}
	if req.OrderingHPI == "" {
		return fmt.Errorf("orderingHpi is required")
	}
	if len(req.Tests) == 0 {
		return fmt.Errorf("at least one test code is required")
	}
	if req.Priority == "" {
		req.Priority = "routine"
	}
	return nil
}

// ---------------------------------------------------------------------------
// Data access
// ---------------------------------------------------------------------------

func (h *LabsHandler) listOrders(
	ctx context.Context,
	tenantID, patientFilter, statusFilter, providerFilter string,
) ([]LabOrder, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, ordering_hpi,
		        encounter_id, tests, priority, clinical_notes,
		        status, resulted_at, tenant_id, created_at, updated_at
		 FROM lab_orders
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter  = '' OR patient_id   = @patient_filter)
		   AND (@status_filter   = '' OR status       = @status_filter)
		   AND (@provider_filter = '' OR ordering_hpi = @provider_filter)
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
		return nil, fmt.Errorf("query lab orders: %w", err)
	}
	defer rows.Close()

	var results []LabOrder
	for rows.Next() {
		var o LabOrder
		if err := rows.Scan(
			&o.ID, &o.PatientID, &o.PatientNHI, &o.OrderingHPI,
			&o.EncounterID, &o.Tests, &o.Priority, &o.ClinicalNotes,
			&o.Status, &o.ResultedAt, &o.TenantID, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan lab order: %w", err)
		}
		results = append(results, o)
	}
	return results, rows.Err()
}

func (h *LabsHandler) getOrderByID(ctx context.Context, id, tenantID string) (LabOrder, error) {
	var o LabOrder
	err := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, ordering_hpi,
		        encounter_id, tests, priority, clinical_notes,
		        status, fhir_report, resulted_at, tenant_id, created_at, updated_at
		 FROM lab_orders
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&o.ID, &o.PatientID, &o.PatientNHI, &o.OrderingHPI,
		&o.EncounterID, &o.Tests, &o.Priority, &o.ClinicalNotes,
		&o.Status, &o.FHIRReport, &o.ResultedAt, &o.TenantID, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return LabOrder{}, errNotFound
		}
		return LabOrder{}, fmt.Errorf("get lab order: %w", err)
	}
	return o, nil
}

func (h *LabsHandler) insertOrder(ctx context.Context, req labOrderCreateRequest, tenantID string) (LabOrder, error) {
	var o LabOrder
	err := h.pool.QueryRow(ctx,
		`INSERT INTO lab_orders
		   (patient_id, patient_nhi, ordering_hpi, encounter_id, tests, priority, clinical_notes, status, tenant_id)
		 VALUES
		   (@patient_id, @patient_nhi, @ordering_hpi, @encounter_id, @tests, @priority, @clinical_notes, @status, @tenant_id)
		 RETURNING id, patient_id, patient_nhi, ordering_hpi,
		           encounter_id, tests, priority, clinical_notes,
		           status, resulted_at, tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"patient_id":     req.PatientID,
			"patient_nhi":    req.PatientNHI,
			"ordering_hpi":   req.OrderingHPI,
			"encounter_id":   req.EncounterID,
			"tests":          req.Tests,
			"priority":       req.Priority,
			"clinical_notes": req.ClinicalNotes,
			"status":         LabOrderStatusRequested,
			"tenant_id":      tenantID,
		},
	).Scan(
		&o.ID, &o.PatientID, &o.PatientNHI, &o.OrderingHPI,
		&o.EncounterID, &o.Tests, &o.Priority, &o.ClinicalNotes,
		&o.Status, &o.ResultedAt, &o.TenantID, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return LabOrder{}, fmt.Errorf("insert lab order: %w", err)
	}
	return o, nil
}

func (h *LabsHandler) storeResult(ctx context.Context, id string, encReport []byte, resultedAt time.Time, tenantID string) (LabOrder, error) {
	var o LabOrder
	err := h.pool.QueryRow(ctx,
		`UPDATE lab_orders
		 SET status      = @status,
		     fhir_report = @fhir_report,
		     resulted_at = @resulted_at,
		     updated_at  = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, ordering_hpi,
		           encounter_id, tests, priority, clinical_notes,
		           status, resulted_at, tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"status":      LabOrderStatusFinal,
			"fhir_report": encReport,
			"resulted_at": resultedAt,
			"id":          id,
			"tenant_id":   tenantID,
		},
	).Scan(
		&o.ID, &o.PatientID, &o.PatientNHI, &o.OrderingHPI,
		&o.EncounterID, &o.Tests, &o.Priority, &o.ClinicalNotes,
		&o.Status, &o.ResultedAt, &o.TenantID, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return LabOrder{}, errNotFound
		}
		return LabOrder{}, fmt.Errorf("store lab result: %w", err)
	}
	return o, nil
}
