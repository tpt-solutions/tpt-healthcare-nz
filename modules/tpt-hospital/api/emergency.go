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
	"github.com/PhillipC05/tpt-healthcare/core/events"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
)

// IncidentType classifies the nature of the emergency.
type IncidentType string

const (
	IncidentTypeMCI      IncidentType = "mci"
	IncidentTypeCBRN     IncidentType = "cbrn"
	IncidentTypeFire     IncidentType = "fire"
	IncidentTypeFlood    IncidentType = "flood"
	IncidentTypeCyber    IncidentType = "cyber"
	IncidentTypePandemic IncidentType = "pandemic"
	IncidentTypeOther    IncidentType = "other"
)

// IncidentStatus is the CIMS lifecycle state of an incident.
type IncidentStatus string

const (
	IncidentDeclared  IncidentStatus = "declared"
	IncidentActivated IncidentStatus = "activated"
	IncidentEscalated IncidentStatus = "escalated"
	IncidentStandDown IncidentStatus = "stand_down"
	IncidentClosed    IncidentStatus = "closed"
)

// CIMSRole enumerates the standard NZ CIMS (Coordinated Incident Management System) roles.
type CIMSRole string

const (
	CIMSRoleIC               CIMSRole = "incident_commander"
	CIMSRoleDeputyIC         CIMSRole = "deputy_ic"
	CIMSRoleSafetyOfficer    CIMSRole = "safety_officer"
	CIMSRoleOpsChief         CIMSRole = "operations_chief"
	CIMSRoleLogisticsChief   CIMSRole = "logistics_chief"
	CIMSRolePlanningChief    CIMSRole = "planning_chief"
	CIMSRoleFinanceChief     CIMSRole = "finance_chief"
	CIMSRoleMedicalDirector  CIMSRole = "medical_director"
	CIMSRoleLiaison          CIMSRole = "liaison_officer"
	CIMSRolePIO              CIMSRole = "public_info_officer"
	CIMSRoleZoneLeader       CIMSRole = "zone_leader"
)

