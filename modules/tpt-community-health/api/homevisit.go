package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// HomeVisit represents a scheduled community home visit.
// vital_signs and wound_assessments are stored as JSONB; clients send/receive raw JSON.
type HomeVisit struct {
	ID                  string          `json:"id"`
	PatientNHI          string          `json:"patientNhi"`
	ClinicianHpi        string          `json:"clinicianHpi"`
	VisitType           string          `json:"visitType"`
	// wound-care | medication-review | assessment | follow-up | palliative | post-acute | diabetes-care | respiratory | rehabilitation | postnatal
	Priority            string          `json:"priority"`
	// urgent | high | routine | low
	Status              string          `json:"status"`
	// scheduled | in-transit | arrived | in-progress | completed | cancelled | rescheduled | dna
	Address             string          `json:"address"`
	SafetyNotes         *string         `json:"safetyNotes"`
	AccessInstructions  *string         `json:"accessInstructions"`
	VitalSigns          json.RawMessage `json:"vitalSigns,omitempty"`
	WoundAssessments    json.RawMessage `json:"woundAssessments,omitempty"`
	Observations        *string         `json:"observations"`
	Concerns            *string         `json:"concerns"`
	Escalations         *string         `json:"escalations"`
	CancellationReason  *string         `json:"cancellationReason"`
	FollowUpRequired    bool            `json:"followUpRequired"`
	FollowUpDetails     *string         `json:"followUpDetails"`
	TenantID            string          `json:"tenantId"`
	ScheduledAt         time.Time       `json:"scheduledAt"`
	ActualStartAt       *time.Time      `json:"actualStartAt"`
	ActualEndAt         *time.Time      `json:"actualEndAt"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}

const hvSelectCols = `id, patient_nhi, clinician_hpi, visit_type, priority, status,
       address, safety_notes, access_instructions,
       vital_signs, wound_assessments,
       observations, concerns, escalations, cancellation_reason,
       follow_up_required, follow_up_details,
       tenant_id, scheduled_at, actual_start_at, actual_end_at, created_at, updated_at`

func scanHomeVisit(row interface{ Scan(...any) error }, v *HomeVisit) error {
	return row.Scan(
		&v.ID, &v.PatientNHI, &v.ClinicianHpi, &v.VisitType, &v.Priority, &v.Status,
		&v.Address, &v.SafetyNotes, &v.AccessInstructions,
		&v.VitalSigns, &v.WoundAssessments,
		&v.Observations, &v.Concerns, &v.Escalations, &v.CancellationReason,
		&v.FollowUpRequired, &v.FollowUpDetails,
		&v.TenantID, &v.ScheduledAt, &v.ActualStartAt, &v.ActualEndAt, &v.CreatedAt, &v.UpdatedAt,
	)
}

type homeVisitHandler struct{ handlerDeps }

func (h *homeVisitHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	statusFilter := r.URL.Query().Get("status")
	visitTypeFilter := r.URL.Query().Get("visitType")

	var rows pgx.Rows
	var err error
	switch {
	case statusFilter != "" && visitTypeFilter != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+hvSelectCols+` FROM community_home_visits WHERE tenant_id = @tenant_id AND status = @status AND visit_type = @visit_type ORDER BY scheduled_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter, "visit_type": visitTypeFilter})
	case statusFilter != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+hvSelectCols+` FROM community_home_visits WHERE tenant_id = @tenant_id AND status = @status ORDER BY scheduled_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	case visitTypeFilter != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+hvSelectCols+` FROM community_home_visits WHERE tenant_id = @tenant_id AND visit_type = @visit_type ORDER BY scheduled_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "visit_type": visitTypeFilter})
	default:
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+hvSelectCols+` FROM community_home_visits WHERE tenant_id = @tenant_id ORDER BY scheduled_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	visits := make([]HomeVisit, 0)
	for rows.Next() {
		var v HomeVisit
		if err := scanHomeVisit(rows, &v); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(v.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		v.PatientNHI = nhi
		visits = append(visits, v)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, visits)
}

func (h *homeVisitHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req HomeVisit
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Priority == "" {
		req.Priority = "routine"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var v HomeVisit
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO community_home_visits
		    (patient_nhi, clinician_hpi, visit_type, priority, status, address,
		     safety_notes, access_instructions, vital_signs, wound_assessments,
		     observations, concerns, escalations, follow_up_required, follow_up_details,
		     tenant_id, scheduled_at)
		VALUES
		    (@patient_nhi, @clinician_hpi, @visit_type, @priority, 'scheduled', @address,
		     @safety_notes, @access_instructions, @vital_signs, @wound_assessments,
		     @observations, @concerns, @escalations, @follow_up_required, @follow_up_details,
		     @tenant_id, COALESCE(@scheduled_at, now()))
		RETURNING `+hvSelectCols,
		pgx.NamedArgs{
			"patient_nhi":         nhiEnc,
			"clinician_hpi":       req.ClinicianHpi,
			"visit_type":          req.VisitType,
			"priority":            req.Priority,
			"address":             req.Address,
			"safety_notes":        req.SafetyNotes,
			"access_instructions": req.AccessInstructions,
			"vital_signs":         nullableJSON(req.VitalSigns),
			"wound_assessments":   nullableJSON(req.WoundAssessments),
			"observations":        req.Observations,
			"concerns":            req.Concerns,
			"escalations":         req.Escalations,
			"follow_up_required":  req.FollowUpRequired,
			"follow_up_details":   req.FollowUpDetails,
			"tenant_id":           tenantID,
			"scheduled_at":        req.ScheduledAt,
		}).Scan(
		&v.ID, &v.PatientNHI, &v.ClinicianHpi, &v.VisitType, &v.Priority, &v.Status,
		&v.Address, &v.SafetyNotes, &v.AccessInstructions,
		&v.VitalSigns, &v.WoundAssessments,
		&v.Observations, &v.Concerns, &v.Escalations, &v.CancellationReason,
		&v.FollowUpRequired, &v.FollowUpDetails,
		&v.TenantID, &v.ScheduledAt, &v.ActualStartAt, &v.ActualEndAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "HomeVisit", v.ID, v.PatientNHI)
	nhi, _ := h.decryptNHI(v.PatientNHI)
	v.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, v)
}

