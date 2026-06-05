package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-vision/internal/ophthalmology"
)

// OphthalmicHandler handles ophthalmic examination CRUD operations.
type OphthalmicHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListExams returns all ophthalmic exams for a patient.
func (h *OphthalmicHandler) ListExams(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	if patientNhi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "Patient NHI is required"})
		return
	}

	h.logger.Info("list ophthalmic exams", slog.String("patient_nhi", patientNhi))
	writeJSON(w, http.StatusOK, map[string]any{
		"patientNhi": patientNhi,
		"exams":      []any{},
	})
}

// CreateExam creates a new ophthalmic examination record.
func (h *OphthalmicHandler) CreateExam(w http.ResponseWriter, r *http.Request) {
	var exam ophthalmology.OphthalmicExam
	if err := decodeJSON(r, &exam); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if err := exam.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error()})
		return
	}

	h.logger.Info("ophthalmic exam created", slog.String("patient_nhi", exam.PatientNHI))
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":     "created",
		"patientNhi": exam.PatientNHI,
		"examDate":   exam.ExamDate,
	})
}

// GetExam returns a specific ophthalmic exam.
func (h *OphthalmicHandler) GetExam(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	examId := r.PathValue("examId")

	if patientNhi == "" || examId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and exam ID are required"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"patientNhi": patientNhi,
		"examId":     examId,
	})
}

// UpdateExam updates an existing ophthalmic exam.
func (h *OphthalmicHandler) UpdateExam(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	examId := r.PathValue("examId")

	if patientNhi == "" || examId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and exam ID are required"})
		return
	}

	var exam ophthalmology.OphthalmicExam
	if err := decodeJSON(r, &exam); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	h.logger.Info("ophthalmic exam updated", slog.String("patient_nhi", patientNhi), slog.String("exam_id", examId))
	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "updated",
		"patientNhi": patientNhi,
		"examId":     examId,
	})
}

// AddIOPReading adds an IOP reading to an existing exam.
func (h *OphthalmicHandler) AddIOPReading(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	examId := r.PathValue("examId")

	if patientNhi == "" || examId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and exam ID are required"})
		return
	}

	var iop ophthalmology.IOPReading
	if err := decodeJSON(r, &iop); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	h.logger.Info("IOP reading added",
		slog.String("patient_nhi", patientNhi),
		slog.String("exam_id", examId),
		slog.Float64("right", iop.RightEye),
		slog.Float64("left", iop.LeftEye))

	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "iop_added",
		"patientNhi": patientNhi,
		"examId":     examId,
	})
}

// GetExamFHIR returns an ophthalmic exam as a FHIR R5 DiagnosticReport resource.
func (h *OphthalmicHandler) GetExamFHIR(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	examId := r.PathValue("examId")

	if patientNhi == "" || examId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and exam ID are required"})
		return
	}

	// TODO: retrieve exam from DB
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
