package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/clinical"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// ICUAdmissionStatus tracks the patient's current ICU status.
type ICUAdmissionStatus string

const (
	ICUStatusActive     ICUAdmissionStatus = "active"
	ICUStatusStepDown   ICUAdmissionStatus = "step-down" // transferred to HDU/CCU
	ICUStatusDischarged ICUAdmissionStatus = "discharged"
)

// VentilationMode enumerates common modes of mechanical ventilation.
type VentilationMode string

const (
	VentModeNone        VentilationMode = "none"
	VentModeAC          VentilationMode = "ac"   // Assist-Control
	VentModeSIMV        VentilationMode = "simv" // Synchronized Intermittent Mandatory Ventilation
	VentModePSV         VentilationMode = "psv"  // Pressure Support Ventilation
	VentModeCPAP        VentilationMode = "cpap"
	VentModeBiPAP       VentilationMode = "bipap"
	VentModeHFOV        VentilationMode = "hfov" // High-Frequency Oscillatory Ventilation
	VentModeSpontaneous VentilationMode = "spontaneous"
)

// ICUVitals holds extended ICU-specific clinical measurements.
type ICUVitals struct {
	SystolicBP  *float64 `json:"systolicBp,omitempty"`
	DiastolicBP *float64 `json:"diastolicBp,omitempty"`
	MAP         *float64 `json:"map,omitempty"` // Mean Arterial Pressure mmHg
	HeartRate   *float64 `json:"heartRate,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"` // °C
	SpO2        *float64 `json:"spo2,omitempty"`        // %
	RespRate    *float64 `json:"respRate,omitempty"`    // breaths/min
	CVP         *float64 `json:"cvp,omitempty"`         // Central Venous Pressure cmH2O
	FiO2        *float64 `json:"fio2,omitempty"`        // Fraction of Inspired Oxygen
	PEEP        *float64 `json:"peep,omitempty"`        // cmH2O
	PIP         *float64 `json:"pip,omitempty"`         // Peak Inspiratory Pressure cmH2O
	TidalVolume *float64 `json:"tidalVolume,omitempty"` // mL
	GCS         *int     `json:"gcs,omitempty"`         // Glasgow Coma Scale 3–15
	PupilsEqual *bool    `json:"pupilsEqual,omitempty"`
	UrineMl     *float64 `json:"urineOutputMl,omitempty"` // hourly urine output mL
}

// SedationLevel uses the RASS (Richmond Agitation-Sedation Scale) -5 to +4.
type SedationLevel int

// ICUAdmission represents an admission to the intensive care unit.
type ICUAdmission struct {
	ID              string             `json:"id"`
	PatientID       string             `json:"patientId"`
	PatientNHI      string             `json:"patientNhi"`
	AdmissionID     string             `json:"admissionId"` // parent inpatient admission
	IntensivistHPI  string             `json:"intensivistHpi"`
	Status          ICUAdmissionStatus `json:"status"`
	BedID           string             `json:"bedId,omitempty"`
	AdmissionReason string             `json:"admissionReason"`
	Diagnosis       string             `json:"diagnosis,omitempty"` // ICD-10-AM
	VentilationMode VentilationMode    `json:"ventilationMode"`
	ApacheScore     *int               `json:"apacheScore,omitempty"`   // APACHE II score on admission
	SedationLevel   *SedationLevel     `json:"sedationLevel,omitempty"` // RASS
	TenantID        string             `json:"tenantId"`
	AdmittedAt      time.Time          `json:"admittedAt"`
	DischargedAt    *time.Time         `json:"dischargedAt,omitempty"`
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}

// ICUChartEntry is a single hourly nursing documentation record.
type ICUChartEntry struct {
	ID             string          `json:"id"`
	ICUAdmissionID string          `json:"icuAdmissionId"`
	NurseHPI       string          `json:"nurseHpi"`
	Vitals         ICUVitals       `json:"vitals"`
	Sedation       *SedationLevel  `json:"sedationLevel,omitempty"`
	VentMode       VentilationMode `json:"ventilationMode,omitempty"`
	Notes          string          `json:"notes,omitempty"`
	TenantID       string          `json:"tenantId"`
	RecordedAt     time.Time       `json:"recordedAt"`
}

