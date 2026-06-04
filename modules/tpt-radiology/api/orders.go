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

// RadiologyOrder is the domain model for a radiology (RIS) order.
type RadiologyOrder struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenantId"`
	PatientNHI      string     `json:"patientNhi"`
	ImagingStudyID  *string    `json:"imagingStudyId,omitempty"`
	AccessionNumber string     `json:"accessionNumber,omitempty"`
	Modality        string     `json:"modality"`
	BodyPart        string     `json:"bodyPart,omitempty"`
	ClinicalInfo    string     `json:"clinicalInfo,omitempty"`
	Priority        string     `json:"priority"`
	Status          string     `json:"status"`
	ReferringHPI    string     `json:"referringHpi"`
	RequestedAt     time.Time  `json:"requestedAt"`
	ScheduledAt     *time.Time `json:"scheduledAt,omitempty"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	LOINCCode       string     `json:"loincCode,omitempty"`
	LOINCDisplay    string     `json:"loincDisplay,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type orderCreateRequest struct {
	PatientNHI   string `json:"patientNhi"`
	Modality     string `json:"modality"`
	BodyPart     string `json:"bodyPart,omitempty"`
	ClinicalInfo string `json:"clinicalInfo,omitempty"`
	Priority     string `json:"priority,omitempty"` // stat, urgent, routine (defaults routine)
	ReferringHPI string `json:"referringHpi"`
	LOINCCode    string `json:"loincCode,omitempty"`
	LOINCDisplay string `json:"loincDisplay,omitempty"`
}

type orderUpdateRequest struct {
	ClinicalInfo   *string `json:"clinicalInfo,omitempty"`
	Priority       *string `json:"priority,omitempty"`
	LOINCCode      *string `json:"loincCode,omitempty"`
	LOINCDisplay   *string `json:"loincDisplay,omitempty"`
	ImagingStudyID *string `json:"imagingStudyId,omitempty"`
}

type orderScheduleRequest struct {
	ScheduledAt time.Time `json:"scheduledAt"`
}

// OrdersHandler handles RIS order workflow routes.
type OrdersHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/radiology-orders.
func (h *OrdersHandler) List(w http.ResponseWriter, r *http.Request) {
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
	orders, err := h.listOrders(ctx, tenantID, q.Get("patientNhi"), q.Get("status"), q.Get("modality"))
	if err != nil {
		h.logger.Error("list radiology orders", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list orders"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "RadiologyOrder",
		ResourceID:   "list",
		TenantID:     tenantID,
	})

	writeJSON(w, http.StatusOK, map[string]any{"orders": orders, "total": len(orders)})
}

// Create handles POST /api/v1/radiology-orders.
func (h *OrdersHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req orderCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "patientNhi is required"})
		return
	}
	if req.Modality == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_MODALITY", Message: "modality is required"})
		return
	}
	if req.ReferringHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_HPI", Message: "referringHpi is required"})
		return
	}
	if req.Priority == "" {
		req.Priority = "routine"
	}

	order, err := h.insertOrder(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert radiology order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create order"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "RadiologyOrder",
		ResourceID:   order.ID,
		TenantID:     tenantID,
		Metadata:     map[string]string{"modality": order.Modality, "priority": order.Priority, "patient_nhi": order.PatientNHI},
	})

	writeJSON(w, http.StatusCreated, order)
}

// Get handles GET /api/v1/radiology-orders/{id}.
func (h *OrdersHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	order, err := h.getOrderByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get radiology order", slog.Any("error", err), slog.String("id", id))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "RadiologyOrder",
		ResourceID:   id,
		TenantID:     tenantID,
	})

	writeJSON(w, http.StatusOK, order)
}

// Update handles PUT /api/v1/radiology-orders/{id}.
func (h *OrdersHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var req orderUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	existing, err := h.getOrderByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get order for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}

	if req.ClinicalInfo != nil {
		existing.ClinicalInfo = *req.ClinicalInfo
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if req.LOINCCode != nil {
		existing.LOINCCode = *req.LOINCCode
	}
	if req.LOINCDisplay != nil {
		existing.LOINCDisplay = *req.LOINCDisplay
	}
	if req.ImagingStudyID != nil {
		existing.ImagingStudyID = req.ImagingStudyID
	}

	updated, err := h.updateOrder(ctx, existing)
	if err != nil {
		h.logger.Error("update radiology order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update order"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "RadiologyOrder",
		ResourceID:   id,
		TenantID:     tenantID,
	})

	writeJSON(w, http.StatusOK, updated)
}

