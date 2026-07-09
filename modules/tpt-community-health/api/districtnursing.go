package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// CarePlan represents a district nursing care plan.
type CarePlan struct {
	ID           string `json:"id"`
	PatientNHI   string `json:"patientNhi"`
	ClinicianHpi string `json:"clinicianHpi"`
	PlanName     string `json:"planName"`
	PlanType     string `json:"planType"`
	// wound-care | palliative | diabetes | heart-failure | copd | post-surgical | post-acute | medication-management
	Status string `json:"status"`
	// draft | active | under-review | completed | suspended
	RiskLevel string `json:"riskLevel"`
	// low | moderate | high | very-high
	PrimaryNeed  string     `json:"primaryNeed"`
	Goals        string     `json:"goals"`
	DhbFunded    bool       `json:"dhbFunded"`
	FundingCode  *string    `json:"fundingCode"`
	ConsentGiven bool       `json:"consentGiven"`
	ConsentAt    *time.Time `json:"consentAt"`
	Notes        *string    `json:"notes"`
	TenantID     string     `json:"tenantId"`
	StartedAt    time.Time  `json:"startedAt"`
	ReviewAt     *time.Time `json:"reviewAt"`
	CompletedAt  *time.Time `json:"completedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

const cpSelectCols = `id, patient_nhi, clinician_hpi, plan_name, plan_type, status,
       risk_level, primary_need, goals, dhb_funded, funding_code,
       consent_given, consent_at, notes,
       tenant_id, started_at, review_at, completed_at, created_at, updated_at`

func scanCarePlan(row interface{ Scan(...any) error }, p *CarePlan) error {
	return row.Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.PlanName, &p.PlanType, &p.Status,
		&p.RiskLevel, &p.PrimaryNeed, &p.Goals, &p.DhbFunded, &p.FundingCode,
		&p.ConsentGiven, &p.ConsentAt, &p.Notes,
		&p.TenantID, &p.StartedAt, &p.ReviewAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
}

type carePlanHandler struct{ handlerDeps }

func (h *carePlanHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	statusFilter := r.URL.Query().Get("status")
	planTypeFilter := r.URL.Query().Get("planType")

	var rows pgx.Rows
	var err error
	switch {
	case statusFilter != "" && planTypeFilter != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+cpSelectCols+` FROM community_care_plans WHERE tenant_id = @tenant_id AND status = @status AND plan_type = @plan_type ORDER BY started_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter, "plan_type": planTypeFilter})
	case statusFilter != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+cpSelectCols+` FROM community_care_plans WHERE tenant_id = @tenant_id AND status = @status ORDER BY started_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	case planTypeFilter != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+cpSelectCols+` FROM community_care_plans WHERE tenant_id = @tenant_id AND plan_type = @plan_type ORDER BY started_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "plan_type": planTypeFilter})
	default:
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+cpSelectCols+` FROM community_care_plans WHERE tenant_id = @tenant_id ORDER BY started_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	plans := make([]CarePlan, 0)
	for rows.Next() {
		var p CarePlan
		if err := scanCarePlan(rows, &p); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, _ := h.decryptNHI(p.PatientNHI)
		p.PatientNHI = nhi
		plans = append(plans, p)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, plans)
}

func (h *carePlanHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req CarePlan
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.RiskLevel == "" {
		req.RiskLevel = "low"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var p CarePlan
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO community_care_plans
		    (patient_nhi, clinician_hpi, plan_name, plan_type, status,
		     risk_level, primary_need, goals, dhb_funded, funding_code,
		     consent_given, consent_at, notes, tenant_id, started_at, review_at)
		VALUES
		    (@patient_nhi, @clinician_hpi, @plan_name, @plan_type, 'draft',
		     @risk_level, @primary_need, @goals, @dhb_funded, @funding_code,
		     @consent_given, @consent_at, @notes, @tenant_id,
		     COALESCE(@started_at, now()), @review_at)
		RETURNING `+cpSelectCols,
		pgx.NamedArgs{
			"patient_nhi":   nhiEnc,
			"clinician_hpi": req.ClinicianHpi,
			"plan_name":     req.PlanName,
			"plan_type":     req.PlanType,
			"risk_level":    req.RiskLevel,
			"primary_need":  req.PrimaryNeed,
			"goals":         req.Goals,
			"dhb_funded":    req.DhbFunded,
			"funding_code":  req.FundingCode,
			"consent_given": req.ConsentGiven,
			"consent_at":    req.ConsentAt,
			"notes":         req.Notes,
			"tenant_id":     tenantID,
			"started_at":    req.StartedAt,
			"review_at":     req.ReviewAt,
		}).Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.PlanName, &p.PlanType, &p.Status,
		&p.RiskLevel, &p.PrimaryNeed, &p.Goals, &p.DhbFunded, &p.FundingCode,
		&p.ConsentGiven, &p.ConsentAt, &p.Notes,
		&p.TenantID, &p.StartedAt, &p.ReviewAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "CarePlan", p.ID, p.PatientNHI)
	nhi, _ := h.decryptNHI(p.PatientNHI)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, p)
}

