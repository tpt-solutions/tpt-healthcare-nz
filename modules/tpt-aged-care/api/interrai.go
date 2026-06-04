package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// InterRAIInstrument identifies the specific interRAI assessment tool used.
// NZ uses the interRAI suite licensed from interRAI.org under the MoH agreement.
type InterRAIInstrument string

const (
	// InstrumentHC is the Home Care assessment — for community-based recipients.
	InstrumentHC InterRAIInstrument = "HC"
	// InstrumentLTCF is the Long-Term Care Facility assessment — for residential care.
	InstrumentLTCF InterRAIInstrument = "LTCF"
	// InstrumentCA is the Contact Assessment — a brief screening tool.
	InstrumentCA InterRAIInstrument = "CA"
	// InstrumentCHA is the Community Health Assessment.
	InstrumentCHA InterRAIInstrument = "CHA"
	// InstrumentPAC is the Post-Acute Care assessment.
	InstrumentPAC InterRAIInstrument = "PAC"
)

// AssessmentStatus tracks where an interRAI assessment is in its lifecycle.
type AssessmentStatus string

const (
	StatusDraft     AssessmentStatus = "draft"
	StatusSubmitted AssessmentStatus = "submitted" // locked, sent to MoH interRAI repository
	StatusAmended   AssessmentStatus = "amended"
)

// CAPDomain represents a Clinical Assessment Protocol triggered by the assessment.
// CAPs are the evidence-based decision-support outputs of an interRAI assessment.
type CAPDomain string

const (
	CAPDelirium            CAPDomain = "delirium"
	CAPCognitiveLoss       CAPDomain = "cognitive-loss"
	CAPVisualFunction      CAPDomain = "visual-function"
	CAPCommunication       CAPDomain = "communication"
	CAPADLFunctionalRehab  CAPDomain = "adl-functional-rehab"
	CAPUrinaryIncontinence CAPDomain = "urinary-incontinence"
	CAPPsychosocial        CAPDomain = "psychosocial"
	CAPMood                CAPDomain = "mood"
	CAPBehaviouralSymptoms CAPDomain = "behavioural-symptoms"
	CAPActivities          CAPDomain = "activities"
	CAPFalls               CAPDomain = "falls"
	CAPNutritional         CAPDomain = "nutritional"
	CAPFeeding             CAPDomain = "feeding"
	CAPPressureUlcer       CAPDomain = "pressure-ulcer"
	CAPPain                CAPDomain = "pain"
	CAPMedications         CAPDomain = "medications"
)

// CAP is a Clinical Assessment Protocol result: whether it was triggered and
// what action has been decided.
type CAP struct {
	Domain    CAPDomain `json:"domain"`
	Triggered bool      `json:"triggered"`
	// Action is one of "care-plan", "monitoring", "no-action".
	Action string `json:"action,omitempty"`
	Notes  string `json:"notes,omitempty"`
}

// InterRAISections holds the structured section scores for any instrument.
// Keys match interRAI section identifiers (e.g., "B" for Cognitive Patterns).
// Values are free-form JSON — the shape varies by instrument and NZ localisation.
type InterRAISections map[string]json.RawMessage

// InterRAIScales holds computed scale scores derived from the sections.
type InterRAIScales struct {
	// ADL Self Performance Hierarchy scale (0–6).
	ADLHierarchy *int `json:"adlHierarchy,omitempty"`
	// Cognitive Performance Scale (0–6).
	CPS *int `json:"cps,omitempty"`
	// Depression Rating Scale (0–14).
	DRS *int `json:"drs,omitempty"`
	// Pain Assessment Scale (0–3).
	Pain *int `json:"pain,omitempty"`
	// Changes in Health, End-stage disease and Signs and Symptoms scale (0–18).
	CHESS *int `json:"chess,omitempty"`
	// Index of Social Engagement (0–6) — LTCF only.
	ISE *int `json:"ise,omitempty"`
	// Aggressive Behaviour Scale (0–12).
	ABS *int `json:"abs,omitempty"`
}

