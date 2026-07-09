// Package api implements HTTP handlers for speech-language therapy services.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/speech"
	"github.com/google/uuid"
)

// SpeechHandler handles speech-language therapy API endpoints.
type SpeechHandler struct {
	hpiClient    *hpi.Client
	consentStore *consent.Store
}

// NewSpeechHandler creates a new speech handler.
func NewSpeechHandler(hpiClient *hpi.Client, consentStore *consent.Store) *SpeechHandler {
	return &SpeechHandler{hpiClient: hpiClient, consentStore: consentStore}
}

// RegisterRoutes registers speech therapy routes.
func (h *SpeechHandler) RegisterRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.Handler) {
	mux.Handle("POST /api/v1/speech/assessments", protect(h.CreateAssessment))
	mux.Handle("GET /api/v1/speech/assessments", protect(h.ListAssessments))
	mux.Handle("GET /api/v1/speech/assessments/{id}", protect(h.GetAssessment))
	mux.Handle("PUT /api/v1/speech/assessments/{id}", protect(h.UpdateAssessment))
	mux.Handle("DELETE /api/v1/speech/assessments/{id}", protect(h.DeleteAssessment))

	mux.Handle("POST /api/v1/speech/therapy-plans", protect(h.CreateTherapyPlan))
	mux.Handle("GET /api/v1/speech/therapy-plans", protect(h.ListTherapyPlans))
	mux.Handle("GET /api/v1/speech/therapy-plans/{id}", protect(h.GetTherapyPlan))
	mux.Handle("PUT /api/v1/speech/therapy-plans/{id}", protect(h.UpdateTherapyPlan))

	mux.Handle("POST /api/v1/speech/session-notes", protect(h.CreateSessionNote))
	mux.Handle("GET /api/v1/speech/session-notes", protect(h.ListSessionNotes))
	mux.Handle("GET /api/v1/speech/session-notes/{id}", protect(h.GetSessionNote))
	mux.Handle("PUT /api/v1/speech/session-notes/{id}", protect(h.UpdateSessionNote))

	mux.Handle("POST /api/v1/speech/swallowing-assessments", protect(h.CreateSwallowingAssessment))
	mux.Handle("GET /api/v1/speech/swallowing-assessments", protect(h.ListSwallowingAssessments))
	mux.Handle("GET /api/v1/speech/swallowing-assessments/{id}", protect(h.GetSwallowingAssessment))
	mux.Handle("PUT /api/v1/speech/swallowing-assessments/{id}", protect(h.UpdateSwallowingAssessment))

	mux.Handle("GET /api/v1/speech/outcome-measures", protect(h.ListOutcomeMeasures))
}

// CreateAssessment creates a new speech-language assessment.
func (h *SpeechHandler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var assessment speech.Assessment
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
func (h *SpeechHandler) GetAssessment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// TODO: fetch from database; stub returns placeholder data.
	assessment := speech.Assessment{
		ID:          id,
		PatientNHI:  "ABC1234",
		ClinicianID: "clin-001",
		Type:        speech.AssessmentLanguage,
		Status:      speech.AssessmentCompleted,
	}

	if !checkConsent(w, r, h.consentStore, assessment.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment)
}

// ListAssessments lists assessments with filters.
func (h *SpeechHandler) ListAssessments(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	assessmentType := query.Get("type")
	status := query.Get("status")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	assessments := []speech.Assessment{}

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
func (h *SpeechHandler) UpdateAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	id := r.PathValue("id")

	var assessment speech.Assessment
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
func (h *SpeechHandler) DeleteAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}
	_ = r.PathValue("id")
	w.WriteHeader(http.StatusNoContent)
}

// CreateTherapyPlan creates a new therapy plan.
func (h *SpeechHandler) CreateTherapyPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var plan speech.TherapyPlan
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

