package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

// FundedHoursHandler handles all /api/v1/funded-hours/* routes.
type FundedHoursHandler struct {
	pool       dbPool
	enc        *encryption.Encryptor
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ---------------------------------------------------------------------------
// Allocation handlers
// ---------------------------------------------------------------------------

// ListAllocations handles GET /api/v1/funded-hours/allocations.
func (h *FundedHoursHandler) ListAllocations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	q := r.URL.Query()
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, service_plan_id,
		        funding_type, status, hours_per_week, service_type,
		        provider_id, provider_name, start_date, end_date, created_at, updated_at
		 FROM aged_care_funded_hours_allocations
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR status = $3)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		tenantID, q.Get("patient"), q.Get("status"),
	)
	if err != nil {
		h.logger.Error("list allocations", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list allocations"})
		return
	}
	defer rows.Close()

	var results []FundedHoursAllocation
	for rows.Next() {
		rec, err := scanAllocation(rows)
		if err != nil {
			h.logger.Error("scan allocation", slog.Any("error", err))
			continue
		}
		results = append(results, allocationToResponse(rec))
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "FundedHoursAllocation",
		ResourceID:   "list",
	})

	writeJSON(w, http.StatusOK, map[string]any{"allocations": results, "total": len(results)})
}

// GetAllocation handles GET /api/v1/funded-hours/allocations/{id}.
func (h *FundedHoursHandler) GetAllocation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	rec, err := h.getAllocationByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "allocation not found"})
			return
		}
		h.logger.Error("get allocation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve allocation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "FundedHoursAllocation",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
	})

	writeJSON(w, http.StatusOK, allocationToResponse(rec))
}

// CreateAllocation handles POST /api/v1/funded-hours/allocations.
func (h *FundedHoursHandler) CreateAllocation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req struct {
		PatientID     string      `json:"patientId"`
		PatientNHI    string      `json:"patientNhi"`
		ServicePlanID string      `json:"servicePlanId,omitempty"`
		FundingType   FundingType `json:"fundingType"`
		HoursPerWeek  float64     `json:"hoursPerWeek"`
		ServiceType   string      `json:"serviceType"`
		ProviderID    string      `json:"providerId,omitempty"`
		ProviderName  string      `json:"providerName,omitempty"`
		StartDate     string      `json:"startDate"`
		EndDate       string      `json:"endDate,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" && req.PatientID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patientId or patientNhi is required"})
		return
	}
	if req.HoursPerWeek <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_HOURS", Message: "hoursPerWeek must be greater than zero"})
		return
	}
	if req.StartDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_START_DATE", Message: "startDate is required"})
		return
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO aged_care_funded_hours_allocations
		   (patient_id, patient_nhi, tenant_id, service_plan_id,
		    funding_type, status, hours_per_week, service_type,
		    provider_id, provider_name, start_date, end_date)
		 VALUES ($1, $2, $3, $4, $5, 'active', $6, $7, $8, $9, $10, $11)
		 RETURNING id, patient_id, patient_nhi, tenant_id, service_plan_id,
		           funding_type, status, hours_per_week, service_type,
		           provider_id, provider_name, start_date, end_date, created_at, updated_at`,
		req.PatientID, req.PatientNHI, tenantID, req.ServicePlanID,
		string(req.FundingType), req.HoursPerWeek, req.ServiceType,
		req.ProviderID, req.ProviderName, req.StartDate, req.EndDate,
	)
	rec, err := scanAllocation(row)
	if err != nil {
		h.logger.Error("insert allocation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create allocation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "FundedHoursAllocation",
		ResourceID:   rec.ID,
		PatientNHI:   req.PatientNHI,
		Details:      map[string]any{"hoursPerWeek": req.HoursPerWeek, "serviceType": req.ServiceType},
	})

	writeJSON(w, http.StatusCreated, allocationToResponse(rec))
}

