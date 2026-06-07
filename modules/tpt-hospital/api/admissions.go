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
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// AdmissionStatus enumerates FHIR R5 Encounter status values for inpatient admissions.
type AdmissionStatus string

const (
	AdmissionStatusAdmitted    AdmissionStatus = "admitted"
	AdmissionStatusInHospital  AdmissionStatus = "in-hospital"
	AdmissionStatusTransferred AdmissionStatus = "transferred"
	AdmissionStatusDischarged  AdmissionStatus = "discharged"
	AdmissionStatusCancelled   AdmissionStatus = "cancelled"
)

// AdmissionType distinguishes the clinical pathway for the admission.
type AdmissionType string

const (
	AdmissionTypeElective   AdmissionType = "elective"
	AdmissionTypeEmergency  AdmissionType = "emergency"
	AdmissionTypeMaternity  AdmissionType = "maternity"
	AdmissionTypeDayStay    AdmissionType = "day-stay"
	AdmissionTypeRehab      AdmissionType = "rehabilitation"
	AdmissionTypeTransfer   AdmissionType = "transfer-in"
)

// DischargeDestination records where the patient went after leaving hospital.
type DischargeDestination string

const (
	DischargeDestinationHome          DischargeDestination = "home"
	DischargeDestinationAgedCare      DischargeDestination = "aged-care"
	DischargeDestinationRehab         DischargeDestination = "rehabilitation"
	DischargeDestinationOtherHospital DischargeDestination = "other-hospital"
	DischargeDestinationDeceased      DischargeDestination = "deceased"
)

// Admission represents an inpatient hospital stay, aligned to FHIR R5 Encounter.
type Admission struct {
	ID                   string               `json:"id"`
	PatientID            string               `json:"patientId"`
	PatientNHI           string               `json:"patientNhi"`
	AdmittingClinicianHPI string              `json:"admittingClinicianHpi"`
	ResponsibleClinicianHPI string            `json:"responsibleClinicianHpi,omitempty"`
	AdmissionType        AdmissionType        `json:"admissionType"`
	Status               AdmissionStatus      `json:"status"`
	WardID               string               `json:"wardId,omitempty"`
	BedID                string               `json:"bedId,omitempty"`
	AdmissionReason      string               `json:"admissionReason"`
	PrimaryDiagnosis     string               `json:"primaryDiagnosis,omitempty"` // ICD-10-AM
	ACCClaimNumber       string               `json:"accClaimNumber,omitempty"`
	ReferringFacilityHPI string               `json:"referringFacilityHpi,omitempty"`
	DischargeDestination DischargeDestination `json:"dischargeDestination,omitempty"`
	DischargeNotes       string               `json:"dischargeNotes,omitempty"`
	TenantID             string               `json:"tenantId"`
	AdmittedAt           time.Time            `json:"admittedAt"`
	DischargedAt         *time.Time           `json:"dischargedAt,omitempty"`
	CreatedAt            time.Time            `json:"createdAt"`
	UpdatedAt            time.Time            `json:"updatedAt"`
}

// DischargeSummary is the clinical document produced at patient discharge.
type DischargeSummary struct {
	ID                string    `json:"id"`
	AdmissionID       string    `json:"admissionId"`
	PatientID         string    `json:"patientId"`
	AuthorHPI         string    `json:"authorHpi"`
	AdmissionDate     time.Time `json:"admissionDate"`
	DischargeDate     time.Time `json:"dischargeDate"`
	PrimaryDiagnosis  string    `json:"primaryDiagnosis"`
	SecondaryDiagnoses []string `json:"secondaryDiagnoses"`
	ProceduresPerformed []string `json:"proceduresPerformed"`
	ClinicalSummary   string    `json:"clinicalSummary"`
	DischargeCondition string   `json:"dischargeCondition"` // good, fair, poor, critical
	FollowUpPlan      string    `json:"followUpPlan"`
	Medications       []string  `json:"medications"` // medication names on discharge
	GPNotified        bool      `json:"gpNotified"`
	GPNotifiedAt      *time.Time `json:"gpNotifiedAt,omitempty"`
	TenantID          string    `json:"tenantId"`
	CreatedAt         time.Time `json:"createdAt"`
}

type admissionCreateRequest struct {
	PatientID             string        `json:"patientId"`
	PatientNHI            string        `json:"patientNhi"`
	AdmittingClinicianHPI string        `json:"admittingClinicianHpi"`
	AdmissionType         AdmissionType `json:"admissionType"`
	AdmissionReason       string        `json:"admissionReason"`
	WardID                string        `json:"wardId,omitempty"`
	BedID                 string        `json:"bedId,omitempty"`
	ACCClaimNumber        string        `json:"accClaimNumber,omitempty"`
	ReferringFacilityHPI  string        `json:"referringFacilityHpi,omitempty"`
}

