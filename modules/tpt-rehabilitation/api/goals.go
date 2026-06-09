package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// RehabGoal represents a therapy goal (STG or LTG) within a rehabilitation admission.
type RehabGoal struct {
	ID            string     `json:"id"`
	AdmissionID   string     `json:"admissionId"`
	Discipline    string     `json:"discipline"`
	GoalType      string     `json:"goalType"`
	GoalText      string     `json:"goalText"`
	TargetDate    *string    `json:"targetDate"`
	Status        string     `json:"status"`
	ProgressNotes *string    `json:"progressNotes"`
	AchievedAt    *time.Time `json:"achievedAt"`
	TenantID      string     `json:"tenantId"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

const goalSelectCols = `id, admission_id, discipline, goal_type, goal_text,
       target_date::text, status, progress_notes, achieved_at, tenant_id, created_at, updated_at`

type goalsHandler struct{ handlerDeps }

func (h *goalsHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	admissionID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+goalSelectCols+` FROM rehab_goals
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		 ORDER BY goal_type DESC, created_at ASC`,
		pgx.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	goals := make([]RehabGoal, 0)
	for rows.Next() {
		var g RehabGoal
		if err := rows.Scan(
			&g.ID, &g.AdmissionID, &g.Discipline, &g.GoalType, &g.GoalText,
			&g.TargetDate, &g.Status, &g.ProgressNotes, &g.AchievedAt, &g.TenantID, &g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		goals = append(goals, g)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, goals)
}

func (h *goalsHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	admissionID := r.PathValue("id")
	var req RehabGoal
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.GoalType == "" {
		req.GoalType = "STG"
	}
	if req.Discipline == "" {
		req.Discipline = "multidisciplinary"
	}
	var g RehabGoal
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO rehab_goals
		    (admission_id, discipline, goal_type, goal_text, target_date,
		     status, progress_notes, tenant_id)
		VALUES
		    (@admission_id, @discipline, @goal_type, @goal_text, @target_date,
		     'active', @progress_notes, @tenant_id)
		RETURNING `+goalSelectCols,
		pgx.NamedArgs{
			"admission_id":   admissionID,
			"discipline":     req.Discipline,
			"goal_type":      req.GoalType,
			"goal_text":      req.GoalText,
			"target_date":    req.TargetDate,
			"progress_notes": req.ProgressNotes,
			"tenant_id":      tenantID,
		}).Scan(
		&g.ID, &g.AdmissionID, &g.Discipline, &g.GoalType, &g.GoalText,
		&g.TargetDate, &g.Status, &g.ProgressNotes, &g.AchievedAt, &g.TenantID, &g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "RehabGoal", g.ID, "")
	writeJSON(w, http.StatusCreated, g)
}

func (h *goalsHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	goalID := r.PathValue("goalId")
	admissionID := r.PathValue("id")
	var g RehabGoal
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+goalSelectCols+` FROM rehab_goals WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": goalID, "admission_id": admissionID, "tenant_id": tenantID}).Scan(
		&g.ID, &g.AdmissionID, &g.Discipline, &g.GoalType, &g.GoalText,
		&g.TargetDate, &g.Status, &g.ProgressNotes, &g.AchievedAt, &g.TenantID, &g.CreatedAt, &g.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "goal not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (h *goalsHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	goalID := r.PathValue("goalId")
	admissionID := r.PathValue("id")
	var req RehabGoal
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var g RehabGoal
	err := h.pool.QueryRow(r.Context(), `
		UPDATE rehab_goals
		SET discipline      = @discipline,
		    goal_type       = @goal_type,
		    goal_text       = @goal_text,
		    target_date     = @target_date,
		    status          = @status,
		    progress_notes  = @progress_notes,
		    achieved_at     = CASE WHEN @status = 'achieved' AND achieved_at IS NULL THEN now() ELSE achieved_at END,
		    updated_at      = now()
		WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id
		RETURNING `+goalSelectCols,
		pgx.NamedArgs{
			"discipline":     req.Discipline,
			"goal_type":      req.GoalType,
			"goal_text":      req.GoalText,
			"target_date":    req.TargetDate,
			"status":         req.Status,
			"progress_notes": req.ProgressNotes,
			"id":             goalID,
			"admission_id":   admissionID,
			"tenant_id":      tenantID,
		}).Scan(
		&g.ID, &g.AdmissionID, &g.Discipline, &g.GoalType, &g.GoalText,
		&g.TargetDate, &g.Status, &g.ProgressNotes, &g.AchievedAt, &g.TenantID, &g.CreatedAt, &g.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "goal not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "RehabGoal", g.ID, "")
	writeJSON(w, http.StatusOK, g)
}