type icuCreateRequest struct {
	PatientID       string          `json:"patientId"`
	PatientNHI      string          `json:"patientNhi"`
	AdmissionID     string          `json:"admissionId"`
	IntensivistHPI  string          `json:"intensivistHpi"`
	BedID           string          `json:"bedId,omitempty"`
	AdmissionReason string          `json:"admissionReason"`
	Diagnosis       string          `json:"diagnosis,omitempty"`
	VentilationMode VentilationMode `json:"ventilationMode,omitempty"`
	ApacheScore     *int            `json:"apacheScore,omitempty"`
}

type icuChartRequest struct {
	NurseHPI string          `json:"nurseHpi"`
	Vitals   ICUVitals       `json:"vitals"`
	Sedation *SedationLevel  `json:"sedationLevel,omitempty"`
	VentMode VentilationMode `json:"ventilationMode,omitempty"`
	Notes    string          `json:"notes,omitempty"`
}

type icuDischargeRequest struct {
	ToWardID string `json:"toWardId,omitempty"`
	ToBedID  string `json:"toBedId,omitempty"`
	StepDown bool   `json:"stepDown,omitempty"` // true = HDU step-down, false = full ward discharge
	Notes    string `json:"notes,omitempty"`
}

// ICUHandler handles all /api/v1/icu routes.
type ICUHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/icu/admissions.
func (h *ICUHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	statusFilter := r.URL.Query().Get("status")
	admissions, err := h.listICUAdmissions(ctx, tenantID.String(), statusFilter)
	if err != nil {
		h.logger.Error("list ICU admissions", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list ICU admissions"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"admissions": admissions, "total": len(admissions)})
}

// Create handles POST /api/v1/icu/admissions.
func (h *ICUHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req icuCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "either patientId or patientNhi is required"})
		return
	}
	if req.AdmissionID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ADMISSION", Message: "admissionId (parent inpatient admission) is required"})
		return
	}
	if req.IntensivistHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_INTENSIVIST", Message: "intensivistHpi is required"})
		return
	}

	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.IntensivistHPI)
	if err != nil {
		h.logger.Error("HPI APC validation for ICU", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate intensivist APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "INVALID_APC", Message: "intensivist does not hold a current Annual Practising Certificate", Details: apcStatus})
		return
	}

	adm, err := h.insertICUAdmission(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert ICU admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create ICU admission"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "ICUAdmission",
		ResourceID: adm.ID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, adm)
}

// Get handles GET /api/v1/icu/admissions/{id}.
func (h *ICUHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	adm, err := h.getICUAdmissionByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ICU admission not found"})
			return
		}
		h.logger.Error("get ICU admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve ICU admission"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "ICUAdmission",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, adm)
}

// AddChart handles POST /api/v1/icu/admissions/{id}/chart.
func (h *ICUHandler) AddChart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if _, err := h.getICUAdmissionByID(ctx, id, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ICU admission not found"})
			return
		}
		h.logger.Error("get ICU admission for chart", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve ICU admission"})
		return
	}

	var req icuChartRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.NurseHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NURSE", Message: "nurseHpi is required"})
		return
	}

	entry, err := h.insertChartEntry(ctx, id, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert ICU chart entry", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to add chart entry"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "ICUChart",
		ResourceID: entry.ID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, entry)
}

// ListChart handles GET /api/v1/icu/admissions/{id}/chart.
func (h *ICUHandler) ListChart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	entries, err := h.listChartEntries(ctx, id, tenantID.String())
	if err != nil {
		h.logger.Error("list ICU chart entries", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list chart entries"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "ICUChart",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "total": len(entries)})
}

// Discharge handles POST /api/v1/icu/admissions/{id}/discharge.
func (h *ICUHandler) Discharge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	existing, err := h.getICUAdmissionByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ICU admission not found"})
			return
		}
		h.logger.Error("get ICU admission for discharge", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve ICU admission"})
		return
	}
	if existing.Status == ICUStatusDischarged {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_DISCHARGED", Message: "ICU admission is already discharged"})
		return
	}

	var req icuDischargeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	now := time.Now().UTC()
	if req.StepDown {
		existing.Status = ICUStatusStepDown
	} else {
		existing.Status = ICUStatusDischarged
	}
	existing.DischargedAt = &now

	discharged, err := h.dischargeICUAdmission(ctx, existing)
	if err != nil {
		h.logger.Error("discharge ICU admission", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DISCHARGE_ERROR", Message: "failed to discharge ICU admission"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "ICUAdmission",
		ResourceID: id, TenantID: tenantID,
		Details:    map[string]any{"action": "discharge", "step_down": req.StepDown},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, discharged)
}

