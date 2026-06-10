package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-community-health/internal/outreach"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OutreachRepository handles persistence of outreach programs and events.
type OutreachRepository struct {
	pool *pgxpool.Pool
}

// NewOutreachRepository creates a new repository.
func NewOutreachRepository(pool *pgxpool.Pool) *OutreachRepository {
	return &OutreachRepository{pool: pool}
}

// ---------------------------------------------------------------------------
// Programs
// ---------------------------------------------------------------------------

func (r *OutreachRepository) CreateProgram(ctx context.Context, p *outreach.Program) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO outreach_programs (id, practice_id, program_name, program_type, description, target_population, status, start_date, end_date, funding_source, funding_code, budget, spent, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, p.ID, p.PracticeID, p.ProgramName, string(p.ProgramType), p.Description, p.TargetPopulation, string(p.Status),
		fromMs(p.StartDate), timeOrNil(p.EndDate), p.FundingSource, p.FundingCode, p.Budget, p.Spent, fromMs(p.CreatedAt), fromMs(p.UpdatedAt))
	return err
}

func (r *OutreachRepository) GetProgram(ctx context.Context, id string) (*outreach.Program, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, practice_id, program_name, program_type, description, target_population, status, start_date, end_date, funding_source, funding_code, budget, spent, created_at, updated_at
		FROM outreach_programs WHERE id = $1
	`, id)
	return scanProgram(row)
}

func (r *OutreachRepository) ListPrograms(ctx context.Context, practiceID, programType, status string, limit, offset int) ([]*outreach.Program, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	idx := 1
	if practiceID != "" {
		where += fmt.Sprintf(" AND practice_id = $%d", idx); idx++; args = append(args, practiceID)
	}
	if programType != "" {
		where += fmt.Sprintf(" AND program_type = $%d", idx); idx++; args = append(args, programType)
	}
	if status != "" {
		where += fmt.Sprintf(" AND status = $%d", idx); idx++; args = append(args, status)
	}
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM outreach_programs %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	a2 := append(args, limit, offset)
	q := fmt.Sprintf(`
		SELECT id, practice_id, program_name, program_type, description, target_population, status, start_date, end_date, funding_source, funding_code, budget, spent, created_at, updated_at
		FROM outreach_programs %s ORDER BY start_date DESC LIMIT $%d OFFSET $%d
	`, where, idx, idx+1)
	rows, err := r.pool.Query(ctx, q, a2...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var programs []*outreach.Program
	for rows.Next() {
		p, err := scanProgram(rows)
		if err != nil {
			return nil, 0, err
		}
		programs = append(programs, p)
	}
	return programs, total, rows.Err()
}

func (r *OutreachRepository) UpdateProgram(ctx context.Context, p *outreach.Program) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outreach_programs SET
			practice_id=$2, program_name=$3, program_type=$4, description=$5, target_population=$6, status=$7,
			start_date=$8, end_date=$9, funding_source=$10, funding_code=$11, budget=$12, spent=$13, updated_at=$14
		WHERE id=$1
	`, p.ID, p.PracticeID, p.ProgramName, string(p.ProgramType), p.Description, p.TargetPopulation, string(p.Status),
		fromMs(p.StartDate), timeOrNil(p.EndDate), p.FundingSource, p.FundingCode, p.Budget, p.Spent, fromMs(p.UpdatedAt))
	return err
}

func (r *OutreachRepository) DeleteProgram(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM outreach_programs WHERE id = $1`, id)
	return err
}

// ---------------------------------------------------------------------------
// Events
// ---------------------------------------------------------------------------

func (r *OutreachRepository) CreateEvent(ctx context.Context, e *outreach.Event) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO outreach_events (id, program_id, event_name, event_type, scheduled_date, estimated_duration_minutes, location_address, latitude, longitude, venue_name, venue_contact, target_attendees, actual_attendees, clinicians, equipment_list, status, cancellation_reason, notes, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
	`, e.ID, e.ProgramID, e.EventName, string(e.EventType), fromMs(e.ScheduledDate), e.EstimatedDuration, e.LocationAddress,
		fltOrNil(e.Latitude), fltOrNil(e.Longitude), e.VenueName, e.VenueContact, intOrNil(e.TargetAttendees), intOrNil(e.ActualAttendees),
		e.Clinicians, e.EquipmentList, string(e.Status), e.CancellationReason, e.Notes, fromMs(e.CreatedAt), fromMs(e.UpdatedAt))
	return err
}

