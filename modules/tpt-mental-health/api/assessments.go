package api

import (
	"context"
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
	"github.com/jackc/pgx/v5/pgxpool"
)

// AssessmentTool enumerates validated mental health screening instruments.
type AssessmentTool string

const (
	ToolPHQ9    AssessmentTool = "PHQ-9"    // Patient Health Questionnaire — depression
	ToolGAD7    AssessmentTool = "GAD-7"    // Generalised Anxiety Disorder scale
	ToolAUDITC  AssessmentTool = "AUDIT-C"  // Alcohol Use Disorders Identification Test
	ToolHoNOS   AssessmentTool = "HoNOS"    // Health of the Nation Outcome Scales
	ToolHoNOSCA AssessmentTool = "HoNOSCA" // HoNOS Children & Adolescents
	ToolHoNOS65 AssessmentTool = "HoNOS65+" // HoNOS for older adults
	ToolCANSAS  AssessmentTool = "CANSAS"   // Camberwell Assessment of Need Short Appraisal
	ToolBASIS32 AssessmentTool = "BASIS-32" // Behaviour and Symptom Identification Scale
	ToolDASS21  AssessmentTool = "DASS-21"  // Depression Anxiety Stress Scales
	ToolMINI    AssessmentTool = "MINI"     // Mini International Neuropsychiatric Interview
)

// Severity labels aligned with PHQ-9 and GAD-7 scoring bands.
type Severity string

const (
	SeverityNone             Severity = ""
	SeverityMinimal          Severity = "minimal"
	SeverityMild             Severity = "mild"
	SeverityModerate         Severity = "moderate"
	SeverityModeratelySevere Severity = "moderately-severe"
	SeveritySevere           Severity = "severe"
)

// AssessmentScores holds structured item-level and total scoring data.
type AssessmentScores struct {
	Total int              `json:"total"`
	Items []AssessmentItem `json:"items,omitempty"`
}

// AssessmentItem holds a single question index and its score.
type AssessmentItem struct {
	Question int `json:"q"`
	Score    int `json:"score"`
}