// ── Fluid Balance ─────────────────────────────────────────────────────────────

type fluidBalanceRequest struct {
	Direction     string `json:"direction"` // "in" or "out"
	FluidType     string `json:"fluidType"`
	VolumeML      int    `json:"volumeMl"`
	ProductName   string `json:"productName"`
	Concentration string `json:"concentration,omitempty"`
	RecordedBy    string `json:"recordedBy"`
	Comments      string `json:"comments,omitempty"`
}

// AddFluidBalance handles POST /api/v1/icu/admissions/{id}/fluid-balance.
func (h *ICUHandler) AddFluidBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if _, err := h.getICUAdmissionByID(ctx, id, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ICU admission not found"})
			return
		}
		h.logger.Error("get ICU admission for fluid balance", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve ICU admission"})
		return
	}

	var req fluidBalanceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Direction != "in" && req.Direction != "out" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_DIRECTION", Message: "direction must be 'in' or 'out'"})
		return
	}
	if req.VolumeML <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_VOLUME", Message: "volumeMl must be positive"})
		return
	}
	if req.RecordedBy == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_RECORDED_BY", Message: "recordedBy is required"})
		return
	}

	entry, err := h.insertFluidBalanceEntry(ctx, id, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert fluid balance entry", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to record fluid balance entry"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "FluidBalance",
		ResourceID: entry.ID, TenantID: tenantID,
		Details:    map[string]any{"direction": req.Direction, "fluid_type": req.FluidType, "volume_ml": req.VolumeML},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, entry)
}

// ListFluidBalance handles GET /api/v1/icu/admissions/{id}/fluid-balance.
func (h *ICUHandler) ListFluidBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	entries, err := h.listFluidBalanceEntries(ctx, id, tenantID.String())
	if err != nil {
		h.logger.Error("list fluid balance entries", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list fluid balance entries"})
		return
	}

	summary := SumFluidBalance(entries)
	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "FluidBalance",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "summary": summary, "total": len(entries)})
}

// ── EWS / PEWS Scoring ──────────────────────────────────────────────────────

type ewsRequest struct {
	RespirationRate int     `json:"respirationRate"`
	SpO2Scale       int     `json:"spo2Scale"`
	SpO2Percent     float64 `json:"spo2Percent"`
	AirOrOxygen     string  `json:"airOrOxygen"`
	SystolicBP      int     `json:"systolicBp"`
	HeartRate       int     `json:"heartRate"`
	Temperature     float64 `json:"temperature"`
	Consciousness   string  `json:"consciousness"`
}

type pewsRequest struct {
	HeartRate       int     `json:"heartRate"`
	RespirationRate int     `json:"respirationRate"`
	SpO2Percent     float64 `json:"spo2Percent"`
	SystolicBP      int     `json:"systolicBp"`
	Temperature     float64 `json:"temperature"`
	Consciousness   string  `json:"consciousness"`
	Behaviour       string  `json:"behaviour"`
	FluidIntake     string  `json:"fluidIntake"`
	PainScore       int     `json:"painScore"`
}

type earlyWarningScoreRecord struct {
	ID               string         `json:"id"`
	ICUAdmissionID   string         `json:"icuAdmissionId"`
	ScoreType        string         `json:"scoreType"`
	TotalScore       int            `json:"totalScore"`
	ClinicalRisk     string         `json:"clinicalRisk"`
	IndividualScores map[string]int `json:"individualScores"`
	RecordedBy       string         `json:"recordedBy"`
	TenantID         string         `json:"tenantId"`
	RecordedAt       time.Time      `json:"recordedAt"`
}

