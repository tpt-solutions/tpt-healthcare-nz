package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// NASCReferral represents a NASC (Needs Assessment and Service Coordination) referral
// generated as part of discharge planning from an inpatient rehabilitation episode.
type NASCReferral struct {
	ID                   string     `json:"id"`
	PatientNHI           string     `json:"patientNhi"`
	ReferrerHpi          string     `json:"referrerHpi"`
	NASCRegion           string     `json:"nascRegion"`
	ReferralReason       string     `json:"referralReason"`
	DischargeAdmissionID *string    `json:"dischargeAdmissionId"`
	Urgency              string     `json:"urgency"`
	SupportNeedsSummary  string     `json:"supportNeedsSummary"`
	Status               string     `json:"status"`
	NASCReference        *string    `json:"nascReference"`
	Notes                *string    `json:"notes"`
	TenantID             string     `json:"tenantId"`
	SubmittedAt          *time.Time `json:"submittedAt"`
	AssessedAt           *time.Time `json:"assessedAt"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

const nascSelectCols = `id, patient_nhi, referrer_hpi, nasc_region, referral_reason,
       discharge_admission_id, urgency, support_needs_summary, status,
       nasc_reference, notes, tenant_id, submitted_at, assessed_at, created_at, updated_at`

func scanNASCReferral(row interface{ Scan(...any) error }, n *NASCReferral) error {
	return row.Scan(
		&n.ID, &n.PatientNHI, &n.ReferrerHpi, &n.NASCRegion, &n.ReferralReason,
		&n.DischargeAdmissionID, &n.Urgency, &n.SupportNeedsSummary, &n.Status,
		&n.NASCReference, &n.Notes, &n.TenantID, &n.SubmittedAt, &n.AssessedAt, &n.CreatedAt, &n.UpdatedAt,
	)
}

type nascHandler struct{ handlerDeps }

func (h *nascHandler) List(w http.ResponseWriter, r *http.Request) {
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
			`SELECT `+nascSelectCols+` FROM rehab_nasc_referrals WHERE tenant_id = @tenant_id AND status = @status ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+nascSelectCols+` FROM rehab_nasc_referrals WHERE tenant_id = @tenant_id ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	referrals := make([]NASCReferral, 0)
	for rows.Next() {
		var n NASCReferral
		if err := scanNASCReferral(rows, &n); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(n.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		n.PatientNHI = nhi
		referrals = append(referrals, n)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, referrals)
}

func (h *nascHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req NASCReferral
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Urgency == "" {
		req.Urgency = "routine"
	}
	if req.ReferralReason == "" {
		req.ReferralReason = "long-term-support"
	}
	if !h.validateHPI(w, r, req.ReferrerHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var n NASCReferral
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO rehab_nasc_referrals
		    (patient_nhi, referrer_hpi, nasc_region, referral_reason,
		     discharge_admission_id, urgency, support_needs_summary,
		     status, nasc_reference, notes, tenant_id)
		VALUES
		    (@patient_nhi, @referrer_hpi, @nasc_region, @referral_reason,
		     @discharge_admission_id, @urgency, @support_needs_summary,
		     'draft', @nasc_reference, @notes, @tenant_id)
		RETURNING `+nascSelectCols,
		pgx.NamedArgs{
			"patient_nhi":             nhiEnc,
			"referrer_hpi":            req.ReferrerHpi,
			"nasc_region":             req.NASCRegion,
			"referral_reason":         req.ReferralReason,
			"discharge_admission_id":  req.DischargeAdmissionID,
			"urgency":                 req.Urgency,
			"support_needs_summary":   req.SupportNeedsSummary,
			"nasc_reference":          req.NASCReference,
			"notes":                   req.Notes,
			"tenant_id":               tenantID,
		}).Scan(
		&n.ID, &n.PatientNHI, &n.ReferrerHpi, &n.NASCRegion, &n.ReferralReason,
		&n.DischargeAdmissionID, &n.Urgency, &n.SupportNeedsSummary, &n.Status,
		&n.NASCReference, &n.Notes, &n.TenantID, &n.SubmittedAt, &n.AssessedAt, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "NASCReferral", n.ID, n.PatientNHI)
	nhi, err := h.decryptNHI(n.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	n.PatientNHI = nhi
	writeJSON(w, http.StatusCreated, n)
}

func (h *nascHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var n NASCReferral
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+nascSelectCols+` FROM rehab_nasc_referrals WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&n.ID, &n.PatientNHI, &n.ReferrerHpi, &n.NASCRegion, &n.ReferralReason,
		&n.DischargeAdmissionID, &n.Urgency, &n.SupportNeedsSummary, &n.Status,
		&n.NASCReference, &n.Notes, &n.TenantID, &n.SubmittedAt, &n.AssessedAt, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "NASC referral not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(n.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	n.PatientNHI = nhi
	writeJSON(w, http.StatusOK, n)
}

func (h *nascHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req NASCReferral
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.ReferrerHpi) {
		return
	}
	var n NASCReferral
	err := h.pool.QueryRow(r.Context(), `
		UPDATE rehab_nasc_referrals
		SET nasc_region            = @nasc_region,
		    referral_reason        = @referral_reason,
		    urgency                = @urgency,
		    support_needs_summary  = @support_needs_summary,
		    status                 = @status,
		    nasc_reference         = @nasc_reference,
		    notes                  = @notes,
		    assessed_at            = CASE WHEN @status = 'assessed' AND assessed_at IS NULL THEN now() ELSE assessed_at END,
		    updated_at             = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+nascSelectCols,
		pgx.NamedArgs{
			"nasc_region":           req.NASCRegion,
			"referral_reason":       req.ReferralReason,
			"urgency":               req.Urgency,
			"support_needs_summary": req.SupportNeedsSummary,
			"status":                req.Status,
			"nasc_reference":        req.NASCReference,
			"notes":                 req.Notes,
			"id":                    id,
			"tenant_id":             tenantID,
		}).Scan(
		&n.ID, &n.PatientNHI, &n.ReferrerHpi, &n.NASCRegion, &n.ReferralReason,
		&n.DischargeAdmissionID, &n.Urgency, &n.SupportNeedsSummary, &n.Status,
		&n.NASCReference, &n.Notes, &n.TenantID, &n.SubmittedAt, &n.AssessedAt, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "NASC referral not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "NASCReferral", n.ID, n.PatientNHI)
	nhi, err := h.decryptNHI(n.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	n.PatientNHI = nhi
	writeJSON(w, http.StatusOK, n)
}

func (h *nascHandler) Submit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM rehab_nasc_referrals WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "NASC referral not found or not in draft status"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE rehab_nasc_referrals
		SET status = 'submitted', submitted_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "NASC referral not found or not in draft status"})
		return
	}
	h.recordAudit(r, "update", "NASCReferral", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}
