package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ---------------------------------------------------------------------------
// Internal record types
// ---------------------------------------------------------------------------

type episodeRecord struct {
	ID                  string
	PatientID           string
	PatientNHI          string
	TenantID            string
	ResponsibleHPI      string
	EpisodeType         string
	Status              string
	AdmissionReasonEnc  []byte
	PrimaryDiagnosis    string
	SecondaryDiagnoses  []string
	WardOrTeam          string
	BedNumber           string
	AdmittedAt          *time.Time
	DischargedAt        *time.Time
	DischargeSummaryEnc []byte
	ExtraSensitive      bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type wardRoundRecord struct {
	ID             string
	EpisodeID      string
	PatientID      string
	PatientNHI     string
	TenantID       string
	ClinicianHPI   string
	NotesEnc       []byte
	MentalState    []byte
	RiskLevel      string
	PlansEnc       []byte
	ExtraSensitive bool
	OccurredAt     time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ---------------------------------------------------------------------------
// Episode database helpers
// ---------------------------------------------------------------------------

func (h *EpisodesHandler) listEpisodes(
	ctx context.Context,
	tenantID uuid.UUID,
	patientFilter, statusFilter, typeFilter string,
) ([]episodeRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		        episode_type, status, admission_reason,
		        primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		        admitted_at, discharged_at, discharge_summary,
		        extra_sensitive, created_at, updated_at
		 FROM mh_episodes
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR status = $3)
		   AND ($4 = '' OR episode_type = $4)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		tenantID, patientFilter, statusFilter, typeFilter,
	)
	if err != nil {
		return nil, fmt.Errorf("query episodes: %w", err)
	}
	defer rows.Close()

	var results []episodeRecord
	for rows.Next() {
		rec, err := scanEpisode(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (h *EpisodesHandler) getEpisodeByID(ctx context.Context, id string, tenantID uuid.UUID) (episodeRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		        episode_type, status, admission_reason,
		        primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		        admitted_at, discharged_at, discharge_summary,
		        extra_sensitive, created_at, updated_at
		 FROM mh_episodes
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanEpisodeRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return episodeRecord{}, errNotFound
		}
		return episodeRecord{}, fmt.Errorf("get episode by id: %w", err)
	}
	return rec, nil
}

func (h *EpisodesHandler) insertEpisode(ctx context.Context, req episodeCreateRequest, tenantID uuid.UUID) (episodeRecord, error) {
	reasonEnc, err := h.enc.Encrypt([]byte(req.AdmissionReason))
	if err != nil {
		return episodeRecord{}, fmt.Errorf("encrypt admission reason: %w", err)
	}

	admittedAt := req.AdmittedAt
	if admittedAt == nil {
		now := time.Now().UTC()
		admittedAt = &now
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO mh_episodes
		   (patient_id, patient_nhi, tenant_id, responsible_hpi,
		    episode_type, status, admission_reason,
		    primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		    admitted_at, extra_sensitive)
		 VALUES
		   ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, TRUE)
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           episode_type, status, admission_reason,
		           primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		           admitted_at, discharged_at, discharge_summary,
		           extra_sensitive, created_at, updated_at`,
		req.PatientID, req.PatientNHI, tenantID, req.ResponsibleHPI,
		string(req.EpisodeType), string(EpisodeActive), reasonEnc,
		req.PrimaryDiagnosis, req.SecondaryDiagnoses, req.WardOrTeam, req.BedNumber,
		admittedAt,
	)
	return scanEpisodeRow(row)
}

func (h *EpisodesHandler) updateEpisode(ctx context.Context, rec episodeRecord, tenantID uuid.UUID) (episodeRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE mh_episodes
		 SET responsible_hpi     = $1,
		     status              = $2,
		     primary_diagnosis   = $3,
		     secondary_diagnoses = $4,
		     ward_or_team        = $5,
		     bed_number          = $6,
		     updated_at          = now()
		 WHERE id = $7 AND tenant_id = $8
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           episode_type, status, admission_reason,
		           primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		           admitted_at, discharged_at, discharge_summary,
		           extra_sensitive, created_at, updated_at`,
		rec.ResponsibleHPI, rec.Status, rec.PrimaryDiagnosis,
		rec.SecondaryDiagnoses, rec.WardOrTeam, rec.BedNumber,
		rec.ID, tenantID,
	)
	updated, err := scanEpisodeRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return episodeRecord{}, errNotFound
		}
		return episodeRecord{}, fmt.Errorf("update episode: %w", err)
	}
	return updated, nil
}