// InterRAIAssessment represents a single interRAI assessment record.
type InterRAIAssessment struct {
	ID              string             `json:"id"`
	PatientID       string             `json:"patientId"`
	PatientNHI      string             `json:"patientNhi"`
	PractitionerHPI string             `json:"practitionerHpi"`
	TenantID        string             `json:"tenantId"`
	Instrument      InterRAIInstrument `json:"instrument"`
	Status          AssessmentStatus   `json:"status"`
	Sections        InterRAISections   `json:"sections"`
	Scales          InterRAIScales     `json:"scales"`
	CAPs            []CAP              `json:"caps"`
	// ClinicalNotes is AES-256-GCM encrypted at rest; decrypted on read.
	ClinicalNotes string    `json:"clinicalNotes,omitempty"`
	AssessedAt    time.Time `json:"assessedAt"`
	SubmittedAt   time.Time `json:"submittedAt,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type interRAICreateRequest struct {
	PatientID       string             `json:"patientId"`
	PatientNHI      string             `json:"patientNhi"`
	PractitionerHPI string             `json:"practitionerHpi"`
	Instrument      InterRAIInstrument `json:"instrument"`
	Sections        InterRAISections   `json:"sections"`
	Scales          InterRAIScales     `json:"scales"`
	ClinicalNotes   string             `json:"clinicalNotes,omitempty"`
	AssessedAt      time.Time          `json:"assessedAt"`
}

type interRAIUpdateRequest struct {
	Sections      InterRAISections  `json:"sections,omitempty"`
	Scales        *InterRAIScales   `json:"scales,omitempty"`
	CAPs          []CAP             `json:"caps,omitempty"`
	ClinicalNotes string            `json:"clinicalNotes,omitempty"`
}

// interRAIRecord is the raw DB row.
type interRAIRecord struct {
	ID              string
	PatientID       string
	PatientNHI      string
	PractitionerHPI string
	TenantID        string
	Instrument      InterRAIInstrument
	Status          AssessmentStatus
	Sections        InterRAISections
	Scales          InterRAIScales
	CAPs            []CAP
	NotesEncrypted  []byte
	AssessedAt      time.Time
	SubmittedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// InterRAIHandler handles all /api/v1/interrai/* routes.
type InterRAIHandler struct {
	pool       dbPool
	enc        *encryption.Encryptor
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/interrai/assessments.
// Query params: patient, instrument, status.
func (h *InterRAIHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	q := r.URL.Query()
	records, err := h.listAssessments(ctx, tenantID, q.Get("patient"), q.Get("instrument"), q.Get("status"))
	if err != nil {
		h.logger.Error("list interRAI assessments", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list assessments"})
		return
	}

	results := make([]InterRAIAssessment, 0, len(records))
	for _, rec := range records {
		a, err := h.decrypt(rec)
		if err != nil {
			h.logger.Error("decrypt interRAI", slog.Any("error", err), slog.String("id", rec.ID))
			continue
		}
		results = append(results, a)
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "InterRAIAssessment",
		ResourceID:   "list",
	})

	writeJSON(w, http.StatusOK, map[string]any{"assessments": results, "total": len(results)})
}

// Get handles GET /api/v1/interrai/assessments/{id}.
func (h *InterRAIHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	rec, err := h.getByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "assessment not found"})
			return
		}
		h.logger.Error("get interRAI", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve assessment"})
		return
	}

	a, err := h.decrypt(rec)
	if err != nil {
		h.logger.Error("decrypt interRAI", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt assessment"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "InterRAIAssessment",
		ResourceID:   id,
		PatientNHI:   a.PatientNHI,
	})

	writeJSON(w, http.StatusOK, a)
}

// Create handles POST /api/v1/interrai/assessments.
func (h *InterRAIHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req interRAICreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" && req.PatientID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patientId or patientNhi is required"})
		return
	}
	if req.PractitionerHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PRACTITIONER", Message: "practitionerHpi is required"})
		return
	}
	if !validInstrument(req.Instrument) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_INSTRUMENT", Message: fmt.Sprintf("unknown interRAI instrument %q", req.Instrument)})
		return
	}

	apcValid, err := h.hpiClient.ValidateAPC(ctx, req.PractitionerHPI)
	if err != nil {
		h.logger.Error("HPI APC check", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not verify practitioner APC"})
		return
	}
	if !apcValid {
		writeJSON(w, http.StatusForbidden, apiError{Code: "INVALID_APC", Message: "practitioner does not hold a current Annual Practising Certificate"})
		return
	}

	rec, err := h.insert(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert interRAI", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to save assessment"})
		return
	}

	a, err := h.decrypt(rec)
	if err != nil {
		h.logger.Error("decrypt after insert", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt assessment"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "InterRAIAssessment",
		ResourceID:   rec.ID,
		PatientNHI:   req.PatientNHI,
		Details:      map[string]any{"instrument": string(req.Instrument)},
	})

	writeJSON(w, http.StatusCreated, a)
}

// Update handles PUT /api/v1/interrai/assessments/{id}.
// Only draft assessments may be updated.
func (h *InterRAIHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	var req interRAIUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	rec, err := h.getByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "assessment not found"})
			return
		}
		h.logger.Error("get interRAI for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve assessment"})
		return
	}
	if rec.Status == StatusSubmitted {
		writeJSON(w, http.StatusConflict, apiError{Code: "IMMUTABLE", Message: "submitted assessments cannot be edited; use the amend workflow"})
		return
	}

	if req.Sections != nil {
		rec.Sections = req.Sections
	}
	if req.Scales != nil {
		rec.Scales = *req.Scales
	}
	if len(req.CAPs) > 0 {
		rec.CAPs = req.CAPs
	}

	notesEnc := rec.NotesEncrypted
	if req.ClinicalNotes != "" {
		notesEnc, err = h.enc.Encrypt([]byte(req.ClinicalNotes))
		if err != nil {
			h.logger.Error("encrypt notes", slog.Any("error", err))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt notes"})
			return
		}
	}

	updated, err := h.update(ctx, rec, notesEnc, tenantID)
	if err != nil {
		h.logger.Error("update interRAI", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update assessment"})
		return
	}

	a, err := h.decrypt(updated)
	if err != nil {
		h.logger.Error("decrypt after update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt assessment"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "InterRAIAssessment",
		ResourceID:   id,
	})

	writeJSON(w, http.StatusOK, a)
}

// Submit handles POST /api/v1/interrai/assessments/{id}/submit.
// Locks the assessment and records it as submitted to the MoH interRAI repository.
func (h *InterRAIHandler) Submit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	rec, err := h.getByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "assessment not found"})
			return
		}
		h.logger.Error("get interRAI for submit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve assessment"})
		return
	}
	if rec.Status == StatusSubmitted {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_SUBMITTED", Message: "assessment is already submitted"})
		return
	}

	now := time.Now().UTC()
	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_interrai_assessments
		 SET status = 'submitted', submitted_at = $1, updated_at = $1
		 WHERE id = $2 AND tenant_id = $3
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, tenant_id,
		           instrument, status, sections, scales, caps, clinical_notes,
		           assessed_at, submitted_at, created_at, updated_at`,
		now, id, tenantID,
	)
	updated, err := scanInterRAI(row)
	if err != nil {
		h.logger.Error("submit interRAI", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SUBMIT_ERROR", Message: "failed to submit assessment"})
		return
	}

	a, err := h.decrypt(updated)
	if err != nil {
		h.logger.Error("decrypt after submit", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt assessment"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "submit",
		ResourceType: "InterRAIAssessment",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
	})

	writeJSON(w, http.StatusOK, a)
}

