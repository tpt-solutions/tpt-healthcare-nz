package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// ConsentType classifies the purpose of the consent or assent form.
type ConsentType string

const (
	ConsentTypeTreatment          ConsentType = "treatment"
	ConsentTypeProcedure          ConsentType = "procedure"
	ConsentTypeInformationSharing ConsentType = "information-sharing"
	ConsentTypeResearch           ConsentType = "research"
	ConsentTypeAssent             ConsentType = "assent"
)

// ConsentStatus tracks the lifecycle of a consent form.
type ConsentStatus string

const (
	ConsentStatusDraft     ConsentStatus = "draft"
	ConsentStatusSigned    ConsentStatus = "signed"
	ConsentStatusDeclined  ConsentStatus = "declined"
	ConsentStatusWithdrawn ConsentStatus = "withdrawn"
)

// ConsentRelationship identifies how the consenting person relates to the patient.
type ConsentRelationship string

const (
	ConsentRelMother    ConsentRelationship = "mother"
	ConsentRelFather    ConsentRelationship = "father"
	ConsentRelGuardian  ConsentRelationship = "guardian"
	ConsentRelCaregiver ConsentRelationship = "caregiver"
	ConsentRelSelf      ConsentRelationship = "self"
	ConsentRelOther     ConsentRelationship = "other"
)

// ConsentForm records consent or assent for a clinical action or information disclosure.
// Applies to parent/guardian proxy consent for neonates and children, and to child assent
// from approximately age 7 onward.
type ConsentForm struct {
	ID                  string     `json:"id"`
	ResourceType        string     `json:"resourceType"`
	ResourceID          string     `json:"resourceId"`
	PatientNHI          string     `json:"patientNhi"`
	ConsentType         string     `json:"consentType"`
	ConsentGiven        bool       `json:"consentGiven"`
	GivenByNHI          *string    `json:"givenByNhi"`
	GivenByName         string     `json:"givenByName"`
	GivenByRelationship string     `json:"givenByRelationship"`
	ClinicianHpi        string     `json:"clinicianHpi"`
	Description         string     `json:"description"`
	Notes               *string    `json:"notes"`
	SignedAt            *time.Time `json:"signedAt"`
	WithdrawnAt         *time.Time `json:"withdrawnAt"`
	WithdrawnReason     *string    `json:"withdrawnReason"`
	Status              string     `json:"status"`
	TenantID            string     `json:"tenantId"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// consentHandler manages consent and assent forms for all MCH clinical records.
type consentHandler struct {
	handlerDeps
}

const consentSelectCols = `
	id, resource_type, resource_id, patient_nhi, consent_type, consent_given,
	given_by_nhi, given_by_name, given_by_relationship, clinician_hpi, description,
	notes, signed_at, withdrawn_at, withdrawn_reason, status,
	tenant_id, created_at, updated_at`

func scanConsent(row pgx.Row, f *ConsentForm) error {
	return row.Scan(
		&f.ID, &f.ResourceType, &f.ResourceID, &f.PatientNHI, &f.ConsentType, &f.ConsentGiven,
		&f.GivenByNHI, &f.GivenByName, &f.GivenByRelationship, &f.ClinicianHpi, &f.Description,
		&f.Notes, &f.SignedAt, &f.WithdrawnAt, &f.WithdrawnReason, &f.Status,
		&f.TenantID, &f.CreatedAt, &f.UpdatedAt,
	)
}

// List returns consent forms for a tenant, optionally filtered by resourceType and resourceId.
func (h *consentHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	q := r.URL.Query()
	resourceType := q.Get("resourceType")
	resourceID := q.Get("resourceId")

	var rows pgx.Rows
	var err error
	if resourceType != "" && resourceID != "" {
		rows, err = h.pool.Query(r.Context(), `
			SELECT `+consentSelectCols+`
			FROM mch_consent_forms
			WHERE tenant_id = @tenant_id
			  AND resource_type = @resource_type
			  AND resource_id = @resource_id
			ORDER BY created_at DESC
		`, pgx.NamedArgs{"tenant_id": tenantID, "resource_type": resourceType, "resource_id": resourceID})
	} else if resourceType != "" {
		rows, err = h.pool.Query(r.Context(), `
			SELECT `+consentSelectCols+`
			FROM mch_consent_forms
			WHERE tenant_id = @tenant_id AND resource_type = @resource_type
			ORDER BY created_at DESC
			LIMIT 200
		`, pgx.NamedArgs{"tenant_id": tenantID, "resource_type": resourceType})
	} else {
		rows, err = h.pool.Query(r.Context(), `
			SELECT `+consentSelectCols+`
			FROM mch_consent_forms
			WHERE tenant_id = @tenant_id
			ORDER BY created_at DESC
			LIMIT 200
		`, pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	forms := make([]ConsentForm, 0)
	for rows.Next() {
		var f ConsentForm
		if err := rows.Scan(
			&f.ID, &f.ResourceType, &f.ResourceID, &f.PatientNHI, &f.ConsentType, &f.ConsentGiven,
			&f.GivenByNHI, &f.GivenByName, &f.GivenByRelationship, &f.ClinicianHpi, &f.Description,
			&f.Notes, &f.SignedAt, &f.WithdrawnAt, &f.WithdrawnReason, &f.Status,
			&f.TenantID, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(f.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		f.PatientNHI = nhi
		forms = append(forms, f)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, forms)
}

// Create records a new consent or assent form.
func (h *consentHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req ConsentForm
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ResourceType == "" || req.ResourceID == "" || req.PatientNHI == "" ||
		req.ConsentType == "" || req.GivenByName == "" || req.GivenByRelationship == "" ||
		req.ClinicianHpi == "" || req.Description == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code:    "MISSING_FIELDS",
			Message: "resourceType, resourceId, patientNhi, consentType, givenByName, givenByRelationship, clinicianHpi, and description are required",
		})
		return
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if !h.validateHPI(w, r, req.ClinicianHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var f ConsentForm
	err = scanConsent(h.pool.QueryRow(r.Context(), `
		INSERT INTO mch_consent_forms
		    (resource_type, resource_id, patient_nhi, consent_type, consent_given,
		     given_by_nhi, given_by_name, given_by_relationship, clinician_hpi, description,
		     notes, signed_at, status, tenant_id)
		VALUES
		    (@resource_type, @resource_id, @patient_nhi, @consent_type, @consent_given,
		     @given_by_nhi, @given_by_name, @given_by_relationship, @clinician_hpi, @description,
		     @notes, @signed_at, @status, @tenant_id)
		RETURNING `+consentSelectCols,
		pgx.NamedArgs{
			"resource_type":         req.ResourceType,
			"resource_id":           req.ResourceID,
			"patient_nhi":           nhiEnc,
			"consent_type":          req.ConsentType,
			"consent_given":         req.ConsentGiven,
			"given_by_nhi":          req.GivenByNHI,
			"given_by_name":         req.GivenByName,
			"given_by_relationship": req.GivenByRelationship,
			"clinician_hpi":         req.ClinicianHpi,
			"description":           req.Description,
			"notes":                 req.Notes,
			"signed_at":             req.SignedAt,
			"status":                req.Status,
			"tenant_id":             tenantID,
		},
	), &f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(f.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	f.PatientNHI = nhi
	h.recordAudit(r, "create", "ConsentForm", f.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, f)
}

// Get returns a single consent form by ID.
func (h *consentHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var f ConsentForm
	err := scanConsent(h.pool.QueryRow(r.Context(), `
		SELECT `+consentSelectCols+`
		FROM mch_consent_forms
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}), &f)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "consent form not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(f.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	f.PatientNHI = nhi
	writeJSON(w, http.StatusOK, f)
}

