package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/events"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// EmergencyHandler handles all /api/v1/emergency routes.
type EmergencyHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	eventBus   *events.Bus
	logger     *slog.Logger
}

// DeclareIncident handles POST /api/v1/emergency/incidents.
func (h *EmergencyHandler) DeclareIncident(w http.ResponseWriter, r *http.Request) {
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

	var req incidentDeclareRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TITLE", Message: "title is required"})
		return
	}
	if req.Type == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TYPE", Message: "type is required"})
		return
	}

	incident, err := h.insertIncident(ctx, req, principal.ID, tenantID.String())
	if err != nil {
		h.logger.Error("declare incident", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to declare incident"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "EmergencyIncident",
		ResourceID: incident.ID, TenantID: tenantID,
		Details:    map[string]any{"type": string(req.Type), "title": req.Title},
		OccurredAt: time.Now().UTC(),
	})
	h.publishIncidentEvent(ctx, events.EventIncidentDeclared, incident)

	writeJSON(w, http.StatusCreated, incident)
}

// ListIncidents handles GET /api/v1/emergency/incidents.
func (h *EmergencyHandler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	statusFilter := r.URL.Query().Get("status")
	incidents, err := h.listIncidents(ctx, tenantID.String(), statusFilter)
	if err != nil {
		h.logger.Error("list incidents", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list incidents"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"incidents": incidents, "total": len(incidents)})
}

// GetIncident handles GET /api/v1/emergency/incidents/{id}.
func (h *EmergencyHandler) GetIncident(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	id := r.PathValue("id")
	incident, err := h.getIncidentByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		h.logger.Error("get incident", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve incident"})
		return
	}

	assignments, _ := h.listCommandAssignments(ctx, id, tenantID.String())
	writeJSON(w, http.StatusOK, map[string]any{"incident": incident, "commandAssignments": assignments})
}

// ActivateIncident handles POST /api/v1/emergency/incidents/{id}/activate.
func (h *EmergencyHandler) ActivateIncident(w http.ResponseWriter, r *http.Request) {
	h.transitionIncident(w, r, IncidentActivated, "activate", events.EventIncidentActivated)
}

// EscalateIncident handles POST /api/v1/emergency/incidents/{id}/escalate.
func (h *EmergencyHandler) EscalateIncident(w http.ResponseWriter, r *http.Request) {
	h.transitionIncident(w, r, IncidentEscalated, "escalate", events.EventIncidentEscalated)
}

// StandDown handles POST /api/v1/emergency/incidents/{id}/stand-down.
func (h *EmergencyHandler) StandDown(w http.ResponseWriter, r *http.Request) {
	h.transitionIncident(w, r, IncidentStandDown, "stand_down", events.EventIncidentStandDown)
}

// CloseIncident handles POST /api/v1/emergency/incidents/{id}/close.
func (h *EmergencyHandler) CloseIncident(w http.ResponseWriter, r *http.Request) {
	h.transitionIncident(w, r, IncidentClosed, "close", "")
}

// transitionIncident is the shared state-machine helper for lifecycle transitions.
func (h *EmergencyHandler) transitionIncident(w http.ResponseWriter, r *http.Request, to IncidentStatus, action, eventType string) {
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
	existing, err := h.getIncidentByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		h.logger.Error("get incident for transition", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve incident"})
		return
	}
	if existing.Status == IncidentClosed {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_CLOSED", Message: "incident is closed"})
		return
	}

	updated, err := h.setIncidentStatus(ctx, id, tenantID.String(), to)
	if err != nil {
		h.logger.Error("transition incident", slog.String("action", action), slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "TRANSITION_ERROR", Message: "failed to update incident status"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: action, ResourceType: "EmergencyIncident",
		ResourceID: id, TenantID: tenantID,
		Details:    map[string]any{"status": string(to)},
		OccurredAt: time.Now().UTC(),
	})
	if eventType != "" {
		h.publishIncidentEvent(ctx, eventType, updated)
	}
	writeJSON(w, http.StatusOK, updated)
}

// AssignRole handles POST /api/v1/emergency/incidents/{id}/assign-role.
func (h *EmergencyHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.getIncidentByID(ctx, id, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve incident"})
		return
	}

	var req incidentAssignRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.CIMSRole == "" || req.PrincipalID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELDS", Message: "cimsRole and principalId are required"})
		return
	}

	// Relieve any currently active holder of this role before assigning the new one.
	_ = h.relieveActiveRoleHolder(ctx, id, tenantID.String(), req.CIMSRole)

	assignment, err := h.insertCommandAssignment(ctx, id, tenantID.String(), req.CIMSRole, req.PrincipalID, principal.ID)
	if err != nil {
		h.logger.Error("assign CIMS role", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ASSIGN_ERROR", Message: "failed to assign role"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "assign_role", ResourceType: "EmergencyIncident",
		ResourceID: id, TenantID: tenantID,
		Details:    map[string]any{"role": string(req.CIMSRole), "assignee": req.PrincipalID},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, assignment)
}

// ListLog handles GET /api/v1/emergency/incidents/{id}/log.
func (h *EmergencyHandler) ListLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	id := r.PathValue("id")
	entries, err := h.listLogEntries(ctx, id, tenantID.String())
	if err != nil {
		h.logger.Error("list incident log", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list log entries"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "total": len(entries)})
}

// AddLog handles POST /api/v1/emergency/incidents/{id}/log.
func (h *EmergencyHandler) AddLog(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.getIncidentByID(ctx, id, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve incident"})
		return
	}

	var req incidentLogRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Message == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_MESSAGE", Message: "message is required"})
		return
	}
	if req.Category == "" {
		req.Category = "general"
	}

	entry, err := h.insertLogEntry(ctx, id, tenantID.String(), principal.ID, req.Category, req.Message)
	if err != nil {
		h.logger.Error("add incident log entry", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to add log entry"})
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

// ListResources handles GET /api/v1/emergency/incidents/{id}/resources.
func (h *EmergencyHandler) ListResources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	id := r.PathValue("id")
	statusFilter := r.URL.Query().Get("status")
	requests, err := h.listResourceRequests(ctx, id, tenantID.String(), statusFilter)
	if err != nil {
		h.logger.Error("list resource requests", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list resource requests"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"requests": requests, "total": len(requests)})
}

// RequestResource handles POST /api/v1/emergency/incidents/{id}/resources.
func (h *EmergencyHandler) RequestResource(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.getIncidentByID(ctx, id, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve incident"})
		return
	}

	var req resourceRequestCreate
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Description == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DESCRIPTION", Message: "description is required"})
		return
	}
	if req.Category == "" {
		req.Category = "other"
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}

	rr, err := h.insertResourceRequest(ctx, id, tenantID.String(), req, principal.ID)
	if err != nil {
		h.logger.Error("request resource", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create resource request"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "ResourceRequest",
		ResourceID: rr.ID, TenantID: tenantID,
		Details:    map[string]any{"category": req.Category, "qty": req.Quantity},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, rr)
}

// UpdateResource handles PATCH /api/v1/emergency/incidents/{id}/resources/{rid}.
func (h *EmergencyHandler) UpdateResource(w http.ResponseWriter, r *http.Request) {
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

	incidentID := r.PathValue("id")
	rid := r.PathValue("rid")

	var req resourceRequestUpdate
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Status == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_STATUS", Message: "status is required"})
		return
	}

	rr, err := h.updateResourceRequest(ctx, rid, incidentID, tenantID.String(), req, principal.ID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "resource request not found"})
			return
		}
		h.logger.Error("update resource request", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update resource request"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "ResourceRequest",
		ResourceID: rid, TenantID: tenantID,
		Details:    map[string]any{"status": string(req.Status)},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, rr)
}