// Assessment represents a completed mental health assessment record.
type Assessment struct {
	ID              string            `json:"id"`
	PatientID       string            `json:"patientId"`
	PatientNHI      string            `json:"patientNhi"`
	PractitionerHPI string            `json:"practitionerHpi"`
	EpisodeID       string            `json:"episodeId,omitempty"`
	TenantID        string            `json:"tenantId"`
	Tool            AssessmentTool    `json:"tool"`
	Scores          AssessmentScores  `json:"scores"`
	Severity        Severity          `json:"severity"`
	ClinicalNotes   string            `json:"clinicalNotes,omitempty"` // decrypted on read
	ExtraSensitive  bool              `json:"extraSensitive"`
	AssessedAt      time.Time         `json:"assessedAt"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

// assessmentCreateRequest is the body for POST /api/v1/assessments.
type assessmentCreateRequest struct {
	PatientID       string           `json:"patientId"`
	PatientNHI      string           `json:"patientNhi"`
	PractitionerHPI string           `json:"practitionerHpi"`
	EpisodeID       string           `json:"episodeId,omitempty"`
	Tool            AssessmentTool   `json:"tool"`
	Scores          AssessmentScores `json:"scores"`
	Severity        Severity         `json:"severity,omitempty"`
	ClinicalNotes   string           `json:"clinicalNotes,omitempty"`
	AssessedAt      time.Time        `json:"assessedAt"`
}

// assessmentUpdateRequest is the body for PUT /api/v1/assessments/{id}.
type assessmentUpdateRequest struct {
	Scores        *AssessmentScores `json:"scores,omitempty"`
	Severity      Severity          `json:"severity,omitempty"`
	ClinicalNotes string            `json:"clinicalNotes,omitempty"`
}

// AssessmentsHandler handles all /api/v1/assessments routes.
type AssessmentsHandler struct {
	pool       *pgxpool.Pool
	enc        *encryption.Encryptor
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/assessments.
// Query params: patient (internal ID), episode, tool.
func (h *AssessmentsHandler) List(w http.ResponseWriter, r *http.Request) {
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
	patientFilter := q.Get("patient")
	episodeFilter := q.Get("episode")
	toolFilter := q.Get("tool")

	records, err := h.listAssessments(ctx, tenantID, patientFilter, episodeFilter, toolFilter)
	if err != nil {
		h.logger.Error("list assessments", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list assessments"})
		return
	}

	responses := make([]Assessment, 0, len(records))
	for _, rec := range records {
		a, err := h.decryptAssessment(rec)
		if err != nil {
			h.logger.Error("decrypt assessment", slog.Any("error", err), slog.String("id", rec.ID))
			continue
		}
		// HIPC: check extra-sensitive access per patient before including.
		if accessErr := checkMHAccess(ctx, h.pool, tenantID, a.PatientNHI, principal); accessErr != nil {
			continue
		}
		responses = append(responses, a)
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MentalHealthAssessment",
		ResourceID:   "list",
		Details:      map[string]any{"patient": patientFilter, "tool": toolFilter},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"assessments": responses,
		"total":       len(responses),
	})
}

// Get handles GET /api/v1/assessments/{id}.
func (h *AssessmentsHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "assessment ID is required"})
		return
	}

	rec, err := h.getByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "assessment not found"})
			return
		}
		h.logger.Error("get assessment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve assessment"})
		return
	}

	a, err := h.decryptAssessment(rec)
	if err != nil {
		h.logger.Error("decrypt assessment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt assessment"})
		return
	}

	if accessErr := checkMHAccess(ctx, h.pool, tenantID, a.PatientNHI, principal); accessErr != nil {
		writeJSON(w, http.StatusForbidden, apiError{Code: "ACCESS_DENIED", Message: accessErr.Error()})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "MentalHealthAssessment",
		ResourceID:   id,
		PatientNHI:   a.PatientNHI,
	})

	writeJSON(w, http.StatusOK, a)
}

// Create handles POST /api/v1/assessments.
// Validates the practitioner's APC before recording the assessment.
func (h *AssessmentsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req assessmentCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patientId or patientNhi is required"})
		return
	}
	if req.PractitionerHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PRACTITIONER", Message: "practitionerHpi is required"})
		return
	}
	if !validTool(req.Tool) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_TOOL", Message: fmt.Sprintf("unknown assessment tool %q", req.Tool)})
		return
	}

	// Validate APC for the assessing practitioner.
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

	rec, err := h.insertAssessment(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert assessment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to save assessment"})
		return
	}

	a, err := h.decryptAssessment(rec)
	if err != nil {
		h.logger.Error("decrypt assessment after insert", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt assessment"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "MentalHealthAssessment",
		ResourceID:   rec.ID,
		PatientNHI:   req.PatientNHI,
		Details:      map[string]any{"tool": string(req.Tool), "severity": string(req.Severity)},
	})

	writeJSON(w, http.StatusCreated, a)
}

// Update handles PUT /api/v1/assessments/{id}.
// Allows correcting scores, severity, and clinical notes on an existing assessment.
func (h *AssessmentsHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "assessment ID is required"})
		return
	}

	var req assessmentUpdateRequest
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
		h.logger.Error("get assessment for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve assessment"})
		return
	}

	if req.Scores != nil {
		rec.Scores = *req.Scores
	}
	if req.Severity != "" {
		rec.Severity = req.Severity
	}

	notesEnc := rec.NotesEncrypted
	if req.ClinicalNotes != "" {
		notesEnc, err = h.enc.Encrypt([]byte(req.ClinicalNotes))
		if err != nil {
			h.logger.Error("encrypt clinical notes", slog.Any("error", err))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt notes"})
			return
		}
	}

	updated, err := h.updateAssessment(ctx, rec, notesEnc, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "assessment not found"})
			return
		}
		h.logger.Error("update assessment", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update assessment"})
		return
	}

	a, err := h.decryptAssessment(updated)
	if err != nil {
		h.logger.Error("decrypt assessment after update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt assessment"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "MentalHealthAssessment",
		ResourceID:   id,
	})

	writeJSON(w, http.StatusOK, a)
}

// ---------------------------------------------------------------------------
// Internal record type (raw DB row)
// ---------------------------------------------------------------------------

type assessmentRecord struct {
	ID              string
	PatientID       string
	PatientNHI      string
	PractitionerHPI string
	EpisodeID       string
	TenantID        string
	Tool            AssessmentTool
	Scores          AssessmentScores
	Severity        Severity
	NotesEncrypted  []byte
	ExtraSensitive  bool
	AssessedAt      time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ---------------------------------------------------------------------------
// Database helpers
// ---------------------------------------------------------------------------

func (h *AssessmentsHandler) listAssessments(
	ctx context.Context,
	tenantID uuid.UUID,
	patientFilter, episodeFilter, toolFilter string,
) ([]assessmentRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, practitioner_hpi, episode_id,
		        tenant_id, tool, scores, severity, clinical_notes,
		        extra_sensitive, assessed_at, created_at, updated_at
		 FROM mh_assessments
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR episode_id::text = $3)
		   AND ($4 = '' OR tool = $4)
		 ORDER BY assessed_at DESC
		 LIMIT 200`,
		tenantID, patientFilter, episodeFilter, toolFilter,
	)
	if err != nil {
		return nil, fmt.Errorf("query assessments: %w", err)
	}
	defer rows.Close()

	var results []assessmentRecord
	for rows.Next() {
		rec, err := scanAssessment(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (h *AssessmentsHandler) getByID(ctx context.Context, id string, tenantID uuid.UUID) (assessmentRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, practitioner_hpi, episode_id,
		        tenant_id, tool, scores, severity, clinical_notes,
		        extra_sensitive, assessed_at, created_at, updated_at
		 FROM mh_assessments
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanAssessment(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return assessmentRecord{}, errNotFound
		}
		return assessmentRecord{}, fmt.Errorf("get assessment by id: %w", err)
	}
	return rec, nil
}

func (h *AssessmentsHandler) insertAssessment(ctx context.Context, req assessmentCreateRequest, tenantID uuid.UUID) (assessmentRecord, error) {
	notesEnc, err := h.enc.Encrypt([]byte(req.ClinicalNotes))
	if err != nil {
		return assessmentRecord{}, fmt.Errorf("encrypt clinical notes: %w", err)
	}

	assessedAt := req.AssessedAt
	if assessedAt.IsZero() {
		assessedAt = time.Now().UTC()
	}

	var episodeID *string
	if req.EpisodeID != "" {
		episodeID = &req.EpisodeID
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO mh_assessments
		   (patient_id, patient_nhi, practitioner_hpi, episode_id, tenant_id,
		    tool, scores, severity, clinical_notes, extra_sensitive, assessed_at)
		 VALUES
		   ($1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, $10)
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, episode_id,
		           tenant_id, tool, scores, severity, clinical_notes,
		           extra_sensitive, assessed_at, created_at, updated_at`,
		req.PatientID, req.PatientNHI, req.PractitionerHPI, episodeID, tenantID,
		string(req.Tool), req.Scores, string(req.Severity), notesEnc, assessedAt,
	)
	return scanAssessmentRow(row)
}

func (h *AssessmentsHandler) updateAssessment(ctx context.Context, rec assessmentRecord, notesEnc []byte, tenantID uuid.UUID) (assessmentRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE mh_assessments
		 SET scores         = $1,
		     severity       = $2,
		     clinical_notes = $3,
		     updated_at     = now()
		 WHERE id = $4 AND tenant_id = $5
		 RETURNING id, patient_id, patient_nhi, practitioner_hpi, episode_id,
		           tenant_id, tool, scores, severity, clinical_notes,
		           extra_sensitive, assessed_at, created_at, updated_at`,
		rec.Scores, string(rec.Severity), notesEnc, rec.ID, tenantID,
	)
	updated, err := scanAssessmentRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return assessmentRecord{}, errNotFound
		}
		return assessmentRecord{}, fmt.Errorf("update assessment: %w", err)
	}
	return updated, nil
}

