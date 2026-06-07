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

// PAStatus tracks a pre-admission assessment through its lifecycle.
type PAStatus string

const (
	PAStatusScheduled  PAStatus = "scheduled"
	PAStatusInProgress PAStatus = "in-progress"
	PAStatusCompleted  PAStatus = "completed"
	PAStatusApproved   PAStatus = "approved"    // cleared for surgery
	PAStatusDeferred   PAStatus = "deferred"    // needs further investigation before clearance
	PAStatusCancelled  PAStatus = "cancelled"
)

// ASAGrade is the American Society of Anesthesiologists physical status classification.
type ASAGrade string

const (
	ASA1 ASAGrade = "I"    // Normal healthy patient
	ASA2 ASAGrade = "II"   // Patient with mild systemic disease
	ASA3 ASAGrade = "III"  // Severe systemic disease
	ASA4 ASAGrade = "IV"   // Severe disease that is a constant threat to life
	ASA5 ASAGrade = "V"    // Moribund patient not expected to survive without operation
	ASA6 ASAGrade = "VI"   // Brain-dead patient (organ donation)
)

// PreAdmissionAssessment documents the pre-operative PAC clinic review.
type PreAdmissionAssessment struct {
	ID                   string    `json:"id"`
	PatientID            string    `json:"patientId"`
	PatientNHI           string    `json:"patientNhi"`
	TheatreBookingID     string    `json:"theatreBookingId,omitempty"`
	AssessorHPI          string    `json:"assessorHpi"` // anaesthetist or PAC nurse
	Status               PAStatus  `json:"status"`
	PlannedProcedure     string    `json:"plannedProcedure"`
	PlannedAnaesthesia   AnaesthesiaType `json:"plannedAnaesthesia"`
	ASAGrade             ASAGrade  `json:"asaGrade,omitempty"`
	AllergiesReviewed    bool      `json:"allergiesReviewed"`
	MedicationsReviewed  bool      `json:"medicationsReviewed"`
	BloodGroupConfirmed  bool      `json:"bloodGroupConfirmed"`
	ConsentObtained      bool      `json:"consentObtained"`
	FastingInstructions  string    `json:"fastingInstructions,omitempty"`
	SpecialInstructions  string    `json:"specialInstructions,omitempty"`
	ClinicalNotes        string    `json:"clinicalNotes,omitempty"`
	DeferralReason       string    `json:"deferralReason,omitempty"`
	InvestigationsRequired []string `json:"investigationsRequired,omitempty"` // e.g. ECG, bloods
	TenantID             string    `json:"tenantId"`
	AssessedAt           *time.Time `json:"assessedAt,omitempty"`
	ApprovedAt           *time.Time `json:"approvedAt,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type paCreateRequest struct {
	PatientID          string          `json:"patientId"`
	PatientNHI         string          `json:"patientNhi"`
	TheatreBookingID   string          `json:"theatreBookingId,omitempty"`
	AssessorHPI        string          `json:"assessorHpi"`
	PlannedProcedure   string          `json:"plannedProcedure"`
	PlannedAnaesthesia AnaesthesiaType `json:"plannedAnaesthesia"`
}

type paUpdateRequest struct {
	AssessorHPI             string          `json:"assessorHpi,omitempty"`
	ASAGrade                ASAGrade        `json:"asaGrade,omitempty"`
	AllergiesReviewed       *bool           `json:"allergiesReviewed,omitempty"`
	MedicationsReviewed     *bool           `json:"medicationsReviewed,omitempty"`
	BloodGroupConfirmed     *bool           `json:"bloodGroupConfirmed,omitempty"`
	ConsentObtained         *bool           `json:"consentObtained,omitempty"`
	FastingInstructions     string          `json:"fastingInstructions,omitempty"`
	SpecialInstructions     string          `json:"specialInstructions,omitempty"`
	ClinicalNotes           string          `json:"clinicalNotes,omitempty"`
	InvestigationsRequired  []string        `json:"investigationsRequired,omitempty"`
}

type paApproveRequest struct {
	Notes string `json:"notes,omitempty"`
}

type paDeferRequest struct {
	DeferralReason         string   `json:"deferralReason"`
	InvestigationsRequired []string `json:"investigationsRequired,omitempty"`
}

// PreAdmissionHandler handles all /api/v1/preadmission routes.
type PreAdmissionHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/preadmission/assessments.
func (h *PreAdmissionHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	statusFilter := r.URL.Query().Get("status")
	assessments, err := h.listAssessments(ctx, tenantID, statusFilter)
	if err != nil {
		h.logger.Error("list PA assessments", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list assessments"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assessments": assessments, "total": len(assessments)})
}

// Create handles POST /api/v1/preadmission/assessments.
func (h *PreAdmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req paCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "either patientId or patientNhi is required"})
		return
	}
	if req.AssessorHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ASSESSOR", Message: "assessorHpi is required"})
		return
	}
	if req.PlannedProcedure == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PROCEDURE", Message: "plannedProcedure is required"})
		return
	}

	a, err := h.insertAssessment(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert PA assessment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create assessment"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "PreAdmissionAssessment",
		ResourceID: a.ID, TenantID: tenantID,
	})
	writeJSON(w, http.StatusCreated, a)
}

// Get handles GET /api/v1/preadmission/assessments/{id}.
func (h *PreAdmissionHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	a, err := h.getAssessmentByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "assessment not found"})
			return
		}
		h.logger.Error("get PA assessment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve assessment"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionRead, ResourceType: "PreAdmissionAssessment",
		ResourceID: id, TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, a)
}

// Update handles PUT /api/v1/preadmission/assessments/{id}.
func (h *PreAdmissionHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getAssessmentByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "assessment not found"})
			return
		}
		h.logger.Error("get PA assessment for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve assessment"})
		return
	}
	if existing.Status == PAStatusApproved || existing.Status == PAStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot update an approved or cancelled assessment"})
		return
	}

	var req paUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if req.AssessorHPI != "" {
		existing.AssessorHPI = req.AssessorHPI
	}
	if req.ASAGrade != "" {
		existing.ASAGrade = req.ASAGrade
	}
	if req.AllergiesReviewed != nil {
		existing.AllergiesReviewed = *req.AllergiesReviewed
	}
	if req.MedicationsReviewed != nil {
		existing.MedicationsReviewed = *req.MedicationsReviewed
	}
	if req.BloodGroupConfirmed != nil {
		existing.BloodGroupConfirmed = *req.BloodGroupConfirmed
	}
	if req.ConsentObtained != nil {
		existing.ConsentObtained = *req.ConsentObtained
	}
	if req.FastingInstructions != "" {
		existing.FastingInstructions = req.FastingInstructions
	}
	if req.SpecialInstructions != "" {
		existing.SpecialInstructions = req.SpecialInstructions
	}
	if req.ClinicalNotes != "" {
		existing.ClinicalNotes = req.ClinicalNotes
	}
	if len(req.InvestigationsRequired) > 0 {
		existing.InvestigationsRequired = req.InvestigationsRequired
	}
	existing.Status = PAStatusInProgress

	updated, err := h.updateAssessment(ctx, existing)
	if err != nil {
		h.logger.Error("update PA assessment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update assessment"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "PreAdmissionAssessment",
		ResourceID: id, TenantID: tenantID,
	})
	writeJSON(w, http.StatusOK, updated)
}

// Complete handles POST /api/v1/preadmission/assessments/{id}/complete — marks assessment complete, pending approval.
func (h *PreAdmissionHandler) Complete(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getAssessmentByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "assessment not found"})
			return
		}
		h.logger.Error("get PA assessment for complete", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve assessment"})
		return
	}

	now := time.Now().UTC()
	existing.Status = PAStatusCompleted
	existing.AssessedAt = &now

	completed, err := h.updateAssessment(ctx, existing)
	if err != nil {
		h.logger.Error("complete PA assessment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "COMPLETE_ERROR", Message: "failed to complete assessment"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "PreAdmissionAssessment",
		ResourceID: id, TenantID: tenantID, Metadata: map[string]string{"action": "complete"},
	})
	writeJSON(w, http.StatusOK, completed)
}

// Approve handles POST /api/v1/preadmission/assessments/{id}/approve — clears patient for surgery.
func (h *PreAdmissionHandler) Approve(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getAssessmentByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "assessment not found"})
			return
		}
		h.logger.Error("get PA assessment for approve", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve assessment"})
		return
	}
	if existing.Status != PAStatusCompleted {
		writeJSON(w, http.StatusConflict, apiError{Code: "NOT_COMPLETED", Message: "assessment must be completed before it can be approved"})
		return
	}

	now := time.Now().UTC()
	existing.Status = PAStatusApproved
	existing.ApprovedAt = &now

	approved, err := h.updateAssessment(ctx, existing)
	if err != nil {
		h.logger.Error("approve PA assessment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "APPROVE_ERROR", Message: "failed to approve assessment"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor: principal, Action: audit.ActionWrite, ResourceType: "PreAdmissionAssessment",
		ResourceID: id, TenantID: tenantID, Metadata: map[string]string{"action": "approve"},
	})
	writeJSON(w, http.StatusOK, approved)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *PreAdmissionHandler) listAssessments(ctx context.Context, tenantID, statusFilter string) ([]PreAdmissionAssessment, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, theatre_booking_id, assessor_hpi, status,
		        planned_procedure, planned_anaesthesia, asa_grade,
		        allergies_reviewed, medications_reviewed, blood_group_confirmed, consent_obtained,
		        fasting_instructions, special_instructions, clinical_notes, deferral_reason,
		        investigations_required, tenant_id, assessed_at, approved_at, created_at, updated_at
		 FROM preadmission_assessments
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY created_at DESC`,
		db.NamedArgs{"tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query PA assessments: %w", err)
	}
	defer rows.Close()

	var results []PreAdmissionAssessment
	for rows.Next() {
		a, err := scanPARow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

func (h *PreAdmissionHandler) getAssessmentByID(ctx context.Context, id, tenantID string) (PreAdmissionAssessment, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, theatre_booking_id, assessor_hpi, status,
		        planned_procedure, planned_anaesthesia, asa_grade,
		        allergies_reviewed, medications_reviewed, blood_group_confirmed, consent_obtained,
		        fasting_instructions, special_instructions, clinical_notes, deferral_reason,
		        investigations_required, tenant_id, assessed_at, approved_at, created_at, updated_at
		 FROM preadmission_assessments
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	a, err := scanPARow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return PreAdmissionAssessment{}, errNotFound
		}
		return PreAdmissionAssessment{}, fmt.Errorf("get PA assessment: %w", err)
	}
	return a, nil
}