// CalculateEWS handles POST /api/v1/icu/admissions/{id}/ews.
func (h *ICUHandler) CalculateEWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if _, err := h.getICUAdmissionByID(ctx, id, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ICU admission not found"})
			return
		}
		h.logger.Error("get ICU admission for EWS", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve ICU admission"})
		return
	}

	var req ewsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	// Delegate to core scoring engine.
	input := clinical.EWSInput{
		RespirationRate: req.RespirationRate,
		SpO2Scale:       req.SpO2Scale,
		SpO2Percent:     req.SpO2Percent,
		AirOrOxygen:     req.AirOrOxygen,
		SystolicBP:      req.SystolicBP,
		HeartRate:       req.HeartRate,
		Temperature:     req.Temperature,
		Consciousness:   req.Consciousness,
	}
	result := clinical.CalculateEWS(input)

	record, err := h.insertEWSRecord(ctx, id, "ews", result.TotalScore, result.ClinicalRisk, result.IndividualScores, principal.ID, tenantID.String())
	if err != nil {
		h.logger.Error("insert EWS record", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to record EWS score"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "EarlyWarningScore",
		ResourceID: record.ID, TenantID: tenantID,
		Details:    map[string]any{"score_type": "ews", "total_score": result.TotalScore, "risk": result.ClinicalRisk},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, record)
}

// CalculatePEWS handles POST /api/v1/icu/admissions/{id}/pews.
func (h *ICUHandler) CalculatePEWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if _, err := h.getICUAdmissionByID(ctx, id, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ICU admission not found"})
			return
		}
		h.logger.Error("get ICU admission for PEWS", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve ICU admission"})
		return
	}

	var req pewsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	input := clinical.PEWSInput{
		HeartRate:       req.HeartRate,
		RespirationRate: req.RespirationRate,
		SpO2Percent:     req.SpO2Percent,
		SystolicBP:      req.SystolicBP,
		Temperature:     req.Temperature,
		Consciousness:   req.Consciousness,
		Behaviour:       req.Behaviour,
		FluidIntake:     req.FluidIntake,
		PainScore:       req.PainScore,
	}
	result := clinical.CalculatePEWS(input)

	record, err := h.insertEWSRecord(ctx, id, "pews", result.TotalScore, result.Escalation, result.IndividualScores, principal.ID, tenantID.String())
	if err != nil {
		h.logger.Error("insert PEWS record", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to record PEWS score"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "EarlyWarningScore",
		ResourceID: record.ID, TenantID: tenantID,
		Details:    map[string]any{"score_type": "pews", "total_score": result.TotalScore, "escalation": result.Escalation},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, record)
}

// ListEWS handles GET /api/v1/icu/admissions/{id}/ews.
func (h *ICUHandler) ListEWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	id := r.PathValue("id")
	records, err := h.listEWSRecords(ctx, id, tenantID.String())
	if err != nil {
		h.logger.Error("list EWS records", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list EWS records"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scores": records, "total": len(records)})
}

// ── Paediatric Dosing Calculator ─────────────────────────────────────────────

type paedDoseRequest struct {
	DrugName      string  `json:"drugName"`
	WeightKg      float64 `json:"weightKg"`
	DosePerKg     float64 `json:"dosePerKg"`
	Frequency     string  `json:"frequency"`
	MaxSingleDose float64 `json:"maxSingleDose"`
	MaxDailyDose  float64 `json:"maxDailyDose"`
	Route         string  `json:"route"`
	AgeMonths     int     `json:"ageMonths"`
}