func (h *EpisodesHandler) dischargeEpisode(
	ctx context.Context,
	id string,
	dischargedAt time.Time,
	status string,
	summaryEnc []byte,
	tenantID uuid.UUID,
) (episodeRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE mh_episodes
		 SET status            = $1,
		     discharged_at     = $2,
		     discharge_summary = $3,
		     updated_at        = now()
		 WHERE id = $4 AND tenant_id = $5
		 RETURNING id, patient_id, patient_nhi, tenant_id, responsible_hpi,
		           episode_type, status, admission_reason,
		           primary_diagnosis, secondary_diagnoses, ward_or_team, bed_number,
		           admitted_at, discharged_at, discharge_summary,
		           extra_sensitive, created_at, updated_at`,
		status, dischargedAt, summaryEnc, id, tenantID,
	)
	updated, err := scanEpisodeRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return episodeRecord{}, errNotFound
		}
		return episodeRecord{}, fmt.Errorf("discharge episode: %w", err)
	}
	return updated, nil
}

func (h *EpisodesHandler) decryptEpisode(rec episodeRecord) (Episode, error) {
	var reason string
	if len(rec.AdmissionReasonEnc) > 0 {
		plain, err := h.enc.Decrypt(rec.AdmissionReasonEnc)
		if err != nil {
			return Episode{}, fmt.Errorf("decrypt admission reason: %w", err)
		}
		reason = string(plain)
	}
	return Episode{
		ID:                 rec.ID,
		PatientID:          rec.PatientID,
		PatientNHI:         rec.PatientNHI,
		TenantID:           rec.TenantID,
		ResponsibleHPI:     rec.ResponsibleHPI,
		EpisodeType:        EpisodeType(rec.EpisodeType),
		Status:             EpisodeStatus(rec.Status),
		AdmissionReason:    reason,
		PrimaryDiagnosis:   rec.PrimaryDiagnosis,
		SecondaryDiagnoses: rec.SecondaryDiagnoses,
		WardOrTeam:         rec.WardOrTeam,
		BedNumber:          rec.BedNumber,
		AdmittedAt:         rec.AdmittedAt,
		DischargedAt:       rec.DischargedAt,
		ExtraSensitive:     rec.ExtraSensitive,
		CreatedAt:          rec.CreatedAt,
		UpdatedAt:          rec.UpdatedAt,
	}, nil
}

func scanEpisode(s rowScanner) (episodeRecord, error) {
	return scanEpisodeRow(s)
}

func scanEpisodeRow(s rowScanner) (episodeRecord, error) {
	var rec episodeRecord
	if err := s.Scan(
		&rec.ID, &rec.PatientID, &rec.PatientNHI, &rec.TenantID, &rec.ResponsibleHPI,
		&rec.EpisodeType, &rec.Status, &rec.AdmissionReasonEnc,
		&rec.PrimaryDiagnosis, &rec.SecondaryDiagnoses, &rec.WardOrTeam, &rec.BedNumber,
		&rec.AdmittedAt, &rec.DischargedAt, &rec.DischargeSummaryEnc,
		&rec.ExtraSensitive, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return episodeRecord{}, err
	}
	return rec, nil
}

// ---------------------------------------------------------------------------
// Ward round database helpers
// ---------------------------------------------------------------------------

func (h *EpisodesHandler) listWardRounds(ctx context.Context, episodeID string, tenantID uuid.UUID) ([]WardRound, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, episode_id, patient_id, patient_nhi, tenant_id,
		        clinician_hpi, notes, mental_state, risk_level, plans,
		        extra_sensitive, occurred_at, created_at, updated_at
		 FROM mh_ward_rounds
		 WHERE episode_id = $1 AND tenant_id = $2
		 ORDER BY occurred_at DESC
		 LIMIT 200`,
		episodeID, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("query ward rounds: %w", err)
	}
	defer rows.Close()

	var results []WardRound
	for rows.Next() {
		rec, err := scanWardRound(rows)
		if err != nil {
			return nil, err
		}
		wr, err := h.decryptWardRound(rec)
		if err != nil {
			return nil, err
		}
		results = append(results, wr)
	}
	return results, rows.Err()
}

