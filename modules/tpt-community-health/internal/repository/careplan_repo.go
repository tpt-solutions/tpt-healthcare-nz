package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-community-health/internal/districtnursing"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CarePlanRepository handles persistence of district nursing care plans.
type CarePlanRepository struct {
	pool *pgxpool.Pool
}

// NewCarePlanRepository creates a new repository.
func NewCarePlanRepository(pool *pgxpool.Pool) *CarePlanRepository {
	return &CarePlanRepository{pool: pool}
}

// Create inserts a care plan.
func (r *CarePlanRepository) Create(ctx context.Context, p *districtnursing.CarePlan) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO district_nursing_care_plans (id, patient_nhi, clinician_id, practice_id, plan_name, plan_type, status, start_date, review_date, end_date, goals, risk_level, consent_given, consent_date, dhb_funded, funding_code, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
	`, p.ID, p.PatientNHI, p.ClinicianID, p.PracticeID, p.PlanName, string(p.PlanType), string(p.Status),
		fromMs(p.StartDate), fromMs(p.ReviewDate), timeOrNil(p.EndDate), p.Goals, string(p.RiskLevel),
		p.ConsentGiven, timeOrNil(p.ConsentDate), p.DHBFunded, p.FundingCode, fromMs(p.CreatedAt), fromMs(p.UpdatedAt))
	return err
}

// GetByID retrieves a care plan.
func (r *CarePlanRepository) GetByID(ctx context.Context, id string) (*districtnursing.CarePlan, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, patient_nhi, clinician_id, practice_id, plan_name, plan_type, status, start_date, review_date, end_date, goals, risk_level, consent_given, consent_date, dhb_funded, funding_code, created_at, updated_at
		FROM district_nursing_care_plans WHERE id = $1
	`, id)
	return scanCarePlan(row)
}

// List lists care plans with filters.
func (r *CarePlanRepository) List(ctx context.Context, patientNHI, clinicianID, planType, status string, limit, offset int) ([]*districtnursing.CarePlan, int, error) {
	where, args, argIdx := buildWhere("1=1", patientNHI, clinicianID, planType, status)
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM district_nursing_care_plans WHERE %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	a2 := append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT id, patient_nhi, clinician_id, practice_id, plan_name, plan_type, status, start_date, review_date, end_date, goals, risk_level, consent_given, consent_date, dhb_funded, funding_code, created_at, updated_at
		FROM district_nursing_care_plans WHERE %s ORDER BY start_date DESC LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	rows, err := r.pool.Query(ctx, q, a2...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var plans []*districtnursing.CarePlan
	for rows.Next() {
		p, err := scanCarePlan(rows)
		if err != nil {
			return nil, 0, err
		}
		plans = append(plans, p)
	}
	return plans, total, rows.Err()
}

// Update updates a care plan.
func (r *CarePlanRepository) Update(ctx context.Context, p *districtnursing.CarePlan) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE district_nursing_care_plans SET
			patient_nhi=$2, clinician_id=$3, practice_id=$4, plan_name=$5, plan_type=$6, status=$7,
			start_date=$8, review_date=$9, end_date=$10, goals=$11, risk_level=$12,
			consent_given=$13, consent_date=$14, dhb_funded=$15, funding_code=$16, updated_at=$17
		WHERE id=$1
	`, p.ID, p.PatientNHI, p.ClinicianID, p.PracticeID, p.PlanName, string(p.PlanType), string(p.Status),
		fromMs(p.StartDate), fromMs(p.ReviewDate), timeOrNil(p.EndDate), p.Goals, string(p.RiskLevel),
		p.ConsentGiven, timeOrNil(p.ConsentDate), p.DHBFunded, p.FundingCode, fromMs(p.UpdatedAt))
	return err
}

// Delete removes a care plan.
func (r *CarePlanRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM district_nursing_care_plans WHERE id = $1`, id)
	return err
}

// ---------------------------------------------------------------------------
// Nursing Visits
// ---------------------------------------------------------------------------

