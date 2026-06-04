package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/subscription"
	"github.com/google/uuid"
)

// StudiesHandler handles FHIR ImagingStudy CRUD and DICOMweb proxy routes.
type StudiesHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	orthanc    *OrthancClient
	subEngine  *subscription.Engine
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ImagingStudy is the domain model for a DICOM imaging study.
type ImagingStudy struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenantId"`
	PatientNHI       string     `json:"patientNhi"`
	StudyInstanceUID string     `json:"studyInstanceUid"`
	AccessionNumber  string     `json:"accessionNumber,omitempty"`
	Modality         string     `json:"modality"`
	BodyPart         string     `json:"bodyPart,omitempty"`
	StudyDate        *time.Time `json:"studyDate,omitempty"`
	Description      string     `json:"description,omitempty"`
	ReferringHPI     string     `json:"referringHpi,omitempty"`
	PerformingHPI    string     `json:"performingHpi,omitempty"`
	Status           string     `json:"status"`
	NumSeries        int        `json:"numSeries"`
	NumInstances     int        `json:"numInstances"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type imagingStudyCreateRequest struct {
	PatientNHI       string `json:"patientNhi"`
	StudyInstanceUID string `json:"studyInstanceUid"`
	AccessionNumber  string `json:"accessionNumber,omitempty"`
	Modality         string `json:"modality"`
	BodyPart         string `json:"bodyPart,omitempty"`
	Description      string `json:"description,omitempty"`
	ReferringHPI     string `json:"referringHpi,omitempty"`
}

type imagingStudyUpdateRequest struct {
	Status        *string `json:"status,omitempty"`
	Description   *string `json:"description,omitempty"`
	PerformingHPI *string `json:"performingHpi,omitempty"`
	NumSeries     *int    `json:"numSeries,omitempty"`
	NumInstances  *int    `json:"numInstances,omitempty"`
}

// List handles GET /api/v1/imaging-studies.
func (h *StudiesHandler) List(w http.ResponseWriter, r *http.Request) {
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

	q := r.URL.Query()
	studies, err := h.listStudies(ctx, tenantID, q.Get("patientNhi"), q.Get("modality"), q.Get("status"))
	if err != nil {
		h.logger.Error("list imaging studies", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list studies"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "ImagingStudy",
		ResourceID:   "list",
		TenantID:     tenantID,
	})

	writeJSON(w, http.StatusOK, map[string]any{"studies": studies, "total": len(studies)})
}

// Create handles POST /api/v1/imaging-studies.
func (h *StudiesHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req imagingStudyCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "patientNhi is required"})
		return
	}
	if req.StudyInstanceUID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_UID", Message: "studyInstanceUid is required"})
		return
	}
	if req.Modality == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_MODALITY", Message: "modality is required"})
		return
	}

	study, err := h.insertStudy(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert imaging study", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create study"})
		return
	}

	if pubErr := h.publishStudyEvent(ctx, study); pubErr != nil {
		h.logger.Error("publish imaging study event", slog.Any("error", pubErr))
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "ImagingStudy",
		ResourceID:   study.ID,
		TenantID:     tenantID,
		Metadata:     map[string]string{"modality": study.Modality, "patient_nhi": study.PatientNHI},
	})

	writeJSON(w, http.StatusCreated, study)
}

// Get handles GET /api/v1/imaging-studies/{id}.
func (h *StudiesHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	study, err := h.getStudyByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "study not found"})
			return
		}
		h.logger.Error("get imaging study", slog.Any("error", err), slog.String("id", id))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve study"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "ImagingStudy",
		ResourceID:   id,
		TenantID:     tenantID,
	})

	writeJSON(w, http.StatusOK, study)
}

// Update handles PUT /api/v1/imaging-studies/{id}.
func (h *StudiesHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var req imagingStudyUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	existing, err := h.getStudyByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "study not found"})
			return
		}
		h.logger.Error("get study for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve study"})
		return
	}

	if req.Status != nil {
		existing.Status = *req.Status
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.PerformingHPI != nil {
		existing.PerformingHPI = *req.PerformingHPI
	}
	if req.NumSeries != nil {
		existing.NumSeries = *req.NumSeries
	}
	if req.NumInstances != nil {
		existing.NumInstances = *req.NumInstances
	}

	updated, err := h.updateStudy(ctx, existing)
	if err != nil {
		h.logger.Error("update imaging study", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update study"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "ImagingStudy",
		ResourceID:   id,
		TenantID:     tenantID,
	})

	writeJSON(w, http.StatusOK, updated)
}

// ---------------------------------------------------------------------------
// DICOMweb proxy handlers
// ---------------------------------------------------------------------------

// QIDOStudies handles GET /api/v1/dicom-web/studies — QIDO-RS study search.
func (h *StudiesHandler) QIDOStudies(w http.ResponseWriter, r *http.Request) {
	h.proxyQIDO(w, r, "/studies")
}

// QIDOSeries handles GET /api/v1/dicom-web/studies/{study}/series.
func (h *StudiesHandler) QIDOSeries(w http.ResponseWriter, r *http.Request) {
	h.proxyQIDO(w, r, "/studies/"+r.PathValue("study")+"/series")
}

// QIDOInstances handles GET .../series/{series}/instances.
func (h *StudiesHandler) QIDOInstances(w http.ResponseWriter, r *http.Request) {
	h.proxyQIDO(w, r, "/studies/"+r.PathValue("study")+"/series/"+r.PathValue("series")+"/instances")
}

// WADOStudy handles GET /api/v1/dicom-web/studies/{study} — WADO-RS study retrieve.
func (h *StudiesHandler) WADOStudy(w http.ResponseWriter, r *http.Request) {
	h.proxyWADO(w, r, "/studies/"+r.PathValue("study"))
}

// WADOInstance handles GET .../instances/{instance} — WADO-RS instance retrieve.
func (h *StudiesHandler) WADOInstance(w http.ResponseWriter, r *http.Request) {
	study, series, instance := r.PathValue("study"), r.PathValue("series"), r.PathValue("instance")
	h.proxyWADO(w, r, "/studies/"+study+"/series/"+series+"/instances/"+instance)
}

// WADOFrame handles GET .../frames/{frame} — WADO-RS frame retrieve.
func (h *StudiesHandler) WADOFrame(w http.ResponseWriter, r *http.Request) {
	study, series, instance, frame := r.PathValue("study"), r.PathValue("series"), r.PathValue("instance"), r.PathValue("frame")
	h.proxyWADO(w, r, "/studies/"+study+"/series/"+series+"/instances/"+instance+"/frames/"+frame)
}

// STOWStudy handles POST /api/v1/dicom-web/studies[/{study}] — STOW-RS store.
func (h *StudiesHandler) STOWStudy(w http.ResponseWriter, r *http.Request) {
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

	path := "/studies"
	if study := r.PathValue("study"); study != "" {
		path += "/" + study
	}

	respBody, err := h.orthanc.ProxySTOW(ctx, path, r.Header.Get("Content-Type"), r.Body)
	if err != nil {
		h.logger.Error("STOW-RS proxy", slog.Any("error", err), slog.String("tenant", tenantID))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "ORTHANC_ERROR", Message: "DICOM store failed"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "ImagingStudy",
		ResourceID:   "stow",
		TenantID:     tenantID,
		Metadata:     map[string]string{"operation": "STOW-RS"},
	})

	w.Header().Set("Content-Type", "application/dicom+json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}

// proxyQIDO is the shared QIDO-RS proxy helper.
func (h *StudiesHandler) proxyQIDO(w http.ResponseWriter, r *http.Request, path string) {
	ctx := r.Context()
	if _, ok := middleware.TenantFromContext(ctx); !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	body, contentType, err := h.orthanc.ProxyQIDO(ctx, path, r.URL.Query())
	if err != nil {
		h.logger.Error("QIDO-RS proxy", slog.Any("error", err), slog.String("path", path))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "ORTHANC_ERROR", Message: "DICOM query failed"})
		return
	}

	if contentType == "" {
		contentType = "application/dicom+json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// proxyWADO streams a WADO-RS response from Orthanc to the client.
func (h *StudiesHandler) proxyWADO(w http.ResponseWriter, r *http.Request, path string) {
	ctx := r.Context()
	if _, ok := middleware.TenantFromContext(ctx); !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	body, contentType, err := h.orthanc.ProxyWADO(ctx, path, r.Header.Get("Accept"))
	if err != nil {
		h.logger.Error("WADO-RS proxy", slog.Any("error", err), slog.String("path", path))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "ORTHANC_ERROR", Message: "DICOM retrieve failed"})
		return
	}
	defer body.Close()

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, body); err != nil {
		h.logger.Error("WADO-RS stream", slog.Any("error", err))
	}
}

// publishStudyEvent notifies subscribers that a new ImagingStudy is available.
func (h *StudiesHandler) publishStudyEvent(ctx context.Context, study ImagingStudy) error {
	payload, err := json.Marshal(map[string]string{
		"resourceType":     "ImagingStudy",
		"id":               study.ID,
		"studyInstanceUid": study.StudyInstanceUID,
		"modality":         study.Modality,
		"patientNhi":       study.PatientNHI,
		"tenantID":         study.TenantID,
	})
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	return h.subEngine.Publish(ctx, ImagingStudyTopic, "ImagingStudy", study.ID, payload)
}

// ---------------------------------------------------------------------------
// Database operations
// ---------------------------------------------------------------------------

func (h *StudiesHandler) listStudies(ctx context.Context, tenantID, patientNHI, modality, status string) ([]ImagingStudy, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, patient_nhi, study_instance_uid,
		        accession_number, modality, body_part, study_date, description,
		        referring_hpi, performing_hpi, status, num_series, num_instances,
		        created_at, updated_at
		 FROM imaging_studies
		 WHERE tenant_id   = @tenant_id
		   AND (@patient_nhi = '' OR patient_nhi = @patient_nhi)
		   AND (@modality    = '' OR modality    = @modality)
		   AND (@status      = '' OR status      = @status)
		 ORDER BY study_date DESC NULLS LAST, created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":  tenantID,
			"patient_nhi": patientNHI,
			"modality":   modality,
			"status":     status,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query imaging studies: %w", err)
	}
	defer rows.Close()

	var results []ImagingStudy
	for rows.Next() {
		var s ImagingStudy
		if err := rows.Scan(
			&s.ID, &s.TenantID, &s.PatientNHI, &s.StudyInstanceUID,
			&s.AccessionNumber, &s.Modality, &s.BodyPart, &s.StudyDate, &s.Description,
			&s.ReferringHPI, &s.PerformingHPI, &s.Status, &s.NumSeries, &s.NumInstances,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan imaging study: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

func (h *StudiesHandler) insertStudy(ctx context.Context, req imagingStudyCreateRequest, tenantID string) (ImagingStudy, error) {
	// Build minimal FHIR ImagingStudy JSON for encrypted blob storage.
	fhirResource := map[string]any{
		"resourceType": "ImagingStudy",
		"id":           uuid.New().String(),
		"status":       "registered",
		"identifier": []map[string]string{{
			"system": "urn:dicom:uid",
			"value":  "urn:oid:" + req.StudyInstanceUID,
		}},
		"subject": map[string]string{
			"identifier": map[string]string{
				"system": "https://standards.digital.health.nz/ns/nhi-id",
				"value":  req.PatientNHI,
			},
		},
	}
	fhirJSON, _ := json.Marshal(fhirResource)

	var encFHIR []byte
	if h.enc != nil {
		var err error
		encFHIR, err = h.enc.Encrypt(fhirJSON)
		if err != nil {
			return ImagingStudy{}, fmt.Errorf("encrypt fhir resource: %w", err)
		}
	} else {
		encFHIR = fhirJSON
	}

	var s ImagingStudy
	err := h.pool.QueryRow(ctx,
		`INSERT INTO imaging_studies
		   (tenant_id, patient_nhi, study_instance_uid, accession_number,
		    modality, body_part, description, referring_hpi, status, fhir_resource)
		 VALUES
		   (@tenant_id, @patient_nhi, @study_instance_uid, @accession_number,
		    @modality, @body_part, @description, @referring_hpi, 'registered', @fhir_resource)
		 RETURNING id, tenant_id, patient_nhi, study_instance_uid,
		           accession_number, modality, body_part, study_date, description,
		           referring_hpi, performing_hpi, status, num_series, num_instances,
		           created_at, updated_at`,
		db.NamedArgs{
			"tenant_id":          tenantID,
			"patient_nhi":        req.PatientNHI,
			"study_instance_uid": req.StudyInstanceUID,
			"accession_number":   req.AccessionNumber,
			"modality":           req.Modality,
			"body_part":          req.BodyPart,
			"description":        req.Description,
			"referring_hpi":      req.ReferringHPI,
			"fhir_resource":      encFHIR,
		},
	).Scan(
		&s.ID, &s.TenantID, &s.PatientNHI, &s.StudyInstanceUID,
		&s.AccessionNumber, &s.Modality, &s.BodyPart, &s.StudyDate, &s.Description,
		&s.ReferringHPI, &s.PerformingHPI, &s.Status, &s.NumSeries, &s.NumInstances,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return ImagingStudy{}, fmt.Errorf("insert imaging study: %w", err)
	}
	return s, nil
}

func (h *StudiesHandler) getStudyByID(ctx context.Context, id, tenantID string) (ImagingStudy, error) {
	var s ImagingStudy
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, patient_nhi, study_instance_uid,
		        accession_number, modality, body_part, study_date, description,
		        referring_hpi, performing_hpi, status, num_series, num_instances,
		        created_at, updated_at
		 FROM imaging_studies
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&s.ID, &s.TenantID, &s.PatientNHI, &s.StudyInstanceUID,
		&s.AccessionNumber, &s.Modality, &s.BodyPart, &s.StudyDate, &s.Description,
		&s.ReferringHPI, &s.PerformingHPI, &s.Status, &s.NumSeries, &s.NumInstances,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return ImagingStudy{}, errNotFound
		}
		return ImagingStudy{}, fmt.Errorf("get imaging study: %w", err)
	}
	return s, nil
}

func (h *StudiesHandler) updateStudy(ctx context.Context, s ImagingStudy) (ImagingStudy, error) {
	var updated ImagingStudy
	err := h.pool.QueryRow(ctx,
		`UPDATE imaging_studies
		 SET status         = @status,
		     description    = @description,
		     performing_hpi = @performing_hpi,
		     num_series     = @num_series,
		     num_instances  = @num_instances,
		     updated_at     = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, tenant_id, patient_nhi, study_instance_uid,
		           accession_number, modality, body_part, study_date, description,
		           referring_hpi, performing_hpi, status, num_series, num_instances,
		           created_at, updated_at`,
		db.NamedArgs{
			"status":         s.Status,
			"description":    s.Description,
			"performing_hpi": s.PerformingHPI,
			"num_series":     s.NumSeries,
			"num_instances":  s.NumInstances,
			"id":             s.ID,
			"tenant_id":      s.TenantID,
		},
	).Scan(
		&updated.ID, &updated.TenantID, &updated.PatientNHI, &updated.StudyInstanceUID,
		&updated.AccessionNumber, &updated.Modality, &updated.BodyPart, &updated.StudyDate, &updated.Description,
		&updated.ReferringHPI, &updated.PerformingHPI, &updated.Status, &updated.NumSeries, &updated.NumInstances,
		&updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return ImagingStudy{}, errNotFound
		}
		return ImagingStudy{}, fmt.Errorf("update imaging study: %w", err)
	}
	return updated, nil
}
