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
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// PPELevel is the Occupational Safety standard protection level for CBRN response.
type PPELevel string

const (
	PPELevelA PPELevel = "level-a" // fully encapsulating suit, SCBA
	PPELevelB PPELevel = "level-b" // high-level respiratory protection
	PPELevelC PPELevel = "level-c" // air-purifying respirator
	PPELevelD PPELevel = "level-d" // standard work uniform
)

// CBRNDeconRecord is a patient-level decontamination record within a CBRN incident.
type CBRNDeconRecord struct {
	ID                     string     `json:"id"`
	MCIPatientID           string     `json:"mciPatientId"`
	IncidentID             string     `json:"incidentId"`
	TenantID               string     `json:"tenantId"`
	TagNumber              int        `json:"tagNumber"`
	AllocatedZone          string     `json:"allocatedZone,omitempty"`
	ContaminationSuspected bool       `json:"contaminationSuspected"`
	ContaminantType        string     `json:"contaminantType,omitempty"`
	DeconStartedAt         *time.Time `json:"deconStartedAt,omitempty"`
	DeconCompleteAt        *time.Time `json:"deconCompleteAt,omitempty"`
	DeconMethod            string     `json:"deconMethod,omitempty"`
	DeconBy                string     `json:"deconBy,omitempty"`
	PPELevelUsed           PPELevel   `json:"ppeLevelUsed,omitempty"`
	ClearedForTreatment    bool       `json:"clearedForTreatment"`
	Notes                  string     `json:"notes,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

// CBRNZone is a derived view of patients grouped by decontamination zone.
type CBRNZone struct {
	Zone                string `json:"zone"`
	TotalPatients       int    `json:"totalPatients"`
	DeconComplete       int    `json:"deconComplete"`
	ClearedForTreatment int    `json:"clearedForTreatment"`
}

type cbrnDeconStartRequest struct {
	ContaminantType string   `json:"contaminantType,omitempty"`
	DeconMethod     string   `json:"deconMethod,omitempty"`
	PPELevelUsed    PPELevel `json:"ppeLevelUsed,omitempty"`
	Notes           string   `json:"notes,omitempty"`
}

type cbrnDeconCompleteRequest struct {
	DeconBy             string `json:"deconBy,omitempty"`
	ClearedForTreatment bool   `json:"clearedForTreatment"`
	Notes               string `json:"notes,omitempty"`
}

// CBRNHandler handles all /api/v1/emergency/incidents/{id}/cbrn routes.
// All endpoints return 422 Unprocessable Entity if the incident is not of type 'cbrn'.
type CBRNHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListZones handles GET /api/v1/emergency/incidents/{id}/cbrn/zones.
// Returns patient counts aggregated by allocated_zone, joined with decon status.
func (h *CBRNHandler) ListZones(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	incidentID := r.PathValue("id")
	if err := h.requireCBRNIncident(ctx, incidentID, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "NOT_CBRN", Message: "incident is not a CBRN type"})
		return
	}

	zones, err := h.listZones(ctx, incidentID, tenantID.String())
	if err != nil {
		h.logger.Error("list CBRN zones", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list zones"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"zones": zones})
}

// ListPatients handles GET /api/v1/emergency/incidents/{id}/cbrn/patients.
// Returns MCI patients joined with their decon log for this incident.
func (h *CBRNHandler) ListPatients(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	incidentID := r.PathValue("id")
	if err := h.requireCBRNIncident(ctx, incidentID, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "NOT_CBRN", Message: "incident is not a CBRN type"})
		return
	}

	patients, err := h.listCBRNPatients(ctx, incidentID, tenantID.String())
	if err != nil {
		h.logger.Error("list CBRN patients", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list patients"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"patients": patients, "total": len(patients)})
}

// StartDecon handles POST /api/v1/emergency/incidents/{id}/cbrn/patients/{pid}/decon-start.
func (h *CBRNHandler) StartDecon(w http.ResponseWriter, r *http.Request) {
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

	if err := h.requireCBRNIncident(ctx, incidentID, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "NOT_CBRN", Message: "incident is not a CBRN type"})
		return
	}

	var req cbrnDeconStartRequest
	_ = decodeJSON(r, &req)

	record, err := h.upsertDeconStart(ctx, pid, incidentID, tenantID.String(), req)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "patient not found"})
			return
		}
		h.logger.Error("start decon", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to start decontamination"})
		return
	}

	// Move patient to warm zone while in decontamination.
	_ = h.updatePatientZone(ctx, pid, incidentID, tenantID.String(), "warm")

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "CBRNDeconRecord",
		ResourceID: record.ID, TenantID: tenantID,
		Details:    map[string]any{"action": "decon_start", "mci_patient": pid},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, record)
}

// CompleteDecon handles POST /api/v1/emergency/incidents/{id}/cbrn/patients/{pid}/decon-complete.
func (h *CBRNHandler) CompleteDecon(w http.ResponseWriter, r *http.Request) {
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

	if err := h.requireCBRNIncident(ctx, incidentID, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "incident not found"})
			return
		}
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "NOT_CBRN", Message: "incident is not a CBRN type"})
		return
	}

	var req cbrnDeconCompleteRequest
	_ = decodeJSON(r, &req)
	if req.DeconBy == "" {
		req.DeconBy = principal.ID
	}

	record, err := h.updateDeconComplete(ctx, pid, incidentID, tenantID.String(), req)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "decon record not found"})
			return
		}
		h.logger.Error("complete decon", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to complete decontamination"})
		return
	}

	// Move patient to cold (treatment) zone once cleared.
	if req.ClearedForTreatment {
		_ = h.updatePatientZone(ctx, pid, incidentID, tenantID.String(), "cold")
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "CBRNDeconRecord",
		ResourceID: record.ID, TenantID: tenantID,
		Details:    map[string]any{"action": "decon_complete", "cleared": req.ClearedForTreatment},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, record)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

// requireCBRNIncident returns errNotFound if the incident does not exist,
// or a sentinel non-nil error if it exists but is not of type 'cbrn'.
var errNotCBRN = errors.New("not a cbrn incident")

func (h *CBRNHandler) requireCBRNIncident(ctx context.Context, incidentID, tenantID string) error {
	var incType string
	err := h.pool.QueryRow(ctx,
		`SELECT type FROM emergency_incidents WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": incidentID, "tenant_id": tenantID},
	).Scan(&incType)
	if err != nil {
		if db.IsNoRows(err) {
			return errNotFound
		}
		return fmt.Errorf("check incident type: %w", err)
	}
	if incType != string(IncidentTypeCBRN) {
		return errNotCBRN
	}
	return nil
}

