package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

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