// GetTherapyPlan retrieves a therapy plan by ID.
func (h *SpeechHandler) GetTherapyPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// TODO: fetch from database; stub returns placeholder data.
	plan := speech.TherapyPlan{
		ID:          id,
		PatientNHI:  "ABC1234",
		ClinicianID: "clin-001",
		Status:      speech.PlanStatusActive,
	}

	if !checkConsent(w, r, h.consentStore, plan.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// ListTherapyPlans lists therapy plans with filters.
func (h *SpeechHandler) ListTherapyPlans(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	status := query.Get("status")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	plans := []speech.TherapyPlan{}

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

// UpdateTherapyPlan updates a therapy plan.
func (h *SpeechHandler) UpdateTherapyPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	id := r.PathValue("id")

	var plan speech.TherapyPlan
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
func (h *SpeechHandler) CreateSessionNote(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var note speech.SessionNote
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
func (h *SpeechHandler) GetSessionNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// TODO: fetch from database; stub returns placeholder data.
	note := speech.SessionNote{
		ID:              id,
		PatientNHI:      "ABC1234",
		ClinicianID:     "clin-001",
		SessionDate:     time.Now().UnixMilli(),
		SessionNumber:   1,
		Setting:         "clinic",
		Subjective:      "Parent reports improved vocabulary use at home",
		Objective:       "Produced 8/10 target words correctly",
		Assessment:      "Progressing well with articulation goals",
		Plan:            "Continue articulation therapy, increase complexity",
		DurationMinutes: 45,
	}

	if !checkConsent(w, r, h.consentStore, note.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// ListSessionNotes lists session notes with filters.
func (h *SpeechHandler) ListSessionNotes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	therapyPlanID := query.Get("therapy_plan_id")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	notes := []speech.SessionNote{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   notes,
		"limit":  limit,
		"offset": offset,
		"total":  len(notes),
		"filters": map[string]string{
			"patient_nhi":     patientNHI,
			"therapy_plan_id": therapyPlanID,
		},
	})
}

// UpdateSessionNote updates a session note.
func (h *SpeechHandler) UpdateSessionNote(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	id := r.PathValue("id")

	var note speech.SessionNote
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

// CreateSwallowingAssessment creates a new swallowing assessment.
func (h *SpeechHandler) CreateSwallowingAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var assessment speech.SwallowingAssessment
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

// GetSwallowingAssessment retrieves a swallowing assessment by ID.
func (h *SpeechHandler) GetSwallowingAssessment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// TODO: fetch from database; stub returns placeholder data.
	assessment := speech.SwallowingAssessment{
		ID:                  id,
		PatientNHI:          "ABC1234",
		ClinicianID:         "clin-001",
		Date:                time.Now().UnixMilli(),
		Status:              speech.AssessmentCompleted,
		DietRecommendations: "IDDSI Level 4 (Pureed) / Level 0 (Thin)",
	}

	if !checkConsent(w, r, h.consentStore, assessment.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment)
}

// ListSwallowingAssessments lists swallowing assessments with filters.
func (h *SpeechHandler) ListSwallowingAssessments(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	assessments := []speech.SwallowingAssessment{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   assessments,
		"limit":  limit,
		"offset": offset,
		"total":  len(assessments),
		"filters": map[string]string{
			"patient_nhi":  patientNHI,
			"clinician_id": clinicianID,
		},
	})
}

// UpdateSwallowingAssessment updates a swallowing assessment.
func (h *SpeechHandler) UpdateSwallowingAssessment(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	id := r.PathValue("id")

	var assessment speech.SwallowingAssessment
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

// ListOutcomeMeasures lists standardised speech-language outcome measures.
func (h *SpeechHandler) ListOutcomeMeasures(w http.ResponseWriter, r *http.Request) {
	measures := []map[string]string{
		{"code": "CELF-5", "name": "Clinical Evaluation of Language Fundamentals - 5th Edition", "domain": "language"},
		{"code": "PPVT-5", "name": "Peabody Picture Vocabulary Test - 5th Edition", "domain": "receptive_vocabulary"},
		{"code": "EVT-3", "name": "Expressive Vocabulary Test - 3rd Edition", "domain": "expressive_vocabulary"},
		{"code": "GFTA-3", "name": "Goldman-Fristoe Test of Articulation - 3rd Edition", "domain": "articulation"},
		{"code": "KLPA-3", "name": "Khan-Lewis Phonological Analysis - 3rd Edition", "domain": "phonology"},
		{"code": "SSI-4", "name": "Stuttering Severity Instrument - 4th Edition", "domain": "fluency"},
		{"code": "VHI-10", "name": "Voice Handicap Index - 10", "domain": "voice"},
		{"code": "EAT-10", "name": "Eating Assessment Tool - 10", "domain": "swallowing"},
		{"code": "SWAL-QOL", "name": "Swallowing Quality of Life", "domain": "swallowing"},
		{"code": "ASHA-NOMS", "name": "ASHA National Outcomes Measurement System", "domain": "functional_communication"},
		{"code": "PLS-5", "name": "Preschool Language Scales - 5th Edition", "domain": "language"},
		{"code": "TOLD-P:5", "name": "Test of Language Development - Primary: 5th Edition", "domain": "language"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measures)
}
