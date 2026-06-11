package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

// ScreeningEnrolment is a patient's enrolment record in a national screening programme.
// programme_type values: cervical | bowel | breast | newborn_metabolic |
//
//	newborn_hearing | antenatal_hiv | antenatal_syphilis
//
// status values: eligible | enrolled | active | suspended | declined | withdrawn | completed
type ScreeningEnrolment struct {
	ID                string     `json:"id"`
	PatientNHI        string     `json:"patientNhi"`
	ProgrammeType     string     `json:"programmeType"`
	Status            string     `json:"status"`
	EnrolledByHpi     string     `json:"enrolledByHpi"`
	RegistryReference *string    `json:"registryReference"`
	LastScreenDate    *string    `json:"lastScreenDate"` // DATE as YYYY-MM-DD
	NextDueDate       *string    `json:"nextDueDate"`    // DATE as YYYY-MM-DD
	Notes             *string    `json:"notes"`
	TenantID          string     `json:"tenantId"`
	EnrolledAt        *time.Time `json:"enrolledAt"`
	SuspendedAt       *time.Time `json:"suspendedAt"`
	WithdrawnAt       *time.Time `json:"withdrawnAt"`
	CompletedAt       *time.Time `json:"completedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

const enrolmentSelectCols = `id, patient_nhi, programme_type, status, enrolled_by_hpi,
       registry_reference, last_screen_date::text, next_due_date::text,
       notes, tenant_id,
       enrolled_at, suspended_at, withdrawn_at, completed_at,
       created_at, updated_at`

func scanEnrolment(row interface{ Scan(...any) error }, e *ScreeningEnrolment) error {
	return row.Scan(
		&e.ID, &e.PatientNHI, &e.ProgrammeType, &e.Status, &e.EnrolledByHpi,
		&e.RegistryReference, &e.LastScreenDate, &e.NextDueDate,
		&e.Notes, &e.TenantID,
		&e.EnrolledAt, &e.SuspendedAt, &e.WithdrawnAt, &e.CompletedAt,
		&e.CreatedAt, &e.UpdatedAt,
	)
}

type enrolmentHandler struct{ handlerDeps }

func (h *enrolmentHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	q := r.URL.Query()
	progType := q.Get("programme_type")
	status := q.Get("status")

	var rows pgx.Rows
	var err error
	switch {
	case progType != "" && status != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+enrolmentSelectCols+` FROM screening_enrolments
			 WHERE tenant_id = @tenant_id AND programme_type = @programme_type AND status = @status
			 ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "programme_type": progType, "status": status})
	case progType != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+enrolmentSelectCols+` FROM screening_enrolments
			 WHERE tenant_id = @tenant_id AND programme_type = @programme_type
			 ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "programme_type": progType})
	case status != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+enrolmentSelectCols+` FROM screening_enrolments
			 WHERE tenant_id = @tenant_id AND status = @status
			 ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": status})
	default:
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+enrolmentSelectCols+` FROM screening_enrolments
			 WHERE tenant_id = @tenant_id ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	enrolments := make([]ScreeningEnrolment, 0)
	for rows.Next() {
		var e ScreeningEnrolment
		if err := scanEnrolment(rows, &e); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(e.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		e.PatientNHI = nhi
		enrolments = append(enrolments, e)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, enrolments)
}

func (h *enrolmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req ScreeningEnrolment
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ProgrammeType == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "programmeType is required"})
		return
	}
	if req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "patientNhi is required"})
		return
	}
	if !h.validateHPI(w, r, req.EnrolledByHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var e ScreeningEnrolment
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO screening_enrolments
		    (patient_nhi, programme_type, status, enrolled_by_hpi,
		     registry_reference, last_screen_date, next_due_date, notes, tenant_id, enrolled_at)
		VALUES
		    (@patient_nhi, @programme_type, 'enrolled', @enrolled_by_hpi,
		     @registry_reference, @last_screen_date, @next_due_date, @notes, @tenant_id, now())
		RETURNING `+enrolmentSelectCols,
		pgx.NamedArgs{
			"patient_nhi":        nhiEnc,
			"programme_type":     req.ProgrammeType,
			"enrolled_by_hpi":    req.EnrolledByHpi,
			"registry_reference": req.RegistryReference,
			"last_screen_date":   req.LastScreenDate,
			"next_due_date":      req.NextDueDate,
			"notes":              req.Notes,
			"tenant_id":          tenantID,
		}).Scan(
		&e.ID, &e.PatientNHI, &e.ProgrammeType, &e.Status, &e.EnrolledByHpi,
		&e.RegistryReference, &e.LastScreenDate, &e.NextDueDate,
		&e.Notes, &e.TenantID,
		&e.EnrolledAt, &e.SuspendedAt, &e.WithdrawnAt, &e.CompletedAt,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "ScreeningEnrolment", e.ID, e.PatientNHI)
	nhi, err := h.decryptNHI(e.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	e.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, e)
}

