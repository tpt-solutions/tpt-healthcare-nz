package api

import (
	"context"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

func (h *HITHHandler) listEpisodes(ctx context.Context, tenantID, statusFilter string) ([]HITHEpisode, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, linked_admission_id, lead_clinician_hpi, status,
		        diagnosis, care_goals, daily_visit_frequency, home_address, emergency_contact,
		        patient_consented, tenant_id, start_date, expected_end_date, actual_end_date,
		        created_at, updated_at
		 FROM hith_episodes
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY start_date DESC`,
		db.NamedArgs{"tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query HITH episodes: %w", err)
	}
	defer rows.Close()

	var results []HITHEpisode
	for rows.Next() {
		ep, err := scanHITHEpisodeRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, ep)
	}
	return results, rows.Err()
}

func (h *HITHHandler) getEpisodeByID(ctx context.Context, id, tenantID string) (HITHEpisode, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, linked_admission_id, lead_clinician_hpi, status,
		        diagnosis, care_goals, daily_visit_frequency, home_address, emergency_contact,
		        patient_consented, tenant_id, start_date, expected_end_date, actual_end_date,
		        created_at, updated_at
		 FROM hith_episodes
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	ep, err := scanHITHEpisodeRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return HITHEpisode{}, errNotFound
		}
		return HITHEpisode{}, fmt.Errorf("get HITH episode: %w", err)
	}
	return ep, nil
}

func (h *HITHHandler) insertEpisode(ctx context.Context, req hithEpisodeCreateRequest, tenantID string) (HITHEpisode, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO hith_episodes
		   (patient_id, patient_nhi, linked_admission_id, lead_clinician_hpi, status,
		    diagnosis, care_goals, daily_visit_frequency, home_address, emergency_contact,
		    patient_consented, tenant_id, start_date, expected_end_date)
		 VALUES
		   (@patient_id, @patient_nhi, @linked_admission_id, @lead_clinician_hpi, @status,
		    @diagnosis, @care_goals, @daily_visit_frequency, @home_address, @emergency_contact,
		    @patient_consented, @tenant_id, @start_date, @expected_end_date)
		 RETURNING id, patient_id, patient_nhi, linked_admission_id, lead_clinician_hpi, status,
		           diagnosis, care_goals, daily_visit_frequency, home_address, emergency_contact,
		           patient_consented, tenant_id, start_date, expected_end_date, actual_end_date,
		           created_at, updated_at`,
		db.NamedArgs{
			"patient_id":            req.PatientID,
			"patient_nhi":           req.PatientNHI,
			"linked_admission_id":   req.LinkedAdmissionID,
			"lead_clinician_hpi":    req.LeadClinicianHPI,
			"status":                HITHStatusActive,
			"diagnosis":             req.Diagnosis,
			"care_goals":            req.CareGoals,
			"daily_visit_frequency": req.DailyVisitFreq,
			"home_address":          req.HomeAddress,
			"emergency_contact":     req.EmergencyContact,
			"patient_consented":     req.PatientConsented,
			"tenant_id":             tenantID,
			"start_date":            req.StartDate,
			"expected_end_date":     req.ExpectedEndDate,
		},
	)
	return scanHITHEpisodeRow(row)
}