type admissionUpdateRequest struct {
	ResponsibleClinicianHPI string `json:"responsibleClinicianHpi,omitempty"`
	WardID                  string `json:"wardId,omitempty"`
	BedID                   string `json:"bedId,omitempty"`
	PrimaryDiagnosis        string `json:"primaryDiagnosis,omitempty"`
	AdmissionReason         string `json:"admissionReason,omitempty"`
}

type dischargeRequest struct {
	Destination  DischargeDestination `json:"destination"`
	DischargeNotes string             `json:"dischargeNotes,omitempty"`
}

type transferRequest struct {
	ToWardID string `json:"toWardId"`
	ToBedID  string `json:"toBedId"`
	Reason   string `json:"reason,omitempty"`
}

type dischargeSummaryCreateRequest struct {
	AuthorHPI           string   `json:"authorHpi"`
	PrimaryDiagnosis    string   `json:"primaryDiagnosis"`
	SecondaryDiagnoses  []string `json:"secondaryDiagnoses,omitempty"`
	ProceduresPerformed []string `json:"proceduresPerformed,omitempty"`
	ClinicalSummary     string   `json:"clinicalSummary"`
	DischargeCondition  string   `json:"dischargeCondition"`
	FollowUpPlan        string   `json:"followUpPlan"`
	Medications         []string `json:"medications,omitempty"`
}

// AdmissionsHandler handles all /api/v1/admissions routes.
type AdmissionsHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/admissions.
func (h *AdmissionsHandler) List(w http.ResponseWriter, r *http.Request) {
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
	statusFilter := q.Get("status")
	wardFilter := q.Get("ward")
	typeFilter := q.Get("type")

	admissions, err := h.listAdmissions(ctx, tenantID, statusFilter, wardFilter, typeFilter)
	if err != nil {
		h.logger.Error("list admissions", slog.Any("error", err), slog.String("tenant", tenantID))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list admissions"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionRead, ResourceType: "Admission",
		ResourceID: "list", TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, map[string]any{"admissions": admissions, "total": len(admissions)})
}

// Create handles POST /api/v1/admissions.
func (h *AdmissionsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req admissionCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "either patientId or patientNhi is required"})
		return
	}
	if req.AdmittingClinicianHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLINICIAN", Message: "admittingClinicianHpi is required"})
		return
	}

	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.AdmittingClinicianHPI)
	if err != nil {
		h.logger.Error("HPI APC validation", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate clinician APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "INVALID_APC", Message: "clinician does not hold a current Annual Practising Certificate", Details: apcStatus})
		return
	}

	adm, err := h.insertAdmission(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create admission"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "Admission",
		ResourceID: adm.ID, TenantID: tenantID,
	})
	writeJSON(w, http.StatusCreated, adm)
}

// Get handles GET /api/v1/admissions/{id}.
func (h *AdmissionsHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	adm, err := h.getAdmissionByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("get admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve admission"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionRead, ResourceType: "Admission",
		ResourceID: id, TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, adm)
}

// Update handles PUT /api/v1/admissions/{id}.
func (h *AdmissionsHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getAdmissionByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("get admission for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve admission"})
		return
	}
	if existing.Status == AdmissionStatusDischarged || existing.Status == AdmissionStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: fmt.Sprintf("cannot update admission in %s status", existing.Status)})
		return
	}

	var req admissionUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if req.ResponsibleClinicianHPI != "" {
		existing.ResponsibleClinicianHPI = req.ResponsibleClinicianHPI
	}
	if req.WardID != "" {
		existing.WardID = req.WardID
	}
	if req.BedID != "" {
		existing.BedID = req.BedID
	}
	if req.PrimaryDiagnosis != "" {
		existing.PrimaryDiagnosis = req.PrimaryDiagnosis
	}
	if req.AdmissionReason != "" {
		existing.AdmissionReason = req.AdmissionReason
	}

	updated, err := h.updateAdmission(ctx, existing)
	if err != nil {
		h.logger.Error("update admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update admission"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "Admission",
		ResourceID: id, TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, updated)
}

