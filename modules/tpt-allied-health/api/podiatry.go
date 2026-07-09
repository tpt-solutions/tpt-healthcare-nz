// Package api implements HTTP handlers for podiatry services.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/podiatry"
	"github.com/google/uuid"
)

// PodiatryHandler handles podiatry API endpoints.
type PodiatryHandler struct {
	hpiClient    *hpi.Client
	consentStore *consent.Store
}

// NewPodiatryHandler creates a new podiatry handler.
func NewPodiatryHandler(hpiClient *hpi.Client, consentStore *consent.Store) *PodiatryHandler {
	return &PodiatryHandler{hpiClient: hpiClient, consentStore: consentStore}
}

// RegisterRoutes registers podiatry routes.
func (h *PodiatryHandler) RegisterRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.Handler) {
	mux.Handle("POST /api/v1/podiatry/assessments", protect(h.CreateAssessment))
	mux.Handle("GET /api/v1/podiatry/assessments", protect(h.ListAssessments))
	mux.Handle("GET /api/v1/podiatry/assessments/{id}", protect(h.GetAssessment))
	mux.Handle("PUT /api/v1/podiatry/assessments/{id}", protect(h.UpdateAssessment))
	mux.Handle("DELETE /api/v1/podiatry/assessments/{id}", protect(h.DeleteAssessment))

	mux.Handle("POST /api/v1/podiatry/treatment-plans", protect(h.CreateTreatmentPlan))
	mux.Handle("GET /api/v1/podiatry/treatment-plans", protect(h.ListTreatmentPlans))
	mux.Handle("GET /api/v1/podiatry/treatment-plans/{id}", protect(h.GetTreatmentPlan))
	mux.Handle("PUT /api/v1/podiatry/treatment-plans/{id}", protect(h.UpdateTreatmentPlan))

	mux.Handle("POST /api/v1/podiatry/session-notes", protect(h.CreateSessionNote))
	mux.Handle("GET /api/v1/podiatry/session-notes", protect(h.ListSessionNotes))
	mux.Handle("GET /api/v1/podiatry/session-notes/{id}", protect(h.GetSessionNote))
	mux.Handle("PUT /api/v1/podiatry/session-notes/{id}", protect(h.UpdateSessionNote))

	mux.Handle("POST /api/v1/podiatry/wound-assessments", protect(h.CreateWoundAssessment))
	mux.Handle("GET /api/v1/podiatry/wound-assessments", protect(h.ListWoundAssessments))
	mux.Handle("GET /api/v1/podiatry/wound-assessments/{id}", protect(h.GetWoundAssessment))
	mux.Handle("PUT /api/v1/podiatry/wound-assessments/{id}", protect(h.UpdateWoundAssessment))

	mux.Handle("GET /api/v1/podiatry/outcome-measures", protect(h.ListOutcomeMeasures))
}

// CreateAssessment creates a new podiatry assessment.
func (h *PodiatryHandler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var assessment podiatry.Assessment
	if err := json.NewDecoder(r.Body).Decode(&assessment); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	assessment.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	assessment.CreatedAt = now
	assessment.UpdatedAt = now

	if err := assessment.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(assessment)
}

// GetAssessment retrieves an assessment by ID.
func (h *PodiatryHandler) GetAssessment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// TODO: fetch from database; stub returns placeholder data.
	assessment := podiatry.Assessment{
		ID:           id,
		PatientNHI:   "ABC1234",
		ClinicianID:  "clin-001",
		Type:         podiatry.AssessmentDiabeticFoot,
		RiskCategory: podiatry.RiskCategoryHigh,
		Status:       podiatry.AssessmentCompleted,
	}

	if !checkConsent(w, r, h.consentStore, assessment.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment)
}

// ListAssessments lists assessments with filters.
func (h *PodiatryHandler) ListAssessments(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	assessmentType := query.Get("type")
	riskCategory := query.Get("risk_category")
	status := query.Get("status")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	assessments := []podiatry.Assessment{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   assessments,
		"limit":  limit,
		"offset": offset,
		"total":  len(assessments),
		"filters": map[string]string{
			"patient_nhi":   patientNHI,
			"clinician_id":  clinicianID,
			"type":          assessmentType,
			"risk_category": riskCategory,
			"status":        status,
		},
	})
}

// UpdateAssessment updates an assessment.
func (h *PodiatryHandler) UpdateAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	id := r.PathValue("id")

	var assessment podiatry.Assessment
	if err := json.NewDecoder(r.Body).Decode(&assessment); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	assessment.ID = id
	assessment.UpdatedAt = time.Now().UnixMilli()

	if err := assessment.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment)
}

// DeleteAssessment deletes an assessment.
func (h *PodiatryHandler) DeleteAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}
	_ = r.PathValue("id")
	w.WriteHeader(http.StatusNoContent)
}

// CreateTreatmentPlan creates a new treatment plan.
func (h *PodiatryHandler) CreateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var plan podiatry.TreatmentPlan
	if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plan.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	plan.CreatedAt = now
	plan.UpdatedAt = now

	if err := plan.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(plan)
}

// GetTreatmentPlan retrieves a treatment plan by ID.
func (h *PodiatryHandler) GetTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// TODO: fetch from database; stub returns placeholder data.
	plan := podiatry.TreatmentPlan{
		ID:          id,
		PatientNHI:  "ABC1234",
		ClinicianID: "clin-001",
		Status:      podiatry.PlanStatusActive,
	}

	if !checkConsent(w, r, h.consentStore, plan.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// ListTreatmentPlans lists treatment plans with filters.
func (h *PodiatryHandler) ListTreatmentPlans(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	status := query.Get("status")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	plans := []podiatry.TreatmentPlan{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   plans,
		"limit":  limit,
		"offset": offset,
		"total":  len(plans),
		"filters": map[string]string{
			"patient_nhi":  patientNHI,
			"clinician_id": clinicianID,
			"status":       status,
		},
	})
}

