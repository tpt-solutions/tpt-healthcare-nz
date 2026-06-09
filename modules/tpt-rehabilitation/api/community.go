package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// CommunityEpisode represents a community rehabilitation episode (post-discharge follow-up).
type CommunityEpisode struct {
	ID                  string     `json:"id"`
	PatientNHI          string     `json:"patientNhi"`
	ClinicianHpi        string     `json:"clinicianHpi"`
	ReferralSource      string     `json:"referralSource"`
	DischargeAdmissionID *string   `json:"dischargeAdmissionId"`
	EpisodeType         string     `json:"episodeType"`
	PrimaryDiagnosis    string     `json:"primaryDiagnosis"`
	Status              string     `json:"status"`
	Disciplines         string     `json:"disciplines"`
	VisitFrequency      string     `json:"visitFrequency"`
	VisitsPlanned       *int16     `json:"visitsPlanned"`
	VisitsCompleted     *int16     `json:"visitsCompleted"`
	Notes               *string    `json:"notes"`
	TenantID            string     `json:"tenantId"`
	ReferredAt          time.Time  `json:"referredAt"`
	StartedAt           *time.Time `json:"startedAt"`
	CompletedAt         *time.Time `json:"completedAt"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

const communitySelectCols = `id, patient_nhi, clinician_hpi, referral_source,
       discharge_admission_id, episode_type, primary_diagnosis, status,
       disciplines, visit_frequency, visits_planned, visits_completed,
       notes, tenant_id, referred_at, started_at, completed_at, created_at, updated_at`

func scanCommunityEpisode(row interface{ Scan(...any) error }, c *CommunityEpisode) error {
	return row.Scan(
		&c.ID, &c.PatientNHI, &c.ClinicianHpi, &c.ReferralSource,
		&c.DischargeAdmissionID, &c.EpisodeType, &c.PrimaryDiagnosis, &c.Status,
		&c.Disciplines, &c.VisitFrequency, &c.VisitsPlanned, &c.VisitsCompleted,
		&c.Notes, &c.TenantID, &c.ReferredAt, &c.StartedAt, &c.CompletedAt, &c.CreatedAt, &c.UpdatedAt,
	)
}

type communityHandler struct{ handlerDeps }

func (h *communityHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	statusFilter := r.URL.Query().Get("status")
	var rows pgx.Rows
	var err error
	if statusFilter != "" {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+communitySelectCols+` FROM rehab_community_episodes WHERE tenant_id = @tenant_id AND status = @status ORDER BY referred_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+communitySelectCols+` FROM rehab_community_episodes WHERE tenant_id = @tenant_id ORDER BY referred_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	episodes := make([]CommunityEpisode, 0)
	for rows.Next() {
		var c CommunityEpisode
		if err := scanCommunityEpisode(rows, &c); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(c.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		c.PatientNHI = nhi
		episodes = append(episodes, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, episodes)
}

func (h *communityHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req CommunityEpisode
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.EpisodeType == "" {
		req.EpisodeType = "post-discharge"
	}
	if req.VisitFrequency == "" {
		req.VisitFrequency = "weekly"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var c CommunityEpisode
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO rehab_community_episodes
		    (patient_nhi, clinician_hpi, referral_source, discharge_admission_id,
		     episode_type, primary_diagnosis, status, disciplines,
		     visit_frequency, visits_planned, visits_completed,
		     notes, tenant_id, referred_at)
		VALUES
		    (@patient_nhi, @clinician_hpi, @referral_source, @discharge_admission_id,
		     @episode_type, @primary_diagnosis, 'referred', @disciplines,
		     @visit_frequency, @visits_planned, 0,
		     @notes, @tenant_id, COALESCE(@referred_at, now()))
		RETURNING `+communitySelectCols,
		pgx.NamedArgs{
			"patient_nhi":             nhiEnc,
			"clinician_hpi":           req.ClinicianHpi,
			"referral_source":         req.ReferralSource,
			"discharge_admission_id":  req.DischargeAdmissionID,
			"episode_type":            req.EpisodeType,
			"primary_diagnosis":       req.PrimaryDiagnosis,
			"disciplines":             req.Disciplines,
			"visit_frequency":         req.VisitFrequency,
			"visits_planned":          req.VisitsPlanned,
			"notes":                   req.Notes,
			"tenant_id":               tenantID,
			"referred_at":             req.ReferredAt,
		}).Scan(
		&c.ID, &c.PatientNHI, &c.ClinicianHpi, &c.ReferralSource,
		&c.DischargeAdmissionID, &c.EpisodeType, &c.PrimaryDiagnosis, &c.Status,
		&c.Disciplines, &c.VisitFrequency, &c.VisitsPlanned, &c.VisitsCompleted,
		&c.Notes, &c.TenantID, &c.ReferredAt, &c.StartedAt, &c.CompletedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "CommunityRehab", c.ID, c.PatientNHI)
	nhi, err := h.decryptNHI(c.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	c.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, c)
}

func (h *communityHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var c CommunityEpisode
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+communitySelectCols+` FROM rehab_community_episodes WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&c.ID, &c.PatientNHI, &c.ClinicianHpi, &c.ReferralSource,
		&c.DischargeAdmissionID, &c.EpisodeType, &c.PrimaryDiagnosis, &c.Status,
		&c.Disciplines, &c.VisitFrequency, &c.VisitsPlanned, &c.VisitsCompleted,
		&c.Notes, &c.TenantID, &c.ReferredAt, &c.StartedAt, &c.CompletedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "community episode not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(c.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	c.PatientNHI = nhi
	writeJSON(w, http.StatusOK, c)
}

func (h *communityHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req CommunityEpisode
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	var c CommunityEpisode
	err := h.pool.QueryRow(r.Context(), `
		UPDATE rehab_community_episodes
		SET clinician_hpi       = @clinician_hpi,
		    status              = @status,
		    disciplines         = @disciplines,
		    visit_frequency     = @visit_frequency,
		    visits_planned      = @visits_planned,
		    visits_completed    = @visits_completed,
		    notes               = @notes,
		    started_at          = COALESCE(started_at, CASE WHEN @status = 'active' THEN now() END),
		    updated_at          = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+communitySelectCols,
		pgx.NamedArgs{
			"clinician_hpi":    req.ClinicianHpi,
			"status":           req.Status,
			"disciplines":      req.Disciplines,
			"visit_frequency":  req.VisitFrequency,
			"visits_planned":   req.VisitsPlanned,
			"visits_completed": req.VisitsCompleted,
			"notes":            req.Notes,
			"id":               id,
			"tenant_id":        tenantID,
		}).Scan(
		&c.ID, &c.PatientNHI, &c.ClinicianHpi, &c.ReferralSource,
		&c.DischargeAdmissionID, &c.EpisodeType, &c.PrimaryDiagnosis, &c.Status,
		&c.Disciplines, &c.VisitFrequency, &c.VisitsPlanned, &c.VisitsCompleted,
		&c.Notes, &c.TenantID, &c.ReferredAt, &c.StartedAt, &c.CompletedAt, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "community episode not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "CommunityRehab", c.ID, c.PatientNHI)
	nhi, err := h.decryptNHI(c.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	c.PatientNHI = nhi
	writeJSON(w, http.StatusOK, c)
}

func (h *communityHandler) Complete(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM rehab_community_episodes WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "community episode not found or already completed"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE rehab_community_episodes
		SET status = 'completed', completed_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('completed', 'withdrawn')
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "community episode not found or already completed"})
		return
	}
	h.recordAudit(r, "update", "CommunityRehab", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}