// Discharge handles POST /api/v1/admissions/{id}/discharge.
func (h *AdmissionsHandler) Discharge(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getAdmissionByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("get admission for discharge", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve admission"})
		return
	}
	if existing.Status == AdmissionStatusDischarged {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_DISCHARGED", Message: "admission is already discharged"})
		return
	}

	var req dischargeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Destination == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DESTINATION", Message: "discharge destination is required"})
		return
	}

	now := time.Now().UTC()
	existing.Status = AdmissionStatusDischarged
	existing.DischargedAt = &now
	existing.DischargeDestination = req.Destination
	existing.DischargeNotes = req.DischargeNotes

	discharged, err := h.dischargeAdmission(ctx, existing)
	if err != nil {
		h.logger.Error("discharge admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DISCHARGE_ERROR", Message: "failed to discharge admission"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "Admission",
		ResourceID: id, TenantID: tenantID,
		Metadata: map[string]string{"action": "discharge", "destination": string(req.Destination)},
	})
	writeJSON(w, http.StatusOK, discharged)
}

// Transfer handles POST /api/v1/admissions/{id}/transfer.
func (h *AdmissionsHandler) Transfer(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getAdmissionByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("get admission for transfer", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve admission"})
		return
	}
	if existing.Status == AdmissionStatusDischarged || existing.Status == AdmissionStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot transfer a discharged or cancelled admission"})
		return
	}

	var req transferRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ToWardID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_WARD", Message: "toWardId is required"})
		return
	}

	existing.WardID = req.ToWardID
	existing.BedID = req.ToBedID
	existing.Status = AdmissionStatusTransferred

	transferred, err := h.updateAdmission(ctx, existing)
	if err != nil {
		h.logger.Error("transfer admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "TRANSFER_ERROR", Message: "failed to transfer admission"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "Admission",
		ResourceID: id, TenantID: tenantID,
		Metadata: map[string]string{"action": "transfer", "toWard": req.ToWardID},
	})
	writeJSON(w, http.StatusOK, transferred)
}

// GetDischargeSummary handles GET /api/v1/admissions/{admissionId}/discharge-summary.
func (h *AdmissionsHandler) GetDischargeSummary(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	summary, err := h.getDischargeSummary(ctx, admissionID, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "discharge summary not found"})
			return
		}
		h.logger.Error("get discharge summary", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve discharge summary"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionRead, ResourceType: "DischargeSummary",
		ResourceID: admissionID, TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, summary)
}

// CreateDischargeSummary handles POST /api/v1/admissions/{admissionId}/discharge-summary.
func (h *AdmissionsHandler) CreateDischargeSummary(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	adm, err := h.getAdmissionByID(ctx, admissionID, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("get admission for discharge summary", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve admission"})
		return
	}
	if adm.Status != AdmissionStatusDischarged {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "NOT_DISCHARGED", Message: "discharge summary can only be created for a discharged admission"})
		return
	}

	var req dischargeSummaryCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ClinicalSummary == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_SUMMARY", Message: "clinicalSummary is required"})
		return
	}

	summary, err := h.insertDischargeSummary(ctx, admissionID, adm, req, tenantID)
	if err != nil {
		h.logger.Error("insert discharge summary", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create discharge summary"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "DischargeSummary",
		ResourceID: summary.ID, TenantID: tenantID,
	})
	writeJSON(w, http.StatusCreated, summary)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *AdmissionsHandler) listAdmissions(ctx context.Context, tenantID, statusFilter, wardFilter, typeFilter string) ([]Admission, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, admitting_clinician_hpi, responsible_clinician_hpi,
		        admission_type, status, ward_id, bed_id, admission_reason, primary_diagnosis,
		        acc_claim_number, referring_facility_hpi, discharge_destination, discharge_notes,
		        tenant_id, admitted_at, discharged_at, created_at, updated_at
		 FROM hospital_admissions
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		   AND (@ward_filter   = '' OR ward_id = @ward_filter)
		   AND (@type_filter   = '' OR admission_type = @type_filter)
		 ORDER BY admitted_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":     tenantID,
			"status_filter": statusFilter,
			"ward_filter":   wardFilter,
			"type_filter":   typeFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query admissions: %w", err)
	}
	defer rows.Close()

	var results []Admission
	for rows.Next() {
		adm, err := scanAdmissionRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, adm)
	}
	return results, rows.Err()
}

