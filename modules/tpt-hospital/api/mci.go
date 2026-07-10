package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// MCITriageCategory is the START/JumpSTART patient classification.
type MCITriageCategory string

const (
	MCIImmediate MCITriageCategory = "immediate" // life-threatening, can survive with immediate care
	MCIDelayed   MCITriageCategory = "delayed"   // serious but not immediately life-threatening
	MCIMinor     MCITriageCategory = "minor"     // walking wounded
	MCIExpectant MCITriageCategory = "expectant" // unsurvivable given available resources
	MCIDeceased  MCITriageCategory = "deceased"
)

// MCITriageMethod distinguishes adult (START) from paediatric (JumpSTART) algorithm.
type MCITriageMethod string

const (
	MCIMethodSTART     MCITriageMethod = "start"
	MCIMethodJumpSTART MCITriageMethod = "jumpstart"
)

// STARTInputs holds the observable findings used by the START or JumpSTART algorithm.
// All boolean fields use pointer semantics so the caller can indicate "not yet assessed".
type STARTInputs struct {
	// Ambulatory — patient walked to assembly point.
	Ambulatory bool `json:"ambulatory"`
	// RespiratoryRate — breaths per minute. 0 = apnoeic after repositioning.
	RespiratoryRate int `json:"respiratoryRate"`
	// RadialPulse — true if palpable radial pulse present.
	RadialPulse bool `json:"radialPulse"`
	// CapRefillSecs — capillary refill time in seconds (used when radial absent).
	CapRefillSecs *float64 `json:"capRefillSecs,omitempty"`
	// FollowsCommands — patient can follow simple verbal commands.
	FollowsCommands bool `json:"followsCommands"`
	// IsPaediatric — true if <8 years or estimated weight <25 kg (triggers JumpSTART).
	IsPaediatric bool `json:"isPaediatric"`
	// PaediatricPulse — (JumpSTART only) palpable peripheral pulse.
	PaediatricPulse bool `json:"paediatricPulse,omitempty"`
	// AVPU — (JumpSTART only) Alert/Voice/Pain/Unresponsive.
	AVPU string `json:"avpu,omitempty"`
}

// MCIPatient is a tagged patient in a mass casualty incident.
type MCIPatient struct {
	ID                  string            `json:"id"`
	IncidentID          string            `json:"incidentId"`
	TenantID            string            `json:"tenantId"`
	TagNumber           int               `json:"tagNumber"`
	NHIMasked           string            `json:"nhiMasked,omitempty"` // display only — last 3 chars
	TriageCategory      MCITriageCategory `json:"triageCategory"`
	TriageMethod        MCITriageMethod   `json:"triageMethod"`
	IsPaediatric        bool              `json:"isPaediatric"`
	AgeYearsApprox      *int              `json:"ageYearsApprox,omitempty"`
	Sex                 string            `json:"sex,omitempty"`
	PresentingComplaint string            `json:"presentingComplaint,omitempty"`
	AllocatedZone       string            `json:"allocatedZone,omitempty"`
	LastReassessedAt    *time.Time        `json:"lastReassessedAt,omitempty"`
	ReassessedBy        string            `json:"reassessedBy,omitempty"`
	Notes               string            `json:"notes,omitempty"`
	CreatedAt           time.Time         `json:"createdAt"`
	UpdatedAt           time.Time         `json:"updatedAt"`
}

// MCISummary is the triage count dashboard for an incident.
type MCISummary struct {
	IncidentID   string         `json:"incidentId"`
	Total        int            `json:"total"`
	Unidentified int            `json:"unidentified"` // no NHI linked yet
	ByCategory   map[string]int `json:"byCategory"`
	GeneratedAt  time.Time      `json:"generatedAt"`
}

type mciTagRequest struct {
	TagNumber           int         `json:"tagNumber"`
	STARTInputs         STARTInputs `json:"startInputs"`
	AgeYearsApprox      *int        `json:"ageYearsApprox,omitempty"`
	Sex                 string      `json:"sex,omitempty"`
	PresentingComplaint string      `json:"presentingComplaint,omitempty"`
	AllocatedZone       string      `json:"allocatedZone,omitempty"`
	Notes               string      `json:"notes,omitempty"`
}

type mciUpdateRequest struct {
	TriageCategory      MCITriageCategory `json:"triageCategory,omitempty"`
	AllocatedZone       string            `json:"allocatedZone,omitempty"`
	Notes               string            `json:"notes,omitempty"`
	PresentingComplaint string            `json:"presentingComplaint,omitempty"`
	STARTInputs         *STARTInputs      `json:"startInputs,omitempty"` // if provided, re-runs algorithm
}

