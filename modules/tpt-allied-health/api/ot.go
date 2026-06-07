// Package api implements HTTP handlers for occupational therapy services.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/ot"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// OTHandler handles occupational therapy API endpoints.
type OTHandler struct {
	hpiClient    *hpi.Client
	consentStore *consent.Store
}

// NewOTHandler creates a new OT handler.
func NewOTHandler(hpiClient *hpi.Client, consentStore *consent.Store) *OTHandler {
	return &OTHandler{hpiClient: hpiClient, consentStore: consentStore}
}

// RegisterRoutes registers OT routes.
func (h *OTHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/ot/assessments", h.CreateAssessment).Methods("POST")
	r.HandleFunc("/api/v1/ot/assessments", h.ListAssessments).Methods("GET")
	r.HandleFunc("/api/v1/ot/assessments/{id}", h.GetAssessment).Methods("GET")
	r.HandleFunc("/api/v1/ot/assessments/{id}", h.UpdateAssessment).Methods("PUT")
	r.HandleFunc("/api/v1/ot/assessments/{id}", h.DeleteAssessment).Methods("DELETE")

	r.HandleFunc("/api/v1/ot/intervention-plans", h.CreateInterventionPlan).Methods("POST")
	r.HandleFunc("/api/v1/ot/intervention-plans", h.ListInterventionPlans).Methods("GET")
	r.HandleFunc("/api/v1/ot/intervention-plans/{id}", h.GetInterventionPlan).Methods("GET")
	r.HandleFunc("/api/v1/ot/intervention-plans/{id}", h.UpdateInterventionPlan).Methods("PUT")

	r.HandleFunc("/api/v1/ot/session-notes", h.CreateSessionNote).Methods("POST")
	r.HandleFunc("/api/v1/ot/session-notes", h.ListSessionNotes).Methods("GET")
	r.HandleFunc("/api/v1/ot/session-notes/{id}", h.GetSessionNote).Methods("GET")
	r.HandleFunc("/api/v1/ot/session-notes/{id}", h.UpdateSessionNote).Methods("PUT")

	r.HandleFunc("/api/v1/ot/outcome-measures", h.ListOutcomeMeasures).Methods("GET")
}

// CreateAssessment creates a new OT assessment.
func (h *OTHandler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var assessment ot.Assessment
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
func (h *OTHandler) GetAssessment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: fetch from database; stub returns placeholder data.
	assessment := ot.Assessment{
		ID:          id,
		PatientNHI:  "ABC1234",
		ClinicianID: "clin-001",
		Type:        ot.AssessmentADL,
		Status:      ot.AssessmentCompleted,
	}

	if !checkConsent(w, r, h.consentStore, assessment.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment)
}

// ListAssessments lists assessments with filters.
func (h *OTHandler) ListAssessments(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	assessmentType := query.Get("type")
	status := query.Get("status")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	assessments := []ot.Assessment{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   assessments,
		"limit":  limit,
		"offset": offset,
		"total":  len(assessments),
		"filters": map[string]string{
			"patient_nhi":  patientNHI,
			"clinician_id": clinicianID,
			"type":         assessmentType,
			"status":       status,
		},
	})
}

// UpdateAssessment updates an assessment.
func (h *OTHandler) UpdateAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var assessment ot.Assessment
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
func (h *OTHandler) DeleteAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}
	_ = mux.Vars(r)["id"]
	w.WriteHeader(http.StatusNoContent)
}

// CreateInterventionPlan creates a new intervention plan.
func (h *OTHandler) CreateInterventionPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var plan ot.InterventionPlan
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

// GetInterventionPlan retrieves an intervention plan by ID.
func (h *OTHandler) GetInterventionPlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: fetch from database; stub returns placeholder data.
	plan := ot.InterventionPlan{
		ID:          id,
		PatientNHI:  "ABC1234",
		ClinicianID: "clin-001",
		Status:      ot.PlanStatusActive,
	}

	if !checkConsent(w, r, h.consentStore, plan.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// ListInterventionPlans lists intervention plans with filters.
func (h *OTHandler) ListInterventionPlans(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	status := query.Get("status")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	plans := []ot.InterventionPlan{}

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

// UpdateInterventionPlan updates an intervention plan.
func (h *OTHandler) UpdateInterventionPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var plan ot.InterventionPlan
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
func (h *OTHandler) CreateSessionNote(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var note ot.SessionNote
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
func (h *OTHandler) GetSessionNote(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: fetch from database; stub returns placeholder data.
	note := ot.SessionNote{
		ID:              id,
		PatientNHI:      "ABC1234",
		ClinicianID:     "clin-001",
		SessionDate:     time.Now().UnixMilli(),
		SessionNumber:   1,
		Location:        "clinic",
		Subjective:      "Patient reports improved independence",
		Objective:       "Able to complete dressing with minimal assistance",
		Assessment:      "Progressing towards independence in ADLs",
		Plan:            "Continue ADL retraining, introduce kitchen tasks",
		DurationMinutes: 45,
	}

	if !checkConsent(w, r, h.consentStore, note.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// ListSessionNotes lists session notes with filters.
func (h *OTHandler) ListSessionNotes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	interventionPlanID := query.Get("intervention_plan_id")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	notes := []ot.SessionNote{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   notes,
		"limit":  limit,
		"offset": offset,
		"total":  len(notes),
		"filters": map[string]string{
			"patient_nhi":          patientNHI,
			"intervention_plan_id": interventionPlanID,
		},
	})
}

// UpdateSessionNote updates a session note.
func (h *OTHandler) UpdateSessionNote(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var note ot.SessionNote
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

// ListOutcomeMeasures lists standardised OT outcome measures.
func (h *OTHandler) ListOutcomeMeasures(w http.ResponseWriter, r *http.Request) {
	measures := []map[string]string{
		{"code": "COPM", "name": "Canadian Occupational Performance Measure", "domain": "performance_satisfaction"},
		{"code": "FIM", "name": "Functional Independence Measure", "domain": "function"},
		{"code": "AMPS", "name": "Assessment of Motor and Process Skills", "domain": "motor_process"},
		{"code": "MOHOST", "name": "Model of Human Occupation Screening Tool", "domain": "occupation"},
		{"code": "BARTHEL", "name": "Barthel Index", "domain": "adl"},
		{"code": "LAWTON", "name": "Lawton IADL Scale", "domain": "iadl"},
		{"code": "MMSE", "name": "Mini-Mental State Examination", "domain": "cognitive"},
		{"code": "MOCA", "name": "Montreal Cognitive Assessment", "domain": "cognitive"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measures)
}
