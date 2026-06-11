package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/nes"
	"github.com/PhillipC05/tpt-healthcare/core/nhi"
)

// PatientsHandler handles all /api/v1/patients routes.
type PatientsHandler struct {
	pool                db.Pool
	enc                 *encryption.Cipher
	nhiClient           *nhi.Client
	nesClient           *nes.Client
	auditTrail          *audit.Trail
	tenantHPIFacilityID string // HPI facility OrgID for this practice (needed for NES transfers)
	logger              *slog.Logger
}

// patientRecord is the internal representation stored in the database.
// PHI fields (nhiEncrypted, name, dob) are AES-256-GCM encrypted at rest.
type patientRecord struct {
	ID           string    `json:"id"`
	NHIEncrypted []byte    `json:"-"`
	NHI          string    `json:"nhi,omitempty"` // plaintext, only populated after decryption
	TenantID     string    `json:"tenantId"`
	FHIRResource []byte    `json:"-"` // encrypted FHIR Patient JSON
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// patientResponse is the API response for a patient resource.
type patientResponse struct {
	ID        string      `json:"id"`
	NHI       string      `json:"nhi"`
	TenantID  string      `json:"tenantId"`
	Patient   *r5.Patient `json:"patient"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// patientCreateRequest is the request body for POST /api/v1/patients.
type patientCreateRequest struct {
	NHI     string      `json:"nhi"`
	Patient *r5.Patient `json:"patient"`
}

// patientUpdateRequest is the request body for PUT /api/v1/patients/{id}.
type patientUpdateRequest struct {
	Patient *r5.Patient `json:"patient"`
}

// enrolmentRequest is the request body for POST and PUT /api/v1/patients/{id}/enrolment.
type enrolmentRequest struct {
	// PractitionerHPI is the individual practitioner's HPI Common Person Number (CPN).
	PractitionerHPI string `json:"practitionerHpi"`
	// PracticeHPI is the HPI facility OrgID of the enrolling practice.
	// Required for UpdateEnrolment; ignored by CreateEnrolment which derives the
	// practice from the authenticated tenant.
	PracticeHPI string `json:"practiceHpi"`
	FundingCode string `json:"fundingCode"`
	StartDate   string `json:"startDate"` // YYYY-MM-DD
}

// List handles GET /api/v1/patients.
// Supports query parameters: name, nhi, dob.
// All results are filtered by the tenant extracted from context.
func (h *PatientsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	q := r.URL.Query()
	nameFilter := q.Get("name")
	nhiFilter := q.Get("nhi")
	dobFilter := q.Get("dob")

	records, err := h.searchPatients(ctx, tenantID.String(), nameFilter, nhiFilter, dobFilter)
	if err != nil {
		h.logger.Error("search patients", slog.Any("error", err), slog.String("tenant", tenantID.String()))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SEARCH_ERROR", Message: "failed to search patients"})
		return
	}

	responses := make([]patientResponse, 0, len(records))
	for _, rec := range records {
		resp, err := h.recordToResponse(ctx, rec)
		if err != nil {
			h.logger.Error("decrypt patient record", slog.Any("error", err), slog.String("id", rec.ID))
			continue
		}
		responses = append(responses, resp)
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "Patient",
		ResourceID:   "search",
		TenantID:     tenantID,
		Details: map[string]any{
			"name": nameFilter,
			"nhi":  nhiFilter,
			"dob":  dobFilter,
		},
		OccurredAt: time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"patients": responses,
		"total":    len(responses),
	})
}

// Get handles GET /api/v1/patients/{id}.
func (h *PatientsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "patient ID is required"})
		return
	}

	rec, err := h.getPatientByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "patient not found"})
			return
		}
		h.logger.Error("get patient", slog.Any("error", err), slog.String("id", id))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve patient"})
		return
	}

	resp, err := h.recordToResponse(ctx, rec)
	if err != nil {
		h.logger.Error("decrypt patient", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient data"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "Patient",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, resp)
}

// GetByNHI handles GET /api/v1/patients/nhi/{nhi}.
// Validates NHI format, queries the Ministry NHI API, and returns the FHIR Patient.
func (h *PatientsHandler) GetByNHI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	nhiValue := r.PathValue("nhi")
	if err := validateNHIFormat(nhiValue); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: err.Error()})
		return
	}

	patient, err := h.nhiClient.Lookup(ctx, nhiValue)
	if err != nil {
		if errors.Is(err, nhi.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NHI_NOT_FOUND", Message: "no patient found for NHI"})
			return
		}
		h.logger.Error("NHI lookup", slog.Any("error", err), slog.String("nhi", nhiValue))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "NHI_LOOKUP_ERROR", Message: "NHI API lookup failed"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "Patient",
		ResourceID:   "nhi-lookup",
		TenantID:     tenantID,
		Details:      map[string]any{"nhi": nhiValue},
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, patient)
}

// Create handles POST /api/v1/patients.
// Validates the NHI, confirms it with the NHI API, then persists the patient record.
func (h *PatientsHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req patientCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if err := validateNHIFormat(req.NHI); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: err.Error()})
		return
	}
	if req.Patient == nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patient FHIR resource is required"})
		return
	}

	// Confirm NHI exists with the Ministry before registration.
	if _, err := h.nhiClient.Lookup(ctx, req.NHI); err != nil {
		if errors.Is(err, nhi.ErrNotFound) {
			writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "NHI_NOT_FOUND", Message: "NHI not found in Ministry registry"})
			return
		}
		h.logger.Error("NHI confirm", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "NHI_CONFIRM_ERROR", Message: "could not confirm NHI with Ministry"})
		return
	}

	rec, err := h.persistPatient(ctx, req.NHI, req.Patient, tenantID.String())
	if err != nil {
		h.logger.Error("persist patient", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "PERSIST_ERROR", Message: "failed to save patient"})
		return
	}

	resp, err := h.recordToResponse(ctx, rec)
	if err != nil {
		h.logger.Error("decrypt patient", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient data"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "Patient",
		ResourceID:   rec.ID,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusCreated, resp)
}

// Update handles PUT /api/v1/patients/{id}.
// Updates patient demographics; the NHI itself is immutable.
func (h *PatientsHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "patient ID is required"})
		return
	}

	var req patientUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Patient == nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patient FHIR resource is required"})
		return
	}

	rec, err := h.getPatientByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "patient not found"})
			return
		}
		h.logger.Error("get patient for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve patient"})
		return
	}

	updated, err := h.updatePatientFHIR(ctx, rec.ID, req.Patient, tenantID.String())
	if err != nil {
		h.logger.Error("update patient", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update patient"})
		return
	}

	resp, err := h.recordToResponse(ctx, updated)
	if err != nil {
		h.logger.Error("decrypt patient", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient data"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "Patient",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, resp)
}

// checkDisclosureConsent returns true when an active disclosure consent exists
// for patientNHI within tenantID, per HIPC Rule 11.
func (h *PatientsHandler) checkDisclosureConsent(ctx context.Context, tenantID, patientNHI string) (bool, error) {
	var granted bool
	err := h.pool.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM consents
		     WHERE tenant_id   = @tenant_id
		       AND patient_nhi = @patient_nhi
		       AND consent_type = 'disclosure'
		       AND granted = TRUE
		       AND revoked_at IS NULL
		       AND (expires_at IS NULL OR expires_at > NOW())
		 )`,
		db.NamedArgs{"tenant_id": tenantID, "patient_nhi": patientNHI},
	).Scan(&granted)
	if err != nil {
		return false, fmt.Errorf("check disclosure consent: %w", err)
	}
	return granted, nil
}

