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
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// TriageCategory implements the NZ Emergency Triage Scale (5-level).
type TriageCategory int

const (
	// TriageCat1 — Immediate: life-threatening. Target seen within 0 minutes.
	TriageCat1 TriageCategory = 1
	// TriageCat2 — Emergent: potentially life-threatening. Target seen within 10 minutes.
	TriageCat2 TriageCategory = 2
	// TriageCat3 — Urgent: potentially serious. Target seen within 30 minutes.
	TriageCat3 TriageCategory = 3
	// TriageCat4 — Semi-urgent: less serious. Target seen within 60 minutes.
	TriageCat4 TriageCategory = 4
	// TriageCat5 — Non-urgent. Target seen within 120 minutes.
	TriageCat5 TriageCategory = 5
)

// EDPresentationStatus tracks the patient journey through the ED.
type EDPresentationStatus string

const (
	EDStatusArrived    EDPresentationStatus = "arrived"
	EDStatusTriaged    EDPresentationStatus = "triaged"
	EDStatusAssigned   EDPresentationStatus = "assigned"     // bed and clinician assigned
	EDStatusBeingSeen  EDPresentationStatus = "being-seen"
	EDStatusWaiting    EDPresentationStatus = "waiting"      // awaiting results / review
	EDStatusDisposed   EDPresentationStatus = "disposed"
)

// EDDisposition records the outcome of the ED visit.
type EDDisposition string

const (
	EDDispositionAdmit      EDDisposition = "admitted"
	EDDispositionTransfer   EDDisposition = "transferred"
	EDDispositionDischarge  EDDisposition = "discharged"
	EDDispositionDNW        EDDisposition = "did-not-wait"
	EDDispositionDeceased   EDDisposition = "deceased"
)

// EDPresentation represents a single patient presentation to the emergency department.
type EDPresentation struct {
	ID               string               `json:"id"`
	PatientID        string               `json:"patientId,omitempty"`
	PatientNHI       string               `json:"patientNhi,omitempty"`
	TriageCategory   TriageCategory       `json:"triageCategory"`
	Status           EDPresentationStatus `json:"status"`
	ChiefComplaint   string               `json:"chiefComplaint"`
	TriageNotes      string               `json:"triageNotes,omitempty"`
	TriageNurseHPI   string               `json:"triageNurseHpi,omitempty"`
	AssignedBedID    string               `json:"assignedBedId,omitempty"`
	AssignedClinicianHPI string           `json:"assignedClinicianHpi,omitempty"`
	Disposition      EDDisposition        `json:"disposition,omitempty"`
	DispositionNotes string               `json:"dispositionNotes,omitempty"`
	AdmissionID      string               `json:"admissionId,omitempty"` // set if admitted
	ArrivalMode      string               `json:"arrivalMode,omitempty"` // ambulance, walk-in, air
	TenantID         string               `json:"tenantId"`
	ArrivedAt        time.Time            `json:"arrivedAt"`
	TriagedAt        *time.Time           `json:"triagedAt,omitempty"`
	DisposedAt       *time.Time           `json:"disposedAt,omitempty"`
	CreatedAt        time.Time            `json:"createdAt"`
	UpdatedAt        time.Time            `json:"updatedAt"`
}

// EDQueueStats is a snapshot of the current ED waiting room.
type EDQueueStats struct {
	TotalWaiting   int            `json:"totalWaiting"`
	ByCategory     map[int]int    `json:"byCategory"`
	OldestWaitMins int            `json:"oldestWaitMins"`
	GeneratedAt    time.Time      `json:"generatedAt"`
}

type edCreateRequest struct {
	PatientID      string         `json:"patientId,omitempty"`
	PatientNHI     string         `json:"patientNhi,omitempty"`
	TriageCategory TriageCategory `json:"triageCategory"`
	ChiefComplaint string         `json:"chiefComplaint"`
	TriageNotes    string         `json:"triageNotes,omitempty"`
	TriageNurseHPI string         `json:"triageNurseHpi,omitempty"`
	ArrivalMode    string         `json:"arrivalMode,omitempty"`
}