// EmergencyIncident is the top-level CIMS incident record.
type EmergencyIncident struct {
	ID             string         `json:"id"`
	TenantID       string         `json:"tenantId"`
	Title          string         `json:"title"`
	Type           IncidentType   `json:"type"`
	Status         IncidentStatus `json:"status"`
	Description    string         `json:"description,omitempty"`
	Location       string         `json:"location,omitempty"`
	CBRNAgent      string         `json:"cbrnAgent,omitempty"`
	SurgeLevel     int            `json:"surgeLevel"`
	DeclaredBy     string         `json:"declaredBy"`
	ICPrincipalID  string         `json:"icPrincipalId,omitempty"`
	DeclaredAt     time.Time      `json:"declaredAt"`
	ActivatedAt    *time.Time     `json:"activatedAt,omitempty"`
	EscalatedAt    *time.Time     `json:"escalatedAt,omitempty"`
	StandDownAt    *time.Time     `json:"standDownAt,omitempty"`
	ClosedAt       *time.Time     `json:"closedAt,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// IncidentCommandAssignment records who holds a CIMS role during an incident.
type IncidentCommandAssignment struct {
	ID          string     `json:"id"`
	IncidentID  string     `json:"incidentId"`
	TenantID    string     `json:"tenantId"`
	CIMSRole    CIMSRole   `json:"cimsRole"`
	PrincipalID string     `json:"principalId"`
	AssignedBy  string     `json:"assignedBy"`
	AssignedAt  time.Time  `json:"assignedAt"`
	RelievedAt  *time.Time `json:"relievedAt,omitempty"`
}

// IncidentLogEntry is an append-only command-log record (for post-incident debrief).
type IncidentLogEntry struct {
	ID         string    `json:"id"`
	IncidentID string    `json:"incidentId"`
	TenantID   string    `json:"tenantId"`
	AuthorID   string    `json:"authorId"`
	Category   string    `json:"category"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ResourceRequest tracks equipment/staff/supply requests during an incident.
type ResourceRequest struct {
	ID          string                `json:"id"`
	IncidentID  string                `json:"incidentId"`
	TenantID    string                `json:"tenantId"`
	Category    string                `json:"category"`
	Description string                `json:"description"`
	Quantity    int                   `json:"quantity"`
	Status      ResourceRequestStatus `json:"status"`
	Priority    string                `json:"priority"`
	RequestedBy string                `json:"requestedBy"`
	FulfilledBy string                `json:"fulfilledBy,omitempty"`
	Notes       string                `json:"notes,omitempty"`
	RequestedAt time.Time             `json:"requestedAt"`
	FulfilledAt *time.Time            `json:"fulfilledAt,omitempty"`
	UpdatedAt   time.Time             `json:"updatedAt"`
}

// ResourceRequestStatus is the lifecycle state of a resource request.
type ResourceRequestStatus string

const (
	ResourceRequested  ResourceRequestStatus = "requested"
	ResourceFulfilled  ResourceRequestStatus = "fulfilled"
	ResourceCancelled  ResourceRequestStatus = "cancelled"
)

// request / response types ────────────────────────────────────────────────────

type incidentDeclareRequest struct {
	Title       string       `json:"title"`
	Type        IncidentType `json:"type"`
	Description string       `json:"description,omitempty"`
	Location    string       `json:"location,omitempty"`
	CBRNAgent   string       `json:"cbrnAgent,omitempty"`
}

type incidentAssignRoleRequest struct {
	CIMSRole    CIMSRole `json:"cimsRole"`
	PrincipalID string   `json:"principalId"`
}

type incidentLogRequest struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

type resourceRequestCreate struct {
	Category    string `json:"category"`
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	Priority    string `json:"priority"`
	Notes       string `json:"notes,omitempty"`
}

type resourceRequestUpdate struct {
	Status      ResourceRequestStatus `json:"status"`
	FulfilledBy string                `json:"fulfilledBy,omitempty"`
	Notes       string                `json:"notes,omitempty"`
}

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

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *EmergencyHandler) insertIncident(ctx context.Context, req incidentDeclareRequest, principalID, tenantID string) (EmergencyIncident, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO emergency_incidents
		   (tenant_id, title, type, description, location, cbrn_agent, declared_by)
		 VALUES
		   (@tenant_id, @title, @type, @description, @location, @cbrn_agent, @declared_by)
		 RETURNING id, tenant_id, title, type, status, description, location, cbrn_agent,
		           surge_level, declared_by, ic_principal_id,
		           declared_at, activated_at, escalated_at, stand_down_at, closed_at,
		           created_at, updated_at`,
		db.NamedArgs{
			"tenant_id":   tenantID,
			"title":       req.Title,
			"type":        req.Type,
			"description": req.Description,
			"location":    req.Location,
			"cbrn_agent":  req.CBRNAgent,
			"declared_by": principalID,
		},
	)
	return scanIncidentRow(row)
}

func (h *EmergencyHandler) listIncidents(ctx context.Context, tenantID, statusFilter string) ([]EmergencyIncident, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, title, type, status, description, location, cbrn_agent,
		        surge_level, declared_by, ic_principal_id,
		        declared_at, activated_at, escalated_at, stand_down_at, closed_at,
		        created_at, updated_at
		 FROM emergency_incidents
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter = '' OR status::text = @status_filter)
		 ORDER BY declared_at DESC`,
		db.NamedArgs{"tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query incidents: %w", err)
	}
	defer rows.Close()

	var results []EmergencyIncident
	for rows.Next() {
		inc, err := scanIncidentRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, inc)
	}
	return results, rows.Err()
}

func (h *EmergencyHandler) getIncidentByID(ctx context.Context, id, tenantID string) (EmergencyIncident, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, title, type, status, description, location, cbrn_agent,
		        surge_level, declared_by, ic_principal_id,
		        declared_at, activated_at, escalated_at, stand_down_at, closed_at,
		        created_at, updated_at
		 FROM emergency_incidents
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	inc, err := scanIncidentRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return EmergencyIncident{}, errNotFound
		}
		return EmergencyIncident{}, fmt.Errorf("get incident: %w", err)
	}
	return inc, nil
}

func (h *EmergencyHandler) setIncidentStatus(ctx context.Context, id, tenantID string, status IncidentStatus) (EmergencyIncident, error) {
	now := time.Now().UTC()
	var tsCol string
	switch status {
	case IncidentActivated:
		tsCol = "activated_at = @ts,"
	case IncidentEscalated:
		tsCol = "escalated_at = @ts,"
	case IncidentStandDown:
		tsCol = "stand_down_at = @ts,"
	case IncidentClosed:
		tsCol = "closed_at = @ts,"
	}

	query := fmt.Sprintf(
		`UPDATE emergency_incidents
		 SET status = @status, %s updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, tenant_id, title, type, status, description, location, cbrn_agent,
		           surge_level, declared_by, ic_principal_id,
		           declared_at, activated_at, escalated_at, stand_down_at, closed_at,
		           created_at, updated_at`,
		tsCol,
	)
	row := h.pool.QueryRow(ctx, query, db.NamedArgs{
		"status":    status,
		"ts":        now,
		"id":        id,
		"tenant_id": tenantID,
	})
	inc, err := scanIncidentRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return EmergencyIncident{}, errNotFound
		}
		return EmergencyIncident{}, fmt.Errorf("set incident status: %w", err)
	}
	return inc, nil
}

