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
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// WardType classifies the clinical function of a ward.
type WardType string

const (
	WardTypeGeneral    WardType = "general"
	WardTypeICU        WardType = "icu"
	WardTypeED         WardType = "ed"
	WardTypeTheatre    WardType = "theatre"
	WardTypeMaternity  WardType = "maternity"
	WardTypePaediatric WardType = "paediatric"
	WardTypeNICU       WardType = "nicu"
	WardTypeOncology   WardType = "oncology"
	WardTypeRehab      WardType = "rehabilitation"
	WardTypeRehab2     WardType = "renal"
	WardTypePsychiatry WardType = "psychiatry"
	WardTypeSurgical   WardType = "surgical"
	WardTypeMedical    WardType = "medical"
	WardTypeCardiac    WardType = "cardiac"
	WardTypeOrtho      WardType = "orthopaedic"
)

// BedStatus tracks the real-time availability of a hospital bed.
type BedStatus string

const (
	BedStatusAvailable   BedStatus = "available"
	BedStatusOccupied    BedStatus = "occupied"
	BedStatusCleaning    BedStatus = "cleaning"
	BedStatusMaintenance BedStatus = "maintenance"
	BedStatusBlocked     BedStatus = "blocked"
)

// Ward represents a hospital ward or clinical unit.
type Ward struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Code          string    `json:"code"` // e.g. "GW1", "ICU", "ED"
	WardType      WardType  `json:"wardType"`
	Floor         string    `json:"floor,omitempty"`
	Building      string    `json:"building,omitempty"`
	TotalBeds     int       `json:"totalBeds"`
	AvailableBeds int       `json:"availableBeds"`
	TenantID      string    `json:"tenantId"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Bed represents a single hospital bed within a ward.
type Bed struct {
	ID          string    `json:"id"`
	WardID      string    `json:"wardId"`
	BedNumber   string    `json:"bedNumber"` // e.g. "1A", "12"
	Status      BedStatus `json:"status"`
	AdmissionID string    `json:"admissionId,omitempty"` // set when occupied
	TenantID    string    `json:"tenantId"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// HospitalCapacitySnapshot is a real-time summary of hospital-wide bed capacity.
type HospitalCapacitySnapshot struct {
	TotalBeds     int            `json:"totalBeds"`
	OccupiedBeds  int            `json:"occupiedBeds"`
	AvailableBeds int            `json:"availableBeds"`
	ByWardType    map[string]int `json:"byWardType"`
	GeneratedAt   time.Time      `json:"generatedAt"`
}

type bedUpdateRequest struct {
	Status      BedStatus `json:"status"`
	AdmissionID string    `json:"admissionId,omitempty"`
}

// WardsHandler handles all /api/v1/wards routes.
type WardsHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListWards handles GET /api/v1/wards.
func (h *WardsHandler) ListWards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	wardType := r.URL.Query().Get("type")
	wards, err := h.listWards(ctx, tenantID.String(), wardType)
	if err != nil {
		h.logger.Error("list wards", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list wards"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"wards": wards, "total": len(wards)})
}

// GetWard handles GET /api/v1/wards/{wardId}.
func (h *WardsHandler) GetWard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	wardID := r.PathValue("wardId")
	ward, err := h.getWardByID(ctx, wardID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ward not found"})
			return
		}
		h.logger.Error("get ward", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve ward"})
		return
	}
	writeJSON(w, http.StatusOK, ward)
}

// ListBeds handles GET /api/v1/wards/{wardId}/beds.
func (h *WardsHandler) ListBeds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	wardID := r.PathValue("wardId")
	statusFilter := r.URL.Query().Get("status")
	beds, err := h.listBeds(ctx, wardID, tenantID.String(), statusFilter)
	if err != nil {
		h.logger.Error("list beds", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list beds"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"beds": beds, "total": len(beds)})
}

// UpdateBed handles PUT /api/v1/wards/{wardId}/beds/{bedId}.
func (h *WardsHandler) UpdateBed(w http.ResponseWriter, r *http.Request) {
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

	wardID := r.PathValue("wardId")
	bedID := r.PathValue("bedId")

	var req bedUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	bed, err := h.updateBed(ctx, wardID, bedID, req, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "bed not found"})
			return
		}
		h.logger.Error("update bed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update bed"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "Bed",
		ResourceID: bedID, TenantID: tenantID,
		Details:    map[string]any{"status": string(req.Status)},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, bed)
}

