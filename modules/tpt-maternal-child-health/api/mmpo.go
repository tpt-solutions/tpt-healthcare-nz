package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// MMPOClaimType identifies the service category under the LMC Schedule of Payments.
type MMPOClaimType string

const (
	MMPOClaimBooking              MMPOClaimType = "booking"
	MMPOClaimAntenatalVisit       MMPOClaimType = "antenatal-visit"
	MMPOClaimIntrapartumPrimary   MMPOClaimType = "intrapartum-primary"
	MMPOClaimIntrapartumSecondary MMPOClaimType = "intrapartum-secondary"
	MMPOClaimPostnatalVisit       MMPOClaimType = "postnatal-visit"
	MMPOClaimOnCall               MMPOClaimType = "on-call"
	MMPOClaimRuralPremium         MMPOClaimType = "rural-premium"
	MMPOClaimOther                MMPOClaimType = "other"
)

// MMPOClaimStatus tracks the lifecycle of an MMPO funding claim.
type MMPOClaimStatus string

const (
	MMPOClaimStatusDraft     MMPOClaimStatus = "draft"
	MMPOClaimStatusSubmitted MMPOClaimStatus = "submitted"
	MMPOClaimStatusAccepted  MMPOClaimStatus = "accepted"
	MMPOClaimStatusRejected  MMPOClaimStatus = "rejected"
	MMPOClaimStatusPaid      MMPOClaimStatus = "paid"
	MMPOClaimStatusWithdrawn MMPOClaimStatus = "withdrawn"
)