func (h *carePlanHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var p CarePlan
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+cpSelectCols+` FROM community_care_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.PlanName, &p.PlanType, &p.Status,
		&p.RiskLevel, &p.PrimaryNeed, &p.Goals, &p.DhbFunded, &p.FundingCode,
		&p.ConsentGiven, &p.ConsentAt, &p.Notes,
		&p.TenantID, &p.StartedAt, &p.ReviewAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, _ := h.decryptNHI(p.PatientNHI)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

func (h *carePlanHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req CarePlan
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var p CarePlan
	err := h.pool.QueryRow(r.Context(), `
		UPDATE community_care_plans SET
		    clinician_hpi  = @clinician_hpi,
		    plan_name      = @plan_name,
		    plan_type      = @plan_type,
		    status         = @status,
		    risk_level     = @risk_level,
		    primary_need   = @primary_need,
		    goals          = @goals,
		    dhb_funded     = @dhb_funded,
		    funding_code   = @funding_code,
		    consent_given  = @consent_given,
		    consent_at     = COALESCE(consent_at, @consent_at),
		    notes          = @notes,
		    review_at      = @review_at,
		    updated_at     = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+cpSelectCols,
		pgx.NamedArgs{
			"clinician_hpi": req.ClinicianHpi,
			"plan_name":     req.PlanName,
			"plan_type":     req.PlanType,
			"status":        req.Status,
			"risk_level":    req.RiskLevel,
			"primary_need":  req.PrimaryNeed,
			"goals":         req.Goals,
			"dhb_funded":    req.DhbFunded,
			"funding_code":  req.FundingCode,
			"consent_given": req.ConsentGiven,
			"consent_at":    req.ConsentAt,
			"notes":         req.Notes,
			"review_at":     req.ReviewAt,
			"id":            id,
			"tenant_id":     tenantID,
		}).Scan(
		&p.ID, &p.PatientNHI, &p.ClinicianHpi, &p.PlanName, &p.PlanType, &p.Status,
		&p.RiskLevel, &p.PrimaryNeed, &p.Goals, &p.DhbFunded, &p.FundingCode,
		&p.ConsentGiven, &p.ConsentAt, &p.Notes,
		&p.TenantID, &p.StartedAt, &p.ReviewAt, &p.CompletedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "CarePlan", p.ID, p.PatientNHI)
	nhi, _ := h.decryptNHI(p.PatientNHI)
	p.PatientNHI = nhi
	writeJSON(w, http.StatusOK, p)
}