func (h *HITHHandler) updateEpisode(ctx context.Context, ep HITHEpisode) (HITHEpisode, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE hith_episodes
		 SET lead_clinician_hpi    = @lead_clinician_hpi,
		     status                = @status,
		     care_goals            = @care_goals,
		     daily_visit_frequency = @daily_visit_frequency,
		     expected_end_date     = @expected_end_date,
		     actual_end_date       = @actual_end_date,
		     updated_at            = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, patient_id, patient_nhi, linked_admission_id, lead_clinician_hpi, status,
		           diagnosis, care_goals, daily_visit_frequency, home_address, emergency_contact,
		           patient_consented, tenant_id, start_date, expected_end_date, actual_end_date,
		           created_at, updated_at`,
		db.NamedArgs{
			"lead_clinician_hpi":    ep.LeadClinicianHPI,
			"status":                ep.Status,
			"care_goals":            ep.CareGoals,
			"daily_visit_frequency": ep.DailyVisitFreq,
			"expected_end_date":     ep.ExpectedEndDate,
			"actual_end_date":       ep.ActualEndDate,
			"id":                    ep.ID,
			"tenant_id":             ep.TenantID,
		},
	)
	updated, err := scanHITHEpisodeRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return HITHEpisode{}, errNotFound
		}
		return HITHEpisode{}, fmt.Errorf("update HITH episode: %w", err)
	}
	return updated, nil
}

func (h *HITHHandler) insertVisit(ctx context.Context, episodeID string, req hithVisitRequest, tenantID string) (HITHVisit, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO hith_visits
		   (episode_id, clinician_hpi, visit_type, vitals, clinical_notes,
		    escalated, escalation_note, next_visit_date, tenant_id, visited_at)
		 VALUES
		   (@episode_id, @clinician_hpi, @visit_type, @vitals, @clinical_notes,
		    @escalated, @escalation_note, @next_visit_date, @tenant_id, now())
		 RETURNING id, episode_id, clinician_hpi, visit_type, vitals, clinical_notes,
		           escalated, escalation_note, next_visit_date, tenant_id, visited_at, created_at`,
		db.NamedArgs{
			"episode_id":      episodeID,
			"clinician_hpi":   req.CliniciandHPI,
			"visit_type":      req.VisitType,
			"vitals":          req.Vitals,
			"clinical_notes":  req.ClinicalNotes,
			"escalated":       req.Escalated,
			"escalation_note": req.EscalationNote,
			"next_visit_date": req.NextVisitDate,
			"tenant_id":       tenantID,
		},
	)
	return scanHITHVisitRow(row)
}

func (h *HITHHandler) listVisits(ctx context.Context, episodeID, tenantID string) ([]HITHVisit, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, episode_id, clinician_hpi, visit_type, vitals, clinical_notes,
		        escalated, escalation_note, next_visit_date, tenant_id, visited_at, created_at
		 FROM hith_visits
		 WHERE episode_id = @episode_id AND tenant_id = @tenant_id
		 ORDER BY visited_at DESC`,
		db.NamedArgs{"episode_id": episodeID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query HITH visits: %w", err)
	}
	defer rows.Close()

	var results []HITHVisit
	for rows.Next() {
		v, err := scanHITHVisitRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, v)
	}
	return results, rows.Err()
}

func (h *HITHHandler) updateVisit(ctx context.Context, visitID, episodeID string, req hithVisitRequest, tenantID string) (HITHVisit, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE hith_visits
		 SET vitals = @vitals, clinical_notes = @clinical_notes,
		     escalated = @escalated, escalation_note = @escalation_note,
		     next_visit_date = @next_visit_date
		 WHERE id = @id AND episode_id = @episode_id AND tenant_id = @tenant_id
		 RETURNING id, episode_id, clinician_hpi, visit_type, vitals, clinical_notes,
		           escalated, escalation_note, next_visit_date, tenant_id, visited_at, created_at`,
		db.NamedArgs{
			"vitals":          req.Vitals,
			"clinical_notes":  req.ClinicalNotes,
			"escalated":       req.Escalated,
			"escalation_note": req.EscalationNote,
			"next_visit_date": req.NextVisitDate,
			"id":              visitID,
			"episode_id":      episodeID,
			"tenant_id":       tenantID,
		},
	)
	v, err := scanHITHVisitRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return HITHVisit{}, errNotFound
		}
		return HITHVisit{}, fmt.Errorf("update HITH visit: %w", err)
	}
	return v, nil
}

func scanHITHEpisodeRow(row dbRow) (HITHEpisode, error) {
	var ep HITHEpisode
	if err := row.Scan(
		&ep.ID, &ep.PatientID, &ep.PatientNHI, &ep.LinkedAdmissionID, &ep.LeadClinicianHPI, &ep.Status,
		&ep.Diagnosis, &ep.CareGoals, &ep.DailyVisitFreq, &ep.HomeAddress, &ep.EmergencyContact,
		&ep.PatientConsented, &ep.TenantID, &ep.StartDate, &ep.ExpectedEndDate, &ep.ActualEndDate,
		&ep.CreatedAt, &ep.UpdatedAt,
	); err != nil {
		return HITHEpisode{}, err
	}
	return ep, nil
}

func scanHITHVisitRow(row dbRow) (HITHVisit, error) {
	var v HITHVisit
	if err := row.Scan(
		&v.ID, &v.EpisodeID, &v.CliniciandHPI, &v.VisitType, &v.Vitals, &v.ClinicalNotes,
		&v.Escalated, &v.EscalationNote, &v.NextVisitDate, &v.TenantID, &v.VisitedAt, &v.CreatedAt,
	); err != nil {
		return HITHVisit{}, err
	}
	return v, nil
}