// Schedule handles POST /api/v1/radiology-orders/{id}/schedule.
func (h *OrdersHandler) Schedule(w http.ResponseWriter, r *http.Request) {
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
	var req orderScheduleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ScheduledAt.IsZero() {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DATE", Message: "scheduledAt is required"})
		return
	}

	existing, err := h.getOrderByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get order for schedule", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}

	if existing.Status == "completed" || existing.Status == "cancelled" {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "cannot schedule a " + existing.Status + " order"})
		return
	}

	t := req.ScheduledAt.UTC()
	existing.ScheduledAt = &t
	existing.Status = "active"

	updated, err := h.updateOrder(ctx, existing)
	if err != nil {
		h.logger.Error("schedule radiology order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCHEDULE_ERROR", Message: "failed to schedule order"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "RadiologyOrder",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "schedule"},
	})

	writeJSON(w, http.StatusOK, updated)
}

// Complete handles POST /api/v1/radiology-orders/{id}/complete.
func (h *OrdersHandler) Complete(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getOrderByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get order for complete", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}

	if existing.Status == "cancelled" {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "cannot complete a cancelled order"})
		return
	}

	now := time.Now().UTC()
	existing.Status = "completed"
	existing.CompletedAt = &now

	updated, err := h.updateOrder(ctx, existing)
	if err != nil {
		h.logger.Error("complete radiology order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "COMPLETE_ERROR", Message: "failed to complete order"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "RadiologyOrder",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "complete"},
	})

	writeJSON(w, http.StatusOK, updated)
}

// Cancel handles POST /api/v1/radiology-orders/{id}/cancel.
func (h *OrdersHandler) Cancel(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getOrderByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get order for cancel", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}

	if existing.Status == "completed" {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "cannot cancel a completed order"})
		return
	}

	existing.Status = "cancelled"
	updated, err := h.updateOrder(ctx, existing)
	if err != nil {
		h.logger.Error("cancel radiology order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CANCEL_ERROR", Message: "failed to cancel order"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "RadiologyOrder",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "cancel"},
	})

	writeJSON(w, http.StatusOK, updated)
}

// ---------------------------------------------------------------------------
// Database operations
// ---------------------------------------------------------------------------

func (h *OrdersHandler) listOrders(ctx context.Context, tenantID, patientNHI, status, modality string) ([]RadiologyOrder, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, patient_nhi, imaging_study_id, accession_number,
		        modality, body_part, clinical_info, priority, status,
		        referring_hpi, requested_at, scheduled_at, completed_at,
		        loinc_code, loinc_display, created_at, updated_at
		 FROM radiology_orders
		 WHERE tenant_id   = @tenant_id
		   AND (@patient_nhi = '' OR patient_nhi = @patient_nhi)
		   AND (@status      = '' OR status      = @status)
		   AND (@modality    = '' OR modality    = @modality)
		 ORDER BY requested_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":  tenantID,
			"patient_nhi": patientNHI,
			"status":     status,
			"modality":   modality,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query radiology orders: %w", err)
	}
	defer rows.Close()

	var results []RadiologyOrder
	for rows.Next() {
		var o RadiologyOrder
		if err := rows.Scan(
			&o.ID, &o.TenantID, &o.PatientNHI, &o.ImagingStudyID, &o.AccessionNumber,
			&o.Modality, &o.BodyPart, &o.ClinicalInfo, &o.Priority, &o.Status,
			&o.ReferringHPI, &o.RequestedAt, &o.ScheduledAt, &o.CompletedAt,
			&o.LOINCCode, &o.LOINCDisplay, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan radiology order: %w", err)
		}
		results = append(results, o)
	}
	return results, rows.Err()
}