type MMPOClaim struct {
	ID                 string     `json:"id"`
	MaternityEpisodeID string     `json:"maternityEpisodeId"`
	LMCHpi             string     `json:"lmcHpi"`
	MmpoProviderNumber string     `json:"mmpoProviderNumber"`
	ClaimType          string     `json:"claimType"`
	ServiceDate        string     `json:"serviceDate"`
	ServiceCode        string     `json:"serviceCode"`
	Units              float64    `json:"units"`
	AmountNzd          *float64   `json:"amountNzd"`
	Status             string     `json:"status"`
	ClaimReference     *string    `json:"claimReference"`
	SubmittedAt        *time.Time `json:"submittedAt"`
	ResponseCode       *string    `json:"responseCode"`
	ResponseMessage    *string    `json:"responseMessage"`
	Notes              *string    `json:"notes"`
	TenantID           string     `json:"tenantId"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// mmpoHandler manages MMPO LMC funding claims.
type mmpoHandler struct {
	handlerDeps
}

func (h *mmpoHandler) List(w http.ResponseWriter, r *http.Request) {
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
			SELECT id, maternity_episode_id, lmc_hpi, mmpo_provider_number, claim_type,
			       service_date::text, service_code, units, amount_nzd, status,
			       claim_reference, submitted_at, response_code, response_message, notes,
			       tenant_id, created_at, updated_at
			FROM mmpo_claims
			WHERE tenant_id = @tenant_id AND status = @status
			ORDER BY service_date DESC
		`, pgx.NamedArgs{"tenant_id": tenantID, "status": statusFilter})
	} else {
		rows, err = h.pool.Query(r.Context(), `
			SELECT id, maternity_episode_id, lmc_hpi, mmpo_provider_number, claim_type,
			       service_date::text, service_code, units, amount_nzd, status,
			       claim_reference, submitted_at, response_code, response_message, notes,
			       tenant_id, created_at, updated_at
			FROM mmpo_claims
			WHERE tenant_id = @tenant_id
			ORDER BY service_date DESC
		`, pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	claims := make([]MMPOClaim, 0)
	for rows.Next() {
		var c MMPOClaim
		if err := rows.Scan(
			&c.ID, &c.MaternityEpisodeID, &c.LMCHpi, &c.MmpoProviderNumber, &c.ClaimType,
			&c.ServiceDate, &c.ServiceCode, &c.Units, &c.AmountNzd, &c.Status,
			&c.ClaimReference, &c.SubmittedAt, &c.ResponseCode, &c.ResponseMessage, &c.Notes,
			&c.TenantID, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		claims = append(claims, c)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, claims)
}

func (h *mmpoHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req MMPOClaim
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.MaternityEpisodeID == "" || req.LMCHpi == "" || req.ServiceDate == "" || req.ServiceCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELDS", Message: "maternityEpisodeId, lmcHpi, serviceDate, and serviceCode are required"})
		return
	}
	if req.Units == 0 {
		req.Units = 1
	}
	if !h.validateHPI(w, r, req.LMCHpi) {
		return
	}
	var c MMPOClaim
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO mmpo_claims
		    (maternity_episode_id, lmc_hpi, mmpo_provider_number, claim_type,
		     service_date, service_code, units, amount_nzd, status, notes, tenant_id)
		VALUES
		    (@episode_id, @lmc_hpi, @mmpo_provider_number, @claim_type,
		     @service_date, @service_code, @units, @amount_nzd, 'draft', @notes, @tenant_id)
		RETURNING id, maternity_episode_id, lmc_hpi, mmpo_provider_number, claim_type,
		          service_date::text, service_code, units, amount_nzd, status,
		          claim_reference, submitted_at, response_code, response_message, notes,
		          tenant_id, created_at, updated_at
	`, pgx.NamedArgs{
		"episode_id":           req.MaternityEpisodeID,
		"lmc_hpi":              req.LMCHpi,
		"mmpo_provider_number": req.MmpoProviderNumber,
		"claim_type":           req.ClaimType,
		"service_date":         req.ServiceDate,
		"service_code":         req.ServiceCode,
		"units":                req.Units,
		"amount_nzd":           req.AmountNzd,
		"notes":                req.Notes,
		"tenant_id":            tenantID,
	}).Scan(
		&c.ID, &c.MaternityEpisodeID, &c.LMCHpi, &c.MmpoProviderNumber, &c.ClaimType,
		&c.ServiceDate, &c.ServiceCode, &c.Units, &c.AmountNzd, &c.Status,
		&c.ClaimReference, &c.SubmittedAt, &c.ResponseCode, &c.ResponseMessage, &c.Notes,
		&c.TenantID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "MMPOClaim", c.ID, "")
	writeJSON(w, http.StatusCreated, c)
}

func (h *mmpoHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var c MMPOClaim
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, maternity_episode_id, lmc_hpi, mmpo_provider_number, claim_type,
		       service_date::text, service_code, units, amount_nzd, status,
		       claim_reference, submitted_at, response_code, response_message, notes,
		       tenant_id, created_at, updated_at
		FROM mmpo_claims
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&c.ID, &c.MaternityEpisodeID, &c.LMCHpi, &c.MmpoProviderNumber, &c.ClaimType,
		&c.ServiceDate, &c.ServiceCode, &c.Units, &c.AmountNzd, &c.Status,
		&c.ClaimReference, &c.SubmittedAt, &c.ResponseCode, &c.ResponseMessage, &c.Notes,
		&c.TenantID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "claim not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *mmpoHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req MMPOClaim
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var c MMPOClaim
	err := h.pool.QueryRow(r.Context(), `
		UPDATE mmpo_claims
		SET claim_type = @claim_type,
		    service_date = @service_date,
		    service_code = @service_code,
		    units = @units,
		    amount_nzd = @amount_nzd,
		    notes = @notes,
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
		RETURNING id, maternity_episode_id, lmc_hpi, mmpo_provider_number, claim_type,
		          service_date::text, service_code, units, amount_nzd, status,
		          claim_reference, submitted_at, response_code, response_message, notes,
		          tenant_id, created_at, updated_at
	`, pgx.NamedArgs{
		"claim_type":   req.ClaimType,
		"service_date": req.ServiceDate,
		"service_code": req.ServiceCode,
		"units":        req.Units,
		"amount_nzd":   req.AmountNzd,
		"notes":        req.Notes,
		"id":           id,
		"tenant_id":    tenantID,
	}).Scan(
		&c.ID, &c.MaternityEpisodeID, &c.LMCHpi, &c.MmpoProviderNumber, &c.ClaimType,
		&c.ServiceDate, &c.ServiceCode, &c.Units, &c.AmountNzd, &c.Status,
		&c.ClaimReference, &c.SubmittedAt, &c.ResponseCode, &c.ResponseMessage, &c.Notes,
		&c.TenantID, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "claim not found or not in draft status"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "MMPOClaim", c.ID, "")
	writeJSON(w, http.StatusOK, c)
}

func (h *mmpoHandler) Submit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE mmpo_claims
		SET status = 'submitted', submitted_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'draft'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "claim not found or not in draft status"})
		return
	}
	h.recordAudit(r, "update", "MMPOClaim", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}

func (h *mmpoHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE mmpo_claims
		SET status = 'withdrawn', updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status IN ('draft', 'submitted')
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "claim not found or cannot be withdrawn"})
		return
	}
	h.recordAudit(r, "delete", "MMPOClaim", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "withdrawn"})
}