// Update allows updating notes, status, and signed_at on an existing consent form.
// Once withdrawn, no further updates are permitted.
func (h *consentHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req ConsentForm
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var f ConsentForm
	err := scanConsent(h.pool.QueryRow(r.Context(), `
		UPDATE mch_consent_forms
		SET consent_given = @consent_given,
		    status        = @status,
		    signed_at     = @signed_at,
		    notes         = @notes,
		    updated_at    = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status != 'withdrawn'
		RETURNING `+consentSelectCols,
		pgx.NamedArgs{
			"consent_given": req.ConsentGiven,
			"status":        req.Status,
			"signed_at":     req.SignedAt,
			"notes":         req.Notes,
			"id":            id,
			"tenant_id":     tenantID,
		},
	), &f)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "consent form not found or already withdrawn"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "ConsentForm", f.ID, f.PatientNHI)
	nhi, err := h.decryptNHI(f.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	f.PatientNHI = nhi
	writeJSON(w, http.StatusOK, f)
}

// Withdraw marks a consent form as withdrawn. Consent withdrawal is permanent within
// this record; a new consent form must be created if consent is later re-obtained.
func (h *consentHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var body struct {
		Reason string `json:"reason"`
	}
	_ = decodeJSON(r, &body)

	var f ConsentForm
	err := scanConsent(h.pool.QueryRow(r.Context(), `
		UPDATE mch_consent_forms
		SET status           = 'withdrawn',
		    withdrawn_at     = now(),
		    withdrawn_reason = @reason,
		    updated_at       = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status != 'withdrawn'
		RETURNING `+consentSelectCols,
		pgx.NamedArgs{
			"reason":    body.Reason,
			"id":        id,
			"tenant_id": tenantID,
		},
	), &f)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "consent form not found or already withdrawn"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "delete", "ConsentForm", f.ID, f.PatientNHI)
	nhi, err := h.decryptNHI(f.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	f.PatientNHI = nhi
	writeJSON(w, http.StatusOK, f)
}