// CalculatePaediatricDose handles POST /api/v1/icu/paediatric-dose.
func (h *ICUHandler) CalculatePaediatricDose(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req paedDoseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.DrugName == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DRUG", Message: "drugName is required"})
		return
	}
	if req.WeightKg <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_WEIGHT", Message: "weightKg must be positive"})
		return
	}
	if req.DosePerKg <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_DOSE", Message: "dosePerKg must be positive"})
		return
	}

	input := clinical.DoseInput{
		DrugName:      req.DrugName,
		WeightKg:      req.WeightKg,
		DosePerKg:     req.DosePerKg,
		Frequency:     req.Frequency,
		MaxSingleDose: req.MaxSingleDose,
		MaxDailyDose:  req.MaxDailyDose,
		Route:         req.Route,
		AgeMonths:     req.AgeMonths,
	}
	result := clinical.CalculatePaedDose(input)

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "calculate", ResourceType: "PaediatricDose",
		ResourceID: req.DrugName, TenantID: tenantID,
		Details:    map[string]any{"drug": req.DrugName, "weight_kg": req.WeightKg, "dose": result.CappedDose},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, result)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *ICUHandler) listICUAdmissions(ctx context.Context, tenantID, statusFilter string) ([]ICUAdmission, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, admission_id, intensivist_hpi, status, bed_id,
		        admission_reason, diagnosis, ventilation_mode, apache_score, sedation_level,
		        tenant_id, admitted_at, discharged_at, created_at, updated_at
		 FROM icu_admissions
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY admitted_at DESC`,
		db.NamedArgs{"tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query ICU admissions: %w", err)
	}
	defer rows.Close()

	var results []ICUAdmission
	for rows.Next() {
		adm, err := scanICUAdmissionRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, adm)
	}
	return results, rows.Err()
}

func (h *ICUHandler) getICUAdmissionByID(ctx context.Context, id, tenantID string) (ICUAdmission, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, admission_id, intensivist_hpi, status, bed_id,
		        admission_reason, diagnosis, ventilation_mode, apache_score, sedation_level,
		        tenant_id, admitted_at, discharged_at, created_at, updated_at
		 FROM icu_admissions
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	adm, err := scanICUAdmissionRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ICUAdmission{}, errNotFound
		}
		return ICUAdmission{}, fmt.Errorf("get ICU admission: %w", err)
	}
	return adm, nil
}

func (h *ICUHandler) insertICUAdmission(ctx context.Context, req icuCreateRequest, tenantID string) (ICUAdmission, error) {
	mode := req.VentilationMode
	if mode == "" {
		mode = VentModeNone
	}
	row := h.pool.QueryRow(ctx,
		`INSERT INTO icu_admissions
		   (patient_id, patient_nhi, admission_id, intensivist_hpi, status, bed_id,
		    admission_reason, diagnosis, ventilation_mode, apache_score, tenant_id, admitted_at)
		 VALUES
		   (@patient_id, @patient_nhi, @admission_id, @intensivist_hpi, @status, @bed_id,
		    @admission_reason, @diagnosis, @ventilation_mode, @apache_score, @tenant_id, now())
		 RETURNING id, patient_id, patient_nhi, admission_id, intensivist_hpi, status, bed_id,
		           admission_reason, diagnosis, ventilation_mode, apache_score, sedation_level,
		           tenant_id, admitted_at, discharged_at, created_at, updated_at`,
		db.NamedArgs{
			"patient_id":       req.PatientID,
			"patient_nhi":      req.PatientNHI,
			"admission_id":     req.AdmissionID,
			"intensivist_hpi":  req.IntensivistHPI,
			"status":           ICUStatusActive,
			"bed_id":           req.BedID,
			"admission_reason": req.AdmissionReason,
			"diagnosis":        req.Diagnosis,
			"ventilation_mode": mode,
			"apache_score":     req.ApacheScore,
			"tenant_id":        tenantID,
		},
	)
	return scanICUAdmissionRow(row)
}

func (h *ICUHandler) insertChartEntry(ctx context.Context, icuAdmissionID string, req icuChartRequest, tenantID string) (ICUChartEntry, error) {
	mode := req.VentMode
	if mode == "" {
		mode = VentModeNone
	}
	row := h.pool.QueryRow(ctx,
		`INSERT INTO icu_chart_entries
		   (icu_admission_id, nurse_hpi, vitals, sedation_level, ventilation_mode, notes, tenant_id, recorded_at)
		 VALUES
		   (@icu_admission_id, @nurse_hpi, @vitals, @sedation_level, @ventilation_mode, @notes, @tenant_id, now())
		 RETURNING id, icu_admission_id, nurse_hpi, vitals, sedation_level, ventilation_mode, notes, tenant_id, recorded_at`,
		db.NamedArgs{
			"icu_admission_id": icuAdmissionID,
			"nurse_hpi":        req.NurseHPI,
			"vitals":           req.Vitals,
			"sedation_level":   req.Sedation,
			"ventilation_mode": mode,
			"notes":            req.Notes,
			"tenant_id":        tenantID,
		},
	)
	var e ICUChartEntry
	if err := row.Scan(
		&e.ID, &e.ICUAdmissionID, &e.NurseHPI, &e.Vitals, &e.Sedation, &e.VentMode, &e.Notes,
		&e.TenantID, &e.RecordedAt,
	); err != nil {
		return ICUChartEntry{}, fmt.Errorf("insert ICU chart entry: %w", err)
	}
	return e, nil
}

func (h *ICUHandler) listChartEntries(ctx context.Context, icuAdmissionID, tenantID string) ([]ICUChartEntry, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, icu_admission_id, nurse_hpi, vitals, sedation_level, ventilation_mode, notes, tenant_id, recorded_at
		 FROM icu_chart_entries
		 WHERE icu_admission_id = @icu_admission_id AND tenant_id = @tenant_id
		 ORDER BY recorded_at DESC`,
		db.NamedArgs{"icu_admission_id": icuAdmissionID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query ICU chart entries: %w", err)
	}
	defer rows.Close()

	var results []ICUChartEntry
	for rows.Next() {
		var e ICUChartEntry
		if err := rows.Scan(
			&e.ID, &e.ICUAdmissionID, &e.NurseHPI, &e.Vitals, &e.Sedation, &e.VentMode, &e.Notes,
			&e.TenantID, &e.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ICU chart entry: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (h *ICUHandler) dischargeICUAdmission(ctx context.Context, a ICUAdmission) (ICUAdmission, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE icu_admissions
		 SET status = @status, discharged_at = @discharged_at, updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, admission_id, intensivist_hpi, status, bed_id,
		           admission_reason, diagnosis, ventilation_mode, apache_score, sedation_level,
		           tenant_id, admitted_at, discharged_at, created_at, updated_at`,
		db.NamedArgs{
			"status":        a.Status,
			"discharged_at": a.DischargedAt,
			"id":            a.ID,
			"tenant_id":     a.TenantID,
		},
	)
	updated, err := scanICUAdmissionRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ICUAdmission{}, errNotFound
		}
		return ICUAdmission{}, fmt.Errorf("discharge ICU admission: %w", err)
	}
	return updated, nil
}

func scanICUAdmissionRow(row dbRow) (ICUAdmission, error) {
	var a ICUAdmission
	if err := row.Scan(
		&a.ID, &a.PatientID, &a.PatientNHI, &a.AdmissionID, &a.IntensivistHPI,
		&a.Status, &a.BedID, &a.AdmissionReason, &a.Diagnosis,
		&a.VentilationMode, &a.ApacheScore, &a.SedationLevel,
		&a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return ICUAdmission{}, err
	}
	return a, nil
}

// ── Fluid Balance DB helpers ─────────────────────────────────────────────────

type fluidBalanceDBEntry struct {
	ID             string    `json:"id"`
	ICUAdmissionID string    `json:"icuAdmissionId"`
	Direction      string    `json:"direction"`
	FluidType      string    `json:"fluidType"`
	VolumeML       int       `json:"volumeMl"`
	ProductName    string    `json:"productName"`
	Concentration  string    `json:"concentration,omitempty"`
	RecordedBy     string    `json:"recordedBy"`
	Shift          string    `json:"shift"`
	Comments       string    `json:"comments,omitempty"`
	TenantID       string    `json:"tenantId"`
	RecordedAt     time.Time `json:"recordedAt"`
}

func (h *ICUHandler) insertFluidBalanceEntry(ctx context.Context, icuAdmissionID string, req fluidBalanceRequest, tenantID string) (fluidBalanceDBEntry, error) {
	now := time.Now()
	shift := shiftFromTime(now)
	row := h.pool.QueryRow(ctx,
		`INSERT INTO fluid_balance_entries
		   (icu_admission_id, direction, fluid_type, volume_ml, product_name,
		    concentration, recorded_by, shift, comments, tenant_id, recorded_at)
		 VALUES
		   (@icu_admission_id, @direction, @fluid_type, @volume_ml, @product_name,
		    @concentration, @recorded_by, @shift, @comments, @tenant_id, now())
		 RETURNING id, icu_admission_id, direction, fluid_type, volume_ml, product_name,
		           concentration, recorded_by, shift, comments, tenant_id, recorded_at`,
		db.NamedArgs{
			"icu_admission_id": icuAdmissionID,
			"direction":        req.Direction,
			"fluid_type":       req.FluidType,
			"volume_ml":        req.VolumeML,
			"product_name":     req.ProductName,
			"concentration":    req.Concentration,
			"recorded_by":      req.RecordedBy,
			"shift":            shift,
			"comments":         req.Comments,
			"tenant_id":        tenantID,
		},
	)
	var e fluidBalanceDBEntry
	if err := row.Scan(
		&e.ID, &e.ICUAdmissionID, &e.Direction, &e.FluidType, &e.VolumeML,
		&e.ProductName, &e.Concentration, &e.RecordedBy, &e.Shift, &e.Comments,
		&e.TenantID, &e.RecordedAt,
	); err != nil {
		return fluidBalanceDBEntry{}, fmt.Errorf("insert fluid balance entry: %w", err)
	}
	return e, nil
}

func (h *ICUHandler) listFluidBalanceEntries(ctx context.Context, icuAdmissionID, tenantID string) ([]BalanceEntry, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, icu_admission_id, '', direction, fluid_type, volume_ml, product_name,
		        concentration, recorded_by, recorded_at, shift, comments
		 FROM fluid_balance_entries
		 WHERE icu_admission_id = @icu_admission_id AND tenant_id = @tenant_id
		 ORDER BY recorded_at ASC`,
		db.NamedArgs{"icu_admission_id": icuAdmissionID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query fluid balance entries: %w", err)
	}
	defer rows.Close()

	var results []BalanceEntry
	for rows.Next() {
		var e BalanceEntry
		if err := rows.Scan(
			&e.ID, &e.AdmissionID, &e.PatientNHI, &e.Direction, &e.FluidType,
			&e.VolumeML, &e.ProductName, &e.Concentration, &e.RecordedBy,
			&e.RecordedAt, &e.Shift, &e.Comments,
		); err != nil {
			return nil, fmt.Errorf("scan fluid balance entry: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// ── EWS/PEWS DB helpers ─────────────────────────────────────────────────────

func (h *ICUHandler) insertEWSRecord(ctx context.Context, icuAdmissionID, scoreType string, totalScore int, clinicalRisk string, individualScores map[string]int, recordedBy, tenantID string) (earlyWarningScoreRecord, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO early_warning_scores
		   (icu_admission_id, score_type, total_score, clinical_risk, individual_scores,
		    recorded_by, tenant_id, recorded_at)
		 VALUES
		   (@icu_admission_id, @score_type, @total_score, @clinical_risk, @individual_scores,
		    @recorded_by, @tenant_id, now())
		 RETURNING id, icu_admission_id, score_type, total_score, clinical_risk,
		           individual_scores, recorded_by, tenant_id, recorded_at`,
		db.NamedArgs{
			"icu_admission_id":  icuAdmissionID,
			"score_type":        scoreType,
			"total_score":       totalScore,
			"clinical_risk":     clinicalRisk,
			"individual_scores": individualScores,
			"recorded_by":       recordedBy,
			"tenant_id":         tenantID,
		},
	)
	var rec earlyWarningScoreRecord
	if err := row.Scan(
		&rec.ID, &rec.ICUAdmissionID, &rec.ScoreType, &rec.TotalScore,
		&rec.ClinicalRisk, &rec.IndividualScores, &rec.RecordedBy,
		&rec.TenantID, &rec.RecordedAt,
	); err != nil {
		return earlyWarningScoreRecord{}, fmt.Errorf("insert EWS record: %w", err)
	}
	return rec, nil
}

func (h *ICUHandler) listEWSRecords(ctx context.Context, icuAdmissionID, tenantID string) ([]earlyWarningScoreRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, icu_admission_id, score_type, total_score, clinical_risk,
		        individual_scores, recorded_by, tenant_id, recorded_at
		 FROM early_warning_scores
		 WHERE icu_admission_id = @icu_admission_id AND tenant_id = @tenant_id
		 ORDER BY recorded_at DESC`,
		db.NamedArgs{"icu_admission_id": icuAdmissionID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query EWS records: %w", err)
	}
	defer rows.Close()

	var results []earlyWarningScoreRecord
	for rows.Next() {
		var rec earlyWarningScoreRecord
		if err := rows.Scan(
			&rec.ID, &rec.ICUAdmissionID, &rec.ScoreType, &rec.TotalScore,
			&rec.ClinicalRisk, &rec.IndividualScores, &rec.RecordedBy,
			&rec.TenantID, &rec.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan EWS record: %w", err)
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}
