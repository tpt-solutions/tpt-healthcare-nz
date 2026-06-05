// Package api implements HTTP handlers for allied health services.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/physio"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/acc"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// PhysioHandler handles physiotherapy API endpoints.
type PhysioHandler struct{}

// NewPhysioHandler creates a new physio handler.
func NewPhysioHandler() *PhysioHandler {
	return &PhysioHandler{}
}

// RegisterRoutes registers physio routes.
func (h *PhysioHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/physio/treatment-plans", h.CreateTreatmentPlan).Methods("POST")
	r.HandleFunc("/api/v1/physio/treatment-plans", h.ListTreatmentPlans).Methods("GET")
	r.HandleFunc("/api/v1/physio/treatment-plans/{id}", h.GetTreatmentPlan).Methods("GET")
	r.HandleFunc("/api/v1/physio/treatment-plans/{id}", h.UpdateTreatmentPlan).Methods("PUT")
	r.HandleFunc("/api/v1/physio/treatment-plans/{id}", h.DeleteTreatmentPlan).Methods("DELETE")

	r.HandleFunc("/api/v1/physio/session-notes", h.CreateSessionNote).Methods("POST")
	r.HandleFunc("/api/v1/physio/session-notes", h.ListSessionNotes).Methods("GET")
	r.HandleFunc("/api/v1/physio/session-notes/{id}", h.GetSessionNote).Methods("GET")
	r.HandleFunc("/api/v1/physio/session-notes/{id}", h.UpdateSessionNote).Methods("PUT")

	r.HandleFunc("/api/v1/physio/outcome-measures", h.ListOutcomeMeasures).Methods("GET")
}

// CreateTreatmentPlan creates a new treatment plan.
func (h *PhysioHandler) CreateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	var plan physio.TreatmentPlan
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
func (h *PhysioHandler) GetTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// In real implementation, fetch from database
	plan := physio.TreatmentPlan{
		ID:         id,
		PatientNHI: "ABC1234",
		ClinicianID: "clin-001",
		Diagnosis:  "Low back pain",
		Status:     physio.PlanStatusActive,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// ListTreatmentPlans lists treatment plans with filters.
func (h *PhysioHandler) ListTreatmentPlans(w http.ResponseWriter, r *http.Request) {
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

	// In real implementation, query database with filters
	plans := []physio.TreatmentPlan{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   plans,
		"limit":  limit,
		"offset": offset,
		"total":  len(plans),
		"filters": map[string]string{
			"patient_nhi":   patientNHI,
			"clinician_id":  clinicianID,
			"status":        status,
		},
	})
}

// UpdateTreatmentPlan updates a treatment plan.
func (h *PhysioHandler) UpdateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var plan physio.TreatmentPlan
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

// DeleteTreatmentPlan deletes a treatment plan.
func (h *PhysioHandler) DeleteTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// In real implementation, soft delete from database
	w.WriteHeader(http.StatusNoContent)
	_ = id
}

// CreateSessionNote creates a new session note.
func (h *PhysioHandler) CreateSessionNote(w http.ResponseWriter, r *http.Request) {
	var note physio.SessionNote
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
func (h *PhysioHandler) GetSessionNote(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	note := physio.SessionNote{
		ID:             id,
		PatientNHI:     "ABC1234",
		ClinicianID:    "clin-001",
		SessionDate:    time.Now().UnixMilli(),
		SessionNumber:  1,
		Subjective:     "Patient reports improved pain",
		Objective:      "ROM improved",
		Assessment:     "Progressing well",
		Plan:           "Continue current program",
		DurationMinutes: 30,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// ListSessionNotes lists session notes with filters.
func (h *PhysioHandler) ListSessionNotes(w http.ResponseWriter, r *http.Request) {
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

	notes := []physio.SessionNote{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   notes,
		"limit":  limit,
		"offset": offset,
		"total":  len(notes),
		"filters": map[string]string{
			"patient_nhi":        patientNHI,
			"treatment_plan_id":  treatmentPlanID,
		},
	})
}

// UpdateSessionNote updates a session note.
func (h *PhysioHandler) UpdateSessionNote(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var note physio.SessionNote
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

// ListOutcomeMeasures lists standardised outcome measures.
func (h *PhysioHandler) ListOutcomeMeasures(w http.ResponseWriter, r *http.Request) {
	measures := []map[string]string{
		{"code": "NDI", "name": "Neck Disability Index", "domain": "cervical_spine"},
		{"code": "ODI", "name": "Oswestry Disability Index", "domain": "lumbar_spine"},
		{"code": "DASH", "name": "Disabilities of Arm, Shoulder and Hand", "domain": "upper_limb"},
		{"code": "LEFS", "name": "Lower Extremity Functional Scale", "domain": "lower_limb"},
		{"code": "FABQ", "name": "Fear-Avoidance Beliefs Questionnaire", "domain": "psychosocial"},
		{"code": "TSK", "name": "Tampa Scale of Kinesiophobia", "domain": "psychosocial"},
		{"code": "VAS", "name": "Visual Analogue Scale", "domain": "pain"},
		{"code": "NPRS", "name": "Numeric Pain Rating Scale", "domain": "pain"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measures)
}

// ACCHandler handles ACC claim endpoints for allied health.
type ACCHandler struct{}

// NewACCHandler creates a new ACC handler.
func NewACCHandler() *ACCHandler {
	return &ACCHandler{}
}

// RegisterRoutes registers ACC routes.
func (h *ACCHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/allied-health/acc/claims", h.CreateClaim).Methods("POST")
	r.HandleFunc("/api/v1/allied-health/acc/claims", h.ListClaims).Methods("GET")
	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}", h.GetClaim).Methods("GET")
	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}", h.UpdateClaim).Methods("PUT")

	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}/sessions", h.CreateSession).Methods("POST")
	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}/sessions", h.ListSessions).Methods("GET")

	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}/reviews", h.CreateReview).Methods("POST")
	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}/reviews", h.ListReviews).Methods("GET")

	r.HandleFunc("/api/v1/allied-health/acc/charge-codes", h.ListChargeCodes).Methods("GET")
	r.HandleFunc("/api/v1/allied-health/acc/charge-codes/{profession}", h.GetChargeCodesByProfession).Methods("GET")
}

