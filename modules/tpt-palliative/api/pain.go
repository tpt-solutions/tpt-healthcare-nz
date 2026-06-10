// Package api implements the pain protocol and assessment HTTP handlers.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-palliative/internal/pain"
)

// PainHandler handles pain assessment and protocol routes.
type PainHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListAssessments GET /api/v1/palliative/pain-assessments
func (h *PainHandler) ListAssessments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []pain.Assessment{})
}

// CreateAssessment POST /api/v1/palliative/pain-assessments
func (h *PainHandler) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientNHI       string  `json:"patientNhi"`
		AssessmentDate   string  `json:"assessmentDate"`
		AssessorID       string  `json:"assessorId"`
		PainScore        int     `json:"painScore"`
		PainType         string  `json:"painType"`
		Location         string  `json:"location,omitempty"`
		Quality          string  `json:"quality,omitempty"`
		Exacerbating     string  `json:"exacerbating,omitempty"`
		Relieving        string  `json:"relieving,omitempty"`
		ImpactSleep      int     `json:"impactSleep"`
		ImpactMobility   int     `json:"impactMobility"`
		ImpactMood       int     `json:"impactMood"`
		BreakthroughEpisodes int `json:"breakthroughEpisodes"`
		Notes            string  `json:"notes,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	sev := pain.SeverityMild
	if req.PainScore >= 7 {
		sev = pain.SeveritySevere
	} else if req.PainScore >= 4 {
		sev = pain.SeverityModerate
	}
	a := pain.Assessment{
		ID:                   genUUID(),
		PatientNHI:           req.PatientNHI,
		AssessmentDate:       parseTime(req.AssessmentDate),
		AssessorID:           req.AssessorID,
		PainScore:            req.PainScore,
		Severity:             sev,
		PainType:             pain.PainType(req.PainType),
		Location:             req.Location,
		Quality:              req.Quality,
		Exacerbating:         req.Exacerbating,
		Relieving:            req.Relieving,
		ImpactSleep:          req.ImpactSleep,
		ImpactMobility:       req.ImpactMobility,
		ImpactMood:           req.ImpactMood,
		BreakthroughEpisodes: req.BreakthroughEpisodes,
		Notes:                req.Notes,
		CreatedAt:            time.Now(),
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "palliative.pain.assessment.created", ResourceID: a.ID, PatientNHI: req.PatientNHI, Details: map[string]any{"pain_score": req.PainScore}, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, a)
}

// GetAssessment GET /api/v1/palliative/pain-assessments/{assessmentId}
func (h *PainHandler) GetAssessment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("assessmentId")
	writeJSON(w, http.StatusOK, pain.Assessment{ID: id, PatientNHI: "ABC1234", PainScore: 5, Severity: pain.SeverityModerate, PainType: pain.TypeNociceptive})
}

// ListProtocols GET /api/v1/palliative/pain-protocols
func (h *PainHandler) ListProtocols(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []pain.ProtocolRecord{})
}

// CreateProtocol POST /api/v1/palliative/pain-protocols
func (h *PainHandler) CreateProtocol(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientNHI           string              `json:"patientNhi"`
		Step                 string              `json:"step"`
		CurrentRegimen       []pain.Medication   `json:"currentRegimen,omitempty"`
		Adjuvants            []pain.Medication   `json:"adjuvants,omitempty"`
		BreakthroughPlan     pain.BreakthroughPlan `json:"breakthroughPlan,omitempty"`
		ReviewFrequencyDays    int                 `json:"reviewFrequencyDays"`
		PrescribedBy         string              `json:"prescribedBy"`
		Goals                []string            `json:"goals,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	p := pain.ProtocolRecord{
		ID:                  genUUID(),
		PatientNHI:          req.PatientNHI,
		Step:                pain.ProtocolStep(req.Step),
		StartDate:           time.Now(),
		CurrentRegimen:      req.CurrentRegimen,
		Adjuvants:           req.Adjuvants,
		BreakthroughPlan:    req.BreakthroughPlan,
		ReviewFrequencyDays: req.ReviewFrequencyDays,
		NextReviewDate:      time.Now().AddDate(0, 0, req.ReviewFrequencyDays),
		PrescribedBy:        req.PrescribedBy,
		Goals:               req.Goals,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "palliative.pain.protocol.created", ResourceID: p.ID, PatientNHI: req.PatientNHI, Details: map[string]any{"step": req.Step}, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, p)
}

// GetProtocol GET /api/v1/palliative/pain-protocols/{protocolId}
func (h *PainHandler) GetProtocol(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("protocolId")
	writeJSON(w, http.StatusOK, pain.ProtocolRecord{ID: id, PatientNHI: "ABC1234", Step: pain.StepThreeStrong})
}

// UpdateProtocol PUT /api/v1/palliative/pain-protocols/{protocolId}
func (h *PainHandler) UpdateProtocol(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("protocolId")
	var req struct {
		Step              string            `json:"step,omitempty"`
		CurrentRegimen    []pain.Medication `json:"currentRegimen,omitempty"`
		Adjuvants         []pain.Medication `json:"adjuvants,omitempty"`
		BreakthroughPlan  *pain.BreakthroughPlan `json:"breakthroughPlan,omitempty"`
		ReviewFrequencyDays int               `json:"reviewFrequencyDays,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	_ = id
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "palliative.pain.protocol.updated", ResourceID: id, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// RecordOutcome POST /api/v1/palliative/pain-protocols/{protocolId}/outcome
func (h *PainHandler) RecordOutcome(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("protocolId")
	var req struct {
		OutcomeScore int    `json:"outcomeScore"`
		OutcomeDate  string `json:"outcomeDate"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "palliative.pain.protocol.outcome", ResourceID: protocolID, Details: map[string]any{"outcome_score": req.OutcomeScore}, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusOK, map[string]any{"protocolId": protocolID, "outcomeScore": req.OutcomeScore, "outcomeDate": parseTime(req.OutcomeDate)})
}