func (h *enrolmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var e ScreeningEnrolment
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+enrolmentSelectCols+` FROM screening_enrolments WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&e.ID, &e.PatientNHI, &e.ProgrammeType, &e.Status, &e.EnrolledByHpi,
		&e.RegistryReference, &e.LastScreenDate, &e.NextDueDate,
		&e.Notes, &e.TenantID,
		&e.EnrolledAt, &e.SuspendedAt, &e.WithdrawnAt, &e.CompletedAt,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "screening enrolment not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "ScreeningEnrolment", e.ID, e.PatientNHI)
	nhi, err := h.decryptNHI(e.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	e.PatientNHI = nhi
	writeJSON(w, http.StatusOK, e)
}

func (h *enrolmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req ScreeningEnrolment
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.EnrolledByHpi) {
		return
	}
	var e ScreeningEnrolment
	err := h.pool.QueryRow(r.Context(), `
		UPDATE screening_enrolments
		SET status             = @status,
		    enrolled_by_hpi    = @enrolled_by_hpi,
		    registry_reference = @registry_reference,
		    last_screen_date   = @last_screen_date,
		    next_due_date      = @next_due_date,
		    notes              = @notes,
		    updated_at         = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+enrolmentSelectCols,
		pgx.NamedArgs{
			"status":             req.Status,
			"enrolled_by_hpi":    req.EnrolledByHpi,
			"registry_reference": req.RegistryReference,
			"last_screen_date":   req.LastScreenDate,
			"next_due_date":      req.NextDueDate,
			"notes":              req.Notes,
			"id":                 id,
			"tenant_id":          tenantID,
		}).Scan(
		&e.ID, &e.PatientNHI, &e.ProgrammeType, &e.Status, &e.EnrolledByHpi,
		&e.RegistryReference, &e.LastScreenDate, &e.NextDueDate,
		&e.Notes, &e.TenantID,
		&e.EnrolledAt, &e.SuspendedAt, &e.WithdrawnAt, &e.CompletedAt,
		&e.CreatedAt, &e.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "screening enrolment not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "ScreeningEnrolment", e.ID, e.PatientNHI)
	nhi, err := h.decryptNHI(e.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	e.PatientNHI = nhi
	writeJSON(w, http.StatusOK, e)
}

// Suspend transitions the enrolment to suspended status.
// A reason note may be provided in the request body.
func (h *enrolmentHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var body struct {
		Notes *string `json:"notes"`
	}
	_ = decodeJSON(r, &body)

	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM screening_enrolments WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "screening enrolment not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE screening_enrolments
		SET status       = 'suspended',
		    suspended_at = now(),
		    notes        = COALESCE(@notes, notes),
		    updated_at   = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status IN ('enrolled', 'active')
	`, pgx.NamedArgs{"notes": body.Notes, "id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "enrolment must be enrolled or active to suspend"})
		return
	}
	h.recordAudit(r, "update", "ScreeningEnrolment", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "suspended"})
}

// Withdraw removes the patient from the programme.
// A reason note may be provided in the request body.
func (h *enrolmentHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var body struct {
		Notes *string `json:"notes"`
	}
	_ = decodeJSON(r, &body)

	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM screening_enrolments WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "screening enrolment not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE screening_enrolments
		SET status       = 'withdrawn',
		    withdrawn_at = now(),
		    notes        = COALESCE(@notes, notes),
		    updated_at   = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('withdrawn', 'completed')
	`, pgx.NamedArgs{"notes": body.Notes, "id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "enrolment is already withdrawn or completed"})
		return
	}
	h.recordAudit(r, "update", "ScreeningEnrolment", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "withdrawn"})
}