func (h *carePlanHandler) Activate(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM community_care_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE community_care_plans
		SET status = 'active', updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATE", Message: "care plan is not in draft status"})
		return
	}
	h.recordAudit(r, "activate", "CarePlan", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

func (h *carePlanHandler) Complete(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM community_care_plans WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE community_care_plans
		SET status = 'completed', completed_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('completed', 'suspended')
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found or already completed"})
		return
	}
	h.recordAudit(r, "complete", "CarePlan", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// ---------------------------------------------------------------------------
// Nursing Visit
// ---------------------------------------------------------------------------

// NursingVisit represents a district nursing visit within a care plan.
// vital_signs, wound_assessments, medications_administered are JSONB.
type NursingVisit struct {
	ID           string `json:"id"`
	CarePlanID   string `json:"carePlanId"`
	PatientNHI   string `json:"patientNhi"`
	ClinicianHpi string `json:"clinicianHpi"`
	VisitType    string `json:"visitType"`
	// scheduled | unscheduled | urgent
	Status string `json:"status"`
	// scheduled | in-progress | completed | cancelled
	VitalSigns              json.RawMessage `json:"vitalSigns,omitempty"`
	WoundAssessments        json.RawMessage `json:"woundAssessments,omitempty"`
	MedicationsAdministered json.RawMessage `json:"medicationsAdministered,omitempty"`
	Observations            *string         `json:"observations"`
	PatientEducation        *string         `json:"patientEducation"`
	Concerns                *string         `json:"concerns"`
	Escalations             *string         `json:"escalations"`
	FollowUpRequired        bool            `json:"followUpRequired"`
	Notes                   *string         `json:"notes"`
	TenantID                string          `json:"tenantId"`
	ScheduledAt             time.Time       `json:"scheduledAt"`
	CompletedAt             *time.Time      `json:"completedAt"`
	CreatedAt               time.Time       `json:"createdAt"`
	UpdatedAt               time.Time       `json:"updatedAt"`
}

const nvSelectCols = `id, care_plan_id, patient_nhi, clinician_hpi, visit_type, status,
       vital_signs, wound_assessments, medications_administered,
       observations, patient_education, concerns, escalations,
       follow_up_required, notes,
       tenant_id, scheduled_at, completed_at, created_at, updated_at`

func scanNursingVisit(row interface{ Scan(...any) error }, v *NursingVisit) error {
	return row.Scan(
		&v.ID, &v.CarePlanID, &v.PatientNHI, &v.ClinicianHpi, &v.VisitType, &v.Status,
		&v.VitalSigns, &v.WoundAssessments, &v.MedicationsAdministered,
		&v.Observations, &v.PatientEducation, &v.Concerns, &v.Escalations,
		&v.FollowUpRequired, &v.Notes,
		&v.TenantID, &v.ScheduledAt, &v.CompletedAt, &v.CreatedAt, &v.UpdatedAt,
	)
}

type nursingVisitHandler struct{ handlerDeps }

func (h *nursingVisitHandler) ListForPlan(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	planID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT `+nvSelectCols+` FROM community_nursing_visits WHERE care_plan_id = @care_plan_id AND tenant_id = @tenant_id ORDER BY scheduled_at DESC`,
		pgx.NamedArgs{"care_plan_id": planID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	visits := make([]NursingVisit, 0)
	for rows.Next() {
		var v NursingVisit
		if err := scanNursingVisit(rows, &v); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, _ := h.decryptNHI(v.PatientNHI)
		v.PatientNHI = nhi
		visits = append(visits, v)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, visits)
}

func (h *nursingVisitHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	planID := r.PathValue("id")
	var req NursingVisit
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.VisitType == "" {
		req.VisitType = "scheduled"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var v NursingVisit
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO community_nursing_visits
		    (care_plan_id, patient_nhi, clinician_hpi, visit_type, status,
		     vital_signs, wound_assessments, medications_administered,
		     observations, patient_education, concerns, escalations,
		     follow_up_required, notes, tenant_id, scheduled_at)
		VALUES
		    (@care_plan_id, @patient_nhi, @clinician_hpi, @visit_type, 'scheduled',
		     @vital_signs, @wound_assessments, @medications_administered,
		     @observations, @patient_education, @concerns, @escalations,
		     @follow_up_required, @notes, @tenant_id, COALESCE(@scheduled_at, now()))
		RETURNING `+nvSelectCols,
		pgx.NamedArgs{
			"care_plan_id":             planID,
			"patient_nhi":              nhiEnc,
			"clinician_hpi":            req.ClinicianHpi,
			"visit_type":               req.VisitType,
			"vital_signs":              nullableJSON(req.VitalSigns),
			"wound_assessments":        nullableJSON(req.WoundAssessments),
			"medications_administered": nullableJSON(req.MedicationsAdministered),
			"observations":             req.Observations,
			"patient_education":        req.PatientEducation,
			"concerns":                 req.Concerns,
			"escalations":              req.Escalations,
			"follow_up_required":       req.FollowUpRequired,
			"notes":                    req.Notes,
			"tenant_id":                tenantID,
			"scheduled_at":             req.ScheduledAt,
		}).Scan(
		&v.ID, &v.CarePlanID, &v.PatientNHI, &v.ClinicianHpi, &v.VisitType, &v.Status,
		&v.VitalSigns, &v.WoundAssessments, &v.MedicationsAdministered,
		&v.Observations, &v.PatientEducation, &v.Concerns, &v.Escalations,
		&v.FollowUpRequired, &v.Notes,
		&v.TenantID, &v.ScheduledAt, &v.CompletedAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "NursingVisit", v.ID, v.PatientNHI)
	nhi, _ := h.decryptNHI(v.PatientNHI)
	v.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, v)
}