func (r *OutreachRepository) GetEvent(ctx context.Context, id string) (*outreach.Event, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, program_id, event_name, event_type, scheduled_date, estimated_duration_minutes, location_address, latitude, longitude, venue_name, venue_contact, target_attendees, actual_attendees, clinicians, equipment_list, status, cancellation_reason, notes, created_at, updated_at
		FROM outreach_events WHERE id = $1
	`, id)
	return scanEvent(row)
}

func (r *OutreachRepository) ListEvents(ctx context.Context, programID string, limit, offset int) ([]*outreach.Event, int, error) {
	where := "WHERE program_id = $1"
	args := []any{programID}
	var total int
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM outreach_events %s", where), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := fmt.Sprintf(`
		SELECT id, program_id, event_name, event_type, scheduled_date, estimated_duration_minutes, location_address, latitude, longitude, venue_name, venue_contact, target_attendees, actual_attendees, clinicians, equipment_list, status, cancellation_reason, notes, created_at, updated_at
		FROM outreach_events %s ORDER BY scheduled_date DESC LIMIT $2 OFFSET $3
	`, where)
	rows, err := r.pool.Query(ctx, q, programID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var events []*outreach.Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, 0, err
		}
		events = append(events, e)
	}
	return events, total, rows.Err()
}

func (r *OutreachRepository) UpdateEvent(ctx context.Context, e *outreach.Event) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE outreach_events SET
			event_name=$2, event_type=$3, scheduled_date=$4, estimated_duration_minutes=$5, location_address=$6,
			latitude=$7, longitude=$8, venue_name=$9, venue_contact=$10, target_attendees=$11, actual_attendees=$12,
			clinicians=$13, equipment_list=$14, status=$15, cancellation_reason=$16, notes=$17, updated_at=$18
		WHERE id=$1
	`, e.ID, e.EventName, string(e.EventType), fromMs(e.ScheduledDate), e.EstimatedDuration, e.LocationAddress,
		fltOrNil(e.Latitude), fltOrNil(e.Longitude), e.VenueName, e.VenueContact, intOrNil(e.TargetAttendees), intOrNil(e.ActualAttendees),
		e.Clinicians, e.EquipmentList, string(e.Status), e.CancellationReason, e.Notes, fromMs(e.UpdatedAt))
	return err
}

// ---------------------------------------------------------------------------
// Attendees
// ---------------------------------------------------------------------------

