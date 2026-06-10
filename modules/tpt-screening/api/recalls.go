package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

// ScreeningRecall is a recall notification for an active screening enrolment.
// recall_type values:  initial | routine | overdue | urgent
// contact_method values: letter | sms | email | phone
// status values: pending | sent | acknowledged | completed | declined | lapsed
type ScreeningRecall struct {
	ID             string     `json:"id"`
	EnrolmentID    string     `json:"enrolmentId"`
	PatientNHI     string     `json:"patientNhi"`
	RecallType     string     `json:"recallType"`
	ContactMethod  string     `json:"contactMethod"`
	DueDate        string     `json:"dueDate"` // DATE as YYYY-MM-DD
	Status         string     `json:"status"`
	Notes          *string    `json:"notes"`
	TenantID       string     `json:"tenantId"`
	SentAt         *time.Time `json:"sentAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt"`
	CompletedAt    *time.Time `json:"completedAt"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

const recallSelectCols = `id, enrolment_id, patient_nhi, recall_type, contact_method,
       due_date::text, status, notes, tenant_id,
       sent_at, acknowledged_at, completed_at, created_at, updated_at`

func scanRecall(row interface{ Scan(...any) error }, rc *ScreeningRecall) error {
	return row.Scan(
		&rc.ID, &rc.EnrolmentID, &rc.PatientNHI, &rc.RecallType, &rc.ContactMethod,
		&rc.DueDate, &rc.Status, &rc.Notes, &rc.TenantID,
		&rc.SentAt, &rc.AcknowledgedAt, &rc.CompletedAt, &rc.CreatedAt, &rc.UpdatedAt,
	)
}

type recallHandler struct{ handlerDeps }

func (h *recallHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	q := r.URL.Query()
	enrolmentID := q.Get("enrolment_id")
	status := q.Get("status")
	overdueOnly := q.Get("overdue") == "true"

	var rows pgx.Rows
	var err error

	switch {
	case overdueOnly:
		// Overdue: due_date < today and still pending or sent
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+recallSelectCols+` FROM screening_recalls
			 WHERE tenant_id = @tenant_id AND due_date < CURRENT_DATE AND status IN ('pending', 'sent')
			 ORDER BY due_date ASC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	case enrolmentID != "" && status != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+recallSelectCols+` FROM screening_recalls
			 WHERE tenant_id = @tenant_id AND enrolment_id = @enrolment_id AND status = @status
			 ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "enrolment_id": enrolmentID, "status": status})
	case enrolmentID != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+recallSelectCols+` FROM screening_recalls
			 WHERE tenant_id = @tenant_id AND enrolment_id = @enrolment_id
			 ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "enrolment_id": enrolmentID})
	case status != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+recallSelectCols+` FROM screening_recalls
			 WHERE tenant_id = @tenant_id AND status = @status
			 ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": status})
	default:
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+recallSelectCols+` FROM screening_recalls
			 WHERE tenant_id = @tenant_id ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	recalls := make([]ScreeningRecall, 0)
	for rows.Next() {
		var rc ScreeningRecall
		if err := scanRecall(rows, &rc); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(rc.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		rc.PatientNHI = nhi
		recalls = append(recalls, rc)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, recalls)
}

