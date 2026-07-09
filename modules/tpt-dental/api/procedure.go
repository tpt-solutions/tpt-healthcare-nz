package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-dental/internal/procedure"
)

// ProcedureHandler handles dental procedure code lookup and treatment record CRUD.
type ProcedureHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// TreatmentRecord represents a dental procedure performed during a patient visit.
type TreatmentRecord struct {
	ID          string `json:"id"`
	PatientNHI  string `json:"patientNhi"`
	VisitID     string `json:"visitId"`
	ToothCode   string `json:"toothCode"`
	Surface     string `json:"surface,omitempty"`
	DCNZCode    string `json:"dcnzCode"`
	Description string `json:"description"`
	Fee         int    `json:"fee"` // NZ cents
	IsACC       bool   `json:"isAcc"`
	ACCClaimID  string `json:"accClaimId,omitempty"`
	ClinicianID string `json:"clinicianId"`
	PracticeID  string `json:"practiceId"`
	PerformedAt int64  `json:"performedAt"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// ListProcedures returns all DCNZ procedure codes, optionally filtered by category.
func (h *ProcedureHandler) ListProcedures(w http.ResponseWriter, r *http.Request) {
	codes := procedure.DCNZCodes()
	writeJSON(w, http.StatusOK, codes)
}

// GetDCNZCode returns details for a specific DCNZ procedure code.
func (h *ProcedureHandler) GetDCNZCode(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_CODE", Message: "DCNZ procedure code is required",
		})
		return
	}

	proc, ok := procedure.LookupDCNZ(code)
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{
			Code: "NOT_FOUND", Message: fmt.Sprintf("DCNZ code %q not found", code),
		})
		return
	}

	writeJSON(w, http.StatusOK, proc)
}

// GetACCCode returns details for a specific ACC dental treatment code.
func (h *ProcedureHandler) GetACCCode(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_CODE", Message: "ACC dental code is required",
		})
		return
	}

	accCode, ok := procedure.LookupACCDental(code)
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{
			Code: "NOT_FOUND", Message: fmt.Sprintf("ACC dental code %q not found", code),
		})
		return
	}

	writeJSON(w, http.StatusOK, accCode)
}

// ByCategory returns DCNZ procedure codes filtered by category.
func (h *ProcedureHandler) ByCategory(w http.ResponseWriter, r *http.Request) {
	cat := r.PathValue("category")
	if cat == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_CATEGORY", Message: "Category is required",
		})
		return
	}

	codes := procedure.ProceduresByCategory(procedure.ProcedureCategory(strings.ToLower(cat)))
	if codes == nil {
		codes = []procedure.ProcedureCode{} // return empty array, not null
	}

	writeJSON(w, http.StatusOK, codes)
}

// ListRecords returns treatment records for a patient.
func (h *ProcedureHandler) ListRecords(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	if patientNhi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_NHI", Message: "Patient NHI is required",
		})
		return
	}

	// Simplified stub — real implementation queries DB.
	records := []TreatmentRecord{}
	writeJSON(w, http.StatusOK, records)
}

// CreateRecord creates a new treatment record.
func (h *ProcedureHandler) CreateRecord(w http.ResponseWriter, r *http.Request) {
	var rec TreatmentRecord
	if err := decodeJSON(r, &rec); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	if rec.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_NHI", Message: "Patient NHI is required",
		})
		return
	}
	if rec.DCNZCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_CODE", Message: "DCNZ procedure code is required",
		})
		return
	}

	now := time.Now().UnixMilli()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	if rec.PerformedAt == 0 {
		rec.PerformedAt = now
	}

	h.logger.Info("treatment record created",
		slog.String("patient_nhi", rec.PatientNHI),
		slog.String("dcnz_code", rec.DCNZCode),
		slog.String("tooth", rec.ToothCode))

	writeJSON(w, http.StatusCreated, rec)
}

// GetRecord returns a specific treatment record.
func (h *ProcedureHandler) GetRecord(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	recordID := r.PathValue("recordId")

	if patientNhi == "" || recordID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_PARAMS", Message: "Patient NHI and record ID are required",
		})
		return
	}

	// Simplified stub — real implementation queries DB.
	writeJSON(w, http.StatusNotFound, apiError{
		Code: "NOT_FOUND", Message: "Treatment record not found",
	})
}

// UpdateRecord updates a treatment record.
func (h *ProcedureHandler) UpdateRecord(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	recordID := r.PathValue("recordId")

	if patientNhi == "" || recordID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_PARAMS", Message: "Patient NHI and record ID are required",
		})
		return
	}

	var rec TreatmentRecord
	if err := decodeJSON(r, &rec); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	rec.ID = recordID
	rec.PatientNHI = patientNhi
	rec.UpdatedAt = time.Now().UnixMilli()

	h.logger.Info("treatment record updated",
		slog.String("record_id", recordID),
		slog.String("patient_nhi", patientNhi))

	writeJSON(w, http.StatusOK, rec)
}

// Ensure TreatmentRecord implements json interfaces.
var _ json.Marshaler = (*TreatmentRecord)(nil)
var _ json.Unmarshaler = (*TreatmentRecord)(nil)

func (r *TreatmentRecord) MarshalJSON() ([]byte, error) {
	type alias TreatmentRecord
	return json.Marshal((*alias)(r))
}

func (r *TreatmentRecord) UnmarshalJSON(data []byte) error {
	type alias TreatmentRecord
	return json.Unmarshal(data, (*alias)(r))
}
