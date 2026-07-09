package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/physio"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateTreatmentPlan creates a new treatment plan.
func (h *PhysioHandler) CreateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var plan physio.TreatmentPlan
	if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plan.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	plan.CreatedAt = now
	plan.UpdatedAt = now

	if err := plan.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	startDate := time.UnixMilli(plan.StartDate)
	reviewDate := time.UnixMilli(plan.ReviewDate)
	var endDate *time.Time
	if plan.EndDate > 0 {
		t := time.UnixMilli(plan.EndDate)
		endDate = &t
	}

	if h.pool != nil {
		_, err := h.pool.Exec(r.Context(), `
			INSERT INTO physio_treatment_plans
				(id, patient_nhi, clinician_id, practice_id, acc_number, referral_source,
				 diagnosis, icd10_code, start_date, review_date, end_date, status, notes, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			plan.ID, plan.PatientNHI, plan.ClinicianID, plan.PracticeID,
			nullStr(plan.ACCNumber), plan.ReferralSource,
			plan.Diagnosis, nullStr(plan.ICD10Code),
			startDate, reviewDate, endDate, string(plan.Status), nullStr(plan.Notes),
			time.UnixMilli(plan.CreatedAt), time.UnixMilli(plan.UpdatedAt),
		)
		if err != nil {
			h.logger.Error("create treatment plan", slog.Any("error", err))
			http.Error(w, "failed to save treatment plan", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(plan)
}

// GetTreatmentPlan retrieves a treatment plan by ID.
func (h *PhysioHandler) GetTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	var plan physio.TreatmentPlan
	var startDate, reviewDate, createdAt, updatedAt time.Time
	var endDate *time.Time
	var status, accNumber, icd10Code, notes string

	err := h.pool.QueryRow(ctx, `
		SELECT id, patient_nhi, clinician_id::text, practice_id::text,
		       COALESCE(acc_number,''), referral_source, diagnosis, COALESCE(icd10_code,''),
		       start_date, review_date, end_date, status, COALESCE(notes,''),
		       created_at, updated_at
		FROM physio_treatment_plans WHERE id=$1`,
		id,
	).Scan(
		&plan.ID, &plan.PatientNHI, &plan.ClinicianID, &plan.PracticeID,
		&accNumber, &plan.ReferralSource, &plan.Diagnosis, &icd10Code,
		&startDate, &reviewDate, &endDate, &status, &notes,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "treatment plan not found", http.StatusNotFound)
			return
		}
		h.logger.Error("get treatment plan", slog.Any("error", err))
		http.Error(w, "failed to fetch treatment plan", http.StatusInternalServerError)
		return
	}

	plan.ACCNumber = accNumber
	plan.ICD10Code = icd10Code
	plan.Notes = notes
	plan.Status = physio.PlanStatus(status)
	plan.StartDate = startDate.UnixMilli()
	plan.ReviewDate = reviewDate.UnixMilli()
	if endDate != nil {
		plan.EndDate = endDate.UnixMilli()
	}
	plan.CreatedAt = createdAt.UnixMilli()
	plan.UpdatedAt = updatedAt.UnixMilli()
	plan.Goals = []physio.TreatmentGoal{}
	plan.Interventions = []physio.Intervention{}
	plan.OutcomeMeasures = []physio.OutcomeMeasure{}

	if !checkConsent(w, r, h.consentStore, plan.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// ListTreatmentPlans lists treatment plans with filters.
func (h *PhysioHandler) ListTreatmentPlans(w http.ResponseWriter, r *http.Request) {
	qp := r.URL.Query()
	patientNHI := qp.Get("patient_nhi")
	clinicianID := qp.Get("clinician_id")
	statusFilter := qp.Get("status")
	limit, offset := parsePagination(r)
	ctx := r.Context()

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	args := make([]any, 0, 5)
	conds := make([]string, 0, 3)
	if patientNHI != "" {
		args = append(args, patientNHI)
		conds = append(conds, fmt.Sprintf("patient_nhi=$%d", len(args)))
	}
	if clinicianID != "" {
		args = append(args, clinicianID)
		conds = append(conds, fmt.Sprintf("clinician_id::text=$%d", len(args)))
	}
	if statusFilter != "" {
		args = append(args, statusFilter)
		conds = append(conds, fmt.Sprintf("status=$%d", len(args)))
	}
	q := `SELECT id, patient_nhi, clinician_id::text, practice_id::text,
	             COALESCE(acc_number,''), referral_source, diagnosis, COALESCE(icd10_code,''),
	             start_date, review_date, end_date, status, COALESCE(notes,''),
	             created_at, updated_at
	      FROM physio_treatment_plans`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit, offset)
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := h.pool.Query(ctx, q, args...)
	if err != nil {
		h.logger.Error("list treatment plans", slog.Any("error", err))
		http.Error(w, "failed to list treatment plans", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	plans := make([]physio.TreatmentPlan, 0)
	for rows.Next() {
		var p physio.TreatmentPlan
		var startDate, reviewDate, createdAt, updatedAt time.Time
		var endDate *time.Time
		var status, accNumber, icd10Code, notes string
		if err := rows.Scan(
			&p.ID, &p.PatientNHI, &p.ClinicianID, &p.PracticeID,
			&accNumber, &p.ReferralSource, &p.Diagnosis, &icd10Code,
			&startDate, &reviewDate, &endDate, &status, &notes,
			&createdAt, &updatedAt,
		); err != nil {
			h.logger.Error("scan treatment plan row", slog.Any("error", err))
			continue
		}
		p.ACCNumber = accNumber
		p.ICD10Code = icd10Code
		p.Notes = notes
		p.Status = physio.PlanStatus(status)
		p.StartDate = startDate.UnixMilli()
		p.ReviewDate = reviewDate.UnixMilli()
		if endDate != nil {
			p.EndDate = endDate.UnixMilli()
		}
		p.CreatedAt = createdAt.UnixMilli()
		p.UpdatedAt = updatedAt.UnixMilli()
		p.Goals = []physio.TreatmentGoal{}
		p.Interventions = []physio.Intervention{}
		p.OutcomeMeasures = []physio.OutcomeMeasure{}
		plans = append(plans, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   plans,
		"limit":  limit,
		"offset": offset,
		"total":  len(plans),
		"filters": map[string]string{
			"patient_nhi":  patientNHI,
			"clinician_id": clinicianID,
			"status":       statusFilter,
		},
	})
}

// UpdateTreatmentPlan updates a treatment plan.
func (h *PhysioHandler) UpdateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	id := r.PathValue("id")

	var plan physio.TreatmentPlan
	if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plan.ID = id
	plan.UpdatedAt = time.Now().UnixMilli()

	if err := plan.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// DeleteTreatmentPlan deletes a treatment plan.
func (h *PhysioHandler) DeleteTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}
	_ = r.PathValue("id")
	w.WriteHeader(http.StatusNoContent)
}
