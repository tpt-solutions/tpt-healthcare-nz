package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/acc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	errClaimNotFound   = fmt.Errorf("acc: claim not found")
	errClaimIneligible = fmt.Errorf("acc: claim not eligible for session")
)

// claimEligibility holds the fields from acc_claims needed to evaluate CanAddSession.
type claimEligibility struct {
	status           string
	approvedSessions int
	usedSessions     int
	expiryDate       time.Time
}

// loadClaimEligibility fetches the minimal claim fields needed for CanAddSession from the DB.
func loadClaimEligibility(ctx context.Context, pool *pgxpool.Pool, claimID string) (*claimEligibility, error) {
	const q = `
		SELECT status, approved_sessions, used_sessions, expiry_date
		FROM acc_claims
		WHERE id = $1`
	var e claimEligibility
	err := pool.QueryRow(ctx, q, claimID).Scan(
		&e.status,
		&e.approvedSessions,
		&e.usedSessions,
		&e.expiryDate,
	)
	if err != nil {
		return nil, fmt.Errorf("acc: load claim %s: %w", claimID, err)
	}
	return &e, nil
}

// toAccClaim converts the eligibility projection to an acc.Claim for CanAddSession.
func (e *claimEligibility) toAccClaim() acc.Claim {
	return acc.Claim{
		Status:           acc.ClaimStatus(e.status),
		ApprovedSessions: e.approvedSessions,
		UsedSessions:     e.usedSessions,
		ExpiryDate:       e.expiryDate.UnixMilli(),
	}
}

// CreateSession creates a new treatment session under a claim.
func (h *ACCHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	claimID := r.PathValue("id")

	var session acc.TreatmentSession
	if err := json.NewDecoder(r.Body).Decode(&session); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	session.ID = uuid.New().String()
	session.ClaimID = claimID
	now := time.Now().UnixMilli()
	session.CreatedAt = now
	session.UpdatedAt = now

	// Validates NHI checksum and verifies the charge code exists in StandardChargeCodes.
	if err := session.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chargeCode := acc.GetChargeCodeByCode(session.ChargeCode)
	session.ChargeAmount = chargeCode.Rate // safe: Validate() already confirmed existence.

	// Enforce claim eligibility and persist atomically.
	if err := h.createSessionTx(r.Context(), &session, claimID); err != nil {
		if err == errClaimNotFound {
			http.Error(w, "claim not found", http.StatusNotFound)
			return
		}
		if err == errClaimIneligible {
			http.Error(w, "claim is not eligible for additional sessions: check status, approved session count, and expiry date", http.StatusUnprocessableEntity)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session)
}

// createSessionTx runs the CanAddSession guard and persists the session and the
// updated used_sessions count in a single transaction.
func (h *ACCHandler) createSessionTx(ctx context.Context, session *acc.TreatmentSession, claimID string) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("acc: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Lock the claim row for the duration of the transaction so concurrent
	// session creation cannot exceed ApprovedSessions.
	const qClaim = `
		SELECT status, approved_sessions, used_sessions, expiry_date
		FROM acc_claims
		WHERE id = $1
		FOR UPDATE`

	var e claimEligibility
	err = tx.QueryRow(ctx, qClaim, claimID).Scan(
		&e.status,
		&e.approvedSessions,
		&e.usedSessions,
		&e.expiryDate,
	)
	if err != nil {
		// pgx returns pgx.ErrNoRows when the row doesn't exist; treat as 404.
		return errClaimNotFound
	}

	claim := e.toAccClaim()
	if !claim.CanAddSession() {
		return errClaimIneligible
	}

	// Persist the session.
	const qInsert = `
		INSERT INTO acc_treatment_sessions (
			id, claim_id, patient_nhi, clinician_id,
			session_date, session_number, duration_minutes,
			charge_code, charge_amount, treatment_type, body_region,
			subjective, objective, assessment, plan, status
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15, $16
		)`
	sessionDate := time.UnixMilli(session.SessionDate)
	_, err = tx.Exec(ctx, qInsert,
		session.ID, claimID, session.PatientNHI, session.ClinicianID,
		sessionDate, session.SessionNumber, session.DurationMinutes,
		session.ChargeCode, session.ChargeAmount, session.TreatmentType, session.BodyRegion,
		session.Subjective, session.Objective, session.Assessment, session.Plan,
		string(session.Status),
	)
	if err != nil {
		return fmt.Errorf("acc: insert session: %w", err)
	}

	// Increment the session counter on the claim.
	const qUpdate = `
		UPDATE acc_claims
		SET used_sessions      = used_sessions + 1,
		    last_treatment_date = $1,
		    updated_at          = NOW()
		WHERE id = $2`
	_, err = tx.Exec(ctx, qUpdate, sessionDate, claimID)
	if err != nil {
		return fmt.Errorf("acc: update claim used_sessions: %w", err)
	}

	return tx.Commit(ctx)
}

// ListSessions lists treatment sessions for a claim.
func (h *ACCHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	claimID := r.PathValue("id")
	limit, offset := parsePagination(r)

	sessions := []acc.TreatmentSession{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":     sessions,
		"limit":    limit,
		"offset":   offset,
		"total":    len(sessions),
		"claim_id": claimID,
	})
}