func (h *OrdersHandler) insertOrder(ctx context.Context, req orderCreateRequest, tenantID string) (RadiologyOrder, error) {
	var o RadiologyOrder
	err := h.pool.QueryRow(ctx,
		`INSERT INTO radiology_orders
		   (tenant_id, patient_nhi, modality, body_part, clinical_info,
		    priority, status, referring_hpi, loinc_code, loinc_display)
		 VALUES
		   (@tenant_id, @patient_nhi, @modality, @body_part, @clinical_info,
		    @priority, 'draft', @referring_hpi, @loinc_code, @loinc_display)
		 RETURNING id, tenant_id, patient_nhi, imaging_study_id, accession_number,
		           modality, body_part, clinical_info, priority, status,
		           referring_hpi, requested_at, scheduled_at, completed_at,
		           loinc_code, loinc_display, created_at, updated_at`,
		db.NamedArgs{
			"tenant_id":    tenantID,
			"patient_nhi":  req.PatientNHI,
			"modality":     req.Modality,
			"body_part":    req.BodyPart,
			"clinical_info": req.ClinicalInfo,
			"priority":     req.Priority,
			"referring_hpi": req.ReferringHPI,
			"loinc_code":   req.LOINCCode,
			"loinc_display": req.LOINCDisplay,
		},
	).Scan(
		&o.ID, &o.TenantID, &o.PatientNHI, &o.ImagingStudyID, &o.AccessionNumber,
		&o.Modality, &o.BodyPart, &o.ClinicalInfo, &o.Priority, &o.Status,
		&o.ReferringHPI, &o.RequestedAt, &o.ScheduledAt, &o.CompletedAt,
		&o.LOINCCode, &o.LOINCDisplay, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return RadiologyOrder{}, fmt.Errorf("insert radiology order: %w", err)
	}
	return o, nil
}

func (h *OrdersHandler) getOrderByID(ctx context.Context, id, tenantID string) (RadiologyOrder, error) {
	var o RadiologyOrder
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, patient_nhi, imaging_study_id, accession_number,
		        modality, body_part, clinical_info, priority, status,
		        referring_hpi, requested_at, scheduled_at, completed_at,
		        loinc_code, loinc_display, created_at, updated_at
		 FROM radiology_orders
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&o.ID, &o.TenantID, &o.PatientNHI, &o.ImagingStudyID, &o.AccessionNumber,
		&o.Modality, &o.BodyPart, &o.ClinicalInfo, &o.Priority, &o.Status,
		&o.ReferringHPI, &o.RequestedAt, &o.ScheduledAt, &o.CompletedAt,
		&o.LOINCCode, &o.LOINCDisplay, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return RadiologyOrder{}, errNotFound
		}
		return RadiologyOrder{}, fmt.Errorf("get radiology order: %w", err)
	}
	return o, nil
}

func (h *OrdersHandler) updateOrder(ctx context.Context, o RadiologyOrder) (RadiologyOrder, error) {
	var updated RadiologyOrder
	err := h.pool.QueryRow(ctx,
		`UPDATE radiology_orders
		 SET imaging_study_id = @imaging_study_id,
		     clinical_info    = @clinical_info,
		     priority         = @priority,
		     status           = @status,
		     scheduled_at     = @scheduled_at,
		     completed_at     = @completed_at,
		     loinc_code       = @loinc_code,
		     loinc_display    = @loinc_display,
		     updated_at       = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, tenant_id, patient_nhi, imaging_study_id, accession_number,
		           modality, body_part, clinical_info, priority, status,
		           referring_hpi, requested_at, scheduled_at, completed_at,
		           loinc_code, loinc_display, created_at, updated_at`,
		db.NamedArgs{
			"imaging_study_id": o.ImagingStudyID,
			"clinical_info":    o.ClinicalInfo,
			"priority":         o.Priority,
			"status":           o.Status,
			"scheduled_at":     o.ScheduledAt,
			"completed_at":     o.CompletedAt,
			"loinc_code":       o.LOINCCode,
			"loinc_display":    o.LOINCDisplay,
			"id":               o.ID,
			"tenant_id":        o.TenantID,
		},
	).Scan(
		&updated.ID, &updated.TenantID, &updated.PatientNHI, &updated.ImagingStudyID, &updated.AccessionNumber,
		&updated.Modality, &updated.BodyPart, &updated.ClinicalInfo, &updated.Priority, &updated.Status,
		&updated.ReferringHPI, &updated.RequestedAt, &updated.ScheduledAt, &updated.CompletedAt,
		&updated.LOINCCode, &updated.LOINCDisplay, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return RadiologyOrder{}, errNotFound
		}
		return RadiologyOrder{}, fmt.Errorf("update radiology order: %w", err)
	}
	return updated, nil
}