// CreateNursingVisit inserts a nursing visit.
func (r *CarePlanRepository) CreateNursingVisit(ctx context.Context, v *districtnursing.NursingVisit) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO district_nursing_visits (id, care_plan_id, patient_nhi, clinician_id, visit_date, visit_type, visit_status, vital_signs, wound_assessments, medications_administered, observations, patient_education, equipment_check, next_visit_date, next_visit_reason, concerns, escalations, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
	`, v.ID, v.CarePlanID, v.PatientNHI, v.ClinicianID, fromMs(v.VisitDate), string(v.VisitType), string(v.VisitStatus),
		v.VitalSigns, v.WoundAssessments, v.MedicationsAdministered, v.Observations, v.PatientEducation, v.EquipmentCheck,
		timeOrNil(v.NextVisitDate), v.NextVisitReason, v.Concerns, v.Escalations, fromMs(v.CreatedAt), fromMs(v.UpdatedAt))
	return err
}

// ListNursingVisits lists visits with filters.
func (r *CarePlanRepository) ListNursingVisits(ctx context.Context, patientNHI, carePlanID string, limit, offset int) ([]*districtnursing.NursingVisit, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	argIdx := 1
	if patientNHI != "" {
		where += fmt.Sprintf(" AND patient_nhi = $%d", argIdx); argIdx++; args = append(args, patientNHI)
	}
	if carePlanID != "" {
		where += fmt.Sprintf(" AND care_plan_id = $%d", argIdx); argIdx++; args = append(args, carePlanID)
	}
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM district_nursing_visits %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	a2 := append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT id, care_plan_id, patient_nhi, clinician_id, visit_date, visit_type, visit_status, vital_signs, wound_assessments, medications_administered, observations, patient_education, equipment_check, next_visit_date, next_visit_reason, concerns, escalations, created_at, updated_at
		FROM district_nursing_visits %s ORDER BY visit_date DESC LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	rows, err := r.pool.Query(ctx, q, a2...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var visits []*districtnursing.NursingVisit
	for rows.Next() {
		var v districtnursing.NursingVisit
		var vd, nvd, ca, ua time.Time
		err := rows.Scan(&v.ID, &v.CarePlanID, &v.PatientNHI, &v.ClinicianID, &vd, &v.VisitType, &v.VisitStatus,
			&v.VitalSigns, &v.WoundAssessments, &v.MedicationsAdministered, &v.Observations, &v.PatientEducation, &v.EquipmentCheck,
			&nvd, &v.NextVisitReason, &v.Concerns, &v.Escalations, &ca, &ua)
		if err != nil {
			return nil, 0, err
		}
		v.VisitDate = toMs(vd)
		if !nvd.IsZero() { v.NextVisitDate = toMs(nvd) }
		v.CreatedAt = toMs(ca); v.UpdatedAt = toMs(ua)
		visits = append(visits, &v)
	}
	return visits, total, rows.Err()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func scanCarePlan(s scanner) (*districtnursing.CarePlan, error) {
	var p districtnursing.CarePlan
	var sd, rd, cd, ca, ua time.Time
	var e *time.Time
	err := s.Scan(&p.ID, &p.PatientNHI, &p.ClinicianID, &p.PracticeID, &p.PlanName, &p.PlanType, &p.Status,
		&sd, &rd, &e, &p.Goals, &p.RiskLevel, &p.ConsentGiven, &cd, &p.DHBFunded, &p.FundingCode, &ca, &ua)
	if err != nil {
		return nil, err
	}
	p.StartDate = toMs(sd); p.ReviewDate = toMs(rd); p.CreatedAt = toMs(ca); p.UpdatedAt = toMs(ua)
	if !cd.IsZero() { p.ConsentDate = toMs(cd) }
	if e != nil && !e.IsZero() { p.EndDate = toMs(*e) }
	return &p, nil
}

func buildWhere(base, patientNHI, clinicianID, typeFilter, status string) (string, []any, int) {
	where := base
	args := []any{}
	idx := 1
	if patientNHI != "" {
		where += fmt.Sprintf(" AND patient_nhi = $%d", idx); idx++; args = append(args, patientNHI)
	}
	if clinicianID != "" {
		where += fmt.Sprintf(" AND clinician_id = $%d", idx); idx++; args = append(args, clinicianID)
	}
	if typeFilter != "" {
		where += fmt.Sprintf(" AND plan_type = $%d", idx); idx++; args = append(args, typeFilter)
	}
	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", idx); idx++; args = append(args, status)
	}
	return where, args, idx
}
