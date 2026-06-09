// Package api implements the advance care planning HTTP handlers.
package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-palliative/internal/acp"
)

type acpHandler struct {
	handlerDeps
}

// ListPlans GET /api/v1/palliative/acp-plans
func (h *acpHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []acp.Plan{})
}

// CreatePlan POST /api/v1/palliative/acp-plans
func (h *acpHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientNHI            string `json:"patientNhi"`
		TreatmentIntent       string `json:"treatmentIntent"`
		DNACPR                bool   `json:"dnacpr"`
		DNACPRSignedBy        string `json:"dnacprSignedBy,omitempty"`
		PreferredPlaceOfCare  string `json:"preferredPlaceOfCare,omitempty"`
		PreferredPlaceOfDeath string `json:"preferredPlaceOfDeath,omitempty"`
		OrganDonationWishes   *bool  `json:"organDonationWishes,omitempty"`
		SpiritualWishes       string `json:"spiritualWishes,omitempty"`
		CulturalWishes        string `json:"culturalWishes,omitempty"`
		ReviewDate            string `json:"reviewDate"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "encryption_error", Message: "failed to encrypt patient NHI"})
		return
	}
	plan := acp.Plan{
		ID:                    genUUID(),
		PatientNHI:            req.PatientNHI,
		Status:                acp.StatusDraft,
		TreatmentIntent:       acp.TreatmentIntentLevel(req.TreatmentIntent),
		DNACPR:                req.DNACPR,
		DNACPRSignedBy:        req.DNACPRSignedBy,
		PreferredPlaceOfCare:  req.PreferredPlaceOfCare,
		PreferredPlaceOfDeath: req.PreferredPlaceOfDeath,
		OrganDonationWishes:   req.OrganDonationWishes,
		SpiritualWishes:       req.SpiritualWishes,
		CulturalWishes:        req.CulturalWishes,
		ReviewDate:            parseTime(req.ReviewDate),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	if req.DNACPR {
		t := time.Now()
		plan.DNACPRDocumentedAt = &t
	}
	h.recordAudit(r, "create", "acp_plan", plan.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, plan)
}

// GetPlan GET /api/v1/palliative/acp-plans/{planId}
func (h *acpHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("planId")
	h.recordAudit(r, "read", "acp_plan", id, "")
	writeJSON(w, http.StatusOK, acp.Plan{ID: id, PatientNHI: "ABC1234", Status: acp.StatusActive, TreatmentIntent: acp.IntentPalliative})
}

// UpdatePlan PUT /api/v1/palliative/acp-plans/{planId}
func (h *acpHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("planId")
	var req struct {
		Status          string `json:"status,omitempty"`
		TreatmentIntent string `json:"treatmentIntent,omitempty"`
		DNACPR          *bool  `json:"dnacpr,omitempty"`
		ReviewDate      string `json:"reviewDate,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "acp_plan", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ListDecisions GET /api/v1/palliative/acp-plans/{planId}/decisions
func (h *acpHandler) ListDecisions(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("planId")
	h.recordAudit(r, "read", "acp_decision", planID, "")
	writeJSON(w, http.StatusOK, []acp.CareDecision{})
}

// AddDecision POST /api/v1/palliative/acp-plans/{planId}/decisions
func (h *acpHandler) AddDecision(w http.ResponseWriter, r *http.Request) {
	planID := r.PathValue("planId")
	var req struct {
		Treatment              string  `json:"treatment"`
		Decision               string  `json:"decision"`
		Reason                 string  `json:"reason,omitempty"`
		TimeLimitedUntil       *string `json:"timeLimitedUntil,omitempty"`
		ClinicalRecommendation string  `json:"clinicalRecommendation,omitempty"`
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
	h.recordAudit(r, "create", "acp_decision", d.ID, planID)
	writeJSON(w, http.StatusCreated, d)
}
