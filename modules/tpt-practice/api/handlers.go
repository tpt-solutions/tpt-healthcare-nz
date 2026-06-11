package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/PhillipC05/tpt-healthcare/core/accounts"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/rbac"
)

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// readJSON decodes the request body into v.
func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// tenantID extracts the tenant UUID from the authenticated principal in context.
func tenantID(r *http.Request) (uuid.UUID, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.UUID{}, false
	}
	return p.TenantID, true
}

// principalID extracts the principal ID from context.
func principalID(r *http.Request) (string, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return "", false
	}
	return p.ID, true
}

// ============================================================
// Onboarding Wizard
// ============================================================

func (s *Server) getOnboardingWizard(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// Read wizard state from onboarding_wizard table.
	var state struct {
		TenantID uuid.UUID `json:"tenant_id"`
		Step     int       `json:"step"`
	}
	state.TenantID = tid
	state.Step = 1
	row := s.cfg.Pool.QueryRow(r.Context(),
		`SELECT COALESCE(step, 1) FROM onboarding_wizard WHERE tenant_id = $1`, tid)
	_ = row.Scan(&state.Step)
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) updateWizardStep(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	step, _ := strconv.Atoi(r.PathValue("step"))
	col := ""
	switch step {
	case 1:
		col = "step1_at"
	case 2:
		col = "step2_at"
	case 3:
		col = "step3_at"
	case 4:
		col = "step4_at"
	case 5:
		col = "step5_at"
	case 6:
		col = "step6_at"
	case 7:
		col = "step7_at"
	default:
		http.Error(w, "invalid step", http.StatusBadRequest)
		return
	}
	_, err := s.cfg.Pool.Exec(r.Context(), `
		INSERT INTO onboarding_wizard (tenant_id, step, `+col+`)
		VALUES ($1, $2, NOW())
		ON CONFLICT (tenant_id) DO UPDATE SET step = EXCLUDED.step, `+col+` = NOW(), updated_at = NOW()`,
		tid, step)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if step == 7 {
		_, _ = s.cfg.Pool.Exec(r.Context(),
			`UPDATE tenants SET wizard_complete = true WHERE id = $1`, tid)
	}
	writeJSON(w, http.StatusOK, map[string]any{"step": step, "ok": true})
}

// ============================================================
// Departments
// ============================================================

func (s *Server) listDepartments(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	depts, err := s.rbacRepo.ListDepartments(r.Context(), tid)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, depts)
}

func (s *Server) createDepartment(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body rbac.Department
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	body.TenantID = tid
	dept, err := s.rbacRepo.CreateDepartment(r.Context(), body)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, dept)
}

func (s *Server) getDepartment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	dept, err := s.rbacRepo.GetDepartment(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, dept)
}

func (s *Server) updateDepartment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var body rbac.Department
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	body.ID = id
	dept, err := s.rbacRepo.UpdateDepartment(r.Context(), body)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, dept)
}

func (s *Server) deleteDepartment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.rbacRepo.DeleteDepartment(r.Context(), id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// Role Assignments
// ============================================================

func (s *Server) listRoleAssignments(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	assignments, err := s.rbacRepo.ListAssignmentsByTenant(r.Context(), tid)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, assignments)
}

func (s *Server) grantRole(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	pid, pOK := principalID(r)
	if !ok || !pOK {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body rbac.RoleAssignment
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	body.TenantID = tid
	body.GrantedBy = pid
	a, err := s.rbacRepo.GrantRole(r.Context(), body)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) revokeRole(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.rbacRepo.RevokeRole(r.Context(), id); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Budget
func (s *Server) listCostCentres(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ccs, err := s.accountsRepo.ListCostCentres(r.Context(), tid)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ccs)
}

func (s *Server) createCostCentre(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body accounts.CostCentre
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	body.TenantID = tid
	cc, err := s.accountsRepo.CreateCostCentre(r.Context(), body)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, cc)
}

func (s *Server) getBudget(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	year, _ := strconv.Atoi(r.PathValue("year"))
	budget, err := s.accountsRepo.GetBudget(r.Context(), id, year)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, budget)
}

func (s *Server) upsertBudgetLine(w http.ResponseWriter, r *http.Request) {
	var body accounts.BudgetLine
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	line, err := s.accountsRepo.UpsertBudgetLine(r.Context(), body)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, line)
}