func (h *PreAdmissionHandler) insertAssessment(ctx context.Context, req paCreateRequest, tenantID string) (PreAdmissionAssessment, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO preadmission_assessments
		   (patient_id, patient_nhi, theatre_booking_id, assessor_hpi, status,
		    planned_procedure, planned_anaesthesia, tenant_id)
		 VALUES
		   (@patient_id, @patient_nhi, @theatre_booking_id, @assessor_hpi, @status,
		    @planned_procedure, @planned_anaesthesia, @tenant_id)
		 RETURNING id, patient_id, patient_nhi, theatre_booking_id, assessor_hpi, status,
		           planned_procedure, planned_anaesthesia, asa_grade,
		           allergies_reviewed, medications_reviewed, blood_group_confirmed, consent_obtained,
		           fasting_instructions, special_instructions, clinical_notes, deferral_reason,
		           investigations_required, tenant_id, assessed_at, approved_at, created_at, updated_at`,
		db.NamedArgs{
			"patient_id":          req.PatientID,
			"patient_nhi":         req.PatientNHI,
			"theatre_booking_id":  req.TheatreBookingID,
			"assessor_hpi":        req.AssessorHPI,
			"status":              PAStatusScheduled,
			"planned_procedure":   req.PlannedProcedure,
			"planned_anaesthesia": req.PlannedAnaesthesia,
			"tenant_id":           tenantID,
		},
	)
	return scanPARow(row)
}

func (h *PreAdmissionHandler) updateAssessment(ctx context.Context, a PreAdmissionAssessment) (PreAdmissionAssessment, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE preadmission_assessments
		 SET assessor_hpi              = @assessor_hpi,
		     status                    = @status,
		     asa_grade                 = @asa_grade,
		     allergies_reviewed        = @allergies_reviewed,
		     medications_reviewed      = @medications_reviewed,
		     blood_group_confirmed     = @blood_group_confirmed,
		     consent_obtained          = @consent_obtained,
		     fasting_instructions      = @fasting_instructions,
		     special_instructions      = @special_instructions,
		     clinical_notes            = @clinical_notes,
		     deferral_reason           = @deferral_reason,
		     investigations_required   = @investigations_required,
		     assessed_at               = @assessed_at,
		     approved_at               = @approved_at,
		     updated_at                = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, theatre_booking_id, assessor_hpi, status,
		           planned_procedure, planned_anaesthesia, asa_grade,
		           allergies_reviewed, medications_reviewed, blood_group_confirmed, consent_obtained,
		           fasting_instructions, special_instructions, clinical_notes, deferral_reason,
		           investigations_required, tenant_id, assessed_at, approved_at, created_at, updated_at`,
		db.NamedArgs{
			"assessor_hpi":            a.AssessorHPI,
			"status":                  a.Status,
			"asa_grade":               a.ASAGrade,
			"allergies_reviewed":      a.AllergiesReviewed,
			"medications_reviewed":    a.MedicationsReviewed,
			"blood_group_confirmed":   a.BloodGroupConfirmed,
			"consent_obtained":        a.ConsentObtained,
			"fasting_instructions":    a.FastingInstructions,
			"special_instructions":    a.SpecialInstructions,
			"clinical_notes":          a.ClinicalNotes,
			"deferral_reason":         a.DeferralReason,
			"investigations_required": a.InvestigationsRequired,
			"assessed_at":             a.AssessedAt,
			"approved_at":             a.ApprovedAt,
			"id":                      a.ID,
			"tenant_id":               a.TenantID,
		},
	)
	updated, err := scanPARow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return PreAdmissionAssessment{}, errNotFound
		}
		return PreAdmissionAssessment{}, fmt.Errorf("update PA assessment: %w", err)
	}
	return updated, nil
}

func scanPARow(row dbRow) (PreAdmissionAssessment, error) {
	var a PreAdmissionAssessment
	if err := row.Scan(
		&a.ID, &a.PatientID, &a.PatientNHI, &a.TheatreBookingID, &a.AssessorHPI, &a.Status,
		&a.PlannedProcedure, &a.PlannedAnaesthesia, &a.ASAGrade,
		&a.AllergiesReviewed, &a.MedicationsReviewed, &a.BloodGroupConfirmed, &a.ConsentObtained,
		&a.FastingInstructions, &a.SpecialInstructions, &a.ClinicalNotes, &a.DeferralReason,
		&a.InvestigationsRequired, &a.TenantID, &a.AssessedAt, &a.ApprovedAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return PreAdmissionAssessment{}, err
	}
	return a, nil
}
