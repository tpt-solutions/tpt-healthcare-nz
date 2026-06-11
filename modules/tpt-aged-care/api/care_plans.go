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
)

// CarePlanType distinguishes the care setting.
type CarePlanType string

const (
	PlanTypeResidential CarePlanType = "residential" // aged residential care facility
	PlanTypeHomeCare    CarePlanType = "home-care"   // home-based support
	PlanTypeDayProgramme CarePlanType = "day-programme"
	PlanTypeRespite     CarePlanType = "respite"
)

// CarePlanStatus tracks the lifecycle of a care plan.
type CarePlanStatus string

const (
	CarePlanActive    CarePlanStatus = "active"
	CarePlanOnHold    CarePlanStatus = "on-hold"
	CarePlanCompleted CarePlanStatus = "completed"
	CarePlanRevoked   CarePlanStatus = "revoked"
)

// GoalStatus tracks an individual care goal.
type GoalStatus string

const (
	GoalInProgress GoalStatus = "in-progress"
	GoalAchieved   GoalStatus = "achieved"
	GoalAbandoned  GoalStatus = "abandoned"
	GoalOnHold     GoalStatus = "on-hold"
)

// CareGoal represents a single goal within a care plan.
type CareGoal struct {
	ID           string     `json:"id"`
	Description  string     `json:"description"`
	Status       GoalStatus `json:"status"`
	TargetDate   string     `json:"targetDate,omitempty"` // YYYY-MM-DD
	AchievedDate string     `json:"achievedDate,omitempty"`
	Notes        string     `json:"notes,omitempty"`
}

// CareIntervention records a planned intervention or care task.
type CareIntervention struct {
	Description string `json:"description"`
	Frequency   string `json:"frequency,omitempty"` // e.g. "daily", "weekly"
	Responsible string `json:"responsible,omitempty"`
}

// CarePlan represents an aged care plan for a patient.
type CarePlan struct {
	ID              string             `json:"id"`
	PatientID       string             `json:"patientId"`
	PatientNHI      string             `json:"patientNhi"`
	TenantID        string             `json:"tenantId"`
	ResponsibleHPI  string             `json:"responsibleHpi"`
	PlanType        CarePlanType       `json:"planType"`
	Status          CarePlanStatus     `json:"status"`
	Goals           []CareGoal         `json:"goals"`
	Interventions   []CareIntervention `json:"interventions"`
	// ClinicalNotes is AES-256-GCM encrypted at rest.
	ClinicalNotes   string             `json:"clinicalNotes,omitempty"`
	StartDate       string             `json:"startDate"` // YYYY-MM-DD
	EndDate         string             `json:"endDate,omitempty"`
	NextReviewDate  string             `json:"nextReviewDate,omitempty"`
	FacilityName    string             `json:"facilityName,omitempty"` // for residential plans
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}

type carePlanCreateRequest struct {
	PatientID      string             `json:"patientId"`
	PatientNHI     string             `json:"patientNhi"`
	ResponsibleHPI string             `json:"responsibleHpi"`
	PlanType       CarePlanType       `json:"planType"`
	Goals          []CareGoal         `json:"goals,omitempty"`
	Interventions  []CareIntervention `json:"interventions,omitempty"`
	ClinicalNotes  string             `json:"clinicalNotes,omitempty"`
	StartDate      string             `json:"startDate"`
	EndDate        string             `json:"endDate,omitempty"`
	NextReviewDate string             `json:"nextReviewDate,omitempty"`
	FacilityName   string             `json:"facilityName,omitempty"`
}

type carePlanUpdateRequest struct {
	Status         CarePlanStatus     `json:"status,omitempty"`
	Goals          []CareGoal         `json:"goals,omitempty"`
	Interventions  []CareIntervention `json:"interventions,omitempty"`
	ClinicalNotes  string             `json:"clinicalNotes,omitempty"`
	EndDate        string             `json:"endDate,omitempty"`
	NextReviewDate string             `json:"nextReviewDate,omitempty"`
	FacilityName   string             `json:"facilityName,omitempty"`
}