func (h *AssessmentsHandler) decryptAssessment(rec assessmentRecord) (Assessment, error) {
	var notes string
	if len(rec.NotesEncrypted) > 0 {
		plain, err := h.enc.Decrypt(rec.NotesEncrypted)
		if err != nil {
			return Assessment{}, fmt.Errorf("decrypt clinical notes: %w", err)
		}
		notes = string(plain)
	}
	return Assessment{
		ID:              rec.ID,
		PatientID:       rec.PatientID,
		PatientNHI:      rec.PatientNHI,
		PractitionerHPI: rec.PractitionerHPI,
		EpisodeID:       rec.EpisodeID,
		TenantID:        rec.TenantID,
		Tool:            rec.Tool,
		Scores:          rec.Scores,
		Severity:        rec.Severity,
		ClinicalNotes:   notes,
		ExtraSensitive:  rec.ExtraSensitive,
		AssessedAt:      rec.AssessedAt,
		CreatedAt:       rec.CreatedAt,
		UpdatedAt:       rec.UpdatedAt,
	}, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAssessment(s rowScanner) (assessmentRecord, error) {
	return scanAssessmentRow(s)
}

func scanAssessmentRow(s rowScanner) (assessmentRecord, error) {
	var rec assessmentRecord
	var tool, severity string
	if err := s.Scan(
		&rec.ID, &rec.PatientID, &rec.PatientNHI, &rec.PractitionerHPI, &rec.EpisodeID,
		&rec.TenantID, &tool, &rec.Scores, &severity, &rec.NotesEncrypted,
		&rec.ExtraSensitive, &rec.AssessedAt, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return assessmentRecord{}, err
	}
	rec.Tool = AssessmentTool(tool)
	rec.Severity = Severity(severity)
	return rec, nil
}

// validTool reports whether t is a recognised assessment instrument.
func validTool(t AssessmentTool) bool {
	switch t {
	case ToolPHQ9, ToolGAD7, ToolAUDITC, ToolHoNOS, ToolHoNOSCA,
		ToolHoNOS65, ToolCANSAS, ToolBASIS32, ToolDASS21, ToolMINI:
		return true
	}
	return false
}
