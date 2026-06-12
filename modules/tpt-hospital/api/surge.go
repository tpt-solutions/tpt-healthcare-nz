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

// SurgeLevel defines the hospital surge tier.
// Mirrors the NZ Health Emergency Planning Guide tier definitions.
type SurgeLevel int

const (
	SurgeLevelNormal        SurgeLevel = 0 // business as usual
	SurgeLevelExpanded      SurgeLevel = 1 // surge beds activated, electives deferred
	SurgeLevelCrisis        SurgeLevel = 2 // discharge acceleration, overflow areas open
	SurgeLevelCatastrophic  SurgeLevel = 3 // crisis standards of care; HEOC notification required
)

// SurgeLevelName returns a human-readable label for a surge level.
func SurgeLevelName(l SurgeLevel) string {
	switch l {
	case SurgeLevelExpanded:
		return "expanded"
	case SurgeLevelCrisis:
		return "crisis"
	case SurgeLevelCatastrophic:
		return "catastrophic"
	default:
		return "normal"
	}
}

// SurgeCapacitySnapshot is a point-in-time hospital capacity record taken during an incident.
type SurgeCapacitySnapshot struct {
	ID                  string     `json:"id"`
	IncidentID          string     `json:"incidentId"`
	TenantID            string     `json:"tenantId"`
	SurgeLevel          SurgeLevel `json:"surgeLevel"`
	SurgeLevelName      string     `json:"surgeLevelName"`
	TotalBeds           int        `json:"totalBeds"`
	OccupiedBeds        int        `json:"occupiedBeds"`
	SurgeBedsActivated  int        `json:"surgeBedsActivated"`
	ICUTotal            int        `json:"icuTotal"`
	ICUOccupied         int        `json:"icuOccupied"`
	EDWaiting           int        `json:"edWaiting"`
	RecordedBy          string     `json:"recordedBy"`
	RecordedAt          time.Time  `json:"recordedAt"`
}

// SurgeStatus is the current surge state with the most recent snapshot.
type SurgeStatus struct {
	IncidentID     string                 `json:"incidentId"`
	CurrentLevel   SurgeLevel             `json:"currentLevel"`
	LevelName      string                 `json:"levelName"`
	LatestSnapshot *SurgeCapacitySnapshot `json:"latestSnapshot,omitempty"`
}

type surgeSnapshotRequest struct {
	SurgeBedsActivated int `json:"surgeBedsActivated"`
	Notes              string `json:"notes,omitempty"`
}

// SurgeHandler handles all /api/v1/emergency/incidents/{id}/surge routes.
type SurgeHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	eventBus   *events.Bus
	logger     *slog.Logger
}

// GetSurgeStatus handles GET /api/v1/emergency/incidents/{id}/surge.
func (h *SurgeHandler) GetSurgeStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	incidentID := r.PathValue("id")
	level, err := h.currentSurgeLevel(ctx, incidentID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		h.logger.Error("get surge level", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve surge status"})
		return
	}

	latest, _ := h.latestSnapshot(ctx, incidentID, tenantID.String())
	status := SurgeStatus{
		IncidentID:   incidentID,
		CurrentLevel: level,
		LevelName:    SurgeLevelName(level),
	}
	if latest != nil {
		status.LatestSnapshot = latest
	}
	writeJSON(w, http.StatusOK, status)
}

