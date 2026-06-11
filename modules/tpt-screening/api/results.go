package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

// ScreeningResult captures the outcome of a screening episode.
// result_category values: normal | abnormal | inadequate | unsatisfactory | pending
type ScreeningResult struct {
	ID                   string     `json:"id"`
	EnrolmentID          string     `json:"enrolmentId"`
	PatientNHI           string     `json:"patientNhi"`
	ProgrammeType        string     `json:"programmeType"`
	ScreenDate           string     `json:"screenDate"` // DATE as YYYY-MM-DD
	ResultCategory       string     `json:"resultCategory"`
	ResultDetail         string     `json:"resultDetail"`
	ReportedByHpi        string     `json:"reportedByHpi"`
	ExternalReferenceID  *string    `json:"externalReferenceId"`
	FollowUpRequired     bool       `json:"followUpRequired"`
	FollowUpAction       *string    `json:"followUpAction"`
	FollowUpDueDate      *string    `json:"followUpDueDate"`      // DATE as YYYY-MM-DD
	FollowUpCompletedAt  *time.Time `json:"followUpCompletedAt"`
	NextDueDate          *string    `json:"nextDueDate"`          // DATE as YYYY-MM-DD
	Notes                *string    `json:"notes"`
	TenantID             string     `json:"tenantId"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

const resultSelectCols = `id, enrolment_id, patient_nhi, programme_type,
       screen_date::text, result_category, result_detail, reported_by_hpi,
       external_reference_id,
       follow_up_required, follow_up_action, follow_up_due_date::text,
       follow_up_completed_at, next_due_date::text,
       notes, tenant_id, created_at, updated_at`

func scanResult(row interface{ Scan(...any) error }, res *ScreeningResult) error {
	return row.Scan(
		&res.ID, &res.EnrolmentID, &res.PatientNHI, &res.ProgrammeType,
		&res.ScreenDate, &res.ResultCategory, &res.ResultDetail, &res.ReportedByHpi,
		&res.ExternalReferenceID,
		&res.FollowUpRequired, &res.FollowUpAction, &res.FollowUpDueDate,
		&res.FollowUpCompletedAt, &res.NextDueDate,
		&res.Notes, &res.TenantID, &res.CreatedAt, &res.UpdatedAt,
	)
}

type resultHandler struct{ handlerDeps }

func (h *resultHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	q := r.URL.Query()
	enrolmentID := q.Get("enrolment_id")
	progType := q.Get("programme_type")
	category := q.Get("result_category")
	pendingFollowUp := q.Get("pending_follow_up") == "true"

	var rows pgx.Rows
	var err error

	switch {
	case pendingFollowUp:
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+resultSelectCols+` FROM screening_results
			 WHERE tenant_id = @tenant_id
			   AND follow_up_required = TRUE
			   AND follow_up_completed_at IS NULL
			 ORDER BY follow_up_due_date ASC NULLS LAST`,
			pgx.NamedArgs{"tenant_id": tenantID})
	case enrolmentID != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+resultSelectCols+` FROM screening_results
			 WHERE tenant_id = @tenant_id AND enrolment_id = @enrolment_id
			 ORDER BY screen_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "enrolment_id": enrolmentID})
	case progType != "" && category != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+resultSelectCols+` FROM screening_results
			 WHERE tenant_id = @tenant_id AND programme_type = @programme_type AND result_category = @result_category
			 ORDER BY screen_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "programme_type": progType, "result_category": category})
	case progType != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+resultSelectCols+` FROM screening_results
			 WHERE tenant_id = @tenant_id AND programme_type = @programme_type
			 ORDER BY screen_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "programme_type": progType})
	case category != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+resultSelectCols+` FROM screening_results
			 WHERE tenant_id = @tenant_id AND result_category = @result_category
			 ORDER BY screen_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "result_category": category})
	default:
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+resultSelectCols+` FROM screening_results
			 WHERE tenant_id = @tenant_id ORDER BY screen_date DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	results := make([]ScreeningResult, 0)
	for rows.Next() {
		var res ScreeningResult
		if err := scanResult(rows, &res); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(res.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		res.PatientNHI = nhi
		results = append(results, res)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *resultHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req ScreeningResult
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.EnrolmentID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "enrolmentId is required"})
		return
	}
	if req.ScreenDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "screenDate is required"})
		return
	}
	if req.ResultCategory == "" {
		req.ResultCategory = "pending"
	}
	if !h.validateHPI(w, r, req.ReportedByHpi) {
		return
	}

	// Fetch enrolment to get encrypted NHI and programme type (validate enrolment belongs to tenant).
	var nhiEnc, progType string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi, programme_type FROM screening_enrolments WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": req.EnrolmentID, "tenant_id": tenantID}).Scan(&nhiEnc, &progType); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "screening enrolment not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}

	var res ScreeningResult
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO screening_results
		    (enrolment_id, patient_nhi, programme_type, screen_date,
		     result_category, result_detail, reported_by_hpi,
		     external_reference_id,
		     follow_up_required, follow_up_action, follow_up_due_date,
		     next_due_date, notes, tenant_id)
		VALUES
		    (@enrolment_id, @patient_nhi, @programme_type, @screen_date,
		     @result_category, @result_detail, @reported_by_hpi,
		     @external_reference_id,
		     @follow_up_required, @follow_up_action, @follow_up_due_date,
		     @next_due_date, @notes, @tenant_id)
		RETURNING `+resultSelectCols,
		pgx.NamedArgs{
			"enrolment_id":          req.EnrolmentID,
			"patient_nhi":           nhiEnc,
			"programme_type":        progType,
			"screen_date":           req.ScreenDate,
			"result_category":       req.ResultCategory,
			"result_detail":         req.ResultDetail,
			"reported_by_hpi":       req.ReportedByHpi,
			"external_reference_id": req.ExternalReferenceID,
			"follow_up_required":    req.FollowUpRequired,
			"follow_up_action":      req.FollowUpAction,
			"follow_up_due_date":    req.FollowUpDueDate,
			"next_due_date":         req.NextDueDate,
			"notes":                 req.Notes,
			"tenant_id":             tenantID,
		}).Scan(
		&res.ID, &res.EnrolmentID, &res.PatientNHI, &res.ProgrammeType,
		&res.ScreenDate, &res.ResultCategory, &res.ResultDetail, &res.ReportedByHpi,
		&res.ExternalReferenceID,
		&res.FollowUpRequired, &res.FollowUpAction, &res.FollowUpDueDate,
		&res.FollowUpCompletedAt, &res.NextDueDate,
		&res.Notes, &res.TenantID, &res.CreatedAt, &res.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}

	// If the result carries a next due date, propagate it to the enrolment.
	if res.NextDueDate != nil {
		_, _ = h.pool.Exec(r.Context(),
			`UPDATE screening_enrolments
			 SET last_screen_date = @screen_date,
			     next_due_date    = @next_due_date,
			     status           = 'active',
			     updated_at       = now()
			 WHERE id = @id AND tenant_id = @tenant_id`,
			pgx.NamedArgs{
				"screen_date":   req.ScreenDate,
				"next_due_date": res.NextDueDate,
				"id":            req.EnrolmentID,
				"tenant_id":     tenantID,
			})
	}

	h.recordAudit(r, "create", "ScreeningResult", res.ID, res.PatientNHI)
	nhi, err := h.decryptNHI(res.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	res.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, res)
}

func (h *resultHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var res ScreeningResult
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+resultSelectCols+` FROM screening_results WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&res.ID, &res.EnrolmentID, &res.PatientNHI, &res.ProgrammeType,
		&res.ScreenDate, &res.ResultCategory, &res.ResultDetail, &res.ReportedByHpi,
		&res.ExternalReferenceID,
		&res.FollowUpRequired, &res.FollowUpAction, &res.FollowUpDueDate,
		&res.FollowUpCompletedAt, &res.NextDueDate,
		&res.Notes, &res.TenantID, &res.CreatedAt, &res.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "screening result not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "ScreeningResult", res.ID, res.PatientNHI)
	nhi, err := h.decryptNHI(res.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	res.PatientNHI = nhi
	writeJSON(w, http.StatusOK, res)
}