func (h *EpisodesHandler) getWardRoundByID(ctx context.Context, id, episodeID string, tenantID uuid.UUID) (WardRound, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, episode_id, patient_id, patient_nhi, tenant_id,
		        clinician_hpi, notes, mental_state, risk_level, plans,
		        extra_sensitive, occurred_at, created_at, updated_at
		 FROM mh_ward_rounds
		 WHERE id = $1 AND episode_id = $2 AND tenant_id = $3`,
		id, episodeID, tenantID,
	)
	rec, err := scanWardRoundRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WardRound{}, errNotFound
		}
		return WardRound{}, fmt.Errorf("get ward round by id: %w", err)
	}
	return h.decryptWardRound(rec)
}

func (h *EpisodesHandler) insertWardRound(
	ctx context.Context,
	episodeID string,
	ep episodeRecord,
	req wardRoundCreateRequest,
	tenantID uuid.UUID,
) (WardRound, error) {
	notesEnc, err := h.enc.Encrypt([]byte(req.Notes))
	if err != nil {
		return WardRound{}, fmt.Errorf("encrypt notes: %w", err)
	}
	plansEnc, err := h.enc.Encrypt([]byte(req.Plans))
	if err != nil {
		return WardRound{}, fmt.Errorf("encrypt plans: %w", err)
	}

	mentalStateJSON, err := json.Marshal(req.MentalState)
	if err != nil {
		return WardRound{}, fmt.Errorf("marshal mental state: %w", err)
	}

	occurredAt := time.Now().UTC()
	if req.OccurredAt != nil {
		occurredAt = *req.OccurredAt
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO mh_ward_rounds
		   (episode_id, patient_id, patient_nhi, tenant_id,
		    clinician_hpi, notes, mental_state, risk_level, plans,
		    extra_sensitive, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, $10)
		 RETURNING id, episode_id, patient_id, patient_nhi, tenant_id,
		           clinician_hpi, notes, mental_state, risk_level, plans,
		           extra_sensitive, occurred_at, created_at, updated_at`,
		episodeID, ep.PatientID, ep.PatientNHI, tenantID,
		req.ClinicianHPI, notesEnc, mentalStateJSON, string(req.RiskLevel), plansEnc,
		occurredAt,
	)
	rec, err := scanWardRoundRow(row)
	if err != nil {
		return WardRound{}, fmt.Errorf("insert ward round: %w", err)
	}
	return h.decryptWardRound(rec)
}

func (h *EpisodesHandler) decryptWardRound(rec wardRoundRecord) (WardRound, error) {
	var notes, plans string
	if len(rec.NotesEnc) > 0 {
		plain, err := h.enc.Decrypt(rec.NotesEnc)
		if err != nil {
			return WardRound{}, fmt.Errorf("decrypt notes: %w", err)
		}
		notes = string(plain)
	}
	if len(rec.PlansEnc) > 0 {
		plain, err := h.enc.Decrypt(rec.PlansEnc)
		if err != nil {
			return WardRound{}, fmt.Errorf("decrypt plans: %w", err)
		}
		plans = string(plain)
	}

	var mentalState map[string]any
	if len(rec.MentalState) > 0 {
		if err := json.Unmarshal(rec.MentalState, &mentalState); err != nil {
			mentalState = map[string]any{}
		}
	}

	return WardRound{
		ID:             rec.ID,
		EpisodeID:      rec.EpisodeID,
		PatientID:      rec.PatientID,
		PatientNHI:     rec.PatientNHI,
		TenantID:       rec.TenantID,
		ClinicianHPI:   rec.ClinicianHPI,
		Notes:          notes,
		MentalState:    mentalState,
		RiskLevel:      RiskLevel(rec.RiskLevel),
		Plans:          plans,
		ExtraSensitive: rec.ExtraSensitive,
		OccurredAt:     rec.OccurredAt,
		CreatedAt:      rec.CreatedAt,
		UpdatedAt:      rec.UpdatedAt,
	}, nil
}

func scanWardRound(s rowScanner) (wardRoundRecord, error) {
	return scanWardRoundRow(s)
}

func scanWardRoundRow(s rowScanner) (wardRoundRecord, error) {
	var rec wardRoundRecord
	if err := s.Scan(
		&rec.ID, &rec.EpisodeID, &rec.PatientID, &rec.PatientNHI, &rec.TenantID,
		&rec.ClinicianHPI, &rec.NotesEnc, &rec.MentalState, &rec.RiskLevel, &rec.PlansEnc,
		&rec.ExtraSensitive, &rec.OccurredAt, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return wardRoundRecord{}, err
	}
	return rec, nil
}

func validEpisodeType(t EpisodeType) bool {
	switch t {
	case EpisodeInpatient, EpisodeCommunity, EpisodeCrisis, EpisodeDayProgramme:
		return true
	}
	return false
}