// RecordSnapshot handles POST /api/v1/emergency/incidents/{id}/surge/snapshot.
// Queries live bed/ICU/ED counts from the existing tables and stores the snapshot.
func (h *SurgeHandler) RecordSnapshot(w http.ResponseWriter, r *http.Request) {
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

	var req surgeSnapshotRequest
	// body is optional — an empty body is valid (auto-queries live counts)
	_ = decodeJSON(r, &req)

	level, err := h.currentSurgeLevel(ctx, incidentID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve incident"})
		return
	}

	// Query live capacity from existing ward/ICU/ED tables.
	counts, err := h.queryLiveCounts(ctx, tenantID.String())
	if err != nil {
		h.logger.Error("query live capacity", slog.Any("error", err))
		// Non-fatal — record snapshot with zero counts rather than fail.
		counts = liveCounts{}
	}
	if req.SurgeBedsActivated > 0 {
		counts.surgeBedsActivated = req.SurgeBedsActivated
	}

	snap, err := h.insertSnapshot(ctx, incidentID, tenantID.String(), level, counts, principal.ID)
	if err != nil {
		h.logger.Error("record surge snapshot", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to record snapshot"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "SurgeCapacitySnapshot",
		ResourceID: snap.ID, TenantID: tenantID,
		Details:    map[string]any{"level": int(level), "occupied": counts.occupied},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, snap)
}

// EscalateSurge handles POST /api/v1/emergency/incidents/{id}/surge/escalate.
func (h *SurgeHandler) EscalateSurge(w http.ResponseWriter, r *http.Request) {
	h.changeSurgeLevel(w, r, 1)
}

// DeEscalateSurge handles POST /api/v1/emergency/incidents/{id}/surge/de-escalate.
func (h *SurgeHandler) DeEscalateSurge(w http.ResponseWriter, r *http.Request) {
	h.changeSurgeLevel(w, r, -1)
}

func (h *SurgeHandler) changeSurgeLevel(w http.ResponseWriter, r *http.Request, delta int) {
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
	current, err := h.currentSurgeLevel(ctx, incidentID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve incident"})
		return
	}

	newLevel := SurgeLevel(int(current) + delta)
	if newLevel < SurgeLevelNormal {
		newLevel = SurgeLevelNormal
	}
	if newLevel > SurgeLevelCatastrophic {
		writeJSON(w, http.StatusConflict, apiError{Code: "MAX_SURGE", Message: "already at catastrophic surge level"})
		return
	}

	if err := h.setSurgeLevel(ctx, incidentID, tenantID.String(), newLevel); err != nil {
		h.logger.Error("set surge level", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update surge level"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "EmergencyIncident",
		ResourceID: incidentID, TenantID: tenantID,
		Details:    map[string]any{"surge_level": int(newLevel), "level_name": SurgeLevelName(newLevel)},
		OccurredAt: time.Now().UTC(),
	})
	h.publishSurgeEvent(ctx, incidentID, tenantID.String(), newLevel)

	writeJSON(w, http.StatusOK, map[string]any{
		"incidentId": incidentID,
		"surgeLevel": int(newLevel),
		"levelName":  SurgeLevelName(newLevel),
	})
}

// ListSnapshots handles GET /api/v1/emergency/incidents/{id}/surge/history.
func (h *SurgeHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	incidentID := r.PathValue("id")
	snapshots, err := h.listSnapshots(ctx, incidentID, tenantID.String())
	if err != nil {
		h.logger.Error("list surge snapshots", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list snapshots"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": snapshots, "total": len(snapshots)})
}

// ── DB helpers ────────────────────────────────────────────────────────────────

type liveCounts struct {
	total              int
	occupied           int
	surgeBedsActivated int
	icuTotal           int
	icuOccupied        int
	edWaiting          int
}

func (h *SurgeHandler) queryLiveCounts(ctx context.Context, tenantID string) (liveCounts, error) {
	var c liveCounts

	// Ward bed counts from existing wards schema.
	_ = h.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_beds), 0), COALESCE(SUM(occupied_beds), 0)
		 FROM wards WHERE tenant_id = @tenant_id`,
		db.NamedArgs{"tenant_id": tenantID},
	).Scan(&c.total, &c.occupied)

	// ICU counts.
	_ = h.pool.QueryRow(ctx,
		`SELECT COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'active') AS occupied
		 FROM icu_admissions WHERE tenant_id = @tenant_id`,
		db.NamedArgs{"tenant_id": tenantID},
	).Scan(&c.icuTotal, &c.icuOccupied)

	// ED waiting (arrived but not disposed).
	_ = h.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM ed_presentations
		 WHERE tenant_id = @tenant_id AND status != 'disposed'`,
		db.NamedArgs{"tenant_id": tenantID},
	).Scan(&c.edWaiting)

	return c, nil
}

