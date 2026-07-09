// Package repository implements persistence for community health domain models.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-community-health/internal/homevisit"
	"github.com/jackc/pgx/v5/pgxpool"
)

// VisitRepository handles persistence of home visits and related records.
type VisitRepository struct {
	pool *pgxpool.Pool
}

// NewVisitRepository creates a new visit repository.
func NewVisitRepository(pool *pgxpool.Pool) *VisitRepository {
	return &VisitRepository{pool: pool}
}

// helpers
func toMs(t time.Time) int64 { return t.UnixMilli() }

func fromMs(ms int64) time.Time { return time.UnixMilli(ms) }

// ---------------------------------------------------------------------------
// Home Visits
// ---------------------------------------------------------------------------

// Create inserts a new home visit.
func (r *VisitRepository) Create(ctx context.Context, v *homevisit.HomeVisit) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO home_visits (
			id, patient_nhi, clinician_id, practice_id, scheduled_date,
			estimated_duration_minutes, actual_start_time, actual_end_time,
			visit_type, priority, status, address, latitude, longitude,
			contact_phone, contact_name, access_instructions, safety_notes,
			transport_mode, route_order, previous_visit_id,
			cancellation_reason, cancellation_notes, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26)
	`,
		v.ID, v.PatientNHI, v.ClinicianID, v.PracticeID, fromMs(v.ScheduledDate),
		v.EstimatedDuration, timeOrNil(v.ActualStartTime), timeOrNil(v.ActualEndTime),
		string(v.VisitType), string(v.Priority), string(v.Status), v.Address,
		fltOrNil(v.Latitude), fltOrNil(v.Longitude), v.ContactPhone, v.ContactName,
		v.AccessInstructions, v.SafetyNotes, string(v.TransportMode),
		intOrNil(v.RouteOrder), v.PreviousVisitID, v.CancellationReason,
		v.CancellationNotes, fromMs(v.CreatedAt), fromMs(v.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert home_visit: %w", err)
	}
	return nil
}

// GetByID retrieves a home visit by UUID.
func (r *VisitRepository) GetByID(ctx context.Context, id string) (*homevisit.HomeVisit, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, patient_nhi, clinician_id, practice_id, scheduled_date,
			estimated_duration_minutes, actual_start_time, actual_end_time,
			visit_type, priority, status, address, latitude, longitude,
			contact_phone, contact_name, access_instructions, safety_notes,
			transport_mode, route_order, previous_visit_id,
			cancellation_reason, cancellation_notes, created_at, updated_at
		FROM home_visits WHERE id = $1
	`, id)

	v, err := scanHomeVisit(row)
	if err != nil {
		return nil, fmt.Errorf("get home_visit: %w", err)
	}
	return v, nil
}

// List retrieves home visits with optional filters.
func (r *VisitRepository) List(ctx context.Context, patientNHI, clinicianID, status, visitType string, limit, offset int) ([]*homevisit.HomeVisit, int, error) {
	where := "WHERE 1=1"
	args := make([]any, 0, 8)
	argIdx := 1
	if patientNHI != "" {
		where += fmt.Sprintf(" AND patient_nhi = $%d", argIdx)
		args = append(args, patientNHI)
		argIdx++
	}
	if clinicianID != "" {
		where += fmt.Sprintf(" AND clinician_id = $%d", argIdx)
		args = append(args, clinicianID)
		argIdx++
	}
	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}
	if visitType != "" {
		where += fmt.Sprintf(" AND visit_type = $%d", argIdx)
		args = append(args, visitType)
		argIdx++
	}

	countArgs := args
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM home_visits %s", where)
	var total int
	if err := r.pool.QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count home_visits: %w", err)
	}

	args = append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT id, patient_nhi, clinician_id, practice_id, scheduled_date,
			estimated_duration_minutes, actual_start_time, actual_end_time,
			visit_type, priority, status, address, latitude, longitude,
			contact_phone, contact_name, access_instructions, safety_notes,
			transport_mode, route_order, previous_visit_id,
			cancellation_reason, cancellation_notes, created_at, updated_at
		FROM home_visits %s ORDER BY scheduled_date DESC LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list home_visits: %w", err)
	}
	defer rows.Close()

	var visits []*homevisit.HomeVisit
	for rows.Next() {
		v, err := scanHomeVisit(rows)
		if err != nil {
			return nil, 0, err
		}
		visits = append(visits, v)
	}
	return visits, total, rows.Err()
}