// UpdateAllocation handles PUT /api/v1/funded-hours/allocations/{id}.
func (h *FundedHoursHandler) UpdateAllocation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	var req struct {
		Status       AllocationStatus `json:"status,omitempty"`
		HoursPerWeek float64          `json:"hoursPerWeek,omitempty"`
		EndDate      string           `json:"endDate,omitempty"`
		ProviderID   string           `json:"providerId,omitempty"`
		ProviderName string           `json:"providerName,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_funded_hours_allocations
		 SET status        = CASE WHEN $1 = '' THEN status ELSE $1::allocation_status END,
		     hours_per_week = CASE WHEN $2 = 0 THEN hours_per_week ELSE $2 END,
		     end_date      = COALESCE(NULLIF($3, ''), end_date),
		     provider_id   = COALESCE(NULLIF($4, ''), provider_id),
		     provider_name = COALESCE(NULLIF($5, ''), provider_name),
		     updated_at    = now()
		 WHERE id = $6 AND tenant_id = $7
		 RETURNING id, patient_id, patient_nhi, tenant_id, service_plan_id,
		           funding_type, status, hours_per_week, service_type,
		           provider_id, provider_name, start_date, end_date, created_at, updated_at`,
		string(req.Status), req.HoursPerWeek, req.EndDate,
		req.ProviderID, req.ProviderName, id, tenantID,
	)
	rec, err := scanAllocation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "allocation not found"})
			return
		}
		h.logger.Error("update allocation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update allocation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "FundedHoursAllocation",
		ResourceID:   id,
	})

	writeJSON(w, http.StatusOK, allocationToResponse(rec))
}

// ---------------------------------------------------------------------------
// Timesheet handlers
// ---------------------------------------------------------------------------

// ListTimesheets handles GET /api/v1/funded-hours/timesheets.
func (h *FundedHoursHandler) ListTimesheets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	q := r.URL.Query()
	rows, err := h.pool.Query(ctx,
		`SELECT id, allocation_id, patient_id, patient_nhi, tenant_id,
		        status, period_start, period_end, entries, total_hours,
		        approved_by_hpi, approved_at, created_at, updated_at
		 FROM aged_care_funded_hours_timesheets
		 WHERE tenant_id = $1
		   AND ($2 = '' OR allocation_id::text = $2)
		   AND ($3 = '' OR patient_id::text = $3)
		   AND ($4 = '' OR status = $4)
		 ORDER BY period_start DESC
		 LIMIT 200`,
		tenantID, q.Get("allocation"), q.Get("patient"), q.Get("status"),
	)
	if err != nil {
		h.logger.Error("list timesheets", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list timesheets"})
		return
	}
	defer rows.Close()

	var results []FundedHoursTimesheet
	for rows.Next() {
		rec, err := scanTimesheet(rows)
		if err != nil {
			h.logger.Error("scan timesheet", slog.Any("error", err))
			continue
		}
		results = append(results, timesheetToResponse(rec))
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "FundedHoursTimesheet",
		ResourceID:   "list",
	})

	writeJSON(w, http.StatusOK, map[string]any{"timesheets": results, "total": len(results)})
}

// GetTimesheet handles GET /api/v1/funded-hours/timesheets/{id}.
func (h *FundedHoursHandler) GetTimesheet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	rec, err := h.getTimesheetByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "timesheet not found"})
			return
		}
		h.logger.Error("get timesheet", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve timesheet"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "FundedHoursTimesheet",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
	})

	writeJSON(w, http.StatusOK, timesheetToResponse(rec))
}

// CreateTimesheet handles POST /api/v1/funded-hours/timesheets.
func (h *FundedHoursHandler) CreateTimesheet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req struct {
		AllocationID string           `json:"allocationId"`
		PatientID    string           `json:"patientId"`
		PatientNHI   string           `json:"patientNhi"`
		PeriodStart  string           `json:"periodStart"`
		PeriodEnd    string           `json:"periodEnd"`
		Entries      []TimesheetEntry `json:"entries"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.AllocationID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ALLOCATION", Message: "allocationId is required"})
		return
	}
	if req.PeriodStart == "" || req.PeriodEnd == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PERIOD", Message: "periodStart and periodEnd are required"})
		return
	}

	var total float64
	for _, e := range req.Entries {
		total += e.HoursWorked
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO aged_care_funded_hours_timesheets
		   (allocation_id, patient_id, patient_nhi, tenant_id, status,
		    period_start, period_end, entries, total_hours)
		 VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8)
		 RETURNING id, allocation_id, patient_id, patient_nhi, tenant_id,
		           status, period_start, period_end, entries, total_hours,
		           approved_by_hpi, approved_at, created_at, updated_at`,
		req.AllocationID, req.PatientID, req.PatientNHI, tenantID,
		req.PeriodStart, req.PeriodEnd, req.Entries, total,
	)
	rec, err := scanTimesheet(row)
	if err != nil {
		h.logger.Error("insert timesheet", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create timesheet"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "FundedHoursTimesheet",
		ResourceID:   rec.ID,
		PatientNHI:   req.PatientNHI,
		Details:      map[string]any{"totalHours": total, "period": req.PeriodStart + "/" + req.PeriodEnd},
	})

	writeJSON(w, http.StatusCreated, timesheetToResponse(rec))
}