func (h *SurgeHandler) currentSurgeLevel(ctx context.Context, incidentID, tenantID string) (SurgeLevel, error) {
	var level int
	err := h.pool.QueryRow(ctx,
		`SELECT surge_level FROM emergency_incidents WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": incidentID, "tenant_id": tenantID},
	).Scan(&level)
	if err != nil {
		if db.IsNoRows(err) {
			return 0, errNotFound
		}
		return 0, fmt.Errorf("get surge level: %w", err)
	}
	return SurgeLevel(level), nil
}

func (h *SurgeHandler) setSurgeLevel(ctx context.Context, incidentID, tenantID string, level SurgeLevel) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE emergency_incidents SET surge_level = @level, updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"level": int(level), "id": incidentID, "tenant_id": tenantID},
	)
	return err
}

func (h *SurgeHandler) insertSnapshot(ctx context.Context, incidentID, tenantID string, level SurgeLevel, c liveCounts, recordedBy string) (SurgeCapacitySnapshot, error) {
	var snap SurgeCapacitySnapshot
	err := h.pool.QueryRow(ctx,
		`INSERT INTO surge_capacity_snapshots
		   (incident_id, tenant_id, surge_level, total_beds, occupied_beds, surge_beds_activated,
		    icu_total, icu_occupied, ed_waiting, recorded_by)
		 VALUES
		   (@incident_id, @tenant_id, @surge_level, @total_beds, @occupied_beds, @surge_beds_activated,
		    @icu_total, @icu_occupied, @ed_waiting, @recorded_by)
		 RETURNING id, incident_id, tenant_id, surge_level, total_beds, occupied_beds,
		           surge_beds_activated, icu_total, icu_occupied, ed_waiting, recorded_by, recorded_at`,
		db.NamedArgs{
			"incident_id":          incidentID,
			"tenant_id":            tenantID,
			"surge_level":          int(level),
			"total_beds":           c.total,
			"occupied_beds":        c.occupied,
			"surge_beds_activated": c.surgeBedsActivated,
			"icu_total":            c.icuTotal,
			"icu_occupied":         c.icuOccupied,
			"ed_waiting":           c.edWaiting,
			"recorded_by":          recordedBy,
		},
	).Scan(
		&snap.ID, &snap.IncidentID, &snap.TenantID, &snap.SurgeLevel,
		&snap.TotalBeds, &snap.OccupiedBeds, &snap.SurgeBedsActivated,
		&snap.ICUTotal, &snap.ICUOccupied, &snap.EDWaiting,
		&snap.RecordedBy, &snap.RecordedAt,
	)
	if err != nil {
		return SurgeCapacitySnapshot{}, fmt.Errorf("insert surge snapshot: %w", err)
	}
	snap.SurgeLevelName = SurgeLevelName(snap.SurgeLevel)
	return snap, nil
}

func (h *SurgeHandler) latestSnapshot(ctx context.Context, incidentID, tenantID string) (*SurgeCapacitySnapshot, error) {
	var snap SurgeCapacitySnapshot
	err := h.pool.QueryRow(ctx,
		`SELECT id, incident_id, tenant_id, surge_level, total_beds, occupied_beds,
		        surge_beds_activated, icu_total, icu_occupied, ed_waiting, recorded_by, recorded_at
		 FROM surge_capacity_snapshots
		 WHERE incident_id = @incident_id AND tenant_id = @tenant_id
		 ORDER BY recorded_at DESC LIMIT 1`,
		db.NamedArgs{"incident_id": incidentID, "tenant_id": tenantID},
	).Scan(
		&snap.ID, &snap.IncidentID, &snap.TenantID, &snap.SurgeLevel,
		&snap.TotalBeds, &snap.OccupiedBeds, &snap.SurgeBedsActivated,
		&snap.ICUTotal, &snap.ICUOccupied, &snap.EDWaiting,
		&snap.RecordedBy, &snap.RecordedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("latest snapshot: %w", err)
	}
	snap.SurgeLevelName = SurgeLevelName(snap.SurgeLevel)
	return &snap, nil
}

func (h *SurgeHandler) listSnapshots(ctx context.Context, incidentID, tenantID string) ([]SurgeCapacitySnapshot, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, incident_id, tenant_id, surge_level, total_beds, occupied_beds,
		        surge_beds_activated, icu_total, icu_occupied, ed_waiting, recorded_by, recorded_at
		 FROM surge_capacity_snapshots
		 WHERE incident_id = @incident_id AND tenant_id = @tenant_id
		 ORDER BY recorded_at DESC`,
		db.NamedArgs{"incident_id": incidentID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query snapshots: %w", err)
	}
	defer rows.Close()

	var results []SurgeCapacitySnapshot
	for rows.Next() {
		var snap SurgeCapacitySnapshot
		if err := rows.Scan(
			&snap.ID, &snap.IncidentID, &snap.TenantID, &snap.SurgeLevel,
			&snap.TotalBeds, &snap.OccupiedBeds, &snap.SurgeBedsActivated,
			&snap.ICUTotal, &snap.ICUOccupied, &snap.EDWaiting,
			&snap.RecordedBy, &snap.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		snap.SurgeLevelName = SurgeLevelName(snap.SurgeLevel)
		results = append(results, snap)
	}
	return results, rows.Err()
}

func (h *SurgeHandler) publishSurgeEvent(ctx context.Context, incidentID, tenantID string, level SurgeLevel) {
	if h.eventBus == nil {
		return
	}
	tid, _ := uuid.Parse(tenantID)
	h.eventBus.PublishAsync(ctx, events.Event{
		ID:            uuid.New(),
		Type:          events.EventSurgeLevelChanged,
		AggregateID:   incidentID,
		AggregateType: "EmergencyIncident",
		TenantID:      tid,
		Payload: map[string]any{
			"incidentId": incidentID,
			"surgeLevel": int(level),
			"levelName":  SurgeLevelName(level),
		},
		OccurredAt: time.Now().UTC(),
	})
}
