package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

type CardiologyAppointment struct {
	ID               string     `json:"id"`
	PatientNHI       string     `json:"patientNhi"`
	ClinicianHpi     string     `json:"clinicianHpi"`
	AppointmentType  string     `json:"appointmentType"`
	Status           string     `json:"status"`
	ReferralSource   string     `json:"referralSource"`
	ReferralDate     *string    `json:"referralDate"`
	Indication       string     `json:"indication"`
	PrimaryDiagnosis string     `json:"primaryDiagnosis"`
	ManagementPlan   string     `json:"managementPlan"`
	FollowUpWeeks    *int       `json:"followUpWeeks"`
	Notes            *string    `json:"notes"`
	TenantID         string     `json:"tenantId"`
	ScheduledAt      time.Time  `json:"scheduledAt"`
	CompletedAt      *time.Time `json:"completedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type outpatientHandler struct{ handlerDeps }

func (h *outpatientHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	statusFilter := r.URL.Query().Get("status")
	var rows pgx.Rows
	var err error
	if statusFilter != "" {
		rows, err = h.pool.Query(r.Context(), `
			SELECT id, patient_nhi, clinician_hpi, appointment_type, status,
			       referral_source, referral_date::text, indication, primary_diagnosis,
			       management_plan, follow_up_weeks, notes,
			       tenant_id, scheduled_at, completed_at, created_at, updated_at
			FROM cardiology_appointments
			WHERE tenant_id = @tenant_id AND status = @status
			ORDER BY scheduled_at DESC
		`, pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(), `
			SELECT id, patient_nhi, clinician_hpi, appointment_type, status,
			       referral_source, referral_date::text, indication, primary_diagnosis,
			       management_plan, follow_up_weeks, notes,
			       tenant_id, scheduled_at, completed_at, created_at, updated_at
			FROM cardiology_appointments
			WHERE tenant_id = @tenant_id
			ORDER BY scheduled_at DESC
		`, pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	appts := make([]CardiologyAppointment, 0)
	for rows.Next() {
		var a CardiologyAppointment
		if err := rows.Scan(
			&a.ID, &a.PatientNHI, &a.ClinicianHpi, &a.AppointmentType, &a.Status,
			&a.ReferralSource, &a.ReferralDate, &a.Indication, &a.PrimaryDiagnosis,
			&a.ManagementPlan, &a.FollowUpWeeks, &a.Notes,
			&a.TenantID, &a.ScheduledAt, &a.CompletedAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(a.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		a.PatientNHI = nhi
		appts = append(appts, a)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, appts)
}

func (h *outpatientHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req CardiologyAppointment
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.AppointmentType == "" {
		req.AppointmentType = "new"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var a CardiologyAppointment
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO cardiology_appointments
		    (patient_nhi, clinician_hpi, appointment_type, status, referral_source,
		     referral_date, indication, primary_diagnosis, management_plan,
		     follow_up_weeks, notes, tenant_id, scheduled_at)
		VALUES
		    (@patient_nhi, @clinician_hpi, @appointment_type, 'scheduled', @referral_source,
		     @referral_date, @indication, @primary_diagnosis, @management_plan,
		     @follow_up_weeks, @notes, @tenant_id, COALESCE(@scheduled_at, now()))
		RETURNING id, patient_nhi, clinician_hpi, appointment_type, status,
		          referral_source, referral_date::text, indication, primary_diagnosis,
		          management_plan, follow_up_weeks, notes,
		          tenant_id, scheduled_at, completed_at, created_at, updated_at
	`, pgx.NamedArgs{
		"patient_nhi":       nhiEnc,
		"clinician_hpi":     req.ClinicianHpi,
		"appointment_type":  req.AppointmentType,
		"referral_source":   req.ReferralSource,
		"referral_date":     req.ReferralDate,
		"indication":        req.Indication,
		"primary_diagnosis": req.PrimaryDiagnosis,
		"management_plan":   req.ManagementPlan,
		"follow_up_weeks":   req.FollowUpWeeks,
		"notes":             req.Notes,
		"tenant_id":         tenantID,
		"scheduled_at":      req.ScheduledAt,
	}).Scan(
		&a.ID, &a.PatientNHI, &a.ClinicianHpi, &a.AppointmentType, &a.Status,
		&a.ReferralSource, &a.ReferralDate, &a.Indication, &a.PrimaryDiagnosis,
		&a.ManagementPlan, &a.FollowUpWeeks, &a.Notes,
		&a.TenantID, &a.ScheduledAt, &a.CompletedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	h.recordAudit(r, "create", "CardiacOutpatient", a.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, a)
}

func (h *outpatientHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var a CardiologyAppointment
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, patient_nhi, clinician_hpi, appointment_type, status,
		       referral_source, referral_date::text, indication, primary_diagnosis,
		       management_plan, follow_up_weeks, notes,
		       tenant_id, scheduled_at, completed_at, created_at, updated_at
		FROM cardiology_appointments
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&a.ID, &a.PatientNHI, &a.ClinicianHpi, &a.AppointmentType, &a.Status,
		&a.ReferralSource, &a.ReferralDate, &a.Indication, &a.PrimaryDiagnosis,
		&a.ManagementPlan, &a.FollowUpWeeks, &a.Notes,
		&a.TenantID, &a.ScheduledAt, &a.CompletedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "appointment not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	writeJSON(w, http.StatusOK, a)
}

func (h *outpatientHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req CardiologyAppointment
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var a CardiologyAppointment
	err := h.pool.QueryRow(r.Context(), `
		UPDATE cardiology_appointments
		SET clinician_hpi      = @clinician_hpi,
		    appointment_type   = @appointment_type,
		    status             = @status,
		    indication         = @indication,
		    primary_diagnosis  = @primary_diagnosis,
		    management_plan    = @management_plan,
		    follow_up_weeks    = @follow_up_weeks,
		    notes              = @notes,
		    updated_at         = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, patient_nhi, clinician_hpi, appointment_type, status,
		          referral_source, referral_date::text, indication, primary_diagnosis,
		          management_plan, follow_up_weeks, notes,
		          tenant_id, scheduled_at, completed_at, created_at, updated_at
	`, pgx.NamedArgs{
		"clinician_hpi":     req.ClinicianHpi,
		"appointment_type":  req.AppointmentType,
		"status":            req.Status,
		"indication":        req.Indication,
		"primary_diagnosis": req.PrimaryDiagnosis,
		"management_plan":   req.ManagementPlan,
		"follow_up_weeks":   req.FollowUpWeeks,
		"notes":             req.Notes,
		"id":                id,
		"tenant_id":         tenantID,
	}).Scan(
		&a.ID, &a.PatientNHI, &a.ClinicianHpi, &a.AppointmentType, &a.Status,
		&a.ReferralSource, &a.ReferralDate, &a.Indication, &a.PrimaryDiagnosis,
		&a.ManagementPlan, &a.FollowUpWeeks, &a.Notes,
		&a.TenantID, &a.ScheduledAt, &a.CompletedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "appointment not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhiEnc := a.PatientNHI
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	h.recordAudit(r, "update", "CardiacOutpatient", a.ID, nhiEnc)
	writeJSON(w, http.StatusOK, a)
}

func (h *outpatientHandler) Complete(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE cardiology_appointments
		SET status = 'completed', completed_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('completed', 'cancelled')
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "appointment not found or already completed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (h *outpatientHandler) DidNotAttend(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE cardiology_appointments
		SET status = 'did-not-attend', updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status IN ('scheduled', 'arrived')
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "appointment not found or not in a schedulable state"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "did-not-attend"})
}