func (h *nursingVisitHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var v NursingVisit
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+nvSelectCols+` FROM community_nursing_visits WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&v.ID, &v.CarePlanID, &v.PatientNHI, &v.ClinicianHpi, &v.VisitType, &v.Status,
		&v.VitalSigns, &v.WoundAssessments, &v.MedicationsAdministered,
		&v.Observations, &v.PatientEducation, &v.Concerns, &v.Escalations,
		&v.FollowUpRequired, &v.Notes,
		&v.TenantID, &v.ScheduledAt, &v.CompletedAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "nursing visit not found"})
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

func (h *nursingVisitHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req NursingVisit
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var v NursingVisit
	err := h.pool.QueryRow(r.Context(), `
		UPDATE community_nursing_visits SET
		    clinician_hpi               = @clinician_hpi,
		    visit_type                  = @visit_type,
		    status                      = @status,
		    vital_signs                 = @vital_signs,
		    wound_assessments           = @wound_assessments,
		    medications_administered    = @medications_administered,
		    observations                = @observations,
		    patient_education           = @patient_education,
		    concerns                    = @concerns,
		    escalations                 = @escalations,
		    follow_up_required          = @follow_up_required,
		    notes                       = @notes,
		    updated_at                  = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+nvSelectCols,
		pgx.NamedArgs{
			"clinician_hpi":            req.ClinicianHpi,
			"visit_type":               req.VisitType,
			"status":                   req.Status,
			"vital_signs":              nullableJSON(req.VitalSigns),
			"wound_assessments":        nullableJSON(req.WoundAssessments),
			"medications_administered": nullableJSON(req.MedicationsAdministered),
			"observations":             req.Observations,
			"patient_education":        req.PatientEducation,
			"concerns":                 req.Concerns,
			"escalations":              req.Escalations,
			"follow_up_required":       req.FollowUpRequired,
			"notes":                    req.Notes,
			"id":                       id,
			"tenant_id":                tenantID,
		}).Scan(
		&v.ID, &v.CarePlanID, &v.PatientNHI, &v.ClinicianHpi, &v.VisitType, &v.Status,
		&v.VitalSigns, &v.WoundAssessments, &v.MedicationsAdministered,
		&v.Observations, &v.PatientEducation, &v.Concerns, &v.Escalations,
		&v.FollowUpRequired, &v.Notes,
		&v.TenantID, &v.ScheduledAt, &v.CompletedAt, &v.CreatedAt, &v.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "nursing visit not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "NursingVisit", v.ID, v.PatientNHI)
	nhi, _ := h.decryptNHI(v.PatientNHI)
	v.PatientNHI = nhi
	writeJSON(w, http.StatusOK, v)
}

func (h *nursingVisitHandler) Complete(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM community_nursing_visits WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "nursing visit not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE community_nursing_visits
		SET status = 'completed', completed_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('completed', 'cancelled')
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "nursing visit not found or already completed"})
		return
	}
	h.recordAudit(r, "complete", "NursingVisit", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}
