package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

// --- Participant handler types (enum constants remain unchanged) ---

type participantHandler struct {
	handlerDeps
}

func (h *participantHandler) List(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, subject_id, status, randomisation_method, screened_at, enrolled_at,
		        randomised_at, completed_at, withdrawn_at, withdrawal_reason, notes,
		        created_at, updated_at
		 FROM ct_participants WHERE protocol_id = $1 ORDER BY created_at DESC`, protocolID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type participantRow struct {
		ID, SubjectID, Status, RandomisationMethod, WithdrawalReason, Notes string
		ScreenedAt, EnrolledAt, RandomisedAt, CompletedAt, WithdrawnAt      *time.Time
		CreatedAt, UpdatedAt                                                time.Time
	}
	var results []participantRow
	for rows.Next() {
		var p participantRow
		if err := rows.Scan(&p.ID, &p.SubjectID, &p.Status, &p.RandomisationMethod,
			&p.ScreenedAt, &p.EnrolledAt, &p.RandomisedAt, &p.CompletedAt,
			&p.WithdrawnAt, &p.WithdrawalReason, &p.Notes,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			h.logger.Error("scan participant row", "error", err)
			continue
		}
		results = append(results, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"participants": results, "total": len(results)})
}

func (h *participantHandler) Screen(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	var req struct {
		NHI, SubjectID, ScreenerHPI string
		Eligible                    bool
		ScreenFailReason            string
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}

	var participantID string
	now := time.Now().UTC()
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO ct_participants (protocol_id, tenant_id, participant_nhi, subject_id, status, screened_at)
		 VALUES ($1, '00000000-0000-0000-0000-000000000000', $2, $3, 'screened', $4) RETURNING id`,
		protocolID, req.NHI, req.SubjectID, now).Scan(&participantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}

	if !req.Eligible {
		_, _ = h.pool.Exec(r.Context(),
			`UPDATE ct_participants SET status='screen-failed', updated_at=now() WHERE id=$1`, participantID)
	}

	_, _ = h.pool.Exec(r.Context(),
		`INSERT INTO ct_screen_log (protocol_id, participant_id, screener_hpi, eligible, screen_fail_reason)
		 VALUES ($1, $2, $3, $4, $5)`,
		protocolID, participantID, req.ScreenerHPI, req.Eligible, req.ScreenFailReason)

	writeJSON(w, http.StatusCreated, map[string]any{"id": participantID, "eligible": req.Eligible})
}

func (h *participantHandler) Get(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	participantID := r.PathValue("participantId")
	var subjectID, status string
	err := h.pool.QueryRow(r.Context(),
		`SELECT subject_id, status FROM ct_participants WHERE id=$1 AND protocol_id=$2`,
		participantID, protocolID).Scan(&subjectID, &status)
	if err != nil {
		if db.IsNoRows(err) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "participant not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": participantID, "subjectId": subjectID, "status": status})
}

func (h *participantHandler) Enrol(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	participantID := r.PathValue("participantId")
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_participants SET status='enrolled', enrolled_at=now(), updated_at=now()
		 WHERE id=$1 AND protocol_id=$2 AND status='screened'`, participantID, protocolID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATE", Message: "participant must be in 'screened' status"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": participantID, "status": "enrolled"})
}

func (h *participantHandler) Randomise(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	participantID := r.PathValue("participantId")
	var req struct {
		Method string `json:"method"`
	}
	_ = decodeJSON(r, &req)
	if req.Method == "" {
		req.Method = "block"
	}

	// Pick a random arm
	var armID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT id FROM ct_protocol_arms WHERE protocol_id=$1 ORDER BY random() LIMIT 1`, protocolID,
	).Scan(&armID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "NO_ARMS", Message: "protocol has no arms defined"})
		return
	}

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_participants SET status='randomised', arm_id=$1, randomisation_method=$2,
		        randomised_at=now(), updated_at=now()
		 WHERE id=$3 AND protocol_id=$4 AND status='enrolled'`,
		armID, req.Method, participantID, protocolID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATE", Message: "participant must be enrolled"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": participantID, "armId": armID, "method": req.Method})
}

func (h *participantHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	participantID := r.PathValue("participantId")
	var req struct {
		Reason string `json:"reason"`
	}
	_ = decodeJSON(r, &req)

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_participants SET status='withdrawn', withdrawn_at=now(), withdrawal_reason=$1, updated_at=now()
		 WHERE id=$2 AND protocol_id=$3 AND status NOT IN ('withdrawn','completed')`,
		req.Reason, participantID, protocolID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATE", Message: "participant cannot be withdrawn"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": participantID, "status": "withdrawn"})
}

func (h *participantHandler) Complete(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	participantID := r.PathValue("participantId")
	tag, err := h.pool.Exec(r.Context(),
		`UPDATE ct_participants SET status='completed', completed_at=now(), updated_at=now()
		 WHERE id=$1 AND protocol_id=$2 AND status='active'`, participantID, protocolID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATE", Message: "participant must be active"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": participantID, "status": "completed"})
}

func (h *participantHandler) GetConsent(w http.ResponseWriter, r *http.Request) {
	participantID := r.PathValue("participantId")
	var participantUUID string
	err := h.pool.QueryRow(r.Context(),
		`SELECT id FROM ct_participants WHERE id=$1`, participantID).Scan(&participantUUID)
	if err != nil {
		if db.IsNoRows(err) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "participant not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, status, consented_at, withdrawn_at, obtained_by_hpi, version, created_at
		 FROM ct_consent_records WHERE participant_id=$1 ORDER BY created_at DESC`, participantID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type consentRecord struct {
		ID, Status, ObtainedByHPI, Version string
		ConsentedAt, WithdrawnAt           *time.Time
		CreatedAt                          time.Time
	}
	var records []consentRecord
	for rows.Next() {
		var c consentRecord
		if err := rows.Scan(&c.ID, &c.Status, &c.ConsentedAt, &c.WithdrawnAt,
			&c.ObtainedByHPI, &c.Version, &c.CreatedAt); err != nil {
			continue
		}
		records = append(records, c)
	}
	writeJSON(w, http.StatusOK, records)
}

func (h *participantHandler) UpdateConsent(w http.ResponseWriter, r *http.Request) {
	participantID := r.PathValue("participantId")
	var req struct {
		Status, ObtainedByHPI, Version string
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}

	var id string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO ct_consent_records (participant_id, status, consented_at, obtained_by_hpi, version)
		 VALUES ($1, $2, now(), $3, $4) RETURNING id`,
		participantID, req.Status, req.ObtainedByHPI, req.Version).Scan(&id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *participantHandler) ScreeningLog(w http.ResponseWriter, r *http.Request) {
	protocolID := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(),
		`SELECT sl.id, sl.screened_at, sl.screener_hpi, sl.eligible, sl.screen_fail_reason,
		        p.subject_id, p.participant_nhi
		 FROM ct_screen_log sl
		 LEFT JOIN ct_participants p ON p.id = sl.participant_id
		 WHERE sl.protocol_id = $1
		 ORDER BY sl.screened_at DESC LIMIT 100`, protocolID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	type logEntry struct {
		ID, ScreenerHPI, FailReason, SubjectID, NHI string
		ScreenedAt                                  time.Time
		Eligible                                    bool
	}
	var entries []logEntry
	for rows.Next() {
		var e logEntry
		if err := rows.Scan(&e.ID, &e.ScreenedAt, &e.ScreenerHPI, &e.Eligible,
			&e.FailReason, &e.SubjectID, &e.NHI); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	writeJSON(w, http.StatusOK, entries)
}
