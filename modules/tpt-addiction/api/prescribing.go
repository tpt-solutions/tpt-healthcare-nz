// Package api implements OST prescribing HTTP handlers.
package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-addiction/internal/prescribing"
	"log/slog"
)

// PrescribingHandler handles OST prescription routes.
type PrescribingHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListPrescriptions GET /api/v1/ost/prescriptions
func (h *PrescribingHandler) ListPrescriptions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []prescribing.OSTPrescription{})
}

// CreatePrescription POST /api/v1/ost/prescriptions
func (h *PrescribingHandler) CreatePrescription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientNHI   string  `json:"patientNhi"`
		ProgrammeID  string  `json:"programmeId"`
		PrescriberID string  `json:"prescriberId"`
		Drug         string  `json:"drug"`
		DoseMg       float64 `json:"doseMg"`
		Formulation  string  `json:"formulation"`
		Frequency    string  `json:"frequency"`
		Supervised   bool    `json:"supervised"`
		TakeHomeDays int     `json:"takeHomeDays"`
		StartDate    string  `json:"startDate"`
		Indication   string  `json:"indication"`
		ClinicalNotes string `json:"clinicalNotes,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	p := prescribing.OSTPrescription{
		ID:            genUUID(),
		PatientNHI:    req.PatientNHI,
		ProgrammeID:   req.ProgrammeID,
		PrescriberID:  req.PrescriberID,
		Drug:          prescribing.OSTDrug(req.Drug),
		DoseMg:        req.DoseMg,
		Formulation:   req.Formulation,
		Frequency:     req.Frequency,
		Supervised:    req.Supervised,
		TakeHomeDays:  req.TakeHomeDays,
		StartDate:     parseTime(req.StartDate),
		Status:        "active",
		Indication:    req.Indication,
		ClinicalNotes: req.ClinicalNotes,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	h.auditTrail.Record(r.Context(), "ost.prescription.created", p.ID, req.PatientNHI, map[string]any{"drug": req.Drug, "dose_mg": req.DoseMg})
	writeJSON(w, http.StatusCreated, p)
}

// GetPrescription GET /api/v1/ost/prescriptions/{prescriptionId}
func (h *PrescribingHandler) GetPrescription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("prescriptionId")
	writeJSON(w, http.StatusOK, prescribing.OSTPrescription{ID: id, Drug: prescribing.DrugMethadone, DoseMg: 50.0, Status: "active"})
}

// UpdatePrescription PUT /api/v1/ost/prescriptions/{prescriptionId}
func (h *PrescribingHandler) UpdatePrescription(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("prescriptionId")
	var req struct {
		Status       string `json:"status,omitempty"`
		EndDate      string `json:"endDate,omitempty"`
		ClinicalNotes string `json:"clinicalNotes,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	_ = id
	h.auditTrail.Record(r.Context(), "ost.prescription.updated", id, "", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// AdjustDose POST /api/v1/ost/prescriptions/{prescriptionId}/adjust
func (h *PrescribingHandler) AdjustDose(w http.ResponseWriter, r *http.Request) {
	prescriptionID := r.PathValue("prescriptionId")
	var req struct {
		PreviousDoseMg float64 `json:"previousDoseMg"`
		NewDoseMg      float64 `json:"newDoseMg"`
		Reason         string  `json:"reason"`
		ClinicalNotes  string  `json:"clinicalNotes,omitempty"`
		WitnessedBy    string  `json:"witnessedBy,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	a := prescribing.DoseAdjustment{
		ID:             genUUID(),
		PrescriptionID: prescriptionID,
		PreviousDoseMg: req.PreviousDoseMg,
		NewDoseMg:      req.NewDoseMg,
		Reason:         req.Reason,
		ClinicalNotes:  req.ClinicalNotes,
		WitnessedBy:    req.WitnessedBy,
		AdjustedAt:     time.Now(),
		CreatedAt:      time.Now(),
	}
	h.auditTrail.Record(r.Context(), "ost.dose.adjusted", a.ID, prescriptionID, map[string]any{"from": req.PreviousDoseMg, "to": req.NewDoseMg})
	writeJSON(w, http.StatusCreated, a)
}

// ListAdjustments GET /api/v1/ost/prescriptions/{prescriptionId}/adjustments
func (h *PrescribingHandler) ListAdjustments(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("prescriptionId")
	writeJSON(w, http.StatusOK, []prescribing.DoseAdjustment{})
}
