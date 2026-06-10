// Package api implements the methadone programme HTTP handlers.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-addiction/internal/methadone"
)

// MethadoneHandler handles methadone programme routes.
type MethadoneHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListProgrammes GET /api/v1/methadone/programmes
func (h *MethadoneHandler) ListProgrammes(w http.ResponseWriter, r *http.Request) {
	// Stub: in production this would query addiction_programmes table with tenant filter.
	writeJSON(w, http.StatusOK, []methadone.Programme{})
}

// CreateProgramme POST /api/v1/methadone/programmes
func (h *MethadoneHandler) CreateProgramme(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PatientNHI      string  `json:"patientNhi"`
		ClinicianID     string  `json:"clinicianId"`
		StartDate       string  `json:"startDate"`
		SubstancePrimary string `json:"substancePrimary"`
		InitialDoseMg   float64 `json:"initialDoseMg"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	p := methadone.Programme{
		ID:             genUUID(),
		PatientNHI:     req.PatientNHI,
		ClinicianID:    req.ClinicianID,
		StartDate:      parseTime(req.StartDate),
		Phase:          methadone.PhaseInduction,
		SubstancePrimary: req.SubstancePrimary,
		InitialDoseMg:  req.InitialDoseMg,
		CurrentDoseMg:  req.InitialDoseMg,
		TakeHomeLevel:  1,
		TakeHomeMaxDays: 0,
		NextReviewDate: time.Now().AddDate(0, 0, 7),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "methadone.programme.created", ResourceID: p.ID, PatientNHI: req.PatientNHI, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, p)
}

// GetProgramme GET /api/v1/methadone/programmes/{programmeId}
func (h *MethadoneHandler) GetProgramme(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("programmeId")
	writeJSON(w, http.StatusOK, methadone.Programme{ID: id, PatientNHI: "ABC1234", Phase: methadone.PhaseMaintenance})
}

// UpdateProgramme PUT /api/v1/methadone/programmes/{programmeId}
func (h *MethadoneHandler) UpdateProgramme(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("programmeId")
	var req struct {
		Phase        string     `json:"phase,omitempty"`
		CurrentDoseMg float64   `json:"currentDoseMg,omitempty"`
		NextReviewDate string   `json:"nextReviewDate,omitempty"`
		EndDate      *string    `json:"endDate,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	_ = id
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "methadone.programme.updated", ResourceID: id, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusOK, map[string]string{"status":"updated"})
}

// ListDoses GET /api/v1/methadone/programmes/{programmeId}/doses
func (h *MethadoneHandler) ListDoses(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("programmeId")
	writeJSON(w, http.StatusOK, []methadone.DoseRecord{})
}

// RecordDose POST /api/v1/methadone/programmes/{programmeId}/doses
func (h *MethadoneHandler) RecordDose(w http.ResponseWriter, r *http.Request) {
	programmeID := r.PathValue("programmeId")
	var req struct {
		DoseMg          float64 `json:"doseMg"`
		Formulation     string  `json:"formulation"`
		WitnessedBy     string  `json:"witnessedBy"`
		DispensedBy     string  `json:"dispensedBy"`
		PharmacistCheck bool    `json:"pharmacistCheck"`
		Status          string  `json:"status"`
		Notes           string  `json:"notes"`
		TakeHome        bool    `json:"takeHome"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	d := methadone.DoseRecord{
		ID:             genUUID(),
		ProgrammeID:    programmeID,
		AdministeredAt: time.Now(),
		DoseMg:         req.DoseMg,
		Formulation:    req.Formulation,
		WitnessedBy:    req.WitnessedBy,
		DispensedBy:    req.DispensedBy,
		PharmacistCheck: req.PharmacistCheck,
		Status:         req.Status,
		Notes:          req.Notes,
		TakeHome:       req.TakeHome,
		CreatedAt:      time.Now(),
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "methadone.dose.recorded", ResourceID: d.ID, Details: map[string]any{"dose_mg": req.DoseMg, "status": req.Status}, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, d)
}

// GetDose GET /api/v1/methadone/programmes/{programmeId}/doses/{doseId}
func (h *MethadoneHandler) GetDose(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("doseId")
	_ = r.PathValue("programmeId")
	writeJSON(w, http.StatusOK, methadone.DoseRecord{ID: id, DoseMg: 50.0, Status: "administered"})
}

// ApproveTakeHome POST /api/v1/methadone/programmes/{programmeId}/take-home
func (h *MethadoneHandler) ApproveTakeHome(w http.ResponseWriter, r *http.Request) {
	programmeID := r.PathValue("programmeId")
	var req struct {
		Level    int    `json:"level"`
		ExpiresAt string `json:"expiresAt,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	th := methadone.TakeHomeApproval{
		ID:         genUUID(),
		ProgrammeID: programmeID,
		Level:      req.Level,
		MaxDays:    methadone.TakeHomeDays(req.Level),
		CreatedAt:  time.Now(),
	}
	if req.ExpiresAt != "" {
		t := parseTime(req.ExpiresAt)
		th.ExpiresAt = &t
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "methadone.takehome.approved", ResourceID: th.ID, Details: map[string]any{"level": req.Level}, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, th)
}

// ListTakeHomeHistory GET /api/v1/methadone/programmes/{programmeId}/take-home
func (h *MethadoneHandler) ListTakeHomeHistory(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("programmeId")
	writeJSON(w, http.StatusOK, []methadone.TakeHomeApproval{})
}

// ListUrineScreens GET /api/v1/methadone/programmes/{programmeId}/urine-screens
func (h *MethadoneHandler) ListUrineScreens(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("programmeId")
	writeJSON(w, http.StatusOK, []methadone.UrineScreen{})
}

// RecordUrineScreen POST /api/v1/methadone/programmes/{programmeId}/urine-screens
func (h *MethadoneHandler) RecordUrineScreen(w http.ResponseWriter, r *http.Request) {
	programmeID := r.PathValue("programmeId")
	var req struct {
		CollectedAt  string               `json:"collectedAt"`
		CollectedBy  string               `json:"collectedBy"`
		LabName      string               `json:"labName"`
		Results      []methadone.DrugResult `json:"results"`
		MSSAResult   string               `json:"mssaResult"`
		ClinicalNotes string              `json:"clinicalNotes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	us := methadone.UrineScreen{
		ID:          genUUID(),
		ProgrammeID: programmeID,
		CollectedAt: parseTime(req.CollectedAt),
		CollectedBy: req.CollectedBy,
		LabName:     req.LabName,
		Results:     req.Results,
		MSSAResult:  req.MSSAResult,
		ClinicalNotes: req.ClinicalNotes,
		CreatedAt:   time.Now(),
	}
	_ = h.auditTrail.Record(r.Context(), audit.Event{Action: "methadone.urine.recorded", ResourceID: us.ID, Details: map[string]any{"mssa_result": req.MSSAResult}, OccurredAt: time.Now().UTC()})
	writeJSON(w, http.StatusCreated, us)
}

// GetUrineScreen GET /api/v1/methadone/programmes/{programmeId}/urine-screens/{screenId}
func (h *MethadoneHandler) GetUrineScreen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("screenId")
	_ = r.PathValue("programmeId")
	writeJSON(w, http.StatusOK, methadone.UrineScreen{ID: id, ProgrammeID: r.PathValue("programmeId"), MSSAResult: "conforming"})
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	if t.IsZero() {
		t = time.Now()
	}
	return t
}

func genUUID() string {
	return "stub-uuid"
}
