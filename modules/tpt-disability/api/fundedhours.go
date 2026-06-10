package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// FundedHours tracks the allocation and usage of funded disability support hours
// per service type and billing period.
type FundedHours struct {
	ID             string     `json:"id"`
	PatientNHI     string     `json:"patientNhi"`
	SupportPlanID  *string    `json:"supportPlanId"`
	ServiceType    string     `json:"serviceType"`
	ProviderName   string     `json:"providerName"`
	ProviderHpi    string     `json:"providerHpi"`
	FundingStream  string     `json:"fundingStream"`
	AllocatedHours float64    `json:"allocatedHours"`
	UsedHours      float64    `json:"usedHours"`
	PeriodType     string     `json:"periodType"`
	PeriodStart    string     `json:"periodStart"`
	PeriodEnd      *string    `json:"periodEnd"`
	Status         string     `json:"status"`
	Notes          *string    `json:"notes"`
	TenantID       string     `json:"tenantId"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

const hoursSelectCols = `id, patient_nhi, support_plan_id,
       service_type, provider_name, provider_hpi, funding_stream,
       allocated_hours, used_hours, period_type,
       period_start::TEXT, period_end::TEXT,
       status, notes, tenant_id, created_at, updated_at`

func scanFundedHours(row interface{ Scan(...any) error }, h *FundedHours) error {
	return row.Scan(
		&h.ID, &h.PatientNHI, &h.SupportPlanID,
		&h.ServiceType, &h.ProviderName, &h.ProviderHpi, &h.FundingStream,
		&h.AllocatedHours, &h.UsedHours, &h.PeriodType,
		&h.PeriodStart, &h.PeriodEnd,
		&h.Status, &h.Notes, &h.TenantID, &h.CreatedAt, &h.UpdatedAt,
	)
}

type fundedHoursHandler struct{ handlerDeps }

func (h *fundedHoursHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	statusFilter := r.URL.Query().Get("status")
	patientFilter := r.URL.Query().Get("patientNhi")
	var rows pgx.Rows
	var err error
	switch {
	case statusFilter != "" && patientFilter != "":
		nhiEnc, encErr := h.encryptNHI(patientFilter)
		if encErr != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
			return
		}
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+hoursSelectCols+` FROM disability_funded_hours WHERE tenant_id = @tenant_id AND status = @status AND patient_nhi = @patient_nhi ORDER BY period_start DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter, "patient_nhi": nhiEnc})
	case statusFilter != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+hoursSelectCols+` FROM disability_funded_hours WHERE tenant_id = @tenant_id AND status = @status ORDER BY period_start DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	default:
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+hoursSelectCols+` FROM disability_funded_hours WHERE tenant_id = @tenant_id ORDER BY period_start DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	records := make([]FundedHours, 0)
	for rows.Next() {
		var fh FundedHours
		if err := scanFundedHours(rows, &fh); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(fh.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		fh.PatientNHI = nhi
		records = append(records, fh)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *fundedHoursHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req FundedHours
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PeriodStart == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "periodStart is required"})
		return
	}
	if req.ServiceType == "" {
		req.ServiceType = "community"
	}
	if req.FundingStream == "" {
		req.FundingStream = "DSS"
	}
	if req.PeriodType == "" {
		req.PeriodType = "weekly"
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var fh FundedHours
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO disability_funded_hours
		    (patient_nhi, support_plan_id, service_type, provider_name, provider_hpi,
		     funding_stream, allocated_hours, used_hours, period_type,
		     period_start, period_end, status, notes, tenant_id)
		VALUES
		    (@patient_nhi, @support_plan_id, @service_type, @provider_name, @provider_hpi,
		     @funding_stream, @allocated_hours, 0, @period_type,
		     @period_start::DATE, @period_end::DATE, 'active', @notes, @tenant_id)
		RETURNING `+hoursSelectCols,
		pgx.NamedArgs{
			"patient_nhi":      nhiEnc,
			"support_plan_id":  req.SupportPlanID,
			"service_type":     req.ServiceType,
			"provider_name":    req.ProviderName,
			"provider_hpi":     req.ProviderHpi,
			"funding_stream":   req.FundingStream,
			"allocated_hours":  req.AllocatedHours,
			"period_type":      req.PeriodType,
			"period_start":     req.PeriodStart,
			"period_end":       req.PeriodEnd,
			"notes":            req.Notes,
			"tenant_id":        tenantID,
		}).Scan(
		&fh.ID, &fh.PatientNHI, &fh.SupportPlanID,
		&fh.ServiceType, &fh.ProviderName, &fh.ProviderHpi, &fh.FundingStream,
		&fh.AllocatedHours, &fh.UsedHours, &fh.PeriodType,
		&fh.PeriodStart, &fh.PeriodEnd,
		&fh.Status, &fh.Notes, &fh.TenantID, &fh.CreatedAt, &fh.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "FundedHours", fh.ID, fh.PatientNHI)
	nhi, err := h.decryptNHI(fh.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	fh.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, fh)
}

func (h *fundedHoursHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var fh FundedHours
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+hoursSelectCols+` FROM disability_funded_hours WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&fh.ID, &fh.PatientNHI, &fh.SupportPlanID,
		&fh.ServiceType, &fh.ProviderName, &fh.ProviderHpi, &fh.FundingStream,
		&fh.AllocatedHours, &fh.UsedHours, &fh.PeriodType,
		&fh.PeriodStart, &fh.PeriodEnd,
		&fh.Status, &fh.Notes, &fh.TenantID, &fh.CreatedAt, &fh.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "funded hours record not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "FundedHours", fh.ID, fh.PatientNHI)
	nhi, err := h.decryptNHI(fh.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	fh.PatientNHI = nhi
	writeJSON(w, http.StatusOK, fh)
}

func (h *fundedHoursHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req FundedHours
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var fh FundedHours
	err := h.pool.QueryRow(r.Context(), `
		UPDATE disability_funded_hours
		SET service_type    = @service_type,
		    provider_name   = @provider_name,
		    provider_hpi    = @provider_hpi,
		    funding_stream  = @funding_stream,
		    allocated_hours = @allocated_hours,
		    period_type     = @period_type,
		    period_end      = @period_end::DATE,
		    status          = @status,
		    notes           = @notes,
		    updated_at      = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+hoursSelectCols,
		pgx.NamedArgs{
			"service_type":    req.ServiceType,
			"provider_name":   req.ProviderName,
			"provider_hpi":    req.ProviderHpi,
			"funding_stream":  req.FundingStream,
			"allocated_hours": req.AllocatedHours,
			"period_type":     req.PeriodType,
			"period_end":      req.PeriodEnd,
			"status":          req.Status,
			"notes":           req.Notes,
			"id":              id,
			"tenant_id":       tenantID,
		}).Scan(
		&fh.ID, &fh.PatientNHI, &fh.SupportPlanID,
		&fh.ServiceType, &fh.ProviderName, &fh.ProviderHpi, &fh.FundingStream,
		&fh.AllocatedHours, &fh.UsedHours, &fh.PeriodType,
		&fh.PeriodStart, &fh.PeriodEnd,
		&fh.Status, &fh.Notes, &fh.TenantID, &fh.CreatedAt, &fh.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "funded hours record not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "FundedHours", fh.ID, fh.PatientNHI)
	nhi, err := h.decryptNHI(fh.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	fh.PatientNHI = nhi
	writeJSON(w, http.StatusOK, fh)
}

// RecordUsage adds consumed hours to an active funded hours allocation.
// used_hours is capped at allocated_hours to prevent over-recording.
func (h *fundedHoursHandler) RecordUsage(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var body struct {
		Hours float64 `json:"hours"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if body.Hours <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_HOURS", Message: "hours must be greater than zero"})
		return
	}
	var nhiEnc string
	var newUsed, allocated float64
	err := h.pool.QueryRow(r.Context(), `
		UPDATE disability_funded_hours
		SET used_hours = LEAST(used_hours + @hours, allocated_hours),
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'active'
		RETURNING patient_nhi, used_hours, allocated_hours
	`, pgx.NamedArgs{"hours": body.Hours, "id": id, "tenant_id": tenantID}).
		Scan(&nhiEnc, &newUsed, &allocated)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "funded hours record not found or not active"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "FundedHours", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]float64{
		"usedHours":      newUsed,
		"allocatedHours": allocated,
		"remainingHours": allocated - newUsed,
	})
}