type carePlanRecord struct {
	ID             string
	PatientID      string
	PatientNHI     string
	TenantID       string
	ResponsibleHPI string
	PlanType       CarePlanType
	Status         CarePlanStatus
	Goals          []CareGoal
	Interventions  []CareIntervention
	NotesEncrypted []byte
	StartDate      string
	EndDate        string
	NextReviewDate string
	FacilityName   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// CarePlansHandler handles all /api/v1/care-plans/* routes.
type CarePlansHandler struct {
	pool       dbPool
	enc        *encryption.Encryptor
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/care-plans.
// Query params: patient, planType, status.
func (h *CarePlansHandler) List(w http.ResponseWriter, r *http.Request) {
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
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		        plan_type, status, goals, interventions, clinical_notes,
		        start_date, end_date, next_review_date, facility_name, created_at, updated_at
		 FROM aged_care_plans
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR plan_type = $3)
		   AND ($4 = '' OR status = $4)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		tenantID, q.Get("patient"), q.Get("planType"), q.Get("status"),
	)
	if err != nil {
		h.logger.Error("list care plans", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list care plans"})
		return
	}
	defer rows.Close()

	var results []CarePlan
	for rows.Next() {
		rec, err := scanCarePlan(rows)
		if err != nil {
			h.logger.Error("scan care plan", slog.Any("error", err))
			continue
		}
		cp, err := h.decrypt(rec)
		if err != nil {
			h.logger.Error("decrypt care plan", slog.Any("error", err))
			continue
		}
		results = append(results, cp)
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "AgedCarePlan",
		ResourceID:   "list",
	})

	writeJSON(w, http.StatusOK, map[string]any{"carePlans": results, "total": len(results)})
}

// Get handles GET /api/v1/care-plans/{id}.
func (h *CarePlansHandler) Get(w http.ResponseWriter, r *http.Request) {
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
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found"})
			return
		}
		h.logger.Error("get care plan", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve care plan"})
		return
	}

	cp, err := h.decrypt(rec)
	if err != nil {
		h.logger.Error("decrypt care plan", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt care plan"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "AgedCarePlan",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
	})

	writeJSON(w, http.StatusOK, cp)
}

// Create handles POST /api/v1/care-plans.
func (h *CarePlansHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req carePlanCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientNHI == "" && req.PatientID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patientId or patientNhi is required"})
		return
	}
	if req.ResponsibleHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PRACTITIONER", Message: "responsibleHpi is required"})
		return
	}
	if !validPlanType(req.PlanType) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_PLAN_TYPE", Message: fmt.Sprintf("unknown plan type %q", req.PlanType)})
		return
	}
	if req.StartDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_START_DATE", Message: "startDate is required"})
		return
	}

	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.ResponsibleHPI)
	if err != nil {
		h.logger.Error("HPI APC check", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not verify practitioner APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusForbidden, apiError{Code: "INVALID_APC", Message: "practitioner does not hold a current Annual Practising Certificate"})
		return
	}

	notesEnc, err := h.enc.Encrypt([]byte(req.ClinicalNotes))
	if err != nil {
		h.logger.Error("encrypt clinical notes", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt notes"})
		return
	}

	if req.Goals == nil {
		req.Goals = []CareGoal{}
	}
	if req.Interventions == nil {
		req.Interventions = []CareIntervention{}
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO aged_care_plans
		   (patient_id, patient_nhi, tenant_id, responsible_hpi,
		    plan_type, status, goals, interventions, clinical_notes,
		    start_date, end_date, next_review_date, facility_name)
		 VALUES ($1, $2, $3, $4, $5, 'active', $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           plan_type, status, goals, interventions, clinical_notes,
		           start_date, end_date, next_review_date, facility_name, created_at, updated_at`,
		req.PatientID, req.PatientNHI, tenantID, req.ResponsibleHPI,
		string(req.PlanType), req.Goals, req.Interventions, notesEnc,
		req.StartDate, req.EndDate, req.NextReviewDate, req.FacilityName,
	)
	rec, err := scanCarePlan(row)
	if err != nil {
		h.logger.Error("insert care plan", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create care plan"})
		return
	}

	cp, err := h.decrypt(rec)
	if err != nil {
		h.logger.Error("decrypt after insert", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt care plan"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "AgedCarePlan",
		ResourceID:   rec.ID,
		PatientNHI:   req.PatientNHI,
		Details:      map[string]any{"planType": string(req.PlanType)},
	})

	writeJSON(w, http.StatusCreated, cp)
}

// Update handles PUT /api/v1/care-plans/{id}.
func (h *CarePlansHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var req carePlanUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	rec, err := h.getByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found"})
			return
		}
		h.logger.Error("get care plan for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve care plan"})
		return
	}

	if req.Status != "" {
		rec.Status = req.Status
	}
	if len(req.Goals) > 0 {
		rec.Goals = req.Goals
	}
	if len(req.Interventions) > 0 {
		rec.Interventions = req.Interventions
	}
	if req.EndDate != "" {
		rec.EndDate = req.EndDate
	}
	if req.NextReviewDate != "" {
		rec.NextReviewDate = req.NextReviewDate
	}
	if req.FacilityName != "" {
		rec.FacilityName = req.FacilityName
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

	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_plans
		 SET status = $1, goals = $2, interventions = $3, clinical_notes = $4,
		     end_date = $5, next_review_date = $6, facility_name = $7, updated_at = now()
		 WHERE id = $8 AND tenant_id = $9
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           plan_type, status, goals, interventions, clinical_notes,
		           start_date, end_date, next_review_date, facility_name, created_at, updated_at`,
		string(rec.Status), rec.Goals, rec.Interventions, notesEnc,
		rec.EndDate, rec.NextReviewDate, rec.FacilityName, id, tenantID,
	)
	updated, err := scanCarePlan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found"})
			return
		}
		h.logger.Error("update care plan", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update care plan"})
		return
	}

	cp, err := h.decrypt(updated)
	if err != nil {
		h.logger.Error("decrypt after update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt care plan"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "AgedCarePlan",
		ResourceID:   id,
	})

	writeJSON(w, http.StatusOK, cp)
}