// HospitalCapacity handles GET /api/v1/wards/capacity.
func (h *WardsHandler) HospitalCapacity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	snapshot, err := h.capacitySnapshot(ctx, tenantID.String())
	if err != nil {
		h.logger.Error("capacity snapshot", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CAPACITY_ERROR", Message: "failed to retrieve capacity"})
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

// ── Patient-Flow Forecasting ──────────────────────────────────────────────────

// FlowForecast provides projected bed demand based on historical length-of-stay.
type FlowForecast struct {
	GeneratedAt      time.Time                `json:"generatedAt"`
	CurrentOccupancy map[string]WardOccupancy `json:"currentOccupancy"`
	ProjectedDemand  []ProjectedDemandWindow  `json:"projectedDemand"`
	Alerts           []FlowAlert              `json:"alerts"`
	AverageLOS       map[string]float64       `json:"averageLosHours"` // avg hours by admission type
}

// WardOccupancy holds current occupancy stats for a ward type.
type WardOccupancy struct {
	TotalBeds     int     `json:"totalBeds"`
	OccupiedBeds  int     `json:"occupiedBeds"`
	AvailableBeds int     `json:"availableBeds"`
	OccupancyPct  float64 `json:"occupancyPct"`
}

// ProjectedDemandWindow shows predicted discharges and occupancy at a future time.
type ProjectedDemandWindow struct {
	HoursFromNow       int     `json:"hoursFromNow"`
	ExpectedDischarges int     `json:"expectedDischarges"`
	ProjectedOccupied  int     `json:"projectedOccupied"`
	ProjectedAvailable int     `json:"projectedAvailable"`
	OccupancyPct       float64 `json:"occupancyPct"`
}

// FlowAlert flags capacity risks.
type FlowAlert struct {
	Level     string  `json:"level"` // "warning", "critical"
	Message   string  `json:"message"`
	WardType  string  `json:"wardType,omitempty"`
	Threshold float64 `json:"threshold"`
	Actual    float64 `json:"actual"`
}

// PatientFlowForecast handles GET /api/v1/wards/flow-forecast.
func (h *WardsHandler) PatientFlowForecast(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	forecast, err := h.buildFlowForecast(ctx, tenantID.String())
	if err != nil {
		h.logger.Error("patient flow forecast", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "FORECAST_ERROR", Message: "failed to generate flow forecast"})
		return
	}
	writeJSON(w, http.StatusOK, forecast)
}

func (h *WardsHandler) buildFlowForecast(ctx context.Context, tenantID string) (FlowForecast, error) {
	now := time.Now().UTC()

	// 1. Get current bed occupancy by ward type.
	capacity, err := h.capacitySnapshot(ctx, tenantID)
	if err != nil {
		return FlowForecast{}, fmt.Errorf("capacity snapshot: %w", err)
	}

	// Build ward-type occupancy map.
	occupancy := make(map[string]WardOccupancy)
	wardTotal := make(map[string]int)
	wardAvail := make(map[string]int)

	bedRows, err := h.pool.Query(ctx,
		`SELECT w.ward_type, b.status, COUNT(*)
		 FROM hospital_beds b
		 JOIN hospital_wards w ON b.ward_id = w.id
		 WHERE b.tenant_id = @tenant_id
		 GROUP BY w.ward_type, b.status`,
		db.NamedArgs{"tenant_id": tenantID},
	)
	if err != nil {
		return FlowForecast{}, fmt.Errorf("query bed occupancy: %w", err)
	}
	defer bedRows.Close()

	for bedRows.Next() {
		var wt, status string
		var count int
		if err := bedRows.Scan(&wt, &status, &count); err != nil {
			return FlowForecast{}, fmt.Errorf("scan bed occupancy: %w", err)
		}
		wardTotal[wt] += count
		if status == "available" {
			wardAvail[wt] += count
		}
	}
	if err := bedRows.Err(); err != nil {
		return FlowForecast{}, err
	}

	for wt, total := range wardTotal {
		avail := wardAvail[wt]
		occ := total - avail
		pct := 0.0
		if total > 0 {
			pct = float64(occ) / float64(total) * 100
		}
		occupancy[wt] = WardOccupancy{
			TotalBeds:     total,
			OccupiedBeds:  occ,
			AvailableBeds: avail,
			OccupancyPct:  pct,
		}
	}

	// 2. Calculate average LOS from recently discharged admissions (last 90 days).
	losRows, err := h.pool.Query(ctx,
		`SELECT admission_type,
		        AVG(EXTRACT(EPOCH FROM (discharged_at - admitted_at)) / 3600) AS avg_los_hours,
		        COUNT(*) AS sample_size
		 FROM hospital_admissions
		 WHERE tenant_id = @tenant_id
		   AND status = 'discharged'
		   AND discharged_at > now() - INTERVAL '90 days'
		   AND discharged_at IS NOT NULL
		 GROUP BY admission_type
		 HAVING COUNT(*) >= 3`,
		db.NamedArgs{"tenant_id": tenantID},
	)
	if err != nil {
		return FlowForecast{}, fmt.Errorf("query avg LOS: %w", err)
	}
	defer losRows.Close()

	avgLOS := make(map[string]float64)
	for losRows.Next() {
		var admType string
		var avgHours float64
		var sampleSize int
		if err := losRows.Scan(&admType, &avgHours, &sampleSize); err != nil {
			return FlowForecast{}, fmt.Errorf("scan LOS row: %w", err)
		}
		avgLOS[admType] = avgHours
	}
	if err := losRows.Err(); err != nil {
		return FlowForecast{}, err
	}

	// 3. Get current active admissions with admitted_at and admission_type.
	admRows, err := h.pool.Query(ctx,
		`SELECT admission_type, admitted_at
		 FROM hospital_admissions
		 WHERE tenant_id = @tenant_id AND status IN ('admitted', 'in-hospital')`,
		db.NamedArgs{"tenant_id": tenantID},
	)
	if err != nil {
		return FlowForecast{}, fmt.Errorf("query active admissions: %w", err)
	}
	defer admRows.Close()

	type activeAdmission struct {
		AdmissionType string
		AdmittedAt    time.Time
	}
	var activeAdmissions []activeAdmission
	for admRows.Next() {
		var a activeAdmission
		if err := admRows.Scan(&a.AdmissionType, &a.AdmittedAt); err != nil {
			return FlowForecast{}, fmt.Errorf("scan active admission: %w", err)
		}
		activeAdmissions = append(activeAdmissions, a)
	}
	if err := admRows.Err(); err != nil {
		return FlowForecast{}, err
	}

	// 4. For each time window, predict how many admissions will discharge.
	windows := []int{4, 8, 12, 24, 48}
	var projected []ProjectedDemandWindow
	var alerts []FlowAlert

	for _, hours := range windows {
		futureTime := now.Add(time.Duration(hours) * time.Hour)
		expectedDischarges := 0

		for _, a := range activeAdmissions {
			los := avgLOS[a.AdmissionType]
			if los <= 0 {
				los = 72 // default 3-day LOS if no history
			}
			predictedDischarge := a.AdmittedAt.Add(time.Duration(los * float64(time.Hour)))
			if predictedDischarge.Before(futureTime) {
				expectedDischarges++
			}
		}

		projectedOccupied := capacity.OccupiedBeds - expectedDischarges
		if projectedOccupied < 0 {
			projectedOccupied = 0
		}
		projectedAvailable := capacity.TotalBeds - projectedOccupied
		pct := 0.0
		if capacity.TotalBeds > 0 {
			pct = float64(projectedOccupied) / float64(capacity.TotalBeds) * 100
		}

		projected = append(projected, ProjectedDemandWindow{
			HoursFromNow:       hours,
			ExpectedDischarges: expectedDischarges,
			ProjectedOccupied:  projectedOccupied,
			ProjectedAvailable: projectedAvailable,
			OccupancyPct:       pct,
		})

		// Generate alerts.
		if pct >= 95 {
			alerts = append(alerts, FlowAlert{
				Level:     "critical",
				Message:   fmt.Sprintf("Projected occupancy at %dh: %.0f%% — at or above critical threshold", hours, pct),
				Threshold: 95,
				Actual:    pct,
			})
		} else if pct >= 85 {
			alerts = append(alerts, FlowAlert{
				Level:     "warning",
				Message:   fmt.Sprintf("Projected occupancy at %dh: %.0f%% — approaching capacity", hours, pct),
				Threshold: 85,
				Actual:    pct,
			})
		}
	}

	return FlowForecast{
		GeneratedAt:      now,
		CurrentOccupancy: occupancy,
		ProjectedDemand:  projected,
		Alerts:           alerts,
		AverageLOS:       avgLOS,
	}, nil
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *WardsHandler) listWards(ctx context.Context, tenantID, wardType string) ([]Ward, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, name, code, ward_type, floor, building,
		        total_beds, available_beds, tenant_id, created_at, updated_at
		 FROM hospital_wards
		 WHERE tenant_id = @tenant_id
		   AND (@ward_type = '' OR ward_type = @ward_type)
		 ORDER BY name`,
		db.NamedArgs{"tenant_id": tenantID, "ward_type": wardType},
	)
	if err != nil {
		return nil, fmt.Errorf("query wards: %w", err)
	}
	defer rows.Close()

	var results []Ward
	for rows.Next() {
		var w Ward
		if err := rows.Scan(
			&w.ID, &w.Name, &w.Code, &w.WardType, &w.Floor, &w.Building,
			&w.TotalBeds, &w.AvailableBeds, &w.TenantID, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ward: %w", err)
		}
		results = append(results, w)
	}
	return results, rows.Err()
}

func (h *WardsHandler) getWardByID(ctx context.Context, id, tenantID string) (Ward, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, name, code, ward_type, floor, building,
		        total_beds, available_beds, tenant_id, created_at, updated_at
		 FROM hospital_wards
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	var w Ward
	if err := row.Scan(
		&w.ID, &w.Name, &w.Code, &w.WardType, &w.Floor, &w.Building,
		&w.TotalBeds, &w.AvailableBeds, &w.TenantID, &w.CreatedAt, &w.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return Ward{}, errNotFound
		}
		return Ward{}, fmt.Errorf("get ward: %w", err)
	}
	return w, nil
}

func (h *WardsHandler) listBeds(ctx context.Context, wardID, tenantID, statusFilter string) ([]Bed, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, ward_id, bed_number, status, admission_id, tenant_id, updated_at
		 FROM hospital_beds
		 WHERE ward_id = @ward_id AND tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY bed_number`,
		db.NamedArgs{"ward_id": wardID, "tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query beds: %w", err)
	}
	defer rows.Close()

	var results []Bed
	for rows.Next() {
		var b Bed
		if err := rows.Scan(&b.ID, &b.WardID, &b.BedNumber, &b.Status, &b.AdmissionID, &b.TenantID, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan bed: %w", err)
		}
		results = append(results, b)
	}
	return results, rows.Err()
}