// CreateClaim creates a new ACC claim.
func (h *ACCHandler) CreateClaim(w http.ResponseWriter, r *http.Request) {
	var claim acc.Claim
	if err := json.NewDecoder(r.Body).Decode(&claim); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	claim.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	claim.CreatedAt = now
	claim.UpdatedAt = now

	if err := claim.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(claim)
}

// GetClaim retrieves an ACC claim by ID.
func (h *ACCHandler) GetClaim(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	claim := acc.Claim{
		ID:             id,
		PatientNHI:     "ABC1234",
		ClinicianID:    "clin-001",
		ClaimType:      acc.ClaimTypePhysiotherapy,
		ACCNumber:      "ACC123456",
		Status:         acc.ClaimStatusAccepted,
		Diagnosis:      "Lumbar strain",
		BodyRegion:     "lumbar_spine",
		ApprovedSessions: 10,
		UsedSessions:   3,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(claim)
}

// ListClaims lists ACC claims with filters.
func (h *ACCHandler) ListClaims(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	claimType := query.Get("claim_type")
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

	claims := []acc.Claim{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   claims,
		"limit":  limit,
		"offset": offset,
		"total":  len(claims),
		"filters": map[string]string{
			"patient_nhi":  patientNHI,
			"clinician_id": clinicianID,
			"claim_type":   claimType,
			"status":       status,
		},
	})
}

// UpdateClaim updates an ACC claim.
func (h *ACCHandler) UpdateClaim(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var claim acc.Claim
	if err := json.NewDecoder(r.Body).Decode(&claim); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	claim.ID = id
	claim.UpdatedAt = time.Now().UnixMilli()

	if err := claim.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(claim)
}

// CreateSession creates a new treatment session under a claim.
func (h *ACCHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	claimID := vars["id"]

	var session acc.TreatmentSession
	if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session.ID = uuid.New().String()
	session.ClaimID = claimID
	now := time.Now().UnixMilli()
	session.CreatedAt = now
	session.UpdatedAt = now

	if err := session.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Auto-populate charge amount from charge code
	if chargeCode := acc.GetChargeCodeByCode(session.ChargeCode); chargeCode != nil {
		session.ChargeAmount = chargeCode.Rate
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session)
}

// ListSessions lists treatment sessions for a claim.
func (h *ACCHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	claimID := vars["id"]

	query := r.URL.Query()
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

	sessions := []acc.TreatmentSession{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":       sessions,
		"limit":      limit,
		"offset":     offset,
		"total":      len(sessions),
		"claim_id":   claimID,
	})
}

// CreateReview creates a new review report.
func (h *ACCHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	claimID := vars["id"]

	var review acc.ReviewReport
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	review.ID = uuid.New().String()
	review.ClaimID = claimID
	now := time.Now().UnixMilli()
	review.CreatedAt = now
	review.UpdatedAt = now

	if err := review.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(review)
}

// ListReviews lists review reports for a claim.
func (h *ACCHandler) ListReviews(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	claimID := vars["id"]

	reviews := []acc.ReviewReport{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":     reviews,
		"claim_id": claimID,
	})
}

// ListChargeCodes lists all ACC charge codes.
func (h *ACCHandler) ListChargeCodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(acc.StandardChargeCodes)
}

// GetChargeCodesByProfession returns charge codes for a profession.
func (h *ACCHandler) GetChargeCodesByProfession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	profession := vars["profession"]

	codes := acc.GetChargeCodesByProfession(profession)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(codes)
}