func (r *OutreachRepository) CreateAttendee(ctx context.Context, a *outreach.Attendee) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO outreach_attendees (id, event_id, patient_nhi, attendee_name, attendee_type, contact_phone, contact_email, demographics, nhi_provided, registration_method, attended_at, services_received, consent_given, follow_up_required, follow_up_details, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
	`, a.ID, a.EventID, a.PatientNHI, a.AttendeeName, string(a.AttendeeType), a.ContactPhone, a.ContactEmail, a.Demographics,
		a.NHIProvided, a.RegistrationMethod, timeOrNil(a.AttendedAt), a.ServicesReceived, a.ConsentGiven, a.FollowUpRequired, a.FollowUpDetails, fromMs(a.CreatedAt), fromMs(a.UpdatedAt))
	return err
}

func (r *OutreachRepository) ListAttendees(ctx context.Context, eventID string, limit, offset int) ([]*outreach.Attendee, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM outreach_attendees WHERE event_id = $1`, eventID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, patient_nhi, attendee_name, attendee_type, contact_phone, contact_email, demographics, nhi_provided, registration_method, attended_at, services_received, consent_given, follow_up_required, follow_up_details, created_at, updated_at
		FROM outreach_attendees WHERE event_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3
	`, eventID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var attendees []*outreach.Attendee
	for rows.Next() {
		var a outreach.Attendee
		var ca, ua time.Time
		var aa *time.Time
		err := rows.Scan(&a.ID, &a.EventID, &a.PatientNHI, &a.AttendeeName, &a.AttendeeType, &a.ContactPhone, &a.ContactEmail, &a.Demographics, &a.NHIProvided, &a.RegistrationMethod, &aa, &a.ServicesReceived, &a.ConsentGiven, &a.FollowUpRequired, &a.FollowUpDetails, &ca, &ua)
		if err != nil {
			return nil, 0, err
		}
		if aa != nil { a.AttendedAt = toMs(*aa) }
		a.CreatedAt = toMs(ca); a.UpdatedAt = toMs(ua)
		attendees = append(attendees, &a)
	}
	return attendees, total, rows.Err()
}

// ---------------------------------------------------------------------------
// Referrals
// ---------------------------------------------------------------------------

func (r *OutreachRepository) CreateReferral(ctx context.Context, ref *outreach.Referral) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO outreach_referrals (id, event_id, patient_nhi, referred_by, referral_date, referral_type, referral_reason, urgency, status, outcome, outcome_date, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, ref.ID, ref.EventID, ref.PatientNHI, ref.ReferredBy, fromMs(ref.ReferralDate), string(ref.ReferralType), ref.ReferralReason, ref.Urgency, ref.Status, ref.Outcome, timeOrNil(ref.OutcomeDate), fromMs(ref.CreatedAt), fromMs(ref.UpdatedAt))
	return err
}

func (r *OutreachRepository) ListReferrals(ctx context.Context, eventID string) ([]*outreach.Referral, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, patient_nhi, referred_by, referral_date, referral_type, referral_reason, urgency, status, outcome, outcome_date, created_at, updated_at
		FROM outreach_referrals WHERE event_id = $1 ORDER BY referral_date DESC
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []*outreach.Referral
	for rows.Next() {
		var ref outreach.Referral
		var rd, ca, ua time.Time
		var od *time.Time
		err := rows.Scan(&ref.ID, &ref.EventID, &ref.PatientNHI, &ref.ReferredBy, &rd, &ref.ReferralType, &ref.ReferralReason, &ref.Urgency, &ref.Status, &ref.Outcome, &od, &ca, &ua)
		if err != nil {
			return nil, err
		}
		ref.ReferralDate = toMs(rd)
		if od != nil { ref.OutcomeDate = toMs(*od) }
		ref.CreatedAt = toMs(ca); ref.UpdatedAt = toMs(ua)
		refs = append(refs, &ref)
	}
	return refs, rows.Err()
}

// ---------------------------------------------------------------------------
// Screenings
// ---------------------------------------------------------------------------

func (r *OutreachRepository) CreateScreening(ctx context.Context, s *outreach.Screening) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO outreach_screenings (id, event_id, patient_nhi, clinician_id, screening_type, screening_date, result_category, result_value, interpretation, consent_given, follow_up_required, follow_up_details, referral_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, s.ID, s.EventID, s.PatientNHI, s.ClinicianID, string(s.ScreeningType), fromMs(s.ScreeningDate), string(s.ResultCategory), s.ResultValue, s.Interpretation, s.ConsentGiven, s.FollowUpRequired, s.FollowUpDetails, s.ReferralID, fromMs(s.CreatedAt), fromMs(s.UpdatedAt))
	return err
}

func (r *OutreachRepository) ListScreenings(ctx context.Context, eventID string) ([]*outreach.Screening, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, patient_nhi, clinician_id, screening_type, screening_date, result_category, result_value, interpretation, consent_given, follow_up_required, follow_up_details, referral_id, created_at, updated_at
		FROM outreach_screenings WHERE event_id = $1 ORDER BY screening_date DESC
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var screenings []*outreach.Screening
	for rows.Next() {
		var s outreach.Screening
		var sd, ca, ua time.Time
		err := rows.Scan(&s.ID, &s.EventID, &s.PatientNHI, &s.ClinicianID, &s.ScreeningType, &sd, &s.ResultCategory, &s.ResultValue, &s.Interpretation, &s.ConsentGiven, &s.FollowUpRequired, &s.FollowUpDetails, &s.ReferralID, &ca, &ua)
		if err != nil {
			return nil, err
		}
		s.ScreeningDate = toMs(sd); s.CreatedAt = toMs(ca); s.UpdatedAt = toMs(ua)
		screenings = append(screenings, &s)
	}
	return screenings, rows.Err()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func scanProgram(s scanner) (*outreach.Program, error) {
	var p outreach.Program
	var sd, ca, ua time.Time
	var e *time.Time
	err := s.Scan(&p.ID, &p.PracticeID, &p.ProgramName, &p.ProgramType, &p.Description, &p.TargetPopulation, &p.Status, &sd, &e, &p.FundingSource, &p.FundingCode, &p.Budget, &p.Spent, &ca, &ua)
	if err != nil {
		return nil, err
	}
	p.StartDate = toMs(sd)
	if e != nil && !e.IsZero() { p.EndDate = toMs(*e) }
	p.CreatedAt = toMs(ca); p.UpdatedAt = toMs(ua)
	return &p, nil
}

func scanEvent(s scanner) (*outreach.Event, error) {
	var e outreach.Event
	var sd, ca, ua time.Time
	var lat, lon *float64
	var ta, aa *int
	err := s.Scan(&e.ID, &e.ProgramID, &e.EventName, &e.EventType, &sd, &e.EstimatedDuration, &e.LocationAddress, &lat, &lon, &e.VenueName, &e.VenueContact, &ta, &aa, &e.Clinicians, &e.EquipmentList, &e.Status, &e.CancellationReason, &e.Notes, &ca, &ua)
	if err != nil {
		return nil, err
	}
	e.ScheduledDate = toMs(sd)
	if lat != nil { e.Latitude = *lat }
	if lon != nil { e.Longitude = *lon }
	if ta != nil { e.TargetAttendees = *ta }
	if aa != nil { e.ActualAttendees = *aa }
	e.CreatedAt = toMs(ca); e.UpdatedAt = toMs(ua)
	return &e, nil
}