type edUpdateRequest struct {
	TriageCategory TriageCategory `json:"triageCategory,omitempty"`
	TriageNotes    string         `json:"triageNotes,omitempty"`
}

type edAssignRequest struct {
	BedID         string `json:"bedId,omitempty"`
	ClinicianHPI  string `json:"clinicianHpi"`
}

type edDisposeRequest struct {
	Disposition      EDDisposition `json:"disposition"`
	DispositionNotes string        `json:"dispositionNotes,omitempty"`
	AdmissionID      string        `json:"admissionId,omitempty"`
}

// EDHandler handles all /api/v1/ed routes.
type EDHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/ed/triage.
func (h *EDHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	statusFilter := r.URL.Query().Get("status")
	catFilter := r.URL.Query().Get("category")

	presentations, err := h.listPresentations(ctx, tenantID.String(), statusFilter, catFilter)
	if err != nil {
		h.logger.Error("list ED presentations", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list presentations"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"presentations": presentations, "total": len(presentations)})
}

// Create handles POST /api/v1/ed/triage — registers a new ED arrival and triage.
func (h *EDHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req edCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ChiefComplaint == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_COMPLAINT", Message: "chiefComplaint is required"})
		return
	}
	if req.TriageCategory < 1 || req.TriageCategory > 5 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_TRIAGE_CATEGORY", Message: "triageCategory must be 1–5"})
		return
	}

	pres, err := h.insertPresentation(ctx, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert ED presentation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create ED presentation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "EDPresentation",
		ResourceID: pres.ID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, pres)
}

// Get handles GET /api/v1/ed/triage/{id}.
func (h *EDHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	pres, err := h.getPresentationByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "presentation not found"})
			return
		}
		h.logger.Error("get ED presentation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve presentation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "EDPresentation",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, pres)
}

// Update handles PUT /api/v1/ed/triage/{id} — update triage category or notes.
func (h *EDHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getPresentationByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "presentation not found"})
			return
		}
		h.logger.Error("get ED presentation for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve presentation"})
		return
	}
	if existing.Status == EDStatusDisposed {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_DISPOSED", Message: "cannot update a disposed presentation"})
		return
	}

	var req edUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.TriageCategory >= 1 && req.TriageCategory <= 5 {
		existing.TriageCategory = req.TriageCategory
	}
	if req.TriageNotes != "" {
		existing.TriageNotes = req.TriageNotes
	}

	updated, err := h.updatePresentation(ctx, existing)
	if err != nil {
		h.logger.Error("update ED presentation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update presentation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "EDPresentation",
		ResourceID: id, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}

// Assign handles POST /api/v1/ed/triage/{id}/assign — assign bed and clinician.
func (h *EDHandler) Assign(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getPresentationByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "presentation not found"})
			return
		}
		h.logger.Error("get ED presentation for assign", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve presentation"})
		return
	}
	if existing.Status == EDStatusDisposed {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_DISPOSED", Message: "cannot assign to a disposed presentation"})
		return
	}

	var req edAssignRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ClinicianHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLINICIAN", Message: "clinicianHpi is required"})
		return
	}

	existing.AssignedBedID = req.BedID
	existing.AssignedClinicianHPI = req.ClinicianHPI
	existing.Status = EDStatusAssigned

	updated, err := h.updatePresentation(ctx, existing)
	if err != nil {
		h.logger.Error("assign ED presentation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ASSIGN_ERROR", Message: "failed to assign presentation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "EDPresentation",
		ResourceID: id, TenantID: tenantID,
		Details:    map[string]any{"action": "assign", "clinician": req.ClinicianHPI},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}

// Dispose handles POST /api/v1/ed/triage/{id}/dispose.
func (h *EDHandler) Dispose(w http.ResponseWriter, r *http.Request) {
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
	existing, err := h.getPresentationByID(ctx, id, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "presentation not found"})
			return
		}
		h.logger.Error("get ED presentation for dispose", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve presentation"})
		return
	}
	if existing.Status == EDStatusDisposed {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_DISPOSED", Message: "presentation is already disposed"})
		return
	}

	var req edDisposeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Disposition == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DISPOSITION", Message: "disposition is required"})
		return
	}

	now := time.Now().UTC()
	existing.Status = EDStatusDisposed
	existing.Disposition = req.Disposition
	existing.DispositionNotes = req.DispositionNotes
	existing.AdmissionID = req.AdmissionID
	existing.DisposedAt = &now

	disposed, err := h.updatePresentation(ctx, existing)
	if err != nil {
		h.logger.Error("dispose ED presentation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DISPOSE_ERROR", Message: "failed to dispose presentation"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "EDPresentation",
		ResourceID: id, TenantID: tenantID,
		Details:    map[string]any{"action": "dispose", "disposition": string(req.Disposition)},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, disposed)
}