// RecordReview handles POST /api/v1/care-plans/{id}/review.
// Records a mandatory periodic review and updates the next review date.
func (h *CarePlansHandler) RecordReview(w http.ResponseWriter, r *http.Request) {
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
	var req struct {
		ReviewerHPI    string `json:"reviewerHpi"`
		NextReviewDate string `json:"nextReviewDate"`
		ReviewNotes    string `json:"reviewNotes,omitempty"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ReviewerHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_REVIEWER", Message: "reviewerHpi is required"})
		return
	}
	if req.NextReviewDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NEXT_REVIEW", Message: "nextReviewDate is required"})
		return
	}

	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_plans
		 SET next_review_date = $1, updated_at = now()
		 WHERE id = $2 AND tenant_id = $3
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           plan_type, status, goals, interventions, clinical_notes,
		           start_date, end_date, next_review_date, facility_name, created_at, updated_at`,
		req.NextReviewDate, id, tenantID,
	)
	rec, err := scanCarePlan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found"})
			return
		}
		h.logger.Error("record review", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "REVIEW_ERROR", Message: "failed to record review"})
		return
	}

	cp, err := h.decrypt(rec)
	if err != nil {
		h.logger.Error("decrypt after review", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt care plan"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "review",
		ResourceType: "AgedCarePlan",
		ResourceID:   id,
		PatientNHI:   rec.PatientNHI,
		Details:      map[string]any{"reviewerHpi": req.ReviewerHPI, "nextReviewDate": req.NextReviewDate},
	})

	writeJSON(w, http.StatusOK, cp)
}