func (s *Server) getVarianceReport(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	year, _ := strconv.Atoi(r.PathValue("year"))
	month := 12 // default to full year; override via ?month=N
	if m := r.URL.Query().Get("month"); m != "" {
		month, _ = strconv.Atoi(m)
	}
	report, err := s.accountsRepo.VarianceReport(r.Context(), id, year, month)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// ============================================================
// Accounting & payroll sync status
// ============================================================

func (s *Server) accountingStatus(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var result struct {
		LastSyncAt   *string `json:"last_sync_at"`
		LastStatus   string  `json:"last_status"`
		LastError    string  `json:"last_error,omitempty"`
		PendingCount int     `json:"pending_count"`
	}
	_ = s.cfg.Pool.QueryRow(r.Context(), `
		SELECT synced_at::text, status, COALESCE(error_text, '')
		FROM accounting_sync_log
		WHERE tenant_id = @tid
		ORDER BY synced_at DESC LIMIT 1`,
		map[string]any{"tid": tid},
	).Scan(&result.LastSyncAt, &result.LastStatus, &result.LastError)
	// Count invoices not yet synced.
	_ = s.cfg.Pool.QueryRow(r.Context(), `
		SELECT COUNT(*) FROM patient_invoices
		WHERE tenant_id = @tid AND accounting_synced_at IS NULL AND status != 'draft'`,
		map[string]any{"tid": tid},
	).Scan(&result.PendingCount)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) triggerAccountingSync(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// Enqueue a sync job via the outbox (River picks it up asynchronously).
	_, err := s.cfg.Pool.Exec(r.Context(), `
		INSERT INTO outbox_messages (id, tenant_id, topic, payload)
		VALUES (gen_random_uuid(), @tid, 'accounting.sync', '{}')`,
		map[string]any{"tid": tid})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (s *Server) payrollStatus(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var result struct {
		LastSyncAt *string `json:"last_sync_at"`
		LastStatus string  `json:"last_status"`
		LastError  string  `json:"last_error,omitempty"`
	}
	_ = s.cfg.Pool.QueryRow(r.Context(), `
		SELECT synced_at::text, status, COALESCE(error_text, '')
		FROM payroll_sync_log
		WHERE tenant_id = @tid
		ORDER BY synced_at DESC LIMIT 1`,
		map[string]any{"tid": tid},
	).Scan(&result.LastSyncAt, &result.LastStatus, &result.LastError)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) triggerPayrollSync(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_, err := s.cfg.Pool.Exec(r.Context(), `
		INSERT INTO outbox_messages (id, tenant_id, topic, payload)
		VALUES (gen_random_uuid(), @tid, 'payroll.sync', '{}')`,
		map[string]any{"tid": tid})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (s *Server) listPayslips(w http.ResponseWriter, r *http.Request) {
	// Payslips are proxied from the connected payroll provider. Since no provider
	// session is bootstrapped here yet we return the sync log so the frontend
	// can show "last sync" context while the payroll OAuth flow is wired up.
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rows, err := s.cfg.Pool.Query(r.Context(), `
		SELECT id::text, entity_ref, external_id, status, synced_at::text
		FROM payroll_sync_log
		WHERE tenant_id = @tid AND entity_type = 'timesheet'
		ORDER BY synced_at DESC LIMIT 50`,
		map[string]any{"tid": tid})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	type payslipEntry struct {
		ID         string `json:"id"`
		EntityRef  string `json:"entity_ref"`
		ExternalID string `json:"external_id,omitempty"`
		Status     string `json:"status"`
		SyncedAt   string `json:"synced_at"`
	}
	var result []payslipEntry
	for rows.Next() {
		var p payslipEntry
		if err := rows.Scan(&p.ID, &p.EntityRef, &p.ExternalID, &p.Status, &p.SyncedAt); err == nil {
			result = append(result, p)
		}
	}
	if result == nil {
		result = []payslipEntry{}
	}
	writeJSON(w, http.StatusOK, result)
}

// ============================================================
// System — backup status
// ============================================================

func (s *Server) backupStatus(w http.ResponseWriter, r *http.Request) {
	// backup_runs is not tenant-scoped (it's a system-level concern).
	// Return the last 5 runs so the dashboard widget can show recent history.
	type backupRun struct {
		ID          string  `json:"id"`
		Label       string  `json:"label"`
		StartedAt   string  `json:"started_at"`
		CompletedAt *string `json:"completed_at,omitempty"`
		Status      string  `json:"status"`
		SizeBytes   int64   `json:"size_bytes"`
		ErrorText   string  `json:"error_text,omitempty"`
	}
	rows, err := s.cfg.Pool.Query(r.Context(), `
		SELECT id, label, started_at::text,
		       completed_at::text, status, size_bytes,
		       COALESCE(error_text, '')
		FROM backup_runs
		ORDER BY started_at DESC LIMIT 5`)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var result []backupRun
	for rows.Next() {
		var b backupRun
		if err := rows.Scan(&b.ID, &b.Label, &b.StartedAt, &b.CompletedAt,
			&b.Status, &b.SizeBytes, &b.ErrorText); err == nil {
			result = append(result, b)
		}
	}
	if result == nil {
		result = []backupRun{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) leaveBalance(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	pid, pok := principalID(r)
	if !ok || !pok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// Return leave summary from approved leave_requests as a lightweight proxy
	// until the payroll provider OAuth is wired.
	type leaveTypeSummary struct {
		LeaveType    string `json:"leave_type"`
		TotalDays    int    `json:"total_days_approved"`
	}
	rows, err := s.cfg.Pool.Query(r.Context(), `
		SELECT leave_type,
		       SUM((end_date - start_date + 1))::int AS days
		FROM leave_requests
		WHERE tenant_id = @tid AND principal_id = @pid AND status = 'approved'
		  AND start_date >= date_trunc('year', CURRENT_DATE)
		GROUP BY leave_type`,
		map[string]any{"tid": tid, "pid": pid})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var result []leaveTypeSummary
	for rows.Next() {
		var ls leaveTypeSummary
		if err := rows.Scan(&ls.LeaveType, &ls.TotalDays); err == nil {
			result = append(result, ls)
		}
	}
	if result == nil {
		result = []leaveTypeSummary{}
	}
	writeJSON(w, http.StatusOK, result)
}