// GetCAPs handles GET /api/v1/interrai/assessments/{id}/caps.
func (h *InterRAIHandler) GetCAPs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	rec, err := h.getByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "assessment not found"})
			return
		}
		h.logger.Error("get interRAI for CAPs", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve assessment"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "InterRAICAPs",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
	})

	writeJSON(w, http.StatusOK, map[string]any{"caps": rec.CAPs, "assessmentId": id})
}

// ---------------------------------------------------------------------------
// DB helpers
// ---------------------------------------------------------------------------

type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Ping(ctx context.Context) error
	Close()
}

func (h *InterRAIHandler) listAssessments(ctx context.Context, tenantID uuid.UUID, patientFilter, instrumentFilter, statusFilter string) ([]interRAIRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, practitioner_hpi, tenant_id,
		        instrument, status, sections, scales, caps, clinical_notes,
		        assessed_at, submitted_at, created_at, updated_at
		 FROM aged_care_interrai_assessments
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR instrument = $3)
		   AND ($4 = '' OR status = $4)
		 ORDER BY assessed_at DESC
		 LIMIT 200`,
		tenantID, patientFilter, instrumentFilter, statusFilter,
	)
	if err != nil {
		return nil, fmt.Errorf("query interRAI assessments: %w", err)
	}
	defer rows.Close()

	var results []interRAIRecord
	for rows.Next() {
		rec, err := scanInterRAI(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (h *InterRAIHandler) getByID(ctx context.Context, id string, tenantID uuid.UUID) (interRAIRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, practitioner_hpi, tenant_id,
		        instrument, status, sections, scales, caps, clinical_notes,
		        assessed_at, submitted_at, created_at, updated_at
		 FROM aged_care_interrai_assessments
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanInterRAI(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return interRAIRecord{}, errNotFound
		}
		return interRAIRecord{}, fmt.Errorf("get interRAI by id: %w", err)
	}
	return rec, nil
}

func (h *InterRAIHandler) insert(ctx context.Context, req interRAICreateRequest, tenantID uuid.UUID) (interRAIRecord, error) {
	notesEnc, err := h.enc.Encrypt([]byte(req.ClinicalNotes))
	if err != nil {
		return interRAIRecord{}, fmt.Errorf("encrypt clinical notes: %w", err)
	}
	assessedAt := req.AssessedAt
	if assessedAt.IsZero() {
		assessedAt = time.Now().UTC()
	}
	row := h.pool.QueryRow(ctx,
		`INSERT INTO aged_care_interrai_assessments
		   (patient_id, patient_nhi, practitioner_hpi, tenant_id,
		    instrument, status, sections, scales, caps, clinical_notes, assessed_at)
		 VALUES ($1, $2, $3, $4, $5, 'draft', $6, $7, '[]'::jsonb, $8, $9)
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, tenant_id,
		           instrument, status, sections, scales, caps, clinical_notes,
		           assessed_at, submitted_at, created_at, updated_at`,
		req.PatientID, req.PatientNHI, req.PractitionerHPI, tenantID,
		string(req.Instrument), req.Sections, req.Scales, notesEnc, assessedAt,
	)
	return scanInterRAI(row)
}

func (h *InterRAIHandler) update(ctx context.Context, rec interRAIRecord, notesEnc []byte, tenantID uuid.UUID) (interRAIRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_interrai_assessments
		 SET sections = $1, scales = $2, caps = $3, clinical_notes = $4, updated_at = now()
		 WHERE id = $5 AND tenant_id = $6
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, tenant_id,
		           instrument, status, sections, scales, caps, clinical_notes,
		           assessed_at, submitted_at, created_at, updated_at`,
		rec.Sections, rec.Scales, rec.CAPs, notesEnc, rec.ID, tenantID,
	)
	updated, err := scanInterRAI(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return interRAIRecord{}, errNotFound
		}
		return interRAIRecord{}, fmt.Errorf("update interRAI: %w", err)
	}
	return updated, nil
}

