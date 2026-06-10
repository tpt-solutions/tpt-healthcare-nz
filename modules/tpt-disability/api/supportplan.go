package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// SupportPlan represents a disability support plan developed following a NASC assessment.
type SupportPlan struct {
	ID              string     `json:"id"`
	PatientNHI      string     `json:"patientNhi"`
	CoordinatorHpi  string     `json:"coordinatorHpi"`
	NASCReferralID  *string    `json:"nascReferralId"`
	FundingStream   string     `json:"fundingStream"`
	PlanType        string     `json:"planType"`
	Status          string     `json:"status"`
	GoalsSummary    string     `json:"goalsSummary"`
	ServicesSummary string     `json:"servicesSummary"`
	ReviewDate      *string    `json:"reviewDate"`
	ClosureReason   *string    `json:"closureReason"`
	Notes           *string    `json:"notes"`
	TenantID        string     `json:"tenantId"`
	ApprovedAt      *time.Time `json:"approvedAt"`
	ClosedAt        *time.Time `json:"closedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

const planSelectCols = `id, patient_nhi, coordinator_hpi, nasc_referral_id,
       funding_stream, plan_type, status, goals_summary, services_summary,
       review_date::TEXT, closure_reason, notes, tenant_id,
       approved_at, closed_at, created_at, updated_at`

func scanSupportPlan(row interface{ Scan(...any) error }, p *SupportPlan) error {
	return row.Scan(
		&p.ID, &p.PatientNHI, &p.CoordinatorHpi, &p.NASCReferralID,
		&p.FundingStream, &p.PlanType, &p.Status, &p.GoalsSummary, &p.ServicesSummary,
		&p.ReviewDate, &p.ClosureReason, &p.Notes, &p.TenantID,
		&p.ApprovedAt, &p.ClosedAt, &p.CreatedAt, &p.UpdatedAt,
	)
}

type supportPlanHandler struct{ handlerDeps }

func (h *supportPlanHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	statusFilter := r.URL.Query().Get("status")
	var rows pgx.Rows
	var err error
	if statusFilter != "" {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+planSelectCols+` FROM disability_support_plans WHERE tenant_id = @tenant_id AND status = @status ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+planSelectCols+` FROM disability_support_plans WHERE tenant_id = @tenant_id ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	plans := make([]SupportPlan, 0)
	for rows.Next() {
		var p SupportPlan
		if err := scanSupportPlan(rows, &p); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(p.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		p.PatientNHI = nhi
		plans = append(plans, p)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, plans)
}

func (h *supportPlanHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req SupportPlan
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.FundingStream == "" {
		req.FundingStream = "DSS"
	}
	if req.PlanType == "" {
		req.PlanType = "initial"
	}
	if !h.validateHPI(w, r, req.CoordinatorHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var p SupportPlan
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO disability_support_plans
		    (patient_nhi, coordinator_hpi, nasc_referral_id, funding_stream,
		     plan_type, status, goals_summary, services_summary,
		     review_date, notes, tenant_id)
		VALUES
		    (@patient_nhi, @coordinator_hpi, @nasc_referral_id, @funding_stream,
		     @plan_type, 'draft', @goals_summary, @services_summary,
		     @review_date::DATE, @notes, @tenant_id)
		RETURNING `+planSelectCols,
		pgx.NamedArgs{
			"patient_nhi":      nhiEnc,
			"coordinator_hpi":  req.CoordinatorHpi,
			"nasc_referral_id": req.NASCReferralID,
			"funding_stream":   req.FundingStream,
			"plan_type":        req.PlanType,
			"goals_summary":    req.GoalsSummary,
			"services_summary": req.ServicesSummary,
			"review_date":      req.ReviewDate,
			"notes":            req.Notes,
			"tenant_id":        tenantID,
		}).Scan(
		&p.ID, &p.PatientNHI, &p.CoordinatorHpi, &p.NASCReferralID,
		&p.FundingStream, &p.PlanType, &p.Status, &p.GoalsSummary, &p.ServicesSummary,
		&p.ReviewDate, &p.ClosureReason, &p.Notes, &p.TenantID,
		&p.ApprovedAt, &p.ClosedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "SupportPlan", p.ID, p.PatientNHI)
	nhi, err := h.decryptNHI(p.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	p.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, p)
}

func (h *supportPlanHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var p SupportPlan
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+planSelectCols+` FROM disability_support_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&p.ID, &p.PatientNHI, &p.CoordinatorHpi, &p.NASCReferralID,
		&p.FundingStream, &p.PlanType, &p.Status, &p.GoalsSummary, &p.ServicesSummary,
		&p.ReviewDate, &p.ClosureReason, &p.Notes, &p.TenantID,
		&p.ApprovedAt, &p.ClosedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "support plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "SupportPlan", p.ID, p.PatientNHI)
	nhi, err := h.decryptNHI(p.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

func (h *supportPlanHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req SupportPlan
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.CoordinatorHpi) {
		return
	}
	var p SupportPlan
	err := h.pool.QueryRow(r.Context(), `
		UPDATE disability_support_plans
		SET coordinator_hpi  = @coordinator_hpi,
		    funding_stream   = @funding_stream,
		    plan_type        = @plan_type,
		    status           = @status,
		    goals_summary    = @goals_summary,
		    services_summary = @services_summary,
		    review_date      = @review_date::DATE,
		    notes            = @notes,
		    updated_at       = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+planSelectCols,
		pgx.NamedArgs{
			"coordinator_hpi":  req.CoordinatorHpi,
			"funding_stream":   req.FundingStream,
			"plan_type":        req.PlanType,
			"status":           req.Status,
			"goals_summary":    req.GoalsSummary,
			"services_summary": req.ServicesSummary,
			"review_date":      req.ReviewDate,
			"notes":            req.Notes,
			"id":               id,
			"tenant_id":        tenantID,
		}).Scan(
		&p.ID, &p.PatientNHI, &p.CoordinatorHpi, &p.NASCReferralID,
		&p.FundingStream, &p.PlanType, &p.Status, &p.GoalsSummary, &p.ServicesSummary,
		&p.ReviewDate, &p.ClosureReason, &p.Notes, &p.TenantID,
		&p.ApprovedAt, &p.ClosedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "support plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "SupportPlan", p.ID, p.PatientNHI)
	nhi, err := h.decryptNHI(p.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

func (h *supportPlanHandler) Approve(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM disability_support_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "support plan not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE disability_support_plans
		SET status = 'active', approved_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "support plan must be in draft status to approve"})
		return
	}
	h.recordAudit(r, "update", "SupportPlan", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

func (h *supportPlanHandler) Close(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var body struct {
		ClosureReason string `json:"closureReason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM disability_support_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "support plan not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE disability_support_plans
		SET status = 'closed', closure_reason = @closure_reason, closed_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('closed')
	`, pgx.NamedArgs{"closure_reason": body.ClosureReason, "id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "support plan not found or already closed"})
		return
	}
	h.recordAudit(r, "update", "SupportPlan", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "closed"})
}