// AddGoal handles POST /api/v1/care-plans/{id}/goals.
func (h *CarePlansHandler) AddGoal(w http.ResponseWriter, r *http.Request) {
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
	var newGoal CareGoal
	if err := decodeJSON(r, &newGoal); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if newGoal.Description == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DESCRIPTION", Message: "goal description is required"})
		return
	}
	if newGoal.ID == "" {
		newGoal.ID = uuid.New().String()
	}
	if newGoal.Status == "" {
		newGoal.Status = GoalInProgress
	}

	rec, err := h.getByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found"})
			return
		}
		h.logger.Error("get care plan for add goal", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve care plan"})
		return
	}

	rec.Goals = append(rec.Goals, newGoal)

	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_plans
		 SET goals = $1, updated_at = now()
		 WHERE id = $2 AND tenant_id = $3
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           plan_type, status, goals, interventions, clinical_notes,
		           start_date, end_date, next_review_date, facility_name, created_at, updated_at`,
		rec.Goals, id, tenantID,
	)
	updated, err := scanCarePlan(row)
	if err != nil {
		h.logger.Error("add goal", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ADD_GOAL_ERROR", Message: "failed to add goal"})
		return
	}

	cp, err := h.decrypt(updated)
	if err != nil {
		h.logger.Error("decrypt after add goal", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt care plan"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "AgedCarePlan",
		ResourceID:   id,
		Details:      map[string]any{"action": "add-goal", "goalId": newGoal.ID},
	})

	writeJSON(w, http.StatusOK, cp)
}

// UpdateGoal handles PUT /api/v1/care-plans/{id}/goals/{goalId}.
func (h *CarePlansHandler) UpdateGoal(w http.ResponseWriter, r *http.Request) {
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
	goalID := r.PathValue("goalId")

	var goalUpdate CareGoal
	if err := decodeJSON(r, &goalUpdate); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	rec, err := h.getByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "care plan not found"})
			return
		}
		h.logger.Error("get care plan for update goal", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve care plan"})
		return
	}

	found := false
	for i, g := range rec.Goals {
		if g.ID == goalID {
			if goalUpdate.Description != "" {
				rec.Goals[i].Description = goalUpdate.Description
			}
			if goalUpdate.Status != "" {
				rec.Goals[i].Status = goalUpdate.Status
			}
			if goalUpdate.TargetDate != "" {
				rec.Goals[i].TargetDate = goalUpdate.TargetDate
			}
			if goalUpdate.AchievedDate != "" {
				rec.Goals[i].AchievedDate = goalUpdate.AchievedDate
			}
			if goalUpdate.Notes != "" {
				rec.Goals[i].Notes = goalUpdate.Notes
			}
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, apiError{Code: "GOAL_NOT_FOUND", Message: "goal not found in care plan"})
		return
	}

	row := h.pool.QueryRow(ctx,
		`UPDATE aged_care_plans
		 SET goals = $1, updated_at = now()
		 WHERE id = $2 AND tenant_id = $3
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           plan_type, status, goals, interventions, clinical_notes,
		           start_date, end_date, next_review_date, facility_name, created_at, updated_at`,
		rec.Goals, id, tenantID,
	)
	updated, err := scanCarePlan(row)
	if err != nil {
		h.logger.Error("update goal", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_GOAL_ERROR", Message: "failed to update goal"})
		return
	}

	cp, err := h.decrypt(updated)
	if err != nil {
		h.logger.Error("decrypt after update goal", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt care plan"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "AgedCarePlan",
		ResourceID:   id,
		Details:      map[string]any{"action": "update-goal", "goalId": goalID},
	})

	writeJSON(w, http.StatusOK, cp)
}

// ---------------------------------------------------------------------------
// DB helpers
// ---------------------------------------------------------------------------

func (h *CarePlansHandler) getByID(ctx context.Context, id string, tenantID uuid.UUID) (carePlanRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		        plan_type, status, goals, interventions, clinical_notes,
		        start_date, end_date, next_review_date, facility_name, created_at, updated_at
		 FROM aged_care_plans
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanCarePlan(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return carePlanRecord{}, errNotFound
		}
		return carePlanRecord{}, fmt.Errorf("get care plan by id: %w", err)
	}
	return rec, nil
}

func (h *CarePlansHandler) decrypt(rec carePlanRecord) (CarePlan, error) {
	var notes string
	if len(rec.NotesEncrypted) > 0 {
		plain, err := h.enc.Decrypt(rec.NotesEncrypted)
		if err != nil {
			return CarePlan{}, fmt.Errorf("decrypt clinical notes: %w", err)
		}
		notes = string(plain)
	}
	return CarePlan{
		ID:             rec.ID,
		PatientID:      rec.PatientID,
		PatientNHI:     rec.PatientNHI,
		TenantID:       rec.TenantID,
		ResponsibleHPI: rec.ResponsibleHPI,
		PlanType:       rec.PlanType,
		Status:         rec.Status,
		Goals:          rec.Goals,
		Interventions:  rec.Interventions,
		ClinicalNotes:  notes,
		StartDate:      rec.StartDate,
		EndDate:        rec.EndDate,
		NextReviewDate: rec.NextReviewDate,
		FacilityName:   rec.FacilityName,
		CreatedAt:      rec.CreatedAt,
		UpdatedAt:      rec.UpdatedAt,
	}, nil
}

func scanCarePlan(s rowScanner) (carePlanRecord, error) {
	var rec carePlanRecord
	var planType, status string
	if err := s.Scan(
		&rec.ID, &rec.PatientID, &rec.PatientNHI, &rec.TenantID, &rec.ResponsibleHPI,
		&planType, &status, &rec.Goals, &rec.Interventions, &rec.NotesEncrypted,
		&rec.StartDate, &rec.EndDate, &rec.NextReviewDate, &rec.FacilityName,
		&rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return carePlanRecord{}, err
	}
	rec.PlanType = CarePlanType(planType)
	rec.Status = CarePlanStatus(status)
	return rec, nil
}

func validPlanType(t CarePlanType) bool {
	switch t {
	case PlanTypeResidential, PlanTypeHomeCare, PlanTypeDayProgramme, PlanTypeRespite:
		return true
	}
	return false
}