func (h *AdmissionsHandler) getAdmissionByID(ctx context.Context, id, tenantID string) (Admission, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, admitting_clinician_hpi, responsible_clinician_hpi,
		        admission_type, status, ward_id, bed_id, admission_reason, primary_diagnosis,
		        acc_claim_number, referring_facility_hpi, discharge_destination, discharge_notes,
		        tenant_id, admitted_at, discharged_at, created_at, updated_at
		 FROM hospital_admissions
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	adm, err := scanAdmissionRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Admission{}, errNotFound
		}
		return Admission{}, fmt.Errorf("get admission: %w", err)
	}
	return adm, nil
}

func (h *AdmissionsHandler) insertAdmission(ctx context.Context, req admissionCreateRequest, tenantID string) (Admission, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO hospital_admissions
		   (patient_id, patient_nhi, admitting_clinician_hpi, admission_type, status,
		    ward_id, bed_id, admission_reason, acc_claim_number, referring_facility_hpi,
		    tenant_id, admitted_at)
		 VALUES
		   (@patient_id, @patient_nhi, @admitting_clinician_hpi, @admission_type, @status,
		    @ward_id, @bed_id, @admission_reason, @acc_claim_number, @referring_facility_hpi,
		    @tenant_id, now())
		 RETURNING id, patient_id, patient_nhi, admitting_clinician_hpi, responsible_clinician_hpi,
		           admission_type, status, ward_id, bed_id, admission_reason, primary_diagnosis,
		           acc_claim_number, referring_facility_hpi, discharge_destination, discharge_notes,
		           tenant_id, admitted_at, discharged_at, created_at, updated_at`,
		db.NamedArgs{
			"patient_id":              req.PatientID,
			"patient_nhi":             req.PatientNHI,
			"admitting_clinician_hpi": req.AdmittingClinicianHPI,
			"admission_type":          req.AdmissionType,
			"status":                  AdmissionStatusAdmitted,
			"ward_id":                 req.WardID,
			"bed_id":                  req.BedID,
			"admission_reason":        req.AdmissionReason,
			"acc_claim_number":        req.ACCClaimNumber,
			"referring_facility_hpi":  req.ReferringFacilityHPI,
			"tenant_id":               tenantID,
		},
	)
	adm, err := scanAdmissionRow(row)
	if err != nil {
		return Admission{}, fmt.Errorf("insert admission: %w", err)
	}
	return adm, nil
}

func (h *AdmissionsHandler) updateAdmission(ctx context.Context, a Admission) (Admission, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE hospital_admissions
		 SET responsible_clinician_hpi = @responsible_clinician_hpi,
		     status                    = @status,
		     ward_id                   = @ward_id,
		     bed_id                    = @bed_id,
		     admission_reason          = @admission_reason,
		     primary_diagnosis         = @primary_diagnosis,
		     updated_at                = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, admitting_clinician_hpi, responsible_clinician_hpi,
		           admission_type, status, ward_id, bed_id, admission_reason, primary_diagnosis,
		           acc_claim_number, referring_facility_hpi, discharge_destination, discharge_notes,
		           tenant_id, admitted_at, discharged_at, created_at, updated_at`,
		db.NamedArgs{
			"responsible_clinician_hpi": a.ResponsibleClinicianHPI,
			"status":                    a.Status,
			"ward_id":                   a.WardID,
			"bed_id":                    a.BedID,
			"admission_reason":          a.AdmissionReason,
			"primary_diagnosis":         a.PrimaryDiagnosis,
			"id":                        a.ID,
			"tenant_id":                 a.TenantID,
		},
	)
	updated, err := scanAdmissionRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Admission{}, errNotFound
		}
		return Admission{}, fmt.Errorf("update admission: %w", err)
	}
	return updated, nil
}

