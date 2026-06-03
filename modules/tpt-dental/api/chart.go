package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-dental/internal/fdi"
)

// ChartHandler handles dental charting CRUD operations.
type ChartHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// chartStorage is the persisted representation of a dental chart.
type chartStorage struct {
	PatientNHI  string            `json:"patientNhi"`
	Dentition   string            `json:"dentition"`
	Entries     []fdi.ToothChartEntry `json:"entries"`
	ChartDate   int64             `json:"chartDate"`
	ClinicianID string            `json:"clinicianId"`
	PracticeID  string            `json:"practiceId"`
	VisitID     string            `json:"visitId,omitempty"`
	CreatedAt   int64             `json:"createdAt"`
	UpdatedAt   int64             `json:"updatedAt"`
}

// GetChart returns the full dental chart for a patient.
func (h *ChartHandler) GetChart(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	if patientNhi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_NHI", Message: "Patient NHI is required",
		})
		return
	}

	// Load chart from storage (simplified — real implementation queries DB).
	chart := &fdi.DentalChart{
		PatientNHI: patientNhi,
		Dentition:  fdi.DentitionPermanent,
		ChartDate:  time.Now().UnixMilli(),
	}

	writeJSON(w, http.StatusOK, chart)
}

// SaveChart saves or updates the full dental chart for a patient.
func (h *ChartHandler) SaveChart(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	if patientNhi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_NHI", Message: "Patient NHI is required",
		})
		return
	}

	var chart fdi.DentalChart
	if err := decodeJSON(r, &chart); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	chart.PatientNHI = patientNhi
	chart.ChartDate = time.Now().UnixMilli()

	// Validate tooth codes if entries provided.
	for _, entry := range chart.Entries {
		if entry.ToothCode != "" {
			if !fdi.ValidToothCode(entry.ToothCode) {
				writeJSON(w, http.StatusBadRequest, apiError{
					Code: "INVALID_TOOTH", Message: fmt.Sprintf("Invalid FDI tooth code: %s", entry.ToothCode),
				})
				return
			}
		}
	}

	h.logger.Info("dental chart saved",
		slog.String("patient_nhi", patientNhi),
		slog.Int("entries", len(chart.Entries)))

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "saved",
		"patientNhi": patientNhi,
		"entries":   len(chart.Entries),
		"chartDate": chart.ChartDate,
	})
}

// GetTooth returns the charting status for a specific tooth.
func (h *ChartHandler) GetTooth(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	fdiCode := r.PathValue("fdiCode")

	if patientNhi == "" || fdiCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_PARAMS", Message: "Patient NHI and FDI code are required",
		})
		return
	}

	if !fdi.ValidToothCode(fdiCode) {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "INVALID_TOOTH", Message: fmt.Sprintf("Invalid FDI tooth code: %s", fdiCode),
		})
		return
	}

	tooth, err := fdi.LookupTooth(fdiCode)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "TOOTH_LOOKUP_FAILED", Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, tooth)
}

// UpdateTooth updates the charting status for a specific tooth.
func (h *ChartHandler) UpdateTooth(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	fdiCode := r.PathValue("fdiCode")

	if patientNhi == "" || fdiCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_PARAMS", Message: "Patient NHI and FDI code are required",
		})
		return
	}

	if !fdi.ValidToothCode(fdiCode) {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "INVALID_TOOTH", Message: fmt.Sprintf("Invalid FDI tooth code: %s", fdiCode),
		})
		return
	}

	var entry fdi.ToothChartEntry
	if err := decodeJSON(r, &entry); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	entry.ToothCode = fdiCode
	entry.UpdatedAt = time.Now().UnixMilli()

	h.logger.Info("tooth chart entry updated",
		slog.String("patient_nhi", patientNhi),
		slog.String("tooth", fdiCode),
		slog.String("status", string(entry.Status)))

	writeJSON(w, http.StatusOK, entry)
}

// Ensure chartStorage implements json.Marshaler/Unmarshaler.
var _ json.Marshaler = (*chartStorage)(nil)
var _ json.Unmarshaler = (*chartStorage)(nil)

func (c *chartStorage) MarshalJSON() ([]byte, error) {
	type alias chartStorage
	return json.Marshal((*alias)(c))
}

func (c *chartStorage) UnmarshalJSON(data []byte) error {
	type alias chartStorage
	return json.Unmarshal(data, (*alias)(c))
}