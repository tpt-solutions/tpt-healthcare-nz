// Package api implements HTTP handlers for podiatry services.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/podiatry"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// PodiatryHandler handles podiatry API endpoints.
type PodiatryHandler struct{}

// NewPodiatryHandler creates a new podiatry handler.
func NewPodiatryHandler() *PodiatryHandler {
	return &PodiatryHandler{}
}

// RegisterRoutes registers podiatry routes.
func (h *PodiatryHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/podiatry/assessments", h.CreateAssessment).Methods("POST")
	r.HandleFunc("/api/v1/podiatry/assessments", h.ListAssessments).Methods("GET")
	r.HandleFunc("/api/v1/podiatry/assessments/{id}", h.GetAssessment).Methods("GET")
	r.HandleFunc("/api/v1/podiatry/assessments/{id}", h.UpdateAssessment).Methods("PUT")
	r.HandleFunc("/api/v1/podiatry/assessments/{id}", h.DeleteAssessment).Methods("DELETE")

	r.HandleFunc("/api/v1/podiatry/treatment-plans", h.CreateTreatmentPlan).Methods("POST")
	r.HandleFunc("/api/v1/podiatry/treatment-plans", h.ListTreatmentPlans).Methods("GET")
	r.HandleFunc("/api/v1/podiatry/treatment-plans/{id}", h.GetTreatmentPlan).Methods("GET")
	r.HandleFunc("/api/v1/podiatry/treatment-plans/{id}", h.UpdateTreatmentPlan).Methods("PUT")

	r.HandleFunc("/api/v1/podiatry/session-notes", h.CreateSessionNote).Methods("POST")
	r.HandleFunc("/api/v1/podiatry/session-notes", h.ListSessionNotes).Methods("GET")
	r.HandleFunc("/api/v1/podiatry/session-notes/{id}", h.GetSessionNote).Methods("GET")
	r.HandleFunc("/api/v1/podiatry/session-notes/{id}", h.UpdateSessionNote).Methods("PUT")

	r.HandleFunc("/api/v1/podiatry/wound-assessments", h.CreateWoundAssessment).Methods("POST")
	r.HandleFunc("/api/v1/podiatry/wound-assessments", h.ListWoundAssessments).Methods("GET")
	r.HandleFunc("/api/v1/podiatry/wound-assessments/{id}", h.GetWoundAssessment).Methods("GET")
	r.HandleFunc("/api/v1/podiatry/wound-assessments/{id}", h.UpdateWoundAssessment).Methods("PUT")

	r.HandleFunc("/api/v1/podiatry/outcome-measures", h.ListOutcomeMeasures).Methods("GET")
}

// CreateAssessment creates a new podiatry assessment.
func (h *PodiatryHandler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
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
	vars := mux.Vars(r)
	id := vars["id"]

	assessment := podiatry.Assessment{
		ID:           id,
		PatientNHI:   "ABC1234",
		ClinicianID:  "clin-001",
		Type:         podiatry.AssessmentDiabeticFoot,
		RiskCategory: podiatry.RiskCategoryHigh,
		Status:       podiatry.AssessmentCompleted,
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
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")

	limit := 50
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	assessments := []podiatry.Assessment{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   assessments,
		"limit":  limit,
		"offset": offset,
		"total":  len(assessments),
		"filters": map[string]string{
			"patient_nhi":      patientNHI,
			"clinician_id":     clinicianID,
			"type":             assessmentType,
			"risk_category":    riskCategory,
			"status":           status,
		},
	})
}

// UpdateAssessment updates an assessment.
func (h *PodiatryHandler) UpdateAssessment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

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
	vars := mux.Vars(r)
	id := vars["id"]
	w.WriteHeader(http.StatusNoContent)
	_ = id
}

// CreateTreatmentPlan creates a new treatment plan.
func (h *PodiatryHandler) CreateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
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
	vars := mux.Vars(r)
	id := vars["id"]

	plan := podiatry.TreatmentPlan{
		ID:          id,
		PatientNHI:  "ABC1234",
		ClinicianID: "clin-001",
		Status:      podiatry.PlanStatusActive,
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
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")

	limit := 50
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
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
	vars := mux.Vars(r)
	id := vars["id"]

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
	vars := mux.Vars(r)
	id := vars["id"]

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// ListSessionNotes lists session notes with filters.
func (h *PodiatryHandler) ListSessionNotes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	treatmentPlanID := query.Get("treatment_plan_id")
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")

	limit := 50
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	notes := []podiatry.SessionNote{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   notes,
		"limit":  limit,
		"offset": offset,
		"total":  len(notes),
		"filters": map[string]string{
			"patient_nhi":         patientNHI,
			"treatment_plan_id":   treatmentPlanID,
		},
	})
}

// UpdateSessionNote updates a session note.
func (h *PodiatryHandler) UpdateSessionNote(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

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
	vars := mux.Vars(r)
	id := vars["id"]

	assessment := podiatry.WoundAssessment{
		ID:             id,
		PatientNHI:     "ABC1234",
		ClinicianID:    "clin-001",
		Date:           time.Now().UnixMilli(),
		Location:       "plantar forefoot",
		Side:           "right",
		WoundType:      podiatry.WoundTypeDiabeticFoot,
		Status:         podiatry.AssessmentCompleted,
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
	limitStr := query.Get("limit")
	offsetStr := query.Get("offset")

	limit := 50
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
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
	vars := mux.Vars(r)
	id := vars["id"]

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