// ApproveTimesheet handles PUT /api/v1/funded-hours/timesheets/{id}/approve.
func (h *FundedHoursHandler) ApproveTimesheet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	var req struct {
		ApproverHPI string `json:"approverHpi"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ApproverHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_APPROVER", Message: "approverHpi is required"})
		return
	}

	now := time.Now().UTC()
	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_funded_hours_timesheets
		 SET status = 'approved', approved_by_hpi = $1, approved_at = $2, updated_at = $2
		 WHERE id = $3 AND tenant_id = $4 AND status = 'pending'
		 RETURNING id, allocation_id, patient_id, patient_nhi, tenant_id,
		           status, period_start, period_end, entries, total_hours,
		           approved_by_hpi, approved_at, created_at, updated_at`,
		req.ApproverHPI, now, id, tenantID,
	)
	rec, err := scanTimesheet(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "pending timesheet not found"})
			return
		}
		h.logger.Error("approve timesheet", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "APPROVE_ERROR", Message: "failed to approve timesheet"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "approve",
		ResourceType: "FundedHoursTimesheet",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
		Details:      map[string]any{"approverHpi": req.ApproverHPI},
	})

	writeJSON(w, http.StatusOK, timesheetToResponse(rec))
}

// GetSummary handles GET /api/v1/funded-hours/summary.
// Returns allocation and delivery totals for a patient for the current week/month.
func (h *FundedHoursHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	patientID := r.URL.Query().Get("patient")
	if patientID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patient query param is required"})
		return
	}

	var summary FundedHoursSummary
	err := h.pool.QueryRow(ctx,
		`SELECT
		     p.patient_id,
		     p.patient_nhi,
		     COALESCE(SUM(a.hours_per_week) FILTER (WHERE a.status = 'active'), 0) AS allocated_per_week,
		     COALESCE(SUM(t.total_hours)    FILTER (
		         WHERE t.period_start >= date_trunc('week', CURRENT_DATE)
		           AND t.status = 'approved'), 0) AS delivered_this_week,
		     COALESCE(SUM(t.total_hours)    FILTER (
		         WHERE t.period_start >= date_trunc('month', CURRENT_DATE)
		           AND t.status = 'approved'), 0) AS delivered_this_month,
		     COUNT(a.id) FILTER (WHERE a.status = 'active')  AS active_allocations
		 FROM (SELECT $1::uuid AS patient_id, $2 AS patient_nhi) p
		 LEFT JOIN aged_care_funded_hours_allocations a
		   ON a.patient_id = p.patient_id AND a.tenant_id = $3
		 LEFT JOIN aged_care_funded_hours_timesheets t
		   ON t.allocation_id = a.id
		 GROUP BY p.patient_id, p.patient_nhi`,
		patientID, "", tenantID,
	).Scan(
		&summary.PatientID, &summary.PatientNHI,
		&summary.AllocatedPerWeek, &summary.DeliveredThisWeek,
		&summary.DeliveredThisMonth, &summary.ActiveAllocations,
	)
	if err != nil {
		h.logger.Error("get funded hours summary", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SUMMARY_ERROR", Message: "failed to compute funded hours summary"})
		return
	}
	summary.UnusedThisWeek = fundedHoursMax(0, summary.AllocatedPerWeek-summary.DeliveredThisWeek)

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "FundedHoursSummary",
		ResourceID:   patientID,
	})

	writeJSON(w, http.StatusOK, summary)
}