// UpdateTreatmentPlan updates a treatment plan.
func (h *PodiatryHandler) UpdateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	id := r.PathValue("id")

	var plan podiatry.TreatmentPlan
	if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plan.ID = id
	plan.UpdatedAt = time.Now().UnixMilli()

	if err := plan.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// CreateSessionNote creates a new session note.
func (h *PodiatryHandler) CreateSessionNote(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var note podiatry.SessionNote
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	note.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	note.CreatedAt = now
	note.UpdatedAt = now

	if err := note.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(note)
}

// GetSessionNote retrieves a session note by ID.
func (h *PodiatryHandler) GetSessionNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// TODO: fetch from database; stub returns placeholder data.
	note := podiatry.SessionNote{
		ID:              id,
		PatientNHI:      "ABC1234",
		ClinicianID:     "clin-001",
		SessionDate:     time.Now().UnixMilli(),
		SessionNumber:   1,
		Location:        "clinic",
		Subjective:      "Patient reports reduced pain",
		Objective:       "Wound dimensions reduced, granulating well",
		Assessment:      "Wound healing progressing as expected",
		Plan:            "Continue current dressing regimen, review in 1 week",
		DurationMinutes: 30,
	}

	if !checkConsent(w, r, h.consentStore, note.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// ListSessionNotes lists session notes with filters.
func (h *PodiatryHandler) ListSessionNotes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	treatmentPlanID := query.Get("treatment_plan_id")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	notes := []podiatry.SessionNote{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   notes,
		"limit":  limit,
		"offset": offset,
		"total":  len(notes),
		"filters": map[string]string{
			"patient_nhi":       patientNHI,
			"treatment_plan_id": treatmentPlanID,
		},
	})
}

// UpdateSessionNote updates a session note.
func (h *PodiatryHandler) UpdateSessionNote(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	id := r.PathValue("id")

	var note podiatry.SessionNote
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	note.ID = id
	note.UpdatedAt = time.Now().UnixMilli()

	if err := note.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// CreateWoundAssessment creates a new wound assessment.
func (h *PodiatryHandler) CreateWoundAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var assessment podiatry.WoundAssessment
	if err := json.NewDecoder(r.Body).Decode(&assessment); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	assessment.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	assessment.CreatedAt = now
	assessment.UpdatedAt = now

	if err := assessment.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(assessment)
}

// GetWoundAssessment retrieves a wound assessment by ID.
func (h *PodiatryHandler) GetWoundAssessment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// TODO: fetch from database; stub returns placeholder data.
	assessment := podiatry.WoundAssessment{
		ID:          id,
		PatientNHI:  "ABC1234",
		ClinicianID: "clin-001",
		Date:        time.Now().UnixMilli(),
		Location:    "plantar forefoot",
		Side:        "right",
		WoundType:   podiatry.WoundTypeDiabeticFoot,
		Status:      podiatry.AssessmentCompleted,
	}

	if !checkConsent(w, r, h.consentStore, assessment.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment)
}

// ListWoundAssessments lists wound assessments with filters.
func (h *PodiatryHandler) ListWoundAssessments(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	woundType := query.Get("wound_type")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	assessments := []podiatry.WoundAssessment{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   assessments,
		"limit":  limit,
		"offset": offset,
		"total":  len(assessments),
		"filters": map[string]string{
			"patient_nhi":  patientNHI,
			"clinician_id": clinicianID,
			"wound_type":   woundType,
		},
	})
}

// UpdateWoundAssessment updates a wound assessment.
func (h *PodiatryHandler) UpdateWoundAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	id := r.PathValue("id")

	var assessment podiatry.WoundAssessment
	if err := json.NewDecoder(r.Body).Decode(&assessment); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	assessment.ID = id
	assessment.UpdatedAt = time.Now().UnixMilli()

	if err := assessment.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment)
}

// ListOutcomeMeasures lists standardised podiatry outcome measures.
func (h *PodiatryHandler) ListOutcomeMeasures(w http.ResponseWriter, r *http.Request) {
	measures := []map[string]string{
		{"code": "VPT", "name": "Vibration Perception Threshold", "domain": "neurological", "unit": "volts"},
		{"code": "ABPI", "name": "Ankle Brachial Pressure Index", "domain": "vascular", "unit": "ratio"},
		{"code": "TBPI", "name": "Toe Brachial Pressure Index", "domain": "vascular", "unit": "ratio"},
		{"code": "WoundArea", "name": "Wound Surface Area", "domain": "wound", "unit": "cm2"},
		{"code": "WoundVolume", "name": "Wound Volume", "domain": "wound", "unit": "ml"},
		{"code": "ManchesterScale", "name": "Manchester Wound Scoring System", "domain": "wound", "unit": "score"},
		{"code": "PUSH", "name": "Pressure Ulcer Scale for Healing", "domain": "wound", "unit": "score"},
		{"code": "BWAT", "name": "Bates-Jensen Wound Assessment Tool", "domain": "wound", "unit": "score"},
		{"code": "FootFunctionIndex", "name": "Foot Function Index", "domain": "function", "unit": "score"},
		{"code": "FAAM", "name": "Foot and Ankle Ability Measure", "domain": "function", "unit": "score"},
		{"code": "LEFS", "name": "Lower Extremity Functional Scale", "domain": "function", "unit": "score"},
		{"code": "VAS", "name": "Visual Analogue Scale", "domain": "pain", "unit": "mm"},
		{"code": "NPRS", "name": "Numeric Pain Rating Scale", "domain": "pain", "unit": "score"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measures)
}