type mciIdentifyRequest struct {
	NHI string `json:"nhi"`
}

// MCIHandler handles all /api/v1/emergency/incidents/{id}/mci routes.
type MCIHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// TagPatient handles POST /api/v1/emergency/incidents/{incidentId}/mci/patients.
func (h *MCIHandler) TagPatient(w http.ResponseWriter, r *http.Request) {
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

	incidentID := r.PathValue("id")

	var req mciTagRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.TagNumber < 1 || req.TagNumber > 999 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_TAG", Message: "tagNumber must be 1–999"})
		return
	}

	category, method := runTriageAlgorithm(req.STARTInputs)

	patient, err := h.insertMCIPatient(ctx, incidentID, tenantID.String(), req, category, method)
	if err != nil {
		h.logger.Error("tag MCI patient", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to tag patient"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "MCIPatient",
		ResourceID: patient.ID, TenantID: tenantID,
		Details:    map[string]any{"tag": req.TagNumber, "category": string(category)},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, patient)
}

// ListPatients handles GET /api/v1/emergency/incidents/{id}/mci/patients.
func (h *MCIHandler) ListPatients(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	incidentID := r.PathValue("id")
	catFilter := r.URL.Query().Get("category")

	patients, err := h.listMCIPatients(ctx, incidentID, tenantID.String(), catFilter)
	if err != nil {
		h.logger.Error("list MCI patients", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list patients"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"patients": patients, "total": len(patients)})
}

// UpdatePatient handles PUT /api/v1/emergency/incidents/{id}/mci/patients/{pid}.
func (h *MCIHandler) UpdatePatient(w http.ResponseWriter, r *http.Request) {
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

	incidentID := r.PathValue("id")
	pid := r.PathValue("pid")

	var req mciUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	// Re-run the triage algorithm if new vital signs are provided.
	if req.STARTInputs != nil {
		cat, _ := runTriageAlgorithm(*req.STARTInputs)
		if req.TriageCategory == "" {
			req.TriageCategory = cat
		}
	}

	patient, err := h.updateMCIPatient(ctx, pid, incidentID, tenantID.String(), req, principal.ID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "patient not found"})
			return
		}
		h.logger.Error("update MCI patient", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update patient"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "MCIPatient",
		ResourceID: pid, TenantID: tenantID,
		Details:    map[string]any{"category": string(patient.TriageCategory)},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, patient)
}

// IdentifyPatient handles POST /api/v1/emergency/incidents/{id}/mci/patients/{pid}/identify.
// Links a physical tag to a patient's NHI after initial triage.
func (h *MCIHandler) IdentifyPatient(w http.ResponseWriter, r *http.Request) {
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

	incidentID := r.PathValue("id")
	pid := r.PathValue("pid")

	var req mciIdentifyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.NHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "nhi is required"})
		return
	}

	nhiEnc, err := h.enc.Encrypt([]byte(req.NHI))
	if err != nil {
		h.logger.Error("encrypt NHI", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to process NHI"})
		return
	}
	masked := maskNHI(req.NHI)

	patient, err := h.linkNHI(ctx, pid, incidentID, tenantID.String(), nhiEnc, masked)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "patient not found"})
			return
		}
		h.logger.Error("identify MCI patient", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to identify patient"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "MCIPatient",
		ResourceID: pid, TenantID: tenantID,
		Details:    map[string]any{"action": "identify", "nhi_masked": masked},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, patient)
}