func (h *homeVisitHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var v HomeVisit
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+hvSelectCols+` FROM community_home_visits WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&v.ID, &v.PatientNHI, &v.ClinicianHpi, &v.VisitType, &v.Priority, &v.Status,
		&v.Address, &v.SafetyNotes, &v.AccessInstructions,
		&v.VitalSigns, &v.WoundAssessments,
		&v.Observations, &v.Concerns, &v.Escalations, &v.CancellationReason,
		&v.FollowUpRequired, &v.FollowUpDetails,
		&v.TenantID, &v.ScheduledAt, &v.ActualStartAt, &v.ActualEndAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "home visit not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, _ := h.decryptNHI(v.PatientNHI)
	v.PatientNHI = nhi
	writeJSON(w, http.StatusOK, v)
}

func (h *homeVisitHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req HomeVisit
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var v HomeVisit
	err := h.pool.QueryRow(r.Context(), `
		UPDATE community_home_visits SET
		    clinician_hpi        = @clinician_hpi,
		    visit_type           = @visit_type,
		    priority             = @priority,
		    status               = @status,
		    address              = @address,
		    safety_notes         = @safety_notes,
		    access_instructions  = @access_instructions,
		    vital_signs          = @vital_signs,
		    wound_assessments    = @wound_assessments,
		    observations         = @observations,
		    concerns             = @concerns,
		    escalations          = @escalations,
		    follow_up_required   = @follow_up_required,
		    follow_up_details    = @follow_up_details,
		    scheduled_at         = @scheduled_at,
		    actual_start_at      = COALESCE(actual_start_at, @actual_start_at),
		    actual_end_at        = COALESCE(actual_end_at, @actual_end_at),
		    updated_at           = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+hvSelectCols,
		pgx.NamedArgs{
			"clinician_hpi":       req.ClinicianHpi,
			"visit_type":          req.VisitType,
			"priority":            req.Priority,
			"status":              req.Status,
			"address":             req.Address,
			"safety_notes":        req.SafetyNotes,
			"access_instructions": req.AccessInstructions,
			"vital_signs":         nullableJSON(req.VitalSigns),
			"wound_assessments":   nullableJSON(req.WoundAssessments),
			"observations":        req.Observations,
			"concerns":            req.Concerns,
			"escalations":         req.Escalations,
			"follow_up_required":  req.FollowUpRequired,
			"follow_up_details":   req.FollowUpDetails,
			"scheduled_at":        req.ScheduledAt,
			"actual_start_at":     req.ActualStartAt,
			"actual_end_at":       req.ActualEndAt,
			"id":                  id,
			"tenant_id":           tenantID,
		}).Scan(
		&v.ID, &v.PatientNHI, &v.ClinicianHpi, &v.VisitType, &v.Priority, &v.Status,
		&v.Address, &v.SafetyNotes, &v.AccessInstructions,
		&v.VitalSigns, &v.WoundAssessments,
		&v.Observations, &v.Concerns, &v.Escalations, &v.CancellationReason,
		&v.FollowUpRequired, &v.FollowUpDetails,
		&v.TenantID, &v.ScheduledAt, &v.ActualStartAt, &v.ActualEndAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "home visit not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "HomeVisit", v.ID, v.PatientNHI)
	nhi, _ := h.decryptNHI(v.PatientNHI)
	v.PatientNHI = nhi
	writeJSON(w, http.StatusOK, v)
}

func (h *homeVisitHandler) Complete(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM community_home_visits WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "home visit not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE community_home_visits
		SET status = 'completed', actual_end_at = COALESCE(actual_end_at, now()), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('completed', 'cancelled')
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "home visit not found or already completed"})
		return
	}
	h.recordAudit(r, "complete", "HomeVisit", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (h *homeVisitHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req struct {
		Reason string `json:"reason"`
	}
	_ = decodeJSON(r, &req)
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM community_home_visits WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "home visit not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE community_home_visits
		SET status = 'cancelled', cancellation_reason = @reason, updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('completed', 'cancelled')
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID, "reason": req.Reason})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "home visit not found or already cancelled"})
		return
	}
	h.recordAudit(r, "cancel", "HomeVisit", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// nullableJSON returns nil when msg is empty or null JSON, otherwise returns the raw bytes.
func nullableJSON(msg json.RawMessage) any {
	if len(msg) == 0 || string(msg) == "null" {
		return nil
	}
	return []byte(msg)
}