// Update patches a home visit.
func (r *VisitRepository) Update(ctx context.Context, v *homevisit.HomeVisit) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE home_visits SET
			patient_nhi=$2, clinician_id=$3, practice_id=$4, scheduled_date=$5,
			estimated_duration_minutes=$6, actual_start_time=$7, actual_end_time=$8,
			visit_type=$9, priority=$10, status=$11, address=$12, latitude=$13,
			longitude=$14, contact_phone=$15, contact_name=$16, access_instructions=$17,
			safety_notes=$18, transport_mode=$19, route_order=$20,
			previous_visit_id=$21, cancellation_reason=$22, cancellation_notes=$23,
			updated_at=$24
		WHERE id=$1
	`,
		v.ID, v.PatientNHI, v.ClinicianID, v.PracticeID, fromMs(v.ScheduledDate),
		v.EstimatedDuration, timeOrNil(v.ActualStartTime), timeOrNil(v.ActualEndTime),
		string(v.VisitType), string(v.Priority), string(v.Status), v.Address,
		fltOrNil(v.Latitude), fltOrNil(v.Longitude), v.ContactPhone, v.ContactName,
		v.AccessInstructions, v.SafetyNotes, string(v.TransportMode),
		intOrNil(v.RouteOrder), v.PreviousVisitID, v.CancellationReason,
		v.CancellationNotes, fromMs(v.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("update home_visit: %w", err)
	}
	return nil
}

// Delete removes a home visit.
func (r *VisitRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM home_visits WHERE id = $1`, id)
	return err
}

// ---------------------------------------------------------------------------
// Visit Notes
// ---------------------------------------------------------------------------

// CreateNote inserts a visit note.
func (r *VisitRepository) CreateNote(ctx context.Context, n *homevisit.HomeVisitNote) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO home_visit_notes (id, home_visit_id, patient_nhi, clinician_id, note_type, narrative, concerns, actions, follow_up_required, follow_up_details, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, n.ID, n.HomeVisitID, n.PatientNHI, n.ClinicianID, string(n.NoteType), n.Narrative, n.Concerns, n.Actions, n.FollowUpRequired, n.FollowUpDetails, fromMs(n.CreatedAt), fromMs(n.UpdatedAt))
	return err
}

// ListNotes lists notes for a home visit.
func (r *VisitRepository) ListNotes(ctx context.Context, visitID string) ([]*homevisit.HomeVisitNote, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, home_visit_id, patient_nhi, clinician_id, note_type, narrative, concerns, actions, follow_up_required, follow_up_details, created_at, updated_at
		FROM home_visit_notes WHERE home_visit_id = $1 ORDER BY created_at DESC
	`, visitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*homevisit.HomeVisitNote
	for rows.Next() {
		var n homevisit.HomeVisitNote
		var ca, ua time.Time
		err := rows.Scan(&n.ID, &n.HomeVisitID, &n.PatientNHI, &n.ClinicianID, &n.NoteType, &n.Narrative, &n.Concerns, &n.Actions, &n.FollowUpRequired, &n.FollowUpDetails, &ca, &ua)
		if err != nil {
			return nil, err
		}
		n.CreatedAt = toMs(ca)
		n.UpdatedAt = toMs(ua)
		notes = append(notes, &n)
	}
	return notes, rows.Err()
}

// ---------------------------------------------------------------------------
// Safety Checks
// ---------------------------------------------------------------------------

// CreateSafetyCheck inserts a safety check.
func (r *VisitRepository) CreateSafetyCheck(ctx context.Context, sc *homevisit.SafetyCheck) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO home_visit_safety_checks (id, home_visit_id, patient_nhi, checked_by, check_date, fall_risk, pressure_injury_risk, fire_safety_ok, smoke_alarms_ok, medication_storage_ok, tripping_hazards_noted, recommendations, actions_taken, next_check_date, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, sc.ID, sc.HomeVisitID, sc.PatientNHI, sc.CheckedBy, fromMs(sc.CheckDate), sc.FallRisk, sc.PressureInjuryRisk, sc.FireSafetyOK, sc.SmokeAlarmsOK, sc.MedicationStorageOK, sc.TrippingHazards, sc.Recommendations, sc.ActionsTaken, timeOrNil(sc.NextCheckDate), fromMs(sc.CreatedAt))
	return err
}

// ListSafetyChecks lists safety checks for a visit.
func (r *VisitRepository) ListSafetyChecks(ctx context.Context, visitID string) ([]*homevisit.SafetyCheck, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, home_visit_id, patient_nhi, checked_by, check_date, fall_risk, pressure_injury_risk, fire_safety_ok, smoke_alarms_ok, medication_storage_ok, tripping_hazards_noted, recommendations, actions_taken, next_check_date, created_at
		FROM home_visit_safety_checks WHERE home_visit_id = $1 ORDER BY check_date DESC
	`, visitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []*homevisit.SafetyCheck
	for rows.Next() {
		var sc homevisit.SafetyCheck
		var cd, ca time.Time
		var nd *time.Time
		err := rows.Scan(&sc.ID, &sc.HomeVisitID, &sc.PatientNHI, &sc.CheckedBy, &cd, &sc.FallRisk, &sc.PressureInjuryRisk, &sc.FireSafetyOK, &sc.SmokeAlarmsOK, &sc.MedicationStorageOK, &sc.TrippingHazards, &sc.Recommendations, &sc.ActionsTaken, &nd, &ca)
		if err != nil {
			return nil, err
		}
		sc.CheckDate = toMs(cd)
		sc.CreatedAt = toMs(ca)
		if nd != nil {
			sc.NextCheckDate = toMs(*nd)
		}
		checks = append(checks, &sc)
	}
	return checks, rows.Err()
}