// Queue handles GET /api/v1/ed/queue — active waiting-room view.
func (h *EDHandler) Queue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	queue, err := h.listPresentations(ctx, tenantID.String(), string(EDStatusArrived), "")
	if err != nil {
		h.logger.Error("ed queue", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "QUEUE_ERROR", Message: "failed to retrieve ED queue"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"queue": queue, "total": len(queue)})
}

// Stats handles GET /api/v1/ed/stats.
func (h *EDHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	stats, err := h.queueStats(ctx, tenantID.String())
	if err != nil {
		h.logger.Error("ed stats", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "STATS_ERROR", Message: "failed to retrieve ED stats"})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *EDHandler) listPresentations(ctx context.Context, tenantID, statusFilter, catFilter string) ([]EDPresentation, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, triage_category, status, chief_complaint,
		        triage_notes, triage_nurse_hpi, assigned_bed_id, assigned_clinician_hpi,
		        disposition, disposition_notes, admission_id, arrival_mode,
		        tenant_id, arrived_at, triaged_at, disposed_at, created_at, updated_at
		 FROM ed_presentations
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		   AND (@cat_filter    = '' OR triage_category::text = @cat_filter)
		 ORDER BY arrived_at ASC`,
		db.NamedArgs{"tenant_id": tenantID, "status_filter": statusFilter, "cat_filter": catFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query ED presentations: %w", err)
	}
	defer rows.Close()

	var results []EDPresentation
	for rows.Next() {
		p, err := scanEDRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (h *EDHandler) getPresentationByID(ctx context.Context, id, tenantID string) (EDPresentation, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, triage_category, status, chief_complaint,
		        triage_notes, triage_nurse_hpi, assigned_bed_id, assigned_clinician_hpi,
		        disposition, disposition_notes, admission_id, arrival_mode,
		        tenant_id, arrived_at, triaged_at, disposed_at, created_at, updated_at
		 FROM ed_presentations
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	p, err := scanEDRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return EDPresentation{}, errNotFound
		}
		return EDPresentation{}, fmt.Errorf("get ED presentation: %w", err)
	}
	return p, nil
}

func (h *EDHandler) insertPresentation(ctx context.Context, req edCreateRequest, tenantID string) (EDPresentation, error) {
	now := time.Now().UTC()
	row := h.pool.QueryRow(ctx,
		`INSERT INTO ed_presentations
		   (patient_id, patient_nhi, triage_category, status, chief_complaint,
		    triage_notes, triage_nurse_hpi, arrival_mode, tenant_id, arrived_at, triaged_at)
		 VALUES
		   (@patient_id, @patient_nhi, @triage_category, @status, @chief_complaint,
		    @triage_notes, @triage_nurse_hpi, @arrival_mode, @tenant_id, @arrived_at, @triaged_at)
		 RETURNING id, patient_id, patient_nhi, triage_category, status, chief_complaint,
		           triage_notes, triage_nurse_hpi, assigned_bed_id, assigned_clinician_hpi,
		           disposition, disposition_notes, admission_id, arrival_mode,
		           tenant_id, arrived_at, triaged_at, disposed_at, created_at, updated_at`,
		db.NamedArgs{
			"patient_id":       req.PatientID,
			"patient_nhi":      req.PatientNHI,
			"triage_category":  req.TriageCategory,
			"status":           EDStatusTriaged,
			"chief_complaint":  req.ChiefComplaint,
			"triage_notes":     req.TriageNotes,
			"triage_nurse_hpi": req.TriageNurseHPI,
			"arrival_mode":     req.ArrivalMode,
			"tenant_id":        tenantID,
			"arrived_at":       now,
			"triaged_at":       now,
		},
	)
	return scanEDRow(row)
}

func (h *EDHandler) updatePresentation(ctx context.Context, p EDPresentation) (EDPresentation, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE ed_presentations
		 SET triage_category         = @triage_category,
		     status                  = @status,
		     triage_notes            = @triage_notes,
		     assigned_bed_id         = @assigned_bed_id,
		     assigned_clinician_hpi  = @assigned_clinician_hpi,
		     disposition             = @disposition,
		     disposition_notes       = @disposition_notes,
		     admission_id            = @admission_id,
		     disposed_at             = @disposed_at,
		     updated_at              = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, triage_category, status, chief_complaint,
		           triage_notes, triage_nurse_hpi, assigned_bed_id, assigned_clinician_hpi,
		           disposition, disposition_notes, admission_id, arrival_mode,
		           tenant_id, arrived_at, triaged_at, disposed_at, created_at, updated_at`,
		db.NamedArgs{
			"triage_category":        p.TriageCategory,
			"status":                 p.Status,
			"triage_notes":           p.TriageNotes,
			"assigned_bed_id":        p.AssignedBedID,
			"assigned_clinician_hpi": p.AssignedClinicianHPI,
			"disposition":            p.Disposition,
			"disposition_notes":      p.DispositionNotes,
			"admission_id":           p.AdmissionID,
			"disposed_at":            p.DisposedAt,
			"id":                     p.ID,
			"tenant_id":              p.TenantID,
		},
	)
	updated, err := scanEDRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return EDPresentation{}, errNotFound
		}
		return EDPresentation{}, fmt.Errorf("update ED presentation: %w", err)
	}
	return updated, nil
}