func (h *EmergencyHandler) relieveActiveRoleHolder(ctx context.Context, incidentID, tenantID string, role CIMSRole) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE incident_command_assignments
		 SET relieved_at = now()
		 WHERE incident_id = @incident_id AND tenant_id = @tenant_id
		   AND cims_role = @role AND relieved_at IS NULL`,
		db.NamedArgs{"incident_id": incidentID, "tenant_id": tenantID, "role": role},
	)
	return err
}

func (h *EmergencyHandler) insertCommandAssignment(ctx context.Context, incidentID, tenantID string, role CIMSRole, principalID, assignedBy string) (IncidentCommandAssignment, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO incident_command_assignments
		   (incident_id, tenant_id, cims_role, principal_id, assigned_by)
		 VALUES (@incident_id, @tenant_id, @role, @principal_id, @assigned_by)
		 RETURNING id, incident_id, tenant_id, cims_role, principal_id, assigned_by, assigned_at, relieved_at`,
		db.NamedArgs{
			"incident_id":  incidentID,
			"tenant_id":    tenantID,
			"role":         role,
			"principal_id": principalID,
			"assigned_by":  assignedBy,
		},
	)
	return scanAssignmentRow(row)
}

func (h *EmergencyHandler) listCommandAssignments(ctx context.Context, incidentID, tenantID string) ([]IncidentCommandAssignment, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, incident_id, tenant_id, cims_role, principal_id, assigned_by, assigned_at, relieved_at
		 FROM incident_command_assignments
		 WHERE incident_id = @incident_id AND tenant_id = @tenant_id
		 ORDER BY assigned_at ASC`,
		db.NamedArgs{"incident_id": incidentID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query command assignments: %w", err)
	}
	defer rows.Close()

	var results []IncidentCommandAssignment
	for rows.Next() {
		a, err := scanAssignmentRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

func (h *EmergencyHandler) insertLogEntry(ctx context.Context, incidentID, tenantID, authorID, category, message string) (IncidentLogEntry, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO incident_log_entries (incident_id, tenant_id, author_id, category, message)
		 VALUES (@incident_id, @tenant_id, @author_id, @category, @message)
		 RETURNING id, incident_id, tenant_id, author_id, category, message, created_at`,
		db.NamedArgs{
			"incident_id": incidentID,
			"tenant_id":   tenantID,
			"author_id":   authorID,
			"category":    category,
			"message":     message,
		},
	)
	var e IncidentLogEntry
	if err := row.Scan(&e.ID, &e.IncidentID, &e.TenantID, &e.AuthorID, &e.Category, &e.Message, &e.CreatedAt); err != nil {
		return IncidentLogEntry{}, fmt.Errorf("insert log entry: %w", err)
	}
	return e, nil
}

func (h *EmergencyHandler) listLogEntries(ctx context.Context, incidentID, tenantID string) ([]IncidentLogEntry, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, incident_id, tenant_id, author_id, category, message, created_at
		 FROM incident_log_entries
		 WHERE incident_id = @incident_id AND tenant_id = @tenant_id
		 ORDER BY created_at ASC`,
		db.NamedArgs{"incident_id": incidentID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query log entries: %w", err)
	}
	defer rows.Close()

	var results []IncidentLogEntry
	for rows.Next() {
		var e IncidentLogEntry
		if err := rows.Scan(&e.ID, &e.IncidentID, &e.TenantID, &e.AuthorID, &e.Category, &e.Message, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan log entry: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (h *EmergencyHandler) listResourceRequests(ctx context.Context, incidentID, tenantID, statusFilter string) ([]ResourceRequest, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, incident_id, tenant_id, category, description, quantity, status, priority,
		        requested_by, fulfilled_by, notes, requested_at, fulfilled_at, updated_at
		 FROM resource_requests
		 WHERE incident_id = @incident_id AND tenant_id = @tenant_id
		   AND (@status_filter = '' OR status::text = @status_filter)
		 ORDER BY requested_at DESC`,
		db.NamedArgs{"incident_id": incidentID, "tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query resource requests: %w", err)
	}
	defer rows.Close()

	var results []ResourceRequest
	for rows.Next() {
		rr, err := scanResourceRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rr)
	}
	return results, rows.Err()
}

