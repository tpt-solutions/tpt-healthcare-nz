package api

import (
	"context"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

func (h *OutpatientHandler) listClinics(ctx context.Context, tenantID, specialty string) ([]OutpatientClinic, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, name, specialty, lead_clinician_hpi, location, active, tenant_id, created_at
		 FROM outpatient_clinics
		 WHERE tenant_id = @tenant_id AND active = true
		   AND (@specialty = '' OR specialty = @specialty)
		 ORDER BY name`,
		db.NamedArgs{"tenant_id": tenantID, "specialty": specialty},
	)
	if err != nil {
		return nil, fmt.Errorf("query outpatient clinics: %w", err)
	}
	defer rows.Close()

	var results []OutpatientClinic
	for rows.Next() {
		var c OutpatientClinic
		if err := rows.Scan(&c.ID, &c.Name, &c.Specialty, &c.LeadClinicianHPI, &c.Location, &c.Active, &c.TenantID, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan clinic: %w", err)
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

func (h *OutpatientHandler) getClinicByID(ctx context.Context, id, tenantID string) (OutpatientClinic, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, name, specialty, lead_clinician_hpi, location, active, tenant_id, created_at
		 FROM outpatient_clinics WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	var c OutpatientClinic
	if err := row.Scan(&c.ID, &c.Name, &c.Specialty, &c.LeadClinicianHPI, &c.Location, &c.Active, &c.TenantID, &c.CreatedAt); err != nil {
		if db.IsNoRows(err) {
			return OutpatientClinic{}, errNotFound
		}
		return OutpatientClinic{}, fmt.Errorf("get clinic: %w", err)
	}
	return c, nil
}

func (h *OutpatientHandler) listAppointments(ctx context.Context, clinicID, tenantID, statusFilter string) ([]OutpatientAppointment, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, clinic_id, patient_id, patient_nhi, clinician_hpi, status, referral_id,
		        reason, notes, scheduled_at, attended_at, tenant_id, created_at, updated_at
		 FROM outpatient_appointments
		 WHERE clinic_id = @clinic_id AND tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY scheduled_at ASC`,
		db.NamedArgs{"clinic_id": clinicID, "tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query outpatient appointments: %w", err)
	}
	defer rows.Close()

	var results []OutpatientAppointment
	for rows.Next() {
		a, err := scanOPApptRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

func (h *OutpatientHandler) getAppointmentByID(ctx context.Context, id, clinicID, tenantID string) (OutpatientAppointment, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, clinic_id, patient_id, patient_nhi, clinician_hpi, status, referral_id,
		        reason, notes, scheduled_at, attended_at, tenant_id, created_at, updated_at
		 FROM outpatient_appointments
		 WHERE id = @id AND clinic_id = @clinic_id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "clinic_id": clinicID, "tenant_id": tenantID},
	)
	a, err := scanOPApptRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return OutpatientAppointment{}, errNotFound
		}
		return OutpatientAppointment{}, fmt.Errorf("get outpatient appointment: %w", err)
	}
	return a, nil
}

func (h *OutpatientHandler) insertAppointment(ctx context.Context, clinicID string, req opAppointmentCreateRequest, tenantID string) (OutpatientAppointment, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO outpatient_appointments
		   (clinic_id, patient_id, patient_nhi, clinician_hpi, status, referral_id, reason, scheduled_at, tenant_id)
		 VALUES
		   (@clinic_id, @patient_id, @patient_nhi, @clinician_hpi, @status, @referral_id, @reason, @scheduled_at, @tenant_id)
		 RETURNING id, clinic_id, patient_id, patient_nhi, clinician_hpi, status, referral_id,
		           reason, notes, scheduled_at, attended_at, tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"clinic_id":     clinicID,
			"patient_id":    req.PatientID,
			"patient_nhi":   req.PatientNHI,
			"clinician_hpi": req.ClinicianHPI,
			"status":        OPApptBooked,
			"referral_id":   req.ReferralID,
			"reason":        req.Reason,
			"scheduled_at":  req.ScheduledAt,
			"tenant_id":     tenantID,
		},
	)
	return scanOPApptRow(row)
}