// Update amends follow-up details on an existing result, including marking it complete.
func (h *resultHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req struct {
		ResultCategory      string     `json:"resultCategory"`
		ResultDetail        string     `json:"resultDetail"`
		FollowUpRequired    bool       `json:"followUpRequired"`
		FollowUpAction      *string    `json:"followUpAction"`
		FollowUpDueDate     *string    `json:"followUpDueDate"`
		FollowUpCompletedAt *time.Time `json:"followUpCompletedAt"`
		NextDueDate         *string    `json:"nextDueDate"`
		Notes               *string    `json:"notes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var res ScreeningResult
	err := h.pool.QueryRow(r.Context(), `
		UPDATE screening_results
		SET result_category       = COALESCE(NULLIF(@result_category, ''), result_category),
		    result_detail         = COALESCE(NULLIF(@result_detail, ''), result_detail),
		    follow_up_required    = @follow_up_required,
		    follow_up_action      = @follow_up_action,
		    follow_up_due_date    = @follow_up_due_date,
		    follow_up_completed_at = @follow_up_completed_at,
		    next_due_date         = COALESCE(@next_due_date, next_due_date),
		    notes                 = COALESCE(@notes, notes),
		    updated_at            = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+resultSelectCols,
		pgx.NamedArgs{
			"result_category":        req.ResultCategory,
			"result_detail":          req.ResultDetail,
			"follow_up_required":     req.FollowUpRequired,
			"follow_up_action":       req.FollowUpAction,
			"follow_up_due_date":     req.FollowUpDueDate,
			"follow_up_completed_at": req.FollowUpCompletedAt,
			"next_due_date":          req.NextDueDate,
			"notes":                  req.Notes,
			"id":                     id,
			"tenant_id":              tenantID,
		}).Scan(
		&res.ID, &res.EnrolmentID, &res.PatientNHI, &res.ProgrammeType,
		&res.ScreenDate, &res.ResultCategory, &res.ResultDetail, &res.ReportedByHpi,
		&res.ExternalReferenceID,
		&res.FollowUpRequired, &res.FollowUpAction, &res.FollowUpDueDate,
		&res.FollowUpCompletedAt, &res.NextDueDate,
		&res.Notes, &res.TenantID, &res.CreatedAt, &res.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "screening result not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "ScreeningResult", res.ID, res.PatientNHI)
	nhi, err := h.decryptNHI(res.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	res.PatientNHI = nhi
	writeJSON(w, http.StatusOK, res)
}