func (h *EmergencyHandler) insertResourceRequest(ctx context.Context, incidentID, tenantID string, req resourceRequestCreate, principalID string) (ResourceRequest, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO resource_requests
		   (incident_id, tenant_id, category, description, quantity, priority, requested_by, notes)
		 VALUES (@incident_id, @tenant_id, @category, @description, @quantity, @priority, @requested_by, @notes)
		 RETURNING id, incident_id, tenant_id, category, description, quantity, status, priority,
		           requested_by, fulfilled_by, notes, requested_at, fulfilled_at, updated_at`,
		db.NamedArgs{
			"incident_id":   incidentID,
			"tenant_id":     tenantID,
			"category":      req.Category,
			"description":   req.Description,
			"quantity":      req.Quantity,
			"priority":      req.Priority,
			"requested_by":  principalID,
			"notes":         req.Notes,
		},
	)
	return scanResourceRow(row)
}

func (h *EmergencyHandler) updateResourceRequest(ctx context.Context, rid, incidentID, tenantID string, req resourceRequestUpdate, principalID string) (ResourceRequest, error) {
	var fulfilledAt interface{} = nil
	if req.Status == ResourceFulfilled {
		now := time.Now().UTC()
		fulfilledAt = now
	}
	fulfilledBy := req.FulfilledBy
	if fulfilledBy == "" && req.Status == ResourceFulfilled {
		fulfilledBy = principalID
	}

	row := h.pool.QueryRow(ctx,
		`UPDATE resource_requests
		 SET status       = @status,
		     fulfilled_by = COALESCE(NULLIF(@fulfilled_by, ''), fulfilled_by),
		     fulfilled_at = COALESCE(@fulfilled_at, fulfilled_at),
		     notes        = COALESCE(NULLIF(@notes, ''), notes),
		     updated_at   = now()
		 WHERE id = @id AND incident_id = @incident_id AND tenant_id = @tenant_id
		 RETURNING id, incident_id, tenant_id, category, description, quantity, status, priority,
		           requested_by, fulfilled_by, notes, requested_at, fulfilled_at, updated_at`,
		db.NamedArgs{
			"status":       req.Status,
			"fulfilled_by": fulfilledBy,
			"fulfilled_at": fulfilledAt,
			"notes":        req.Notes,
			"id":           rid,
			"incident_id":  incidentID,
			"tenant_id":    tenantID,
		},
	)
	rr, err := scanResourceRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ResourceRequest{}, errNotFound
		}
		return ResourceRequest{}, fmt.Errorf("update resource request: %w", err)
	}
	return rr, nil
}

func (h *EmergencyHandler) publishIncidentEvent(ctx context.Context, eventType string, incident EmergencyIncident) {
	if h.eventBus == nil {
		return
	}
	tid, _ := uuid.Parse(incident.TenantID)
	h.eventBus.PublishAsync(ctx, events.Event{
		ID:            uuid.New(),
		Type:          eventType,
		AggregateID:   incident.ID,
		AggregateType: "EmergencyIncident",
		TenantID:      tid,
		Payload:       incident,
		OccurredAt:    time.Now().UTC(),
	})
}

// ── Scan helpers ──────────────────────────────────────────────────────────────

func scanIncidentRow(row dbRow) (EmergencyIncident, error) {
	var inc EmergencyIncident
	if err := row.Scan(
		&inc.ID, &inc.TenantID, &inc.Title, &inc.Type, &inc.Status,
		&inc.Description, &inc.Location, &inc.CBRNAgent,
		&inc.SurgeLevel, &inc.DeclaredBy, &inc.ICPrincipalID,
		&inc.DeclaredAt, &inc.ActivatedAt, &inc.EscalatedAt, &inc.StandDownAt, &inc.ClosedAt,
		&inc.CreatedAt, &inc.UpdatedAt,
	); err != nil {
		return EmergencyIncident{}, err
	}
	return inc, nil
}

func scanAssignmentRow(row dbRow) (IncidentCommandAssignment, error) {
	var a IncidentCommandAssignment
	if err := row.Scan(
		&a.ID, &a.IncidentID, &a.TenantID, &a.CIMSRole, &a.PrincipalID,
		&a.AssignedBy, &a.AssignedAt, &a.RelievedAt,
	); err != nil {
		return IncidentCommandAssignment{}, err
	}
	return a, nil
}

func scanResourceRow(row dbRow) (ResourceRequest, error) {
	var rr ResourceRequest
	if err := row.Scan(
		&rr.ID, &rr.IncidentID, &rr.TenantID, &rr.Category, &rr.Description,
		&rr.Quantity, &rr.Status, &rr.Priority, &rr.RequestedBy, &rr.FulfilledBy,
		&rr.Notes, &rr.RequestedAt, &rr.FulfilledAt, &rr.UpdatedAt,
	); err != nil {
		return ResourceRequest{}, err
	}
	return rr, nil
}
