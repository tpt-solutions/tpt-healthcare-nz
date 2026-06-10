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

// ICAlertSeverity classifies the urgency of an infection control alert.
type ICAlertSeverity string

const (
	ICAlertSeverityInfo     ICAlertSeverity = "info"
	ICAlertSeverityWarning  ICAlertSeverity = "warning"
	ICAlertSeverityCritical ICAlertSeverity = "critical"
)

// IsolationType enumerates standard infection control precaution types.
type IsolationType string

const (
	IsolationContact          IsolationType = "contact"
	IsolationDroplet          IsolationType = "droplet"
	IsolationAirborne         IsolationType = "airborne"
	IsolationProtective       IsolationType = "protective"   // immunocompromised patient
	IsolationCombined         IsolationType = "combined"     // droplet + contact
	IsolationEnhancedContact  IsolationType = "enhanced-contact"
)

// HAIType lists healthcare-associated infection organism categories.
type HAIType string

const (
	HAITypeMRSA   HAIType = "mrsa"
	HAITypeVRE    HAIType = "vre"
	HAITypeCDiff  HAIType = "c-difficile"
	HAITypeCRE    HAIType = "cre"
	HAITypeESBL   HAIType = "esbl"
	HAITypeCOVID  HAIType = "covid-19"
	HAITypeInfluenza HAIType = "influenza"
	HAITypeOther  HAIType = "other"
)

// ICAlert is a hospital-wide or ward-specific infection control alert.
type ICAlert struct {
	ID           string          `json:"id"`
	AlertType    HAIType         `json:"alertType"`
	Severity     ICAlertSeverity `json:"severity"`
	WardID       string          `json:"wardId,omitempty"` // empty = hospital-wide
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Actions      []string        `json:"actions,omitempty"`
	Active       bool            `json:"active"`
	TenantID     string          `json:"tenantId"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	ResolvedAt   *time.Time      `json:"resolvedAt,omitempty"`
}

// IsolationOrder is an active isolation precaution on a patient admission.
type IsolationOrder struct {
	ID             string        `json:"id"`
	AdmissionID    string        `json:"admissionId"`
	PatientID      string        `json:"patientId"`
	IsolationType  IsolationType `json:"isolationType"`
	Reason         string        `json:"reason"`
	Organism       HAIType       `json:"organism,omitempty"`
	PPERequired    []string      `json:"ppeRequired"` // e.g. ["gloves","gown","mask","eyewear"]
	SpecialNotes   string        `json:"specialNotes,omitempty"`
	OrderedByHPI   string        `json:"orderedByHpi"`
	TenantID       string        `json:"tenantId"`
	StartedAt      time.Time     `json:"startedAt"`
	EndedAt        *time.Time    `json:"endedAt,omitempty"`
}

type icAlertCreateRequest struct {
	AlertType   HAIType         `json:"alertType"`
	Severity    ICAlertSeverity `json:"severity"`
	WardID      string          `json:"wardId,omitempty"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Actions     []string        `json:"actions,omitempty"`
}

type icAlertUpdateRequest struct {
	Severity    ICAlertSeverity `json:"severity,omitempty"`
	Description string          `json:"description,omitempty"`
	Actions     []string        `json:"actions,omitempty"`
	Active      *bool           `json:"active,omitempty"`
}

type isolationCreateRequest struct {
	IsolationType IsolationType `json:"isolationType"`
	Reason        string        `json:"reason"`
	Organism      HAIType       `json:"organism,omitempty"`
	PPERequired   []string      `json:"ppeRequired,omitempty"`
	SpecialNotes  string        `json:"specialNotes,omitempty"`
	OrderedByHPI  string        `json:"orderedByHpi"`
}

// InfectionControlHandler handles infection control routes.
type InfectionControlHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListAlerts handles GET /api/v1/infection-control/alerts.
func (h *InfectionControlHandler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	activeOnly := r.URL.Query().Get("active") != "false"
	wardFilter := r.URL.Query().Get("ward")

	alerts, err := h.listAlerts(ctx, tenantID.String(), activeOnly, wardFilter)
	if err != nil {
		h.logger.Error("list IC alerts", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list alerts"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"alerts": alerts, "total": len(alerts)})
}

// CreateAlert handles POST /api/v1/infection-control/alerts.
func (h *InfectionControlHandler) CreateAlert(w http.ResponseWriter, r *http.Request) {
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

	var req icAlertCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TITLE", Message: "title is required"})
		return
	}

	alert, err := h.insertAlert(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("create IC alert", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create alert"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "ICAlert",
		ResourceID: alert.ID, TenantID: tenantID,
		Details:    map[string]any{"alert_type": string(req.AlertType), "severity": string(req.Severity)},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, alert)
}

// GetAlert handles GET /api/v1/infection-control/alerts/{id}.
func (h *InfectionControlHandler) GetAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	id := r.PathValue("id")
	alert, err := h.getAlertByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "alert not found"})
			return
		}
		h.logger.Error("get IC alert", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve alert"})
		return
	}
	writeJSON(w, http.StatusOK, alert)
}

