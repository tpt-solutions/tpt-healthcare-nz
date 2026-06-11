package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// NASCReferral represents a NASC (Needs Assessment and Service Coordination) referral.
// NASC is the entry point to NZ Ministry of Health funded disability support services.
type NASCReferral struct {
	ID                   string     `json:"id"`
	PatientNHI           string     `json:"patientNhi"`
	ReferrerHpi          string     `json:"referrerHpi"`
	NASCOrganisation     string     `json:"nascOrganisation"`
	ReferralReason       string     `json:"referralReason"`
	FundingStream        string     `json:"fundingStream"`
	Urgency              string     `json:"urgency"`
	SupportNeedsSummary  string     `json:"supportNeedsSummary"`
	EligibilityStatus    string     `json:"eligibilityStatus"`
	NASCReference        *string    `json:"nascReference"`
	Status               string     `json:"status"`
	Notes                *string    `json:"notes"`
	TenantID             string     `json:"tenantId"`
	SubmittedAt          *time.Time `json:"submittedAt"`
	AssessedAt           *time.Time `json:"assessedAt"`
	AllocatedAt          *time.Time `json:"allocatedAt"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

const nascSelectCols = `id, patient_nhi, referrer_hpi, nasc_organisation, referral_reason,
       funding_stream, urgency, support_needs_summary, eligibility_status,
       nasc_reference, status, notes, tenant_id,
       submitted_at, assessed_at, allocated_at, created_at, updated_at`

func scanNASCReferral(row interface{ Scan(...any) error }, n *NASCReferral) error {
	return row.Scan(
		&n.ID, &n.PatientNHI, &n.ReferrerHpi, &n.NASCOrganisation, &n.ReferralReason,
		&n.FundingStream, &n.Urgency, &n.SupportNeedsSummary, &n.EligibilityStatus,
		&n.NASCReference, &n.Status, &n.Notes, &n.TenantID,
		&n.SubmittedAt, &n.AssessedAt, &n.AllocatedAt, &n.CreatedAt, &n.UpdatedAt,
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
			`SELECT `+nascSelectCols+` FROM disability_nasc_referrals WHERE tenant_id = @tenant_id AND status = @status ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+nascSelectCols+` FROM disability_nasc_referrals WHERE tenant_id = @tenant_id ORDER BY created_at DESC`,
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
	if req.FundingStream == "" {
		req.FundingStream = "DSS"
	}
	if req.Urgency == "" {
		req.Urgency = "routine"
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
		INSERT INTO disability_nasc_referrals
		    (patient_nhi, referrer_hpi, nasc_organisation, referral_reason,
		     funding_stream, urgency, support_needs_summary,
		     eligibility_status, nasc_reference, status, notes, tenant_id)
		VALUES
		    (@patient_nhi, @referrer_hpi, @nasc_organisation, @referral_reason,
		     @funding_stream, @urgency, @support_needs_summary,
		     'pending', @nasc_reference, 'draft', @notes, @tenant_id)
		RETURNING `+nascSelectCols,
		pgx.NamedArgs{
			"patient_nhi":           nhiEnc,
			"referrer_hpi":          req.ReferrerHpi,
			"nasc_organisation":     req.NASCOrganisation,
			"referral_reason":       req.ReferralReason,
			"funding_stream":        req.FundingStream,
			"urgency":               req.Urgency,
			"support_needs_summary": req.SupportNeedsSummary,
			"nasc_reference":        req.NASCReference,
			"notes":                 req.Notes,
			"tenant_id":             tenantID,
		}).Scan(
		&n.ID, &n.PatientNHI, &n.ReferrerHpi, &n.NASCOrganisation, &n.ReferralReason,
		&n.FundingStream, &n.Urgency, &n.SupportNeedsSummary, &n.EligibilityStatus,
		&n.NASCReference, &n.Status, &n.Notes, &n.TenantID,
		&n.SubmittedAt, &n.AssessedAt, &n.AllocatedAt, &n.CreatedAt, &n.UpdatedAt,
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
		`SELECT `+nascSelectCols+` FROM disability_nasc_referrals WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&n.ID, &n.PatientNHI, &n.ReferrerHpi, &n.NASCOrganisation, &n.ReferralReason,
		&n.FundingStream, &n.Urgency, &n.SupportNeedsSummary, &n.EligibilityStatus,
		&n.NASCReference, &n.Status, &n.Notes, &n.TenantID,
		&n.SubmittedAt, &n.AssessedAt, &n.AllocatedAt, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "NASC referral not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "NASCReferral", n.ID, n.PatientNHI)
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
		UPDATE disability_nasc_referrals
		SET nasc_organisation     = @nasc_organisation,
		    referral_reason       = @referral_reason,
		    funding_stream        = @funding_stream,
		    urgency               = @urgency,
		    support_needs_summary = @support_needs_summary,
		    eligibility_status    = @eligibility_status,
		    nasc_reference        = @nasc_reference,
		    status                = @status,
		    notes                 = @notes,
		    updated_at            = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING `+nascSelectCols,
		pgx.NamedArgs{
			"nasc_organisation":     req.NASCOrganisation,
			"referral_reason":       req.ReferralReason,
			"funding_stream":        req.FundingStream,
			"urgency":               req.Urgency,
			"support_needs_summary": req.SupportNeedsSummary,
			"eligibility_status":    req.EligibilityStatus,
			"nasc_reference":        req.NASCReference,
			"status":                req.Status,
			"notes":                 req.Notes,
			"id":                    id,
			"tenant_id":             tenantID,
		}).Scan(
		&n.ID, &n.PatientNHI, &n.ReferrerHpi, &n.NASCOrganisation, &n.ReferralReason,
		&n.FundingStream, &n.Urgency, &n.SupportNeedsSummary, &n.EligibilityStatus,
		&n.NASCReference, &n.Status, &n.Notes, &n.TenantID,
		&n.SubmittedAt, &n.AssessedAt, &n.AllocatedAt, &n.CreatedAt, &n.UpdatedAt,
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
		`SELECT patient_nhi FROM disability_nasc_referrals WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "NASC referral not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE disability_nasc_referrals
		SET status = 'submitted', submitted_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "NASC referral is not in draft status"})
		return
	}
	h.recordAudit(r, "update", "NASCReferral", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}

// Assess records the outcome of a NASC needs assessment, setting eligibility and
// transitioning status to assessed or allocated as appropriate.
func (h *nascHandler) Assess(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var body struct {
		EligibilityStatus string  `json:"eligibilityStatus"`
		NASCReference     *string `json:"nascReference"`
		Notes             *string `json:"notes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if body.EligibilityStatus == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "eligibilityStatus is required"})
		return
	}
	var nhiEnc string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT patient_nhi FROM disability_nasc_referrals WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&nhiEnc); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "NASC referral not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE disability_nasc_referrals
		SET eligibility_status = @eligibility_status,
		    nasc_reference     = COALESCE(@nasc_reference, nasc_reference),
		    notes              = COALESCE(@notes, notes),
		    status             = 'assessed',
		    assessed_at        = now(),
		    updated_at         = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status IN ('submitted', 'acknowledged')
	`, pgx.NamedArgs{
		"eligibility_status": body.EligibilityStatus,
		"nasc_reference":     body.NASCReference,
		"notes":              body.Notes,
		"id":                 id,
		"tenant_id":          tenantID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "NASC referral must be submitted or acknowledged to assess"})
		return
	}
	h.recordAudit(r, "update", "NASCReferral", id, nhiEnc)
	writeJSON(w, http.StatusOK, map[string]string{"status": "assessed", "eligibilityStatus": body.EligibilityStatus})
}
