// Package api implements the hospice / palliative patient HTTP handlers.
package api

import (
	"log/slog"
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-palliative/internal/hospice"
)

// HospiceHandler handles hospice / palliative care routes.
type HospiceHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListPatients GET /api/v1/palliative/patients
func (h *HospiceHandler) ListPatients(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []hospice.Patient{})
}

// CreatePatient POST /api/v1/palliative/patients
func (h *HospiceHandler) CreatePatient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientNHI           string    `json:"patientNhi"`
		PrimaryDiagnosis     string    `json:"primaryDiagnosis"`
		PerformanceStatus      string    `json:"performanceStatus"`
		CareSetting            string    `json:"careSetting"`
		ResponsibleClinicianID string    `json:"responsibleClinicianId"`
		AdmissionDate          string    `json:"admissionDate"`
		PreferredPlaceOfDeath  *string   `json:"preferredPlaceOfDeath,omitempty"`
		DNACPRInPlace          bool      `json:"dnacprInPlace"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	p := hospice.Patient{
		ID:                     genUUID(),
		PatientNHI:             req.PatientNHI,
		PrimaryDiagnosis:       req.PrimaryDiagnosis,
		PerformanceStatus:        hospice.PerformanceStatus(req.PerformanceStatus),
		CareSetting:              hospice.CareSetting(req.CareSetting),
		ResponsibleClinicianID: req.ResponsibleClinicianID,
		AdmissionDate:          parseTime(req.AdmissionDate),
		PreferredPlaceOfDeath:  req.PreferredPlaceOfDeath,
		DNACPRInPlace:          req.DNACPRInPlace,
	}
	h.auditTrail.Record(r.Context(), "palliative.patient.created", p.ID, req.PatientNHI, nil)
	writeJSON(w, http.StatusCreated, p)
}

// GetPatient GET /api/v1/palliative/patients/{patientId}
func (h *HospiceHandler) GetPatient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("patientId")
	writeJSON(w, http.StatusOK, hospice.Patient{ID: id, PatientNHI: "ABC1234", PerformanceStatus: hospice.PPS60, CareSetting: hospice.SettingHome})
}

// UpdatePatient PUT /api/v1/palliative/patients/{patientId}
func (h *HospiceHandler) UpdatePatient(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("patientId")
	var req struct {
		PerformanceStatus       string    `json:"performanceStatus,omitempty"`
		CareSetting             string    `json:"careSetting,omitempty"`
		ExpectedDischargeDate   *string   `json:"expectedDischargeDate,omitempty"`
		DischargeDate           *string   `json:"dischargeDate,omitempty"`
		DischargeReason         *string   `json:"dischargeReason,omitempty"`
		DNACPRInPlace           *bool     `json:"dnacprInPlace,omitempty"`
		PreferredPlaceOfDeath   *string   `json:"preferredPlaceOfDeath,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	_ = id
	h.auditTrail.Record(r.Context(), "palliative.patient.updated", id, "", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ListVisits GET /api/v1/palliative/patients/{patientId}/visits
func (h *HospiceHandler) ListVisits(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("patientId")
	writeJSON(w, http.StatusOK, []hospice.VisitRecord{})
}

// RecordVisit POST /api/v1/palliative/patients/{patientId}/visits
func (h *HospiceHandler) RecordVisit(w http.ResponseWriter, r *http.Request) {
	patientID := r.PathValue("patientId")
	var req struct {
		VisitType     string              `json:"visitType"`
		VisitDate     string              `json:"visitDate"`
		ClinicianID   string              `json:"clinicianId"`
		Disciplines   []string            `json:"disciplines,omitempty"`
		Symptoms      []hospice.Symptom   `json:"symptoms,omitempty"`
		Notes         string              `json:"notes,omitempty"`
		NextReviewDate *string            `json:"nextReviewDate,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	v := hospice.VisitRecord{
		ID:         genUUID(),
		PatientID:   patientID,
		VisitType:   req.VisitType,
		VisitDate:   parseTime(req.VisitDate),
		ClinicianID: req.ClinicianID,
		Disciplines: req.Disciplines,
		Symptoms:    req.Symptoms,
		Notes:       req.Notes,
	}
	if req.NextReviewDate != nil {
		t := parseTime(*req.NextReviewDate)
		v.NextReviewDate = &t
	}
	h.auditTrail.Record(r.Context(), "palliative.visit.recorded", v.ID, patientID, nil)
	writeJSON(w, http.StatusCreated, v)
}

// ListGoalsOfCare GET /api/v1/palliative/patients/{patientId}/goals-of-care
func (h *HospiceHandler) ListGoalsOfCare(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("patientId")
	writeJSON(w, http.StatusOK, []hospice.GoalOfCare{})
}

// AddGoalOfCare POST /api/v1/palliative/patients/{patientId}/goals-of-care
func (h *HospiceHandler) AddGoalOfCare(w http.ResponseWriter, r *http.Request) {
	patientID := r.PathValue("patientId")
	var req struct {
		Goal     string `json:"goal"`
		Category string `json:"category"`
		Priority int    `json:"priority"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	g := hospice.GoalOfCare{
		ID:       genUUID(),
		Goal:     req.Goal,
		Category: req.Category,
		Priority: req.Priority,
		Achieved: false,
	}
	h.auditTrail.Record(r.Context(), "palliative.goal.added", g.ID, patientID, map[string]any{"goal": req.Goal})
	writeJSON(w, http.StatusCreated, g)
}
