package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// ACCPlan represents an ACC rehabilitation plan for injury-related rehabilitation.
type ACCPlan struct {
	ID                   string     `json:"id"`
	PatientNHI           string     `json:"patientNhi"`
	ClinicianHpi         string     `json:"clinicianHpi"`
	AccClaimNumber       string     `json:"accClaimNumber"`
	AccContractType      string     `json:"accContractType"`
	InjuryDescription    string     `json:"injuryDescription"`
	RehabilitationGoals  string     `json:"rehabilitationGoals"`
	Status               string     `json:"status"`
	FundingApprovedNZD   *float64   `json:"fundingApprovedNzd"`
	FundingSpentNZD      *float64   `json:"fundingSpentNzd"`
	ReviewDate           *string    `json:"reviewDate"`
	PlanDate             *string    `json:"planDate"`
	Notes                *string    `json:"notes"`
	TenantID             string     `json:"tenantId"`
	ApprovedAt           *time.Time `json:"approvedAt"`
	CompletedAt          *time.Time `json:"completedAt"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

const accPlanSelectCols = `id, patient_nhi, clinician_hpi, acc_claim_number, acc_contract_type,
       injury_description, rehabilitation_goals, status,
       funding_approved_nzd, funding_spent_nzd,
       review_date::text, plan_date::text, notes,
       tenant_id, approved_at, completed_at, created_at, updated_at`

func scanACCPlan(row interface{ Scan(...any) error }, p *ACCPlan) error {
	return row.Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.AccClaimNumber, &p.AccContractType,
		&p.InjuryDescription, &p.RehabilitationGoals, &p.Status,
		&p.FundingApprovedNZD, &p.FundingSpentNZD,
		&p.ReviewDate, &p.PlanDate, &p.Notes,
		&p.TenantID, &p.ApprovedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
}

type accHandler struct{ handlerDeps }

func (h *accHandler) List(w http.ResponseWriter, r *http.Request) {
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
			`SELECT `+accPlanSelectCols+` FROM rehab_acc_plans WHERE tenant_id = @tenant_id AND status = @status ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+accPlanSelectCols+` FROM rehab_acc_plans WHERE tenant_id = @tenant_id ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	plans := make([]ACCPlan, 0)
	for rows.Next() {
		var p ACCPlan
		if err := scanACCPlan(rows, &p); err != nil {
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

func (h *accHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req ACCPlan
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.AccContractType == "" {
		req.AccContractType = "social-rehabilitation"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var p ACCPlan
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO rehab_acc_plans
		    (patient_nhi, clinician_hpi, acc_claim_number, acc_contract_type,
		     injury_description, rehabilitation_goals, status,
		     funding_approved_nzd, funding_spent_nzd,
		     review_date, plan_date, notes, tenant_id)
		VALUES
		    (@patient_nhi, @clinician_hpi, @acc_claim_number, @acc_contract_type,
		     @injury_description, @rehabilitation_goals, 'draft',
		     @funding_approved_nzd, 0,
		     @review_date, COALESCE(@plan_date, now()::date), @notes, @tenant_id)
		RETURNING `+accPlanSelectCols,
		pgx.NamedArgs{
			"patient_nhi":           nhiEnc,
			"clinician_hpi":         req.ClinicianHpi,
			"acc_claim_number":      req.AccClaimNumber,
			"acc_contract_type":     req.AccContractType,
			"injury_description":    req.InjuryDescription,
			"rehabilitation_goals":  req.RehabilitationGoals,
			"funding_approved_nzd":  req.FundingApprovedNZD,
			"review_date":           req.ReviewDate,
			"plan_date":             req.PlanDate,
			"notes":                 req.Notes,
			"tenant_id":             tenantID,
		}).Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.AccClaimNumber, &p.AccContractType,
		&p.InjuryDescription, &p.RehabilitationGoals, &p.Status,
		&p.FundingApprovedNZD, &p.FundingSpentNZD,
		&p.ReviewDate, &p.PlanDate, &p.Notes,
		&p.TenantID, &p.ApprovedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "ACCPlan", p.ID, p.PatientNHI)
	nhi, err := h.decryptNHI(p.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	p.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, p)
}

func (h *accHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var p ACCPlan
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+accPlanSelectCols+` FROM rehab_acc_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.AccClaimNumber, &p.AccContractType,
		&p.InjuryDescription, &p.RehabilitationGoals, &p.Status,
		&p.FundingApprovedNZD, &p.FundingSpentNZD,
		&p.ReviewDate, &p.PlanDate, &p.Notes,
		&p.TenantID, &p.ApprovedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ACC plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(p.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

func (h *accHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req ACCPlan
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var p ACCPlan
	err := h.pool.QueryRow(r.Context(), `
		UPDATE rehab_acc_plans
		SET clinician_hpi          = @clinician_hpi,
		    acc_contract_type      = @acc_contract_type,
		    injury_description     = @injury_description,
		    rehabilitation_goals   = @rehabilitation_goals,
		    status                 = @status,
		    funding_approved_nzd   = @funding_approved_nzd,
		    funding_spent_nzd      = @funding_spent_nzd,
		    review_date            = @review_date,
		    notes                  = @notes,
		    completed_at           = CASE WHEN @status = 'completed' AND completed_at IS NULL THEN now() ELSE completed_at END,
		    updated_at             = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+accPlanSelectCols,
		pgx.NamedArgs{
			"clinician_hpi":        req.ClinicianHpi,
			"acc_contract_type":    req.AccContractType,
			"injury_description":   req.InjuryDescription,
			"rehabilitation_goals": req.RehabilitationGoals,
			"status":               req.Status,
			"funding_approved_nzd": req.FundingApprovedNZD,
			"funding_spent_nzd":    req.FundingSpentNZD,
			"review_date":          req.ReviewDate,
			"notes":                req.Notes,
			"id":                   id,
			"tenant_id":            tenantID,
		}).Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.AccClaimNumber, &p.AccContractType,
		&p.InjuryDescription, &p.RehabilitationGoals, &p.Status,
		&p.FundingApprovedNZD, &p.FundingSpentNZD,
		&p.ReviewDate, &p.PlanDate, &p.Notes,
		&p.TenantID, &p.ApprovedAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ACC plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "ACCPlan", p.ID, p.PatientNHI)
	nhi, err := h.decryptNHI(p.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

func (h *accHandler) Submit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM rehab_acc_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ACC plan not found or not in draft status"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE rehab_acc_plans
		SET status = 'submitted', updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ACC plan not found or not in draft status"})
		return
	}
	h.recordAudit(r, "update", "ACCPlan", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}

func (h *accHandler) Approve(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM rehab_acc_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ACC plan not found or not in submitted status"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE rehab_acc_plans
		SET status = 'approved', approved_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'submitted'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ACC plan not found or not in submitted status"})
		return
	}
	h.recordAudit(r, "update", "ACCPlan", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}