func (h *CBRNHandler) listZones(ctx context.Context, incidentID, tenantID string) ([]CBRNZone, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT
		     mp.allocated_zone,
		     COUNT(DISTINCT mp.id) AS total_patients,
		     COUNT(DISTINCT d.mci_patient_id) FILTER (WHERE d.decon_complete_at IS NOT NULL) AS decon_complete,
		     COUNT(DISTINCT d.mci_patient_id) FILTER (WHERE d.cleared_for_treatment = true)  AS cleared
		 FROM mci_patients mp
		 LEFT JOIN cbrn_decon_log d ON d.mci_patient_id = mp.id AND d.incident_id = mp.incident_id
		 WHERE mp.incident_id = @incident_id AND mp.tenant_id = @tenant_id
		 GROUP BY mp.allocated_zone
		 ORDER BY mp.allocated_zone`,
		db.NamedArgs{"incident_id": incidentID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("list CBRN zones: %w", err)
	}
	defer rows.Close()

	var results []CBRNZone
	for rows.Next() {
		var z CBRNZone
		if err := rows.Scan(&z.Zone, &z.TotalPatients, &z.DeconComplete, &z.ClearedForTreatment); err != nil {
			return nil, fmt.Errorf("scan zone: %w", err)
		}
		results = append(results, z)
	}
	return results, rows.Err()
}

func (h *CBRNHandler) listCBRNPatients(ctx context.Context, incidentID, tenantID string) ([]CBRNDeconRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT d.id, d.mci_patient_id, d.incident_id, d.tenant_id,
		        mp.tag_number, mp.allocated_zone,
		        d.contamination_suspected, d.contaminant_type,
		        d.decon_started_at, d.decon_complete_at, d.decon_method,
		        d.decon_by, d.ppe_level_used, d.cleared_for_treatment, d.notes,
		        d.created_at, d.updated_at
		 FROM cbrn_decon_log d
		 JOIN mci_patients mp ON mp.id = d.mci_patient_id
		 WHERE d.incident_id = @incident_id AND d.tenant_id = @tenant_id
		 ORDER BY mp.tag_number ASC`,
		db.NamedArgs{"incident_id": incidentID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("list CBRN patients: %w", err)
	}
	defer rows.Close()

	var results []CBRNDeconRecord
	for rows.Next() {
		rec, err := scanCBRNDeconRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

// upsertDeconStart creates a new decon record or updates an existing one for this patient.
func (h *CBRNHandler) upsertDeconStart(ctx context.Context, mciPatientID, incidentID, tenantID string, req cbrnDeconStartRequest) (CBRNDeconRecord, error) {
	now := time.Now().UTC()
	row := h.pool.QueryRow(ctx,
		`INSERT INTO cbrn_decon_log
		   (mci_patient_id, incident_id, tenant_id, contamination_suspected,
		    contaminant_type, decon_started_at, decon_method, ppe_level_used, notes)
		 VALUES
		   (@mci_patient_id, @incident_id, @tenant_id, true,
		    @contaminant_type, @decon_started_at, @decon_method, @ppe_level_used, @notes)
		 ON CONFLICT (mci_patient_id) DO UPDATE
		   SET contaminant_type = COALESCE(EXCLUDED.contaminant_type, cbrn_decon_log.contaminant_type),
		       decon_started_at = COALESCE(cbrn_decon_log.decon_started_at, EXCLUDED.decon_started_at),
		       decon_method     = COALESCE(EXCLUDED.decon_method, cbrn_decon_log.decon_method),
		       ppe_level_used   = COALESCE(EXCLUDED.ppe_level_used, cbrn_decon_log.ppe_level_used),
		       notes            = COALESCE(EXCLUDED.notes, cbrn_decon_log.notes),
		       updated_at       = now()
		 RETURNING id, mci_patient_id, incident_id, tenant_id,
		           0::int AS tag_number, ''::text AS allocated_zone,
		           contamination_suspected, contaminant_type,
		           decon_started_at, decon_complete_at, decon_method,
		           decon_by, ppe_level_used, cleared_for_treatment, notes,
		           created_at, updated_at`,
		db.NamedArgs{
			"mci_patient_id":   mciPatientID,
			"incident_id":      incidentID,
			"tenant_id":        tenantID,
			"contaminant_type": req.ContaminantType,
			"decon_started_at": now,
			"decon_method":     req.DeconMethod,
			"ppe_level_used":   string(req.PPELevelUsed),
			"notes":            req.Notes,
		},
	)
	rec, err := scanCBRNDeconRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return CBRNDeconRecord{}, errNotFound
		}
		return CBRNDeconRecord{}, fmt.Errorf("upsert decon start: %w", err)
	}
	return rec, nil
}