// GetEnrolment handles GET /api/v1/patients/{id}/enrolment.
// Returns the patient's NES enrolment status for this practice.
func (h *PatientsHandler) GetEnrolment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "patient ID is required"})
		return
	}

	rec, err := h.getPatientByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "patient not found"})
			return
		}
		h.logger.Error("get patient for enrolment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve patient"})
		return
	}

	nhiPlain, err := h.enc.Decrypt(rec.NHIEncrypted)
	if err != nil {
		h.logger.Error("decrypt NHI", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt NHI"})
		return
	}

	// HIPC Rule 11: verify disclosure consent before returning enrolment data.
	hasConsent, err := h.checkDisclosureConsent(ctx, tenantID.String(), string(nhiPlain))
	if err != nil {
		h.logger.Error("check disclosure consent", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CONSENT_ERROR", Message: "failed to verify disclosure consent"})
		return
	}
	if !hasConsent {
		writeJSON(w, http.StatusForbidden, apiError{Code: "CONSENT_REQUIRED", Message: "patient has not consented to disclosure of enrolment information"})
		return
	}

	enrolment, err := h.nesClient.GetEnrolment(ctx, string(nhiPlain))
	if err != nil {
		if errors.Is(err, nes.ErrNotEnrolled) {
			writeJSON(w, http.StatusOK, map[string]any{"enrolled": false, "patientId": id})
			return
		}
		h.logger.Error("NES get enrolment", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "NES_ERROR", Message: "failed to retrieve enrolment from NES"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "Coverage",
		ResourceID:   id,
		TenantID:     tenantID,
		OccurredAt:   time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, enrolment)
}

// CreateEnrolment handles POST /api/v1/patients/{id}/enrolment.
// Enrols the patient in this practice via the NES API.
func (h *PatientsHandler) CreateEnrolment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "patient ID is required"})
		return
	}

	var req enrolmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PractitionerHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_HPI", Message: "practitionerHpi is required"})
		return
	}

	rec, err := h.getPatientByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "patient not found"})
			return
		}
		h.logger.Error("get patient for enrolment create", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve patient"})
		return
	}

	nhiPlain, err := h.enc.Decrypt(rec.NHIEncrypted)
	if err != nil {
		h.logger.Error("decrypt NHI for enrolment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt NHI"})
		return
	}

	// HIPC Rule 11: verify disclosure consent before submitting enrolment data.
	hasConsent, err := h.checkDisclosureConsent(ctx, tenantID, string(nhiPlain))
	if err != nil {
		h.logger.Error("check disclosure consent for enrolment create", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CONSENT_ERROR", Message: "failed to verify disclosure consent"})
		return
	}
	if !hasConsent {
		writeJSON(w, http.StatusForbidden, apiError{Code: "CONSENT_REQUIRED", Message: "patient has not consented to disclosure of enrolment information"})
		return
	}

	enrolment, err := h.nesClient.Enrol(ctx, nes.EnrolRequest{
		NHI:             string(nhiPlain),
		PractitionerHPI: req.PractitionerHPI,
		FundingCode:     req.FundingCode,
		StartDate:       req.StartDate,
	})
	if err != nil {
		h.logger.Error("NES enrol", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "NES_ENROL_ERROR", Message: "failed to enrol patient via NES"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Coverage",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, enrolment)
}

// UpdateEnrolment handles PUT /api/v1/patients/{id}/enrolment.
// Updates enrolment details (practitioner, funding code) via the NES API.
func (h *PatientsHandler) UpdateEnrolment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "patient ID is required"})
		return
	}

	var req enrolmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PractitionerHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_HPI", Message: "practitionerHpi is required"})
		return
	}

	rec, err := h.getPatientByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "patient not found"})
			return
		}
		h.logger.Error("get patient for enrolment update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve patient"})
		return
	}

	nhiPlain, err := h.enc.Decrypt(rec.NHIEncrypted)
	if err != nil {
		h.logger.Error("decrypt NHI for enrolment update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt NHI"})
		return
	}

	// HIPC Rule 11: verify disclosure consent before transmitting enrolment data.
	hasConsent, err := h.checkDisclosureConsent(ctx, tenantID, string(nhiPlain))
	if err != nil {
		h.logger.Error("check disclosure consent for enrolment update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CONSENT_ERROR", Message: "failed to verify disclosure consent"})
		return
	}
	if !hasConsent {
		writeJSON(w, http.StatusForbidden, apiError{Code: "CONSENT_REQUIRED", Message: "patient has not consented to disclosure of enrolment information"})
		return
	}

	if req.PracticeHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PRACTICE_HPI", Message: "practiceHpi (HPI facility OrgID) is required for enrolment update"})
		return
	}

	// PracticeID is the HPI facility OrgID (e.g. "G00001-A"), not the individual's CPN.
	enrolment, err := h.nesClient.Update(ctx, nes.Enrolment{
		NHI:        string(nhiPlain),
		PracticeID: req.PracticeHPI,
		Status:     nes.Active,
	})
	if err != nil {
		h.logger.Error("NES update enrolment", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "NES_UPDATE_ERROR", Message: "failed to update enrolment via NES"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Coverage",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "update-enrolment"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, enrolment)
}

// transferRequest is the body for POST /api/v1/patients/{id}/enrolment/transfer.
type transferRequest struct {
	ToPractitionerHPI string `json:"toPractitionerHpi"`
	FundingCode       string `json:"fundingCode,omitempty"`
	TransferDate      string `json:"transferDate"` // YYYY-MM-DD
	Reason            string `json:"reason,omitempty"`
}

// TransferEnrolment handles POST /api/v1/patients/{id}/enrolment/transfer.
// Transfers the patient's enrolment to a different practice via the NES API.
func (h *PatientsHandler) TransferEnrolment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "patient ID is required"})
		return
	}

	var req transferRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ToPractitionerHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_HPI", Message: "toPractitionerHpi is required"})
		return
	}
	if req.TransferDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DATE", Message: "transferDate (YYYY-MM-DD) is required"})
		return
	}

	rec, err := h.getPatientByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "patient not found"})
			return
		}
		h.logger.Error("get patient for enrolment transfer", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve patient"})
		return
	}

	nhiPlain, err := h.enc.Decrypt(rec.NHIEncrypted)
	if err != nil {
		h.logger.Error("decrypt NHI for transfer", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt NHI"})
		return
	}

	// HIPC Rule 11: verify disclosure consent before transferring enrolment data.
	hasConsent, err := h.checkDisclosureConsent(ctx, tenantID, string(nhiPlain))
	if err != nil {
		h.logger.Error("check disclosure consent for enrolment transfer", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CONSENT_ERROR", Message: "failed to verify disclosure consent"})
		return
	}
	if !hasConsent {
		writeJSON(w, http.StatusForbidden, apiError{Code: "CONSENT_REQUIRED", Message: "patient has not consented to disclosure of enrolment information"})
		return
	}

	// The fromPracticeID is this tenant's HPI facility OrgID, configured at server startup.
	// An empty facility ID means this server is not configured for NES transfers.
	if h.tenantHPIFacilityID == "" {
		writeJSON(w, http.StatusServiceUnavailable, apiError{
			Code:    "TRANSFER_UNAVAILABLE",
			Message: "NES transfer is not available: tenant HPI facility ID is not configured",
		})
		return
	}

	if err := h.nesClient.Transfer(ctx, string(nhiPlain), h.tenantHPIFacilityID, req.ToPractitionerHPI); err != nil {
		h.logger.Error("NES transfer enrolment", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "NES_TRANSFER_ERROR", Message: "failed to transfer enrolment via NES"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Coverage",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "transfer", "to_hpi": req.ToPractitionerHPI},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"patientId":        id,
		"toPractitionerHpi": req.ToPractitionerHPI,
		"transferDate":     req.TransferDate,
		"status":           "transferred",
	})
}

// validateNHIFormat validates the NHI using the checksum-aware validator in core/nhi/.
// This covers both structural format and the Luhn check digit for old-format NHIs.
func validateNHIFormat(nhiValue string) error {
	if !nhi.ValidateNHI(nhiValue) {
		return fmt.Errorf("invalid NHI %q: must be a valid NHI number (AAA9999 or AAA99AA format with correct check digit)", nhiValue)
	}
	return nil
}

// errNotFound is a sentinel for missing records.
var errNotFound = errors.New("record not found")

// searchPatients queries the patient table for matching records within the tenant.
// All filter values are applied as ILIKE / exact matches against decrypted index columns.
func (h *PatientsHandler) searchPatients(ctx context.Context, tenantID, name, nhiFilter, dob string) ([]patientRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, nhi_encrypted, tenant_id, fhir_resource, created_at, updated_at
		 FROM patients
		 WHERE tenant_id = @tenant_id
		   AND (@name_filter = '' OR name_search ILIKE '%' || @name_filter || '%')
		   AND (@nhi_filter  = '' OR nhi_index = @nhi_filter)
		   AND (@dob_filter  = '' OR dob_index = @dob_filter)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":   tenantID,
			"name_filter": name,
			"nhi_filter":  nhiFilter,
			"dob_filter":  dob,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query patients: %w", err)
	}
	defer rows.Close()

	var results []patientRecord
	for rows.Next() {
		var rec patientRecord
		if err := rows.Scan(&rec.ID, &rec.NHIEncrypted, &rec.TenantID, &rec.FHIRResource, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan patient row: %w", err)
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

// getPatientByID retrieves a single patient record by internal UUID, enforcing tenant isolation.
func (h *PatientsHandler) getPatientByID(ctx context.Context, id, tenantID string) (patientRecord, error) {
	var rec patientRecord
	err := h.pool.QueryRow(ctx,
		`SELECT id, nhi_encrypted, tenant_id, fhir_resource, created_at, updated_at
		 FROM patients
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(&rec.ID, &rec.NHIEncrypted, &rec.TenantID, &rec.FHIRResource, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if db.IsNoRows(err) {
			return patientRecord{}, errNotFound
		}
		return patientRecord{}, fmt.Errorf("get patient by id: %w", err)
	}
	return rec, nil
}

// persistPatient encrypts PHI and inserts a new patient record.
func (h *PatientsHandler) persistPatient(ctx context.Context, nhiValue string, patient *r5.Patient, tenantID string) (patientRecord, error) {
	nhiEnc, err := h.enc.Encrypt([]byte(nhiValue))
	if err != nil {
		return patientRecord{}, fmt.Errorf("encrypt NHI: %w", err)
	}

	fhirJSON, err := patient.MarshalJSON()
	if err != nil {
		return patientRecord{}, fmt.Errorf("marshal patient FHIR: %w", err)
	}
	fhirEnc, err := h.enc.Encrypt(fhirJSON)
	if err != nil {
		return patientRecord{}, fmt.Errorf("encrypt FHIR resource: %w", err)
	}

	nameSearch := patient.SearchName()
	dobIndex := patient.BirthDate

	var rec patientRecord
	err = h.pool.QueryRow(ctx,
		`INSERT INTO patients (nhi_encrypted, nhi_index, tenant_id, fhir_resource, name_search, dob_index)
		 VALUES (@nhi_encrypted, @nhi_index, @tenant_id, @fhir_resource, @name_search, @dob_index)
		 RETURNING id, nhi_encrypted, tenant_id, fhir_resource, created_at, updated_at`,
		db.NamedArgs{
			"nhi_encrypted": nhiEnc,
			"nhi_index":     nhiValue, // stored encrypted at DB level via column-level encryption policy
			"tenant_id":     tenantID,
			"fhir_resource": fhirEnc,
			"name_search":   nameSearch,
			"dob_index":     dobIndex,
		},
	).Scan(&rec.ID, &rec.NHIEncrypted, &rec.TenantID, &rec.FHIRResource, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		return patientRecord{}, fmt.Errorf("insert patient: %w", err)
	}
	return rec, nil
}

// updatePatientFHIR updates only the FHIR resource blob of an existing patient.
func (h *PatientsHandler) updatePatientFHIR(ctx context.Context, id string, patient *r5.Patient, tenantID string) (patientRecord, error) {
	fhirJSON, err := patient.MarshalJSON()
	if err != nil {
		return patientRecord{}, fmt.Errorf("marshal patient FHIR: %w", err)
	}
	fhirEnc, err := h.enc.Encrypt(fhirJSON)
	if err != nil {
		return patientRecord{}, fmt.Errorf("encrypt FHIR resource: %w", err)
	}
	nameSearch := patient.SearchName()

	var rec patientRecord
	err = h.pool.QueryRow(ctx,
		`UPDATE patients
		 SET fhir_resource = @fhir_resource,
		     name_search   = @name_search,
		     updated_at    = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, nhi_encrypted, tenant_id, fhir_resource, created_at, updated_at`,
		db.NamedArgs{
			"fhir_resource": fhirEnc,
			"name_search":   nameSearch,
			"id":            id,
			"tenant_id":     tenantID,
		},
	).Scan(&rec.ID, &rec.NHIEncrypted, &rec.TenantID, &rec.FHIRResource, &rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if db.IsNoRows(err) {
			return patientRecord{}, errNotFound
		}
		return patientRecord{}, fmt.Errorf("update patient FHIR: %w", err)
	}
	return rec, nil
}

// recordToResponse decrypts a patientRecord and returns an API-safe patientResponse.
func (h *PatientsHandler) recordToResponse(_ context.Context, rec patientRecord) (patientResponse, error) {
	nhiPlain, err := h.enc.Decrypt(rec.NHIEncrypted)
	if err != nil {
		return patientResponse{}, fmt.Errorf("decrypt NHI: %w", err)
	}

	fhirJSON, err := h.enc.Decrypt(rec.FHIRResource)
	if err != nil {
		return patientResponse{}, fmt.Errorf("decrypt FHIR resource: %w", err)
	}

	var patient r5.Patient
	if err := patient.UnmarshalJSON(fhirJSON); err != nil {
		return patientResponse{}, fmt.Errorf("unmarshal FHIR Patient: %w", err)
	}

	return patientResponse{
		ID:        rec.ID,
		NHI:       string(nhiPlain),
		TenantID:  rec.TenantID,
		Patient:   &patient,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}, nil
}