func (h *recallHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req ScreeningRecall
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.EnrolmentID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "enrolmentId is required"})
		return
	}
	if req.DueDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "dueDate is required"})
		return
	}
	if req.RecallType == "" {
		req.RecallType = "routine"
	}
	if req.ContactMethod == "" {
		req.ContactMethod = "letter"
	}

	// Fetch patient NHI from the enrolment so we can store it denormalised and encrypted.
	// This avoids joins on every recall query while keeping the NHI out of plaintext.
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM screening_enrolments WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": req.EnrolmentID, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "screening enrolment not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}

	var rc ScreeningRecall
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO screening_recalls
		    (enrolment_id, patient_nhi, recall_type, contact_method,
		     due_date, status, notes, tenant_id)
		VALUES
		    (@enrolment_id, @patient_nhi, @recall_type, @contact_method,
		     @due_date, 'pending', @notes, @tenant_id)
		RETURNING `+recallSelectCols,
		pgx.NamedArgs{
			"enrolment_id":   req.EnrolmentID,
			"patient_nhi":    nhiEnc,
			"recall_type":    req.RecallType,
			"contact_method": req.ContactMethod,
			"due_date":       req.DueDate,
			"notes":          req.Notes,
			"tenant_id":      tenantID,
		}).Scan(
		&rc.ID, &rc.EnrolmentID, &rc.PatientNHI, &rc.RecallType, &rc.ContactMethod,
		&rc.DueDate, &rc.Status, &rc.Notes, &rc.TenantID,
		&rc.SentAt, &rc.AcknowledgedAt, &rc.CompletedAt, &rc.CreatedAt, &rc.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "ScreeningRecall", rc.ID, rc.PatientNHI)
	nhi, err := h.decryptNHI(rc.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	rc.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, rc)
}

func (h *recallHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var rc ScreeningRecall
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+recallSelectCols+` FROM screening_recalls WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&rc.ID, &rc.EnrolmentID, &rc.PatientNHI, &rc.RecallType, &rc.ContactMethod,
		&rc.DueDate, &rc.Status, &rc.Notes, &rc.TenantID,
		&rc.SentAt, &rc.AcknowledgedAt, &rc.CompletedAt, &rc.CreatedAt, &rc.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "screening recall not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "ScreeningRecall", rc.ID, rc.PatientNHI)
	nhi, err := h.decryptNHI(rc.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	rc.PatientNHI = nhi
	writeJSON(w, http.StatusOK, rc)
}

func (h *recallHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req ScreeningRecall
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var rc ScreeningRecall
	err := h.pool.QueryRow(r.Context(), `
		UPDATE screening_recalls
		SET recall_type    = @recall_type,
		    contact_method = @contact_method,
		    due_date       = @due_date,
		    notes          = @notes,
		    updated_at     = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'pending'
		RETURNING `+recallSelectCols,
		pgx.NamedArgs{
			"recall_type":    req.RecallType,
			"contact_method": req.ContactMethod,
			"due_date":       req.DueDate,
			"notes":          req.Notes,
			"id":             id,
			"tenant_id":      tenantID,
		}).Scan(
		&rc.ID, &rc.EnrolmentID, &rc.PatientNHI, &rc.RecallType, &rc.ContactMethod,
		&rc.DueDate, &rc.Status, &rc.Notes, &rc.TenantID,
		&rc.SentAt, &rc.AcknowledgedAt, &rc.CompletedAt, &rc.CreatedAt, &rc.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "recall not found or not in pending status"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "ScreeningRecall", rc.ID, rc.PatientNHI)
	nhi, err := h.decryptNHI(rc.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	rc.PatientNHI = nhi
	writeJSON(w, http.StatusOK, rc)
}

// Send marks the recall as dispatched to the patient.
func (h *recallHandler) Send(w http.ResponseWriter, r *http.Request) {
	h.transitionRecall(w, r, "sent", "sent_at = now(),", []string{"pending"})
}

// Acknowledge records that the patient has acknowledged receipt of the recall.
func (h *recallHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	h.transitionRecall(w, r, "acknowledged", "acknowledged_at = now(),", []string{"sent"})
}

// Complete marks the recall as resolved (patient attended screening).
func (h *recallHandler) Complete(w http.ResponseWriter, r *http.Request) {
	h.transitionRecall(w, r, "completed", "completed_at = now(),", []string{"sent", "acknowledged"})
}

// transitionRecall handles the common status-change pattern for send/acknowledge/complete.
// extraSet must include a trailing comma; fromStatuses are the valid prior states.
func (h *recallHandler) transitionRecall(w http.ResponseWriter, r *http.Request, toStatus, extraSet string, fromStatuses []string) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")

	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM screening_recalls WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "screening recall not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}

	// Build IN clause from fromStatuses (safe: values are hardcoded by caller, not user input).
	inClause := "'"
	for i, s := range fromStatuses {
		if i > 0 {
			inClause += "', '"
		}
		inClause += s
	}
	inClause += "'"

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE screening_recalls
		 SET status     = @status,
		     `+extraSet+`
		     updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id AND status IN (`+inClause+`)`,
		pgx.NamedArgs{"status": toStatus, "id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "recall is not in a valid state for this transition"})
		return
	}
	h.recordAudit(r, "update", "ScreeningRecall", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": toStatus})
}