func (h *WardsHandler) updateBed(ctx context.Context, wardID, bedID string, req bedUpdateRequest, tenantID string) (Bed, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE hospital_beds
		 SET status = @status, admission_id = @admission_id, updated_at = now()
		 WHERE id = @id AND ward_id = @ward_id AND tenant_id = @tenant_id
		 RETURNING id, ward_id, bed_number, status, admission_id, tenant_id, updated_at`,
		db.NamedArgs{
			"status":       req.Status,
			"admission_id": req.AdmissionID,
			"id":           bedID,
			"ward_id":      wardID,
			"tenant_id":    tenantID,
		},
	)
	var b Bed
	if err := row.Scan(&b.ID, &b.WardID, &b.BedNumber, &b.Status, &b.AdmissionID, &b.TenantID, &b.UpdatedAt); err != nil {
		if db.IsNoRows(err) {
			return Bed{}, errNotFound
		}
		return Bed{}, fmt.Errorf("update bed: %w", err)
	}
	return b, nil
}

func (h *WardsHandler) capacitySnapshot(ctx context.Context, tenantID string) (HospitalCapacitySnapshot, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT ward_type,
		        COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'available') AS available
		 FROM hospital_beds b
		 JOIN hospital_wards w ON b.ward_id = w.id
		 WHERE b.tenant_id = @tenant_id
		 GROUP BY ward_type`,
		db.NamedArgs{"tenant_id": tenantID},
	)
	if err != nil {
		return HospitalCapacitySnapshot{}, fmt.Errorf("capacity query: %w", err)
	}
	defer rows.Close()

	snapshot := HospitalCapacitySnapshot{
		ByWardType:  make(map[string]int),
		GeneratedAt: time.Now().UTC(),
	}
	for rows.Next() {
		var wt string
		var total, available int
		if err := rows.Scan(&wt, &total, &available); err != nil {
			return HospitalCapacitySnapshot{}, fmt.Errorf("scan capacity row: %w", err)
		}
		snapshot.TotalBeds += total
		snapshot.AvailableBeds += available
		snapshot.OccupiedBeds += total - available
		snapshot.ByWardType[wt] = available
	}
	return snapshot, rows.Err()
}