func (h *EDHandler) queueStats(ctx context.Context, tenantID string) (EDQueueStats, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT triage_category, COUNT(*),
		        EXTRACT(EPOCH FROM (now() - MIN(arrived_at))) / 60 AS oldest_wait_mins
		 FROM ed_presentations
		 WHERE tenant_id = @tenant_id AND status != 'disposed'
		 GROUP BY triage_category`,
		db.NamedArgs{"tenant_id": tenantID},
	)
	if err != nil {
		return EDQueueStats{}, fmt.Errorf("ed stats query: %w", err)
	}
	defer rows.Close()

	stats := EDQueueStats{ByCategory: make(map[int]int), GeneratedAt: time.Now().UTC()}
	for rows.Next() {
		var cat, count int
		var oldestMins float64
		if err := rows.Scan(&cat, &count, &oldestMins); err != nil {
			return EDQueueStats{}, fmt.Errorf("scan ed stats: %w", err)
		}
		stats.TotalWaiting += count
		stats.ByCategory[cat] = count
		if int(oldestMins) > stats.OldestWaitMins {
			stats.OldestWaitMins = int(oldestMins)
		}
	}
	return stats, rows.Err()
}

func scanEDRow(row dbRow) (EDPresentation, error) {
	var p EDPresentation
	if err := row.Scan(
		&p.ID, &p.PatientID, &p.PatientNHI, &p.TriageCategory, &p.Status, &p.ChiefComplaint,
		&p.TriageNotes, &p.TriageNurseHPI, &p.AssignedBedID, &p.AssignedClinicianHPI,
		&p.Disposition, &p.DispositionNotes, &p.AdmissionID, &p.ArrivalMode,
		&p.TenantID, &p.ArrivedAt, &p.TriagedAt, &p.DisposedAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return EDPresentation{}, err
	}
	return p, nil
}