func (h *CBRNHandler) updateDeconComplete(ctx context.Context, mciPatientID, incidentID, tenantID string, req cbrnDeconCompleteRequest) (CBRNDeconRecord, error) {
	now := time.Now().UTC()
	row := h.pool.QueryRow(ctx,
		`UPDATE cbrn_decon_log
		 SET decon_complete_at    = @decon_complete_at,
		     decon_by             = @decon_by,
		     cleared_for_treatment = @cleared,
		     notes                = COALESCE(NULLIF(@notes, ''), notes),
		     updated_at           = now()
		 WHERE mci_patient_id = @mci_patient_id AND incident_id = @incident_id AND tenant_id = @tenant_id
		 RETURNING id, mci_patient_id, incident_id, tenant_id,
		           0::int AS tag_number, ''::text AS allocated_zone,
		           contamination_suspected, contaminant_type,
		           decon_started_at, decon_complete_at, decon_method,
		           decon_by, ppe_level_used, cleared_for_treatment, notes,
		           created_at, updated_at`,
		db.NamedArgs{
			"decon_complete_at": now,
			"decon_by":          req.DeconBy,
			"cleared":           req.ClearedForTreatment,
			"notes":             req.Notes,
			"mci_patient_id":    mciPatientID,
			"incident_id":       incidentID,
			"tenant_id":         tenantID,
		},
	)
	rec, err := scanCBRNDeconRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return CBRNDeconRecord{}, errNotFound
		}
		return CBRNDeconRecord{}, fmt.Errorf("update decon complete: %w", err)
	}
	return rec, nil
}

func (h *CBRNHandler) updatePatientZone(ctx context.Context, mciPatientID, incidentID, tenantID, zone string) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE mci_patients SET allocated_zone = @zone, updated_at = now()
		 WHERE id = @id AND incident_id = @incident_id AND tenant_id = @tenant_id`,
		db.NamedArgs{"zone": zone, "id": mciPatientID, "incident_id": incidentID, "tenant_id": tenantID},
	)
	return err
}

func scanCBRNDeconRow(row dbRow) (CBRNDeconRecord, error) {
	var rec CBRNDeconRecord
	var ppeLevelStr string
	if err := row.Scan(
		&rec.ID, &rec.MCIPatientID, &rec.IncidentID, &rec.TenantID,
		&rec.TagNumber, &rec.AllocatedZone,
		&rec.ContaminationSuspected, &rec.ContaminantType,
		&rec.DeconStartedAt, &rec.DeconCompleteAt, &rec.DeconMethod,
		&rec.DeconBy, &ppeLevelStr, &rec.ClearedForTreatment, &rec.Notes,
		&rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return CBRNDeconRecord{}, err
	}
	rec.PPELevelUsed = PPELevel(ppeLevelStr)
	return rec, nil
}