// Summary handles GET /api/v1/emergency/incidents/{id}/mci/summary.
func (h *MCIHandler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	incidentID := r.PathValue("id")
	summary, err := h.mciSummary(ctx, incidentID, tenantID.String())
	if err != nil {
		h.logger.Error("MCI summary", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SUMMARY_ERROR", Message: "failed to generate summary"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// ── Triage algorithm ─────────────────────────────────────────────────────────

// runTriageAlgorithm applies START (adults) or JumpSTART (paediatric) based on the inputs.
// Returns the triage category and the method used.
//
// START decision tree (Simple Triage And Rapid Treatment):
//  1. Ambulatory → MINOR
//  2. Breathing after airway repositioning? No → EXPECTANT
//  3. RR < 10 or > 30 → IMMEDIATE
//  4. No radial pulse, or cap refill > 2s → IMMEDIATE
//  5. Cannot follow commands → IMMEDIATE
//  6. Otherwise → DELAYED
//
// JumpSTART modifications (<8 yr / <25 kg):
//   - Apnoeic + peripheral pulse present → attempt 5 rescue breaths → if breathing IMMEDIATE else EXPECTANT
//   - AVPU < A → IMMEDIATE (uses AVPU instead of "follow commands")
func runTriageAlgorithm(in STARTInputs) (MCITriageCategory, MCITriageMethod) {
	method := MCIMethodSTART
	if in.IsPaediatric {
		method = MCIMethodJumpSTART
	}

	// Step 1: ambulatory → MINOR
	if in.Ambulatory {
		return MCIMinor, method
	}

	// Step 2: breathing
	if in.RespiratoryRate == 0 {
		if in.IsPaediatric && in.PaediatricPulse {
			// JumpSTART: apnoeic with pulse — attempt rescue breaths.
			// Clinician records outcome as a follow-up re-triage; initial is IMMEDIATE.
			return MCIImmediate, method
		}
		return MCIExpectant, method
	}

	// Step 3: respiratory rate out of normal range
	if in.RespiratoryRate < 10 || in.RespiratoryRate > 30 {
		return MCIImmediate, method
	}

	// Step 4: perfusion
	if !in.RadialPulse {
		return MCIImmediate, method
	}
	if in.CapRefillSecs != nil && *in.CapRefillSecs > 2.0 {
		return MCIImmediate, method
	}

	// Step 5: mental status
	if in.IsPaediatric {
		// JumpSTART uses AVPU; anything below Alert (V/P/U) → IMMEDIATE
		switch in.AVPU {
		case "v", "p", "u", "V", "P", "U":
			return MCIImmediate, method
		}
	} else {
		if !in.FollowsCommands {
			return MCIImmediate, method
		}
	}

	return MCIDelayed, method
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *MCIHandler) insertMCIPatient(ctx context.Context, incidentID, tenantID string, req mciTagRequest, cat MCITriageCategory, method MCITriageMethod) (MCIPatient, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO mci_patients
		   (incident_id, tenant_id, tag_number, triage_category, triage_method,
		    is_paediatric, age_years_approx, sex, presenting_complaint, allocated_zone, notes)
		 VALUES
		   (@incident_id, @tenant_id, @tag_number, @triage_category, @triage_method,
		    @is_paediatric, @age_years_approx, @sex, @presenting_complaint, @allocated_zone, @notes)
		 RETURNING id, incident_id, tenant_id, tag_number, nhi_masked,
		           triage_category, triage_method, is_paediatric, age_years_approx, sex,
		           presenting_complaint, allocated_zone, last_reassessed_at, reassessed_by,
		           notes, created_at, updated_at`,
		db.NamedArgs{
			"incident_id":          incidentID,
			"tenant_id":            tenantID,
			"tag_number":           req.TagNumber,
			"triage_category":      cat,
			"triage_method":        method,
			"is_paediatric":        req.STARTInputs.IsPaediatric,
			"age_years_approx":     req.AgeYearsApprox,
			"sex":                  req.Sex,
			"presenting_complaint": req.PresentingComplaint,
			"allocated_zone":       req.AllocatedZone,
			"notes":                req.Notes,
		},
	)
	return scanMCIPatientRow(row)
}

func (h *MCIHandler) listMCIPatients(ctx context.Context, incidentID, tenantID, catFilter string) ([]MCIPatient, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, incident_id, tenant_id, tag_number, nhi_masked,
		        triage_category, triage_method, is_paediatric, age_years_approx, sex,
		        presenting_complaint, allocated_zone, last_reassessed_at, reassessed_by,
		        notes, created_at, updated_at
		 FROM mci_patients
		 WHERE incident_id = @incident_id AND tenant_id = @tenant_id
		   AND (@cat_filter = '' OR triage_category::text = @cat_filter)
		 ORDER BY tag_number ASC`,
		db.NamedArgs{"incident_id": incidentID, "tenant_id": tenantID, "cat_filter": catFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query MCI patients: %w", err)
	}
	defer rows.Close()

	var results []MCIPatient
	for rows.Next() {
		p, err := scanMCIPatientRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (h *MCIHandler) updateMCIPatient(ctx context.Context, pid, incidentID, tenantID string, req mciUpdateRequest, reassessedBy string) (MCIPatient, error) {
	now := time.Now().UTC()
	row := h.pool.QueryRow(ctx,
		`UPDATE mci_patients
		 SET triage_category      = COALESCE(NULLIF(@triage_category, ''), triage_category),
		     allocated_zone       = COALESCE(NULLIF(@allocated_zone, ''), allocated_zone),
		     presenting_complaint = COALESCE(NULLIF(@presenting_complaint, ''), presenting_complaint),
		     notes                = COALESCE(NULLIF(@notes, ''), notes),
		     last_reassessed_at   = @reassessed_at,
		     reassessed_by        = @reassessed_by,
		     updated_at           = now()
		 WHERE id = @id AND incident_id = @incident_id AND tenant_id = @tenant_id
		 RETURNING id, incident_id, tenant_id, tag_number, nhi_masked,
		           triage_category, triage_method, is_paediatric, age_years_approx, sex,
		           presenting_complaint, allocated_zone, last_reassessed_at, reassessed_by,
		           notes, created_at, updated_at`,
		db.NamedArgs{
			"triage_category":      string(req.TriageCategory),
			"allocated_zone":       req.AllocatedZone,
			"presenting_complaint": req.PresentingComplaint,
			"notes":                req.Notes,
			"reassessed_at":        now,
			"reassessed_by":        reassessedBy,
			"id":                   pid,
			"incident_id":          incidentID,
			"tenant_id":            tenantID,
		},
	)
	p, err := scanMCIPatientRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return MCIPatient{}, errNotFound
		}
		return MCIPatient{}, fmt.Errorf("update MCI patient: %w", err)
	}
	return p, nil
}

func (h *MCIHandler) linkNHI(ctx context.Context, pid, incidentID, tenantID string, nhiEnc []byte, nhiMasked string) (MCIPatient, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE mci_patients
		 SET nhi_encrypted = @nhi_encrypted,
		     nhi_masked     = @nhi_masked,
		     updated_at     = now()
		 WHERE id = @id AND incident_id = @incident_id AND tenant_id = @tenant_id
		 RETURNING id, incident_id, tenant_id, tag_number, nhi_masked,
		           triage_category, triage_method, is_paediatric, age_years_approx, sex,
		           presenting_complaint, allocated_zone, last_reassessed_at, reassessed_by,
		           notes, created_at, updated_at`,
		db.NamedArgs{
			"nhi_encrypted": nhiEnc,
			"nhi_masked":    nhiMasked,
			"id":            pid,
			"incident_id":   incidentID,
			"tenant_id":     tenantID,
		},
	)
	p, err := scanMCIPatientRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return MCIPatient{}, errNotFound
		}
		return MCIPatient{}, fmt.Errorf("link NHI to MCI patient: %w", err)
	}
	return p, nil
}

func (h *MCIHandler) mciSummary(ctx context.Context, incidentID, tenantID string) (MCISummary, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT triage_category, COUNT(*),
		        COUNT(*) FILTER (WHERE nhi_encrypted IS NULL) AS unidentified
		 FROM mci_patients
		 WHERE incident_id = @incident_id AND tenant_id = @tenant_id
		 GROUP BY triage_category`,
		db.NamedArgs{"incident_id": incidentID, "tenant_id": tenantID},
	)
	if err != nil {
		return MCISummary{}, fmt.Errorf("mci summary query: %w", err)
	}
	defer rows.Close()

	summary := MCISummary{
		IncidentID:  incidentID,
		ByCategory:  make(map[string]int),
		GeneratedAt: time.Now().UTC(),
	}
	for rows.Next() {
		var cat string
		var count, unidentified int
		if err := rows.Scan(&cat, &count, &unidentified); err != nil {
			return MCISummary{}, fmt.Errorf("scan mci summary: %w", err)
		}
		summary.ByCategory[cat] = count
		summary.Total += count
		summary.Unidentified += unidentified
	}
	return summary, rows.Err()
}

// maskNHI returns the last 3 characters of an NHI for display purposes.
func maskNHI(nhi string) string {
	if len(nhi) <= 3 {
		return nhi
	}
	masked := make([]byte, len(nhi))
	for i := range masked {
		masked[i] = '*'
	}
	copy(masked[len(nhi)-3:], nhi[len(nhi)-3:])
	return string(masked)
}

func scanMCIPatientRow(row dbRow) (MCIPatient, error) {
	var p MCIPatient
	if err := row.Scan(
		&p.ID, &p.IncidentID, &p.TenantID, &p.TagNumber, &p.NHIMasked,
		&p.TriageCategory, &p.TriageMethod, &p.IsPaediatric, &p.AgeYearsApprox, &p.Sex,
		&p.PresentingComplaint, &p.AllocatedZone, &p.LastReassessedAt, &p.ReassessedBy,
		&p.Notes, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return MCIPatient{}, err
	}
	return p, nil
}