// UpdateAlert handles PUT /api/v1/infection-control/alerts/{id}.
func (h *InfectionControlHandler) UpdateAlert(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getAlertByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "alert not found"})
			return
		}
		h.logger.Error("get IC alert for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve alert"})
		return
	}

	var req icAlertUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Severity != "" {
		existing.Severity = req.Severity
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if len(req.Actions) > 0 {
		existing.Actions = req.Actions
	}
	if req.Active != nil {
		existing.Active = *req.Active
		if !*req.Active {
			now := time.Now().UTC()
			existing.ResolvedAt = &now
		}
	}

	updated, err := h.updateAlert(ctx, existing)
	if err != nil {
		h.logger.Error("update IC alert", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update alert"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "ICAlert",
		ResourceID: id, TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, updated)
}

// ListIsolation handles GET /api/v1/admissions/{admissionId}/isolation.
func (h *InfectionControlHandler) ListIsolation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	admissionID := r.PathValue("admissionId")
	orders, err := h.listIsolationOrders(ctx, admissionID, tenantID)
	if err != nil {
		h.logger.Error("list isolation orders", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list isolation orders"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders, "total": len(orders)})
}

// ApplyIsolation handles POST /api/v1/admissions/{admissionId}/isolation.
func (h *InfectionControlHandler) ApplyIsolation(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	var req isolationCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.IsolationType == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TYPE", Message: "isolationType is required"})
		return
	}
	if req.Reason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_REASON", Message: "reason is required"})
		return
	}

	order, err := h.insertIsolationOrder(ctx, admissionID, req, tenantID)
	if err != nil {
		h.logger.Error("apply isolation order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to apply isolation order"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "IsolationOrder",
		ResourceID: order.ID, TenantID: tenantID,
		Metadata: map[string]string{"admissionId": admissionID, "type": string(req.IsolationType)},
	})
	writeJSON(w, http.StatusCreated, order)
}

// RemoveIsolation handles DELETE /api/v1/admissions/{admissionId}/isolation/{isolationId}.
func (h *InfectionControlHandler) RemoveIsolation(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	isolationID := r.PathValue("isolationId")

	if err := h.endIsolationOrder(ctx, isolationID, admissionID, tenantID); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "isolation order not found"})
			return
		}
		h.logger.Error("remove isolation order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "REMOVE_ERROR", Message: "failed to remove isolation order"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "IsolationOrder",
		ResourceID: isolationID, TenantID: tenantID,
		Metadata: map[string]string{"action": "remove", "admissionId": admissionID},
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *InfectionControlHandler) listAlerts(ctx context.Context, tenantID string, activeOnly bool, wardFilter string) ([]ICAlert, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, alert_type, severity, ward_id, title, description, actions, active,
		        tenant_id, created_at, updated_at, resolved_at
		 FROM ic_alerts
		 WHERE tenant_id = @tenant_id
		   AND (@active_only = false OR active = true)
		   AND (@ward_filter = '' OR ward_id = @ward_filter OR ward_id IS NULL)
		 ORDER BY severity DESC, created_at DESC`,
		db.NamedArgs{"tenant_id": tenantID, "active_only": activeOnly, "ward_filter": wardFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query IC alerts: %w", err)
	}
	defer rows.Close()

	var results []ICAlert
	for rows.Next() {
		var a ICAlert
		if err := rows.Scan(
			&a.ID, &a.AlertType, &a.Severity, &a.WardID, &a.Title, &a.Description, &a.Actions, &a.Active,
			&a.TenantID, &a.CreatedAt, &a.UpdatedAt, &a.ResolvedAt,
		); err != nil {
			return nil, fmt.Errorf("scan IC alert: %w", err)
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

func (h *InfectionControlHandler) getAlertByID(ctx context.Context, id, tenantID string) (ICAlert, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, alert_type, severity, ward_id, title, description, actions, active,
		        tenant_id, created_at, updated_at, resolved_at
		 FROM ic_alerts
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	var a ICAlert
	if err := row.Scan(
		&a.ID, &a.AlertType, &a.Severity, &a.WardID, &a.Title, &a.Description, &a.Actions, &a.Active,
		&a.TenantID, &a.CreatedAt, &a.UpdatedAt, &a.ResolvedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return ICAlert{}, errNotFound
		}
		return ICAlert{}, fmt.Errorf("get IC alert: %w", err)
	}
	return a, nil
}

func (h *InfectionControlHandler) insertAlert(ctx context.Context, req icAlertCreateRequest, tenantID string) (ICAlert, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO ic_alerts
		   (alert_type, severity, ward_id, title, description, actions, active, tenant_id)
		 VALUES
		   (@alert_type, @severity, @ward_id, @title, @description, @actions, true, @tenant_id)
		 RETURNING id, alert_type, severity, ward_id, title, description, actions, active,
		           tenant_id, created_at, updated_at, resolved_at`,
		db.NamedArgs{
			"alert_type":  req.AlertType,
			"severity":    req.Severity,
			"ward_id":     req.WardID,
			"title":       req.Title,
			"description": req.Description,
			"actions":     req.Actions,
			"tenant_id":   tenantID,
		},
	)
	var a ICAlert
	if err := row.Scan(
		&a.ID, &a.AlertType, &a.Severity, &a.WardID, &a.Title, &a.Description, &a.Actions, &a.Active,
		&a.TenantID, &a.CreatedAt, &a.UpdatedAt, &a.ResolvedAt,
	); err != nil {
		return ICAlert{}, fmt.Errorf("insert IC alert: %w", err)
	}
	return a, nil
}

