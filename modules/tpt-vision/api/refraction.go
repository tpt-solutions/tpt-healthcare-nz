package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-vision/internal/refraction"
)

// RefractionHandler handles refraction/prescription CRUD operations.
type RefractionHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListPrescriptions returns all prescriptions for a patient.
func (h *RefractionHandler) ListPrescriptions(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	if patientNhi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "Patient NHI is required"})
		return
	}

	h.logger.Info("list prescriptions", slog.String("patient_nhi", patientNhi))
	// TODO: retrieve from DB
	writeJSON(w, http.StatusOK, map[string]any{
		"patientNhi":   patientNhi,
		"prescriptions": []any{},
	})
}

// CreatePrescription creates a new refraction prescription.
func (h *RefractionHandler) CreatePrescription(w http.ResponseWriter, r *http.Request) {
	var presc refraction.Prescription
	if err := decodeJSON(r, &presc); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if err := presc.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error()})
		return
	}

	h.logger.Info("prescription created", slog.String("patient_nhi", presc.PatientNHI))
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":       "created",
		"patientNhi":   presc.PatientNHI,
		"issuedDate":   presc.IssuedDate,
	})
}

// GetPrescription returns a specific prescription.
func (h *RefractionHandler) GetPrescription(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	prescriptionId := r.PathValue("prescriptionId")

	if patientNhi == "" || prescriptionId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and prescription ID are required"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"patientNhi":    patientNhi,
		"prescriptionId": prescriptionId,
	})
}

// UpdatePrescription updates an existing prescription.
func (h *RefractionHandler) UpdatePrescription(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	prescriptionId := r.PathValue("prescriptionId")

	if patientNhi == "" || prescriptionId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and prescription ID are required"})
		return
	}

	var presc refraction.Prescription
	if err := decodeJSON(r, &presc); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	h.logger.Info("prescription updated", slog.String("patient_nhi", patientNhi), slog.String("prescription_id", prescriptionId))
	writeJSON(w, http.StatusOK, map[string]string{
		"status":         "updated",
		"patientNhi":     patientNhi,
		"prescriptionId": prescriptionId,
	})
}

// CurrentPrescription returns the patient's current (most recent) prescription.
func (h *RefractionHandler) CurrentPrescription(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	if patientNhi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "Patient NHI is required"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"patientNhi": patientNhi,
		"hasCurrent": false,
	})
}

// ConvertSnellen converts a Snellen fraction to LogMAR.
func (h *RefractionHandler) ConvertSnellen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Snellen string `json:"snellen"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if req.Snellen == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_SNELLEN", Message: "Snellen fraction is required (e.g. 6/6)"})
		return
	}

	logMAR, err := refraction.SnellenToLogMAR(req.Snellen)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_SNELLEN", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"snellen": req.Snellen,
		"logMAR":  logMAR,
	})
}

// GetPrescriptionFHIR returns a prescription as a FHIR R5 Observation resource.
func (h *RefractionHandler) GetPrescriptionFHIR(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	prescriptionId := r.PathValue("prescriptionId")

	if patientNhi == "" || prescriptionId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and prescription ID are required"})
		return
	}

	// TODO: retrieve prescription from DB
	// For now, return a placeholder
	writeJSON(w, http.StatusOK, map[string]any{
		"resourceType": "OperationOutcome",
		"issue": []map[string]any{
			{
				"severity": "information",
				"code":     "not-implemented",
				"details": map[string]any{
					"text": "FHIR endpoint - implement DB retrieval",
				},
			},
		},
	})
}