func (h *InterRAIHandler) decrypt(rec interRAIRecord) (InterRAIAssessment, error) {
	var notes string
	if len(rec.NotesEncrypted) > 0 {
		plain, err := h.enc.Decrypt(rec.NotesEncrypted)
		if err != nil {
			return InterRAIAssessment{}, fmt.Errorf("decrypt clinical notes: %w", err)
		}
		notes = string(plain)
	}
	a := InterRAIAssessment{
		ID:              rec.ID,
		PatientID:       rec.PatientID,
		PatientNHI:      rec.PatientNHI,
		PractitionerHPI: rec.PractitionerHPI,
		TenantID:        rec.TenantID,
		Instrument:      rec.Instrument,
		Status:          rec.Status,
		Sections:        rec.Sections,
		Scales:          rec.Scales,
		CAPs:            rec.CAPs,
		ClinicalNotes:   notes,
		AssessedAt:      rec.AssessedAt,
		CreatedAt:       rec.CreatedAt,
		UpdatedAt:       rec.UpdatedAt,
	}
	if rec.SubmittedAt != nil {
		a.SubmittedAt = *rec.SubmittedAt
	}
	return a, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanInterRAI(s rowScanner) (interRAIRecord, error) {
	var rec interRAIRecord
	var instrument, status string
	if err := s.Scan(
		&rec.ID, &rec.PatientID, &rec.PatientNHI, &rec.PractitionerHPI, &rec.TenantID,
		&instrument, &status, &rec.Sections, &rec.Scales, &rec.CAPs, &rec.NotesEncrypted,
		&rec.AssessedAt, &rec.SubmittedAt, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return interRAIRecord{}, err
	}
	rec.Instrument = InterRAIInstrument(instrument)
	rec.Status = AssessmentStatus(status)
	return rec, nil
}

func validInstrument(i InterRAIInstrument) bool {
	switch i {
	case InstrumentHC, InstrumentLTCF, InstrumentCA, InstrumentCHA, InstrumentPAC:
		return true
	}
	return false
}