func (h *InfectionControlHandler) updateAlert(ctx context.Context, a ICAlert) (ICAlert, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE ic_alerts
		 SET severity = @severity, description = @description, actions = @actions,
		     active = @active, resolved_at = @resolved_at, updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, alert_type, severity, ward_id, title, description, actions, active,
		           tenant_id, created_at, updated_at, resolved_at`,
		db.NamedArgs{
			"severity":    a.Severity,
			"description": a.Description,
			"actions":     a.Actions,
			"active":      a.Active,
			"resolved_at": a.ResolvedAt,
			"id":          a.ID,
			"tenant_id":   a.TenantID,
		},
	)
	var updated ICAlert
	if err := row.Scan(
		&updated.ID, &updated.AlertType, &updated.Severity, &updated.WardID, &updated.Title, &updated.Description, &updated.Actions, &updated.Active,
		&updated.TenantID, &updated.CreatedAt, &updated.UpdatedAt, &updated.ResolvedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return ICAlert{}, errNotFound
		}
		return ICAlert{}, fmt.Errorf("update IC alert: %w", err)
	}
	return updated, nil
}

func (h *InfectionControlHandler) listIsolationOrders(ctx context.Context, admissionID, tenantID string) ([]IsolationOrder, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, admission_id, patient_id, isolation_type, reason, organism,
		        ppe_required, special_notes, ordered_by_hpi, tenant_id, started_at, ended_at
		 FROM isolation_orders
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		 ORDER BY started_at DESC`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query isolation orders: %w", err)
	}
	defer rows.Close()

	var results []IsolationOrder
	for rows.Next() {
		var o IsolationOrder
		if err := rows.Scan(
			&o.ID, &o.AdmissionID, &o.PatientID, &o.IsolationType, &o.Reason, &o.Organism,
			&o.PPERequired, &o.SpecialNotes, &o.OrderedByHPI, &o.TenantID, &o.StartedAt, &o.EndedAt,
		); err != nil {
			return nil, fmt.Errorf("scan isolation order: %w", err)
		}
		results = append(results, o)
	}
	return results, rows.Err()
}

func (h *InfectionControlHandler) insertIsolationOrder(ctx context.Context, admissionID string, req isolationCreateRequest, tenantID string) (IsolationOrder, error) {
	var patientID string
	if err := h.pool.QueryRow(ctx,
		`SELECT patient_id FROM hospital_admissions WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": admissionID, "tenant_id": tenantID},
	).Scan(&patientID); err != nil {
		if db.IsNoRows(err) {
			return IsolationOrder{}, errNotFound
		}
		return IsolationOrder{}, fmt.Errorf("get admission for isolation: %w", err)
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO isolation_orders
		   (admission_id, patient_id, isolation_type, reason, organism,
		    ppe_required, special_notes, ordered_by_hpi, tenant_id, started_at)
		 VALUES
		   (@admission_id, @patient_id, @isolation_type, @reason, @organism,
		    @ppe_required, @special_notes, @ordered_by_hpi, @tenant_id, now())
		 RETURNING id, admission_id, patient_id, isolation_type, reason, organism,
		           ppe_required, special_notes, ordered_by_hpi, tenant_id, started_at, ended_at`,
		db.NamedArgs{
			"admission_id":   admissionID,
			"patient_id":     patientID,
			"isolation_type": req.IsolationType,
			"reason":         req.Reason,
			"organism":       req.Organism,
			"ppe_required":   req.PPERequired,
			"special_notes":  req.SpecialNotes,
			"ordered_by_hpi": req.OrderedByHPI,
			"tenant_id":      tenantID,
		},
	)
	var o IsolationOrder
	if err := row.Scan(
		&o.ID, &o.AdmissionID, &o.PatientID, &o.IsolationType, &o.Reason, &o.Organism,
		&o.PPERequired, &o.SpecialNotes, &o.OrderedByHPI, &o.TenantID, &o.StartedAt, &o.EndedAt,
	); err != nil {
		return IsolationOrder{}, fmt.Errorf("insert isolation order: %w", err)
	}
	return o, nil
}

func (h *InfectionControlHandler) endIsolationOrder(ctx context.Context, id, admissionID, tenantID string) error {
	tag, err := h.pool.Exec(ctx,
		`UPDATE isolation_orders SET ended_at = now()
		 WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id AND ended_at IS NULL`,
		db.NamedArgs{"id": id, "admission_id": admissionID, "tenant_id": tenantID},
	)
	if err != nil {
		return fmt.Errorf("end isolation order: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}
