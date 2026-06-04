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

// ============================================================
// Roster (stubs — full implementation in roster.go)
// ============================================================

func (s *Server) listShifts(w http.ResponseWriter, r *http.Request)  { writeJSON(w, http.StatusOK, []any{}) }
func (s *Server) createShift(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusCreated, map[string]string{"status": "created"}) }
func (s *Server) deleteShift(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }

// Rooms
func (s *Server) listRooms(w http.ResponseWriter, r *http.Request)   { writeJSON(w, http.StatusOK, []any{}) }
func (s *Server) createRoom(w http.ResponseWriter, r *http.Request)  { writeJSON(w, http.StatusCreated, map[string]string{"status": "created"}) }
func (s *Server) listBookings(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, []any{}) }
func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusCreated, map[string]string{"status": "created"}) }
func (s *Server) deleteBooking(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }

// Leave
func (s *Server) listLeaveRequests(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, []any{}) }
func (s *Server) createLeaveRequest(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusCreated, map[string]string{"status": "created"}) }
func (s *Server) approveLeave(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status": "approved"}) }
func (s *Server) declineLeave(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status": "declined"}) }

// Inventory
func (s *Server) listStockItems(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, []any{}) }
func (s *Server) createStockItem(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusCreated, map[string]string{"status": "created"}) }
func (s *Server) getStockItem(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]string{}) }
func (s *Server) receiveStock(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status": "received"}) }
func (s *Server) consumeStock(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status": "consumed"}) }
func (s *Server) listPurchaseOrders(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, []any{}) }
func (s *Server) createPurchaseOrder(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusCreated, map[string]string{"status": "created"}) }
func (s *Server) listColdChainBreaches(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, []any{}) }

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

// Accounting & payroll sync
func (s *Server) accountingStatus(w http.ResponseWriter, r *http.Request)   { writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}) }
func (s *Server) triggerAccountingSync(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"}) }
func (s *Server) payrollStatus(w http.ResponseWriter, r *http.Request)      { writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}) }
func (s *Server) triggerPayrollSync(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"}) }
func (s *Server) listPayslips(w http.ResponseWriter, r *http.Request)       { writeJSON(w, http.StatusOK, []any{}) }
func (s *Server) leaveBalance(w http.ResponseWriter, r *http.Request)       { writeJSON(w, http.StatusOK, map[string]any{}) }
