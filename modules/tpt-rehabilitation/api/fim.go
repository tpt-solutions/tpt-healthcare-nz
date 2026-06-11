package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// FIMScore is a complete Functional Independence Measure assessment (18 items, total 18–126).
// Motor FIM (13 items): self-care (6), sphincter (2), transfer (3), locomotion (2).
// Cognitive FIM (5 items): communication (2), social cognition (3).
// Each item is scored 1 (total assistance) to 7 (complete independence).
type FIMScore struct {
	ID             string     `json:"id"`
	AdmissionID    string     `json:"admissionId"`
	AssessedByHpi  string     `json:"assessedByHpi"`
	AssessmentType string     `json:"assessmentType"`
	// Self-care
	Eating           int16 `json:"eating"`
	Grooming         int16 `json:"grooming"`
	Bathing          int16 `json:"bathing"`
	DressingUpper    int16 `json:"dressingUpper"`
	DressingLower    int16 `json:"dressingLower"`
	Toileting        int16 `json:"toileting"`
	// Sphincter control
	BladderManagement int16 `json:"bladderManagement"`
	BowelManagement   int16 `json:"bowelManagement"`
	// Transfers
	TransferBedChair  int16 `json:"transferBedChair"`
	TransferToilet    int16 `json:"transferToilet"`
	TransferBath      int16 `json:"transferBath"`
	// Locomotion
	WalkWheelchair int16 `json:"walkWheelchair"`
	Stairs         int16 `json:"stairs"`
	// Communication
	Comprehension int16 `json:"comprehension"`
	Expression    int16 `json:"expression"`
	// Social cognition
	SocialInteraction int16 `json:"socialInteraction"`
	ProblemSolving    int16 `json:"problemSolving"`
	Memory            int16 `json:"memory"`
	// Totals
	MotorFIMTotal    int16   `json:"motorFimTotal"`
	CognitiveFIMTotal int16  `json:"cognitiveFimTotal"`
	TotalFIMScore    int16   `json:"totalFimScore"`
	Notes            *string `json:"notes"`
	TenantID         string  `json:"tenantId"`
	AssessedAt       time.Time `json:"assessedAt"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

const fimSelectCols = `id, admission_id, assessed_by_hpi, assessment_type,
       eating, grooming, bathing, dressing_upper, dressing_lower, toileting,
       bladder_management, bowel_management,
       transfer_bed_chair, transfer_toilet, transfer_bath,
       walk_wheelchair, stairs,
       comprehension, expression,
       social_interaction, problem_solving, memory,
       motor_fim_total, cognitive_fim_total, total_fim_score,
       notes, tenant_id, assessed_at, created_at, updated_at`

func scanFIM(row interface{ Scan(...any) error }, f *FIMScore) error {
	return row.Scan(
		&f.ID, &f.AdmissionID, &f.AssessedByHpi, &f.AssessmentType,
		&f.Eating, &f.Grooming, &f.Bathing, &f.DressingUpper, &f.DressingLower, &f.Toileting,
		&f.BladderManagement, &f.BowelManagement,
		&f.TransferBedChair, &f.TransferToilet, &f.TransferBath,
		&f.WalkWheelchair, &f.Stairs,
		&f.Comprehension, &f.Expression,
		&f.SocialInteraction, &f.ProblemSolving, &f.Memory,
		&f.MotorFIMTotal, &f.CognitiveFIMTotal, &f.TotalFIMScore,
		&f.Notes, &f.TenantID, &f.AssessedAt, &f.CreatedAt, &f.UpdatedAt,
	)
}

type fimHandler struct{ handlerDeps }

func (h *fimHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	admissionID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+fimSelectCols+` FROM rehab_fim_scores
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		 ORDER BY assessed_at ASC`,
		pgx.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	scores := make([]FIMScore, 0)
	for rows.Next() {
		var f FIMScore
		if err := scanFIM(rows, &f); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		scores = append(scores, f)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, scores)
}

func (h *fimHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	admissionID := r.PathValue("id")
	var req FIMScore
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.AssessmentType == "" {
		req.AssessmentType = "admission"
	}
	if !h.validateHPI(w, r, req.AssessedByHpi) {
		return
	}
	// Compute totals server-side so they are always correct.
	motor := req.Eating + req.Grooming + req.Bathing + req.DressingUpper + req.DressingLower +
		req.Toileting + req.BladderManagement + req.BowelManagement +
		req.TransferBedChair + req.TransferToilet + req.TransferBath +
		req.WalkWheelchair + req.Stairs
	cognitive := req.Comprehension + req.Expression + req.SocialInteraction + req.ProblemSolving + req.Memory
	total := motor + cognitive

	var f FIMScore
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO rehab_fim_scores
		    (admission_id, assessed_by_hpi, assessment_type,
		     eating, grooming, bathing, dressing_upper, dressing_lower, toileting,
		     bladder_management, bowel_management,
		     transfer_bed_chair, transfer_toilet, transfer_bath,
		     walk_wheelchair, stairs,
		     comprehension, expression,
		     social_interaction, problem_solving, memory,
		     motor_fim_total, cognitive_fim_total, total_fim_score,
		     notes, tenant_id, assessed_at)
		VALUES
		    (@admission_id, @assessed_by_hpi, @assessment_type,
		     @eating, @grooming, @bathing, @dressing_upper, @dressing_lower, @toileting,
		     @bladder_management, @bowel_management,
		     @transfer_bed_chair, @transfer_toilet, @transfer_bath,
		     @walk_wheelchair, @stairs,
		     @comprehension, @expression,
		     @social_interaction, @problem_solving, @memory,
		     @motor_fim_total, @cognitive_fim_total, @total_fim_score,
		     @notes, @tenant_id, COALESCE(@assessed_at, now()))
		RETURNING `+fimSelectCols,
		pgx.NamedArgs{
			"admission_id":       admissionID,
			"assessed_by_hpi":    req.AssessedByHpi,
			"assessment_type":    req.AssessmentType,
			"eating":             req.Eating,
			"grooming":           req.Grooming,
			"bathing":            req.Bathing,
			"dressing_upper":     req.DressingUpper,
			"dressing_lower":     req.DressingLower,
			"toileting":          req.Toileting,
			"bladder_management": req.BladderManagement,
			"bowel_management":   req.BowelManagement,
			"transfer_bed_chair": req.TransferBedChair,
			"transfer_toilet":    req.TransferToilet,
			"transfer_bath":      req.TransferBath,
			"walk_wheelchair":    req.WalkWheelchair,
			"stairs":             req.Stairs,
			"comprehension":      req.Comprehension,
			"expression":         req.Expression,
			"social_interaction": req.SocialInteraction,
			"problem_solving":    req.ProblemSolving,
			"memory":             req.Memory,
			"motor_fim_total":    motor,
			"cognitive_fim_total": cognitive,
			"total_fim_score":    total,
			"notes":              req.Notes,
			"tenant_id":          tenantID,
			"assessed_at":        req.AssessedAt,
		}).Scan(
		&f.ID, &f.AdmissionID, &f.AssessedByHpi, &f.AssessmentType,
		&f.Eating, &f.Grooming, &f.Bathing, &f.DressingUpper, &f.DressingLower, &f.Toileting,
		&f.BladderManagement, &f.BowelManagement,
		&f.TransferBedChair, &f.TransferToilet, &f.TransferBath,
		&f.WalkWheelchair, &f.Stairs,
		&f.Comprehension, &f.Expression,
		&f.SocialInteraction, &f.ProblemSolving, &f.Memory,
		&f.MotorFIMTotal, &f.CognitiveFIMTotal, &f.TotalFIMScore,
		&f.Notes, &f.TenantID, &f.AssessedAt, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "FIMScore", f.ID, "")
	writeJSON(w, http.StatusCreated, f)
}

func (h *fimHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	assessmentID := r.PathValue("assessmentId")
	admissionID := r.PathValue("id")
	var f FIMScore
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+fimSelectCols+` FROM rehab_fim_scores WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": assessmentID, "admission_id": admissionID, "tenant_id": tenantID}).Scan(
		&f.ID, &f.AdmissionID, &f.AssessedByHpi, &f.AssessmentType,
		&f.Eating, &f.Grooming, &f.Bathing, &f.DressingUpper, &f.DressingLower, &f.Toileting,
		&f.BladderManagement, &f.BowelManagement,
		&f.TransferBedChair, &f.TransferToilet, &f.TransferBath,
		&f.WalkWheelchair, &f.Stairs,
		&f.Comprehension, &f.Expression,
		&f.SocialInteraction, &f.ProblemSolving, &f.Memory,
		&f.MotorFIMTotal, &f.CognitiveFIMTotal, &f.TotalFIMScore,
		&f.Notes, &f.TenantID, &f.AssessedAt, &f.CreatedAt, &f.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "FIM assessment not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, f)
}