func (h *OutpatientHandler) updateAppointment(ctx context.Context, a OutpatientAppointment) (OutpatientAppointment, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE outpatient_appointments
		 SET clinician_hpi = @clinician_hpi, status = @status, reason = @reason, notes = @notes,
		     scheduled_at = @scheduled_at, attended_at = @attended_at, updated_at = now()
		 WHERE id = @id AND clinic_id = @clinic_id AND tenant_id = @tenant_id
		 RETURNING id, clinic_id, patient_id, patient_nhi, clinician_hpi, status, referral_id,
		           reason, notes, scheduled_at, attended_at, tenant_id, created_at, updated_at`,
		db.NamedArgs{
			"clinician_hpi": a.ClinicianHPI,
			"status":        a.Status,
			"reason":        a.Reason,
			"notes":         a.Notes,
			"scheduled_at":  a.ScheduledAt,
			"attended_at":   a.AttendedAt,
			"id":            a.ID,
			"clinic_id":     a.ClinicID,
			"tenant_id":     a.TenantID,
		},
	)
	updated, err := scanOPApptRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return OutpatientAppointment{}, errNotFound
		}
		return OutpatientAppointment{}, fmt.Errorf("update outpatient appointment: %w", err)
	}
	return updated, nil
}

func (h *OutpatientHandler) listWaitlist(ctx context.Context, tenantID, clinicFilter, priorityFilter string) ([]WaitlistEntry, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, clinic_id, patient_id, patient_nhi, priority, reason, referral_id,
		        added_at, target_date, appointment_id, tenant_id
		 FROM outpatient_waitlist
		 WHERE tenant_id = @tenant_id AND appointment_id IS NULL
		   AND (@clinic_filter    = '' OR clinic_id = @clinic_filter)
		   AND (@priority_filter  = '' OR priority = @priority_filter)
		 ORDER BY CASE priority WHEN 'urgent' THEN 1 WHEN 'semi-urgent' THEN 2 ELSE 3 END, added_at ASC`,
		db.NamedArgs{"tenant_id": tenantID, "clinic_filter": clinicFilter, "priority_filter": priorityFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query waitlist: %w", err)
	}
	defer rows.Close()

	var results []WaitlistEntry
	for rows.Next() {
		var e WaitlistEntry
		if err := rows.Scan(
			&e.ID, &e.ClinicID, &e.PatientID, &e.PatientNHI, &e.Priority, &e.Reason, &e.ReferralID,
			&e.AddedAt, &e.TargetDate, &e.AppointmentID, &e.TenantID,
		); err != nil {
			return nil, fmt.Errorf("scan waitlist entry: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (h *OutpatientHandler) insertWaitlistEntry(ctx context.Context, req waitlistAddRequest, tenantID string) (WaitlistEntry, error) {
	priority := req.Priority
	if priority == "" {
		priority = WaitlistRoutine
	}
	row := h.pool.QueryRow(ctx,
		`INSERT INTO outpatient_waitlist
		   (clinic_id, patient_id, patient_nhi, priority, reason, referral_id, target_date, tenant_id, added_at)
		 VALUES
		   (@clinic_id, @patient_id, @patient_nhi, @priority, @reason, @referral_id, @target_date, @tenant_id, now())
		 RETURNING id, clinic_id, patient_id, patient_nhi, priority, reason, referral_id,
		           added_at, target_date, appointment_id, tenant_id`,
		db.NamedArgs{
			"clinic_id":   req.ClinicID,
			"patient_id":  req.PatientID,
			"patient_nhi": req.PatientNHI,
			"priority":    priority,
			"reason":      req.Reason,
			"referral_id": req.ReferralID,
			"target_date": req.TargetDate,
			"tenant_id":   tenantID,
		},
	)
	var e WaitlistEntry
	if err := row.Scan(
		&e.ID, &e.ClinicID, &e.PatientID, &e.PatientNHI, &e.Priority, &e.Reason, &e.ReferralID,
		&e.AddedAt, &e.TargetDate, &e.AppointmentID, &e.TenantID,
	); err != nil {
		return WaitlistEntry{}, fmt.Errorf("insert waitlist entry: %w", err)
	}
	return e, nil
}

func (h *OutpatientHandler) updateWaitlistEntry(ctx context.Context, id string, priority WaitlistPriority, targetDate *time.Time, tenantID string) (WaitlistEntry, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE outpatient_waitlist
		 SET priority = COALESCE(NULLIF(@priority, ''), priority),
		     target_date = COALESCE(@target_date, target_date)
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, clinic_id, patient_id, patient_nhi, priority, reason, referral_id,
		           added_at, target_date, appointment_id, tenant_id`,
		db.NamedArgs{"priority": priority, "target_date": targetDate, "id": id, "tenant_id": tenantID},
	)
	var e WaitlistEntry
	if err := row.Scan(
		&e.ID, &e.ClinicID, &e.PatientID, &e.PatientNHI, &e.Priority, &e.Reason, &e.ReferralID,
		&e.AddedAt, &e.TargetDate, &e.AppointmentID, &e.TenantID,
	); err != nil {
		if db.IsNoRows(err) {
			return WaitlistEntry{}, errNotFound
		}
		return WaitlistEntry{}, fmt.Errorf("update waitlist entry: %w", err)
	}
	return e, nil
}

func (h *OutpatientHandler) deleteWaitlistEntry(ctx context.Context, id, tenantID string) error {
	tag, err := h.pool.Exec(ctx,
		`DELETE FROM outpatient_waitlist WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	if err != nil {
		return fmt.Errorf("delete waitlist entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

func scanOPApptRow(row dbRow) (OutpatientAppointment, error) {
	var a OutpatientAppointment
	if err := row.Scan(
		&a.ID, &a.ClinicID, &a.PatientID, &a.PatientNHI, &a.ClinicianHPI, &a.Status, &a.ReferralID,
		&a.Reason, &a.Notes, &a.ScheduledAt, &a.AttendedAt, &a.TenantID, &a.CreatedAt, &a.UpdatedAt,
	); err != nil {
		return OutpatientAppointment{}, err
	}
	return a, nil
}
