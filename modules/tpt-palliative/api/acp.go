// Package api implements the advance care planning HTTP handlers.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-palliative/internal/acp"
)

// ACPHandler handles advance care planning routes.
type ACPHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListPlans GET /api/v1/palliative/acp-plans
func (h *ACPHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []acp.Plan{})
}

// CreatePlan POST /api/v1/palliative/acp-plans
func (h *ACPHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientNHI          string   `json:"patientNhi"`
		TreatmentIntent     string   `json:"treatmentIntent"`
		DNACPR              bool     `json:"dnacpr"`
		PreferredPlaceOfCare string  `json:"preferredPlaceOfCare,omitempty"`
		PreferredPlaceOfDeath string `json:"preferredPlaceOfDeath,omitempty"`
		OrganDonationWishes *bool    `json:"organDonationWishes,omitempty"`
		SpiritualWishes      string  `json:"spiritualWishes,omitempty"`
		CulturalWishes       string  `json:"culturalWishes,omitempty"`
		ReviewDate           string  `json:"reviewDate"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	plan := acp.Plan{
		ID:                   genUUID(),
		PatientNHI:           req.PatientNHI,
		Status:               acp.StatusDraft,
		TreatmentIntent:      acp.TreatmentIntentLevel(req.TreatmentIntent),
		DNACPR:               req.DNACPR,
		PreferredPlaceOfCare: req.PreferredPlaceOfCare,
		PreferredPlaceOfDeath: req.PreferredPlaceOfDeath,
		OrganDonationWishes:  req.OrganDonationWishes,
		SpiritualWishes:      req.SpiritualWishes,
		CulturalWishes:       req.CulturalWishes,
		ReviewDate:           parseTime(req.ReviewDate),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
	if req.DNACPR {
		t := time.Now()
		plan.DNACPRDocumentedAt = &t
	}
	h.auditTrail.Record(r.Context(), "palliative.acp.created", plan.ID, req.PatientNHI, nil)
	writeJSON(w, http.StatusCreated, plan)
}

// GetPlan GET /api/v1/palliative/acp-plans/{planId}
func (h *ACPHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("planId")
	writeJSON(w, http.StatusOK, acp.Plan{ID: id, PatientNHI: "ABC1234", Status: acp.StatusActive, TreatmentIntent: acp.IntentPalliative})
}

// UpdatePlan PUT /api/v1/palliative/acp-plans/{planId}
func (h *ACPHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("planId")
	var req struct {
		Status              string    `json:"status,omitempty"`
		TreatmentIntent     string    `json:"treatmentIntent,omitempty"`
		DNACPR              *bool     `json:"dnacpr,omitempty"`
		ReviewDate          string    `json:"reviewDate,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	_ = id
	h.auditTrail.Record(r.Context(), "palliative.acp.updated", id, "", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ListDecisions GET /api/v1/palliative/acp-plans/{planId}/decisions
func (h *ACPHandler) ListDecisions(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("planId")
	writeJSON(w, http.StatusOK, []acp.CareDecision{})
}

// AddDecision POST /api/v1/palliative/acp-plans/{planId}/decisions
func (h *ACPHandler) AddDecision(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("planId")
	var req struct {
		Treatment   string  `json:"treatment"`
		Decision    string  `json:"decision"`
		Reason      string  `json:"reason,omitempty"`
		TimeLimitedUntil *string `json:"timeLimitedUntil,omitempty"`
		ClinicalRecommendation string `json:"clinicalRecommendation,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	d := acp.CareDecision{
		ID:                     genUUID(),
		Treatment:              req.Treatment,
		Decision:               req.Decision,
		Reason:                 req.Reason,
		ClinicalRecommendation: req.ClinicalRecommendation,
		CreatedAt:              time.Now(),
	}
	if req.TimeLimitedUntil != nil {
		t := parseTime(*req.TimeLimitedUntil)
		d.TimeLimitedUntil = &t
	}
	h.auditTrail.Record(r.Context(), "palliative.acp.decision.added", d.ID, planID, map[string]any{"treatment": req.Treatment, "decision": req.Decision})
	writeJSON(w, http.StatusCreated, d)
}
