package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-dental/internal/fdi"
	"github.com/jackc/pgx/v5"
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
	PatientNHI  string                `json:"patientNhi"`
	Dentition   string                `json:"dentition"`
	Entries     []fdi.ToothChartEntry `json:"entries"`
	ChartDate   int64                 `json:"chartDate"`
	ClinicianID string                `json:"clinicianId"`
	PracticeID  string                `json:"practiceId"`
	VisitID     string                `json:"visitId,omitempty"`
	CreatedAt   int64                 `json:"createdAt"`
	UpdatedAt   int64                 `json:"updatedAt"`
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

	ctx := r.Context()
	tenantID, _ := middleware.TenantFromContext(ctx)

	var chart fdi.DentalChart
	var entriesJSON json.RawMessage

	err := h.pool.QueryRow(ctx, `
		SELECT patient_nhi, COALESCE(visit_id,''), dentition, entries,
		       clinician_id, practice_id, chart_date
		FROM dental_charts
		WHERE tenant_id=$1 AND patient_nhi=$2
		ORDER BY chart_date DESC
		LIMIT 1`,
		tenantID, patientNhi,
	).Scan(
		&chart.PatientNHI, &chart.VisitID, &chart.Dentition,
		&entriesJSON, &chart.ClinicianID, &chart.PracticeID, &chart.ChartDate,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return an empty chart — patient has no recorded history yet.
			writeJSON(w, http.StatusOK, &fdi.DentalChart{
				PatientNHI: patientNhi,
				Dentition:  fdi.DentitionPermanent,
				Entries:    []fdi.ToothChartEntry{},
				ChartDate:  time.Now().UnixMilli(),
			})
			return
		}
		h.logger.Error("get dental chart", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to fetch chart"})
		return
	}

	if err := json.Unmarshal(entriesJSON, &chart.Entries); err != nil {
		chart.Entries = []fdi.ToothChartEntry{}
	}

	writeJSON(w, http.StatusOK, &chart)
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

	ctx := r.Context()
	tenantID, _ := middleware.TenantFromContext(ctx)

	entriesJSON, err := json.Marshal(chart.Entries)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "MARSHAL_ERROR", Message: "failed to serialise entries"})
		return
	}

	dentition := chart.Dentition
	if dentition == "" {
		dentition = fdi.DentitionPermanent
	}

	_, err = h.pool.Exec(ctx, `
		INSERT INTO dental_charts
			(tenant_id, patient_nhi, visit_id, dentition, entries,
			 clinician_id, practice_id, chart_date, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW(),NOW())
		ON CONFLICT (tenant_id, patient_nhi)
		DO UPDATE SET
			visit_id     = EXCLUDED.visit_id,
			dentition    = EXCLUDED.dentition,
			entries      = EXCLUDED.entries,
			clinician_id = EXCLUDED.clinician_id,
			practice_id  = EXCLUDED.practice_id,
			chart_date   = EXCLUDED.chart_date,
			updated_at   = NOW()`,
		tenantID, patientNhi,
		nilIfEmpty(chart.VisitID), string(dentition), entriesJSON,
		chart.ClinicianID, chart.PracticeID, chart.ChartDate,
	)
	if err != nil {
		h.logger.Error("save dental chart", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to save chart"})
		return
	}

	h.logger.Info("dental chart saved",
		slog.String("patient_nhi", patientNhi),
		slog.Int("entries", len(chart.Entries)))

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "saved",
		"patientNhi": patientNhi,
		"entries":    len(chart.Entries),
		"chartDate":  chart.ChartDate,
	})
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
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