func (h *AdmissionsHandler) dischargeAdmission(ctx context.Context, a Admission) (Admission, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE hospital_admissions
		 SET status                = @status,
		     discharged_at         = @discharged_at,
		     discharge_destination = @discharge_destination,
		     discharge_notes       = @discharge_notes,
		     updated_at            = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, admitting_clinician_hpi, responsible_clinician_hpi,
		           admission_type, status, ward_id, bed_id, admission_reason, primary_diagnosis,
		           acc_claim_number, referring_facility_hpi, discharge_destination, discharge_notes,
		           tenant_id, admitted_at, discharged_at, created_at, updated_at`,
		db.NamedArgs{
			"status":               a.Status,
			"discharged_at":        a.DischargedAt,
			"discharge_destination": a.DischargeDestination,
			"discharge_notes":      a.DischargeNotes,
			"id":                   a.ID,
			"tenant_id":            a.TenantID,
		},
	)
	discharged, err := scanAdmissionRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Admission{}, errNotFound
		}
		return Admission{}, fmt.Errorf("discharge admission: %w", err)
	}
	return discharged, nil
}

func (h *AdmissionsHandler) getDischargeSummary(ctx context.Context, admissionID, tenantID string) (DischargeSummary, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, admission_id, patient_id, author_hpi, admission_date, discharge_date,
		        primary_diagnosis, secondary_diagnoses, procedures_performed,
		        clinical_summary, discharge_condition, follow_up_plan, medications,
		        gp_notified, gp_notified_at, tenant_id, created_at
		 FROM hospital_discharge_summaries
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID},
	)
	var s DischargeSummary
	if err := row.Scan(
		&s.ID, &s.AdmissionID, &s.PatientID, &s.AuthorHPI,
		&s.AdmissionDate, &s.DischargeDate,
		&s.PrimaryDiagnosis, &s.SecondaryDiagnoses, &s.ProceduresPerformed,
		&s.ClinicalSummary, &s.DischargeCondition, &s.FollowUpPlan, &s.Medications,
		&s.GPNotified, &s.GPNotifiedAt, &s.TenantID, &s.CreatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return DischargeSummary{}, errNotFound
		}
		return DischargeSummary{}, fmt.Errorf("get discharge summary: %w", err)
	}
	return s, nil
}

func (h *AdmissionsHandler) insertDischargeSummary(ctx context.Context, admissionID string, adm Admission, req dischargeSummaryCreateRequest, tenantID string) (DischargeSummary, error) {
	var dischargeDate time.Time
	if adm.DischargedAt != nil {
		dischargeDate = *adm.DischargedAt
	}
	row := h.pool.QueryRow(ctx,
		`INSERT INTO hospital_discharge_summaries
		   (admission_id, patient_id, author_hpi, admission_date, discharge_date,
		    primary_diagnosis, secondary_diagnoses, procedures_performed,
		    clinical_summary, discharge_condition, follow_up_plan, medications, tenant_id)
		 VALUES
		   (@admission_id, @patient_id, @author_hpi, @admission_date, @discharge_date,
		    @primary_diagnosis, @secondary_diagnoses, @procedures_performed,
		    @clinical_summary, @discharge_condition, @follow_up_plan, @medications, @tenant_id)
		 RETURNING id, admission_id, patient_id, author_hpi, admission_date, discharge_date,
		           primary_diagnosis, secondary_diagnoses, procedures_performed,
		           clinical_summary, discharge_condition, follow_up_plan, medications,
		           gp_notified, gp_notified_at, tenant_id, created_at`,
		db.NamedArgs{
			"admission_id":         admissionID,
			"patient_id":           adm.PatientID,
			"author_hpi":           req.AuthorHPI,
			"admission_date":       adm.AdmittedAt,
			"discharge_date":       dischargeDate,
			"primary_diagnosis":    req.PrimaryDiagnosis,
			"secondary_diagnoses":  req.SecondaryDiagnoses,
			"procedures_performed": req.ProceduresPerformed,
			"clinical_summary":     req.ClinicalSummary,
			"discharge_condition":  req.DischargeCondition,
			"follow_up_plan":       req.FollowUpPlan,
			"medications":          req.Medications,
			"tenant_id":            tenantID,
		},
	)
	var s DischargeSummary
	if err := row.Scan(
		&s.ID, &s.AdmissionID, &s.PatientID, &s.AuthorHPI,
		&s.AdmissionDate, &s.DischargeDate,
		&s.PrimaryDiagnosis, &s.SecondaryDiagnoses, &s.ProceduresPerformed,
		&s.ClinicalSummary, &s.DischargeCondition, &s.FollowUpPlan, &s.Medications,
		&s.GPNotified, &s.GPNotifiedAt, &s.TenantID, &s.CreatedAt,
	); err != nil {
		return DischargeSummary{}, fmt.Errorf("insert discharge summary: %w", err)
	}
	return s, nil
}

type dbRow interface {
	Scan(dest ...any) error
}

func scanAdmissionRow(row dbRow) (Admission, error) {
	var a Admission
	if err := row.Scan(
		&a.ID, &a.PatientID, &a.PatientNHI, &a.AdmittingClinicianHPI, &a.ResponsibleClinicianHPI,
		&a.AdmissionType, &a.Status, &a.WardID, &a.BedID, &a.AdmissionReason, &a.PrimaryDiagnosis,
		&a.ACCClaimNumber, &a.ReferringFacilityHPI, &a.DischargeDestination, &a.DischargeNotes,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return Admission{}, err
	}
	return a, nil
}
