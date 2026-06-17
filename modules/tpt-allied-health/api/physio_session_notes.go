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
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
)

// CreateSessionNote creates a new session note.
func (h *PhysioHandler) CreateSessionNote(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var note physio.SessionNote
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	note.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	note.CreatedAt = now
	note.UpdatedAt = now

	if err := note.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(note)
}

// GetSessionNote retrieves a session note by ID.
func (h *PhysioHandler) GetSessionNote(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	ctx := r.Context()

	var note physio.SessionNote
	var sessionDate, createdAt, updatedAt time.Time
	var chargeCode string

	err := h.pool.QueryRow(ctx, `
		SELECT id, patient_nhi, clinician_id::text, practice_id::text,
		       COALESCE(treatment_plan_id::text,''), session_date, session_number,
		       COALESCE(subjective,''), COALESCE(objective,''), COALESCE(assessment,''),
		       COALESCE(plan,''), duration_minutes, COALESCE(charge_code,''),
		       created_at, updated_at
		FROM physio_session_notes WHERE id=$1`,
		id,
	).Scan(
		&note.ID, &note.PatientNHI, &note.ClinicianID, &note.PracticeID,
		&note.TreatmentPlanID, &sessionDate, &note.SessionNumber,
		&note.Subjective, &note.Objective, &note.Assessment,
		&note.Plan, &note.DurationMinutes, &chargeCode,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "session note not found", http.StatusNotFound)
			return
		}
		h.logger.Error("get session note", slog.Any("error", err))
		http.Error(w, "failed to fetch session note", http.StatusInternalServerError)
		return
	}

	note.ChargeCode = chargeCode
	note.SessionDate = sessionDate.UnixMilli()
	note.CreatedAt = createdAt.UnixMilli()
	note.UpdatedAt = updatedAt.UnixMilli()
	note.Interventions = []physio.Intervention{}
	note.OutcomeMeasures = []physio.OutcomeMeasure{}

	if !checkConsent(w, r, h.consentStore, note.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// ListSessionNotes lists session notes with filters.
func (h *PhysioHandler) ListSessionNotes(w http.ResponseWriter, r *http.Request) {
	qp := r.URL.Query()
	patientNHI := qp.Get("patient_nhi")
	treatmentPlanID := qp.Get("treatment_plan_id")
	limit, offset := parsePagination(r)
	ctx := r.Context()

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	args := make([]any, 0, 4)
	conds := make([]string, 0, 2)
	if patientNHI != "" {
		args = append(args, patientNHI)
		conds = append(conds, fmt.Sprintf("patient_nhi=$%d", len(args)))
	}
	if treatmentPlanID != "" {
		args = append(args, treatmentPlanID)
		conds = append(conds, fmt.Sprintf("treatment_plan_id::text=$%d", len(args)))
	}
	q := `SELECT id, patient_nhi, clinician_id::text, practice_id::text,
	             COALESCE(treatment_plan_id::text,''), session_date, session_number,
	             COALESCE(subjective,''), COALESCE(objective,''), COALESCE(assessment,''),
	             COALESCE(plan,''), duration_minutes, COALESCE(charge_code,''),
	             created_at, updated_at
	      FROM physio_session_notes`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit, offset)
	q += fmt.Sprintf(" ORDER BY session_date DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := h.pool.Query(ctx, q, args...)
	if err != nil {
		h.logger.Error("list session notes", slog.Any("error", err))
		http.Error(w, "failed to list session notes", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	notes := make([]physio.SessionNote, 0)
	for rows.Next() {
		var n physio.SessionNote
		var sessionDate, createdAt, updatedAt time.Time
		var chargeCode string
		if err := rows.Scan(
			&n.ID, &n.PatientNHI, &n.ClinicianID, &n.PracticeID,
			&n.TreatmentPlanID, &sessionDate, &n.SessionNumber,
			&n.Subjective, &n.Objective, &n.Assessment,
			&n.Plan, &n.DurationMinutes, &chargeCode,
			&createdAt, &updatedAt,
		); err != nil {
			h.logger.Error("scan session note row", slog.Any("error", err))
			continue
		}
		n.ChargeCode = chargeCode
		n.SessionDate = sessionDate.UnixMilli()
		n.CreatedAt = createdAt.UnixMilli()
		n.UpdatedAt = updatedAt.UnixMilli()
		n.Interventions = []physio.SessionIntervention{}
		n.OutcomeMeasures = []physio.SessionOutcomeMeasure{}
		notes = append(notes, n)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   notes,
		"limit":  limit,
		"offset": offset,
		"total":  len(notes),
		"filters": map[string]string{
			"patient_nhi":       patientNHI,
			"treatment_plan_id": treatmentPlanID,
		},
	})
}

// UpdateSessionNote updates a session note.
func (h *PhysioHandler) UpdateSessionNote(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	id := mux.Vars(r)["id"]

	var note physio.SessionNote
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	note.ID = id
	note.UpdatedAt = time.Now().UnixMilli()

	if err := note.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// ListOutcomeMeasures lists standardised outcome measures.
func (h *PhysioHandler) ListOutcomeMeasures(w http.ResponseWriter, r *http.Request) {
	measures := []map[string]string{
		{"code": "NDI", "name": "Neck Disability Index", "domain": "cervical_spine"},
		{"code": "ODI", "name": "Oswestry Disability Index", "domain": "lumbar_spine"},
		{"code": "DASH", "name": "Disabilities of Arm, Shoulder and Hand", "domain": "upper_limb"},
		{"code": "LEFS", "name": "Lower Extremity Functional Scale", "domain": "lower_limb"},
		{"code": "FABQ", "name": "Fear-Avoidance Beliefs Questionnaire", "domain": "psychosocial"},
		{"code": "TSK", "name": "Tampa Scale of Kinesiophobia", "domain": "psychosocial"},
		{"code": "VAS", "name": "Visual Analogue Scale", "domain": "pain"},
		{"code": "NPRS", "name": "Numeric Pain Rating Scale", "domain": "pain"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(measures)
}
