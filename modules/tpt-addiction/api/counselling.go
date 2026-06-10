// Package api implements addiction counselling HTTP handlers.
package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-addiction/internal/counselling"
	"log/slog"
)

// CounsellingHandler handles addiction counselling routes.
type CounsellingHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListSessions GET /api/v1/counselling/sessions
func (h *CounsellingHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []counselling.CounsellingSession{})
}

// CreateSession POST /api/v1/counselling/sessions
func (h *CounsellingHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientNHI      string `json:"patientNhi"`
		ClinicianID     string `json:"clinicianId"`
		SessionType     string `json:"sessionType"` // individual | group | family
		SessionDate     string `json:"sessionDate"`
		DurationMin     int    `json:"durationMin"`
		Modality        string `json:"modality"`
		PresentingIssue string `json:"presentingIssue"`
		ClinicalNotes   string `json:"clinicalNotes"`
		RiskAssessment  string `json:"riskAssessment,omitempty"`
		ReadinessScore  int    `json:"readinessScore"`
		HomeworkGiven   string `json:"homeworkGiven,omitempty"`
		NextSessionDate string `json:"nextSessionDate,omitempty"`
		BillingType     string `json:"billingType"`
		FeeInCents      int    `json:"feeInCents"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	cs := counselling.CounsellingSession{
		ID:              genUUID(),
		PatientNHI:      req.PatientNHI,
		ClinicianID:     req.ClinicianID,
		SessionType:     counselling.SessionType(req.SessionType),
		SessionDate:     parseTime(req.SessionDate),
		DurationMin:     req.DurationMin,
		Modality:        req.Modality,
		PresentingIssue: req.PresentingIssue,
		ClinicalNotes:   req.ClinicalNotes,
		RiskAssessment:  req.RiskAssessment,
		ReadinessScore:  req.ReadinessScore,
		HomeworkGiven:   req.HomeworkGiven,
		BillingType:     req.BillingType,
		FeeInCents:      req.FeeInCents,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if req.NextSessionDate != "" {
		t := parseTime(req.NextSessionDate)
		cs.NextSessionDate = &t
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "addiction.counselling.session", ResourceID: cs.ID, PatientNHI: req.PatientNHI, Details: map[string]any{"modality": req.Modality, "type": req.SessionType}, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, cs)
}

// GetSession GET /api/v1/counselling/sessions/{sessionId}
func (h *CounsellingHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionId")
	writeJSON(w, http.StatusOK, counselling.CounsellingSession{
		ID:      id,
		Modality: "motivational_interviewing",
	})
}

// UpdateSession PUT /api/v1/counselling/sessions/{sessionId}
func (h *CounsellingHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("sessionId")
	var req struct {
		ClinicalNotes   string `json:"clinicalNotes,omitempty"`
		RiskAssessment  string `json:"riskAssessment,omitempty"`
		ReadinessScore  int    `json:"readinessScore,omitempty"`
		NextSessionDate string `json:"nextSessionDate,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	_ = id
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "addiction.counselling.session.updated", ResourceID: id, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ListGroupSessions GET /api/v1/counselling/group-sessions
func (h *CounsellingHandler) ListGroupSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []counselling.GroupSession{})
}

// CreateGroupSession POST /api/v1/counselling/group-sessions
func (h *CounsellingHandler) CreateGroupSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string   `json:"name"`
		ClinicianID  string   `json:"clinicianId"`
		ScheduledAt  string   `json:"scheduledAt"`
		DurationMin  int      `json:"durationMin"`
		Topic        string   `json:"topic"`
		MaxAttendees int      `json:"maxAttendees"`
		Attendees    []string `json:"attendees"`
		Notes        string   `json:"notes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	gs := counselling.GroupSession{
		ID:           genUUID(),
		Name:         req.Name,
		ClinicianID:  req.ClinicianID,
		ScheduledAt:  parseTime(req.ScheduledAt),
		DurationMin:  req.DurationMin,
		Topic:        req.Topic,
		MaxAttendees: req.MaxAttendees,
		Attendees:    req.Attendees,
		Notes:        req.Notes,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "addiction.group.session.created", ResourceID: gs.ID, Details: map[string]any{"topic": req.Topic, "attendees": len(req.Attendees)}, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, gs)
}

// ListTreatmentPlans GET /api/v1/counselling/treatment-plans
func (h *CounsellingHandler) ListTreatmentPlans(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []counselling.TreatmentPlan{})
}

// CreateTreatmentPlan POST /api/v1/counselling/treatment-plans
func (h *CounsellingHandler) CreateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientNHI  string `json:"patientNhi"`
		ProgrammeID string `json:"programmeId,omitempty"`
		ClinicianID string `json:"clinicianId"`
		StartDate   string `json:"startDate"`
		ReviewDate  string `json:"reviewDate"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	tp := counselling.TreatmentPlan{
		ID:          genUUID(),
		PatientNHI:  req.PatientNHI,
		ProgrammeID: req.ProgrammeID,
		ClinicianID: req.ClinicianID,
		StartDate:   parseTime(req.StartDate),
		ReviewDate:  parseTime(req.ReviewDate),
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "addiction.treatmentplan.created", ResourceID: tp.ID, PatientNHI: req.PatientNHI, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, tp)
}

// GetTreatmentPlan GET /api/v1/counselling/treatment-plans/{planId}
func (h *CounsellingHandler) GetTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("planId")
	writeJSON(w, http.StatusOK, counselling.TreatmentPlan{ID: id, Status: "active"})
}

// UpdateTreatmentPlan PUT /api/v1/counselling/treatment-plans/{planId}
func (h *CounsellingHandler) UpdateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("planId")
	var req struct {
		ReviewDate string `json:"reviewDate,omitempty"`
		Status     string `json:"status,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	_ = id
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "addiction.treatmentplan.updated", ResourceID: id, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// AddGoal POST /api/v1/counselling/treatment-plans/{planId}/goals
func (h *CounsellingHandler) AddGoal(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("planId")
	var req struct {
		Description string `json:"description"`
		TargetDate  string `json:"targetDate,omitempty"`
		Status      string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	g := counselling.Goal{
		ID:          genUUID(),
		PlanID:      planID,
		Description: req.Description,
		Status:      req.Status,
		CreatedAt:   time.Now(),
	}
	if req.TargetDate != "" {
		t := parseTime(req.TargetDate)
		g.TargetDate = &t
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "addiction.treatmentplan.goal.added", ResourceID: g.ID, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, g)
}

// RecordRelapse POST /api/v1/counselling/treatment-plans/{planId}/relapses
func (h *CounsellingHandler) RecordRelapse(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("planId")
	var req struct {
		OccurredAt    string `json:"occurredAt"`
		SubstanceUsed string `json:"substanceUsed"`
		TriggerNotes  string `json:"triggerNotes,omitempty"`
		Severity      string `json:"severity"`
		Intervention  string `json:"intervention,omitempty"`
		PlanModified  bool   `json:"planModified"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	e := counselling.RelapseEvent{
		ID:            genUUID(),
		PlanID:        planID,
		OccurredAt:    parseTime(req.OccurredAt),
		SubstanceUsed: req.SubstanceUsed,
		TriggerNotes:  req.TriggerNotes,
		Severity:      req.Severity,
		Intervention:  req.Intervention,
		PlanModified:  req.PlanModified,
		CreatedAt:     time.Now(),
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "addiction.relapse.recorded", ResourceID: e.ID, Details: map[string]any{"severity": req.Severity, "substance": req.SubstanceUsed}, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, e)
}