// ---------------------------------------------------------------------------
// Equipment Checks
// ---------------------------------------------------------------------------

// CreateEquipmentCheck inserts an equipment check.
func (r *VisitRepository) CreateEquipmentCheck(ctx context.Context, ec *homevisit.EquipmentCheck) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO home_visit_equipment (id, home_visit_id, equipment_name, serial_number, checked_at, status, notes, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`, ec.ID, ec.HomeVisitID, ec.EquipmentName, ec.SerialNumber, fromMs(ec.CheckedAt), string(ec.Status), ec.Notes, fromMs(ec.CreatedAt), fromMs(ec.UpdatedAt))
	return err
}

// ListEquipmentChecks lists equipment checks for a visit.
func (r *VisitRepository) ListEquipmentChecks(ctx context.Context, visitID string) ([]*homevisit.EquipmentCheck, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, home_visit_id, equipment_name, serial_number, checked_at, status, notes, created_at, updated_at
		FROM home_visit_equipment WHERE home_visit_id = $1 ORDER BY checked_at DESC
	`, visitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checks []*homevisit.EquipmentCheck
	for rows.Next() {
		var ec homevisit.EquipmentCheck
		var cat, ca, ua time.Time
		err := rows.Scan(&ec.ID, &ec.HomeVisitID, &ec.EquipmentName, &ec.SerialNumber, &cat, &ec.Status, &ec.Notes, &ca, &ua)
		if err != nil {
			return nil, err
		}
		ec.CheckedAt = toMs(cat)
		ec.CreatedAt = toMs(ca)
		ec.UpdatedAt = toMs(ua)
		checks = append(checks, &ec)
	}
	return checks, rows.Err()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type scanner interface{ Scan(...any) error }

func scanHomeVisit(s scanner) (*homevisit.HomeVisit, error) {
	var v homevisit.HomeVisit
	var sd, ast, aet, ca, ua time.Time
	var lat, lon *float64
	var prev, cn, cnrs *string
	var ro *int

	err := s.Scan(
		&v.ID, &v.PatientNHI, &v.ClinicianID, &v.PracticeID, &sd,
		&v.EstimatedDuration, &ast, &aet,
		&v.VisitType, &v.Priority, &v.Status, &v.Address, &lat, &lon,
		&v.ContactPhone, &v.ContactName, &v.AccessInstructions, &v.SafetyNotes,
		&v.TransportMode, &ro, &prev,
		&cn, &cnrs, &ca, &ua,
	)
	if err != nil {
		return nil, err
	}
	v.ScheduledDate = toMs(sd)
	if !ast.IsZero() {
		v.ActualStartTime = toMs(ast)
	}
	if !aet.IsZero() {
		v.ActualEndTime = toMs(aet)
	}
	if lat != nil {
		v.Latitude = *lat
	}
	if lon != nil {
		v.Longitude = *lon
	}
	if ro != nil {
		v.RouteOrder = *ro
	}
	if prev != nil {
		v.PreviousVisitID = *prev
	}
	if cn != nil {
		v.CancellationReason = *cn
	}
	if cnrs != nil {
		v.CancellationNotes = *cnrs
	}
	v.CreatedAt = toMs(ca)
	v.UpdatedAt = toMs(ua)
	return &v, nil
}

func timeOrNil(ms int64) any {
	if ms == 0 {
		return nil
	}
	return time.UnixMilli(ms)
}

func fltOrNil(f float64) any {
	if f == 0 {
		return nil
	}
	return f
}

func intOrNil(i int) any {
	if i == 0 {
		return nil
	}
	return i
}
