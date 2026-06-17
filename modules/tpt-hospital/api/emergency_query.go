package api

import (
	"context"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/events"
	"github.com/google/uuid"
)

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
			"incident_id":  incidentID,
			"tenant_id":    tenantID,
			"category":     req.Category,
			"description":  req.Description,
			"quantity":     req.Quantity,
			"priority":     req.Priority,
			"requested_by": principalID,
			"notes":        req.Notes,
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
