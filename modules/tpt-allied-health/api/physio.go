// Package api implements HTTP handlers for allied health services.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/acc"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/physio"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PhysioHandler handles physiotherapy API endpoints.
type PhysioHandler struct {
	hpiClient    *hpi.Client
	consentStore *consent.Store
}

// NewPhysioHandler creates a new physio handler.
func NewPhysioHandler(hpiClient *hpi.Client, consentStore *consent.Store) *PhysioHandler {
	return &PhysioHandler{hpiClient: hpiClient, consentStore: consentStore}
}

// RegisterRoutes registers physio routes.
func (h *PhysioHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/physio/treatment-plans", h.CreateTreatmentPlan).Methods("POST")
	r.HandleFunc("/api/v1/physio/treatment-plans", h.ListTreatmentPlans).Methods("GET")
	r.HandleFunc("/api/v1/physio/treatment-plans/{id}", h.GetTreatmentPlan).Methods("GET")
	r.HandleFunc("/api/v1/physio/treatment-plans/{id}", h.UpdateTreatmentPlan).Methods("PUT")
	r.HandleFunc("/api/v1/physio/treatment-plans/{id}", h.DeleteTreatmentPlan).Methods("DELETE")

	r.HandleFunc("/api/v1/physio/session-notes", h.CreateSessionNote).Methods("POST")
	r.HandleFunc("/api/v1/physio/session-notes", h.ListSessionNotes).Methods("GET")
	r.HandleFunc("/api/v1/physio/session-notes/{id}", h.GetSessionNote).Methods("GET")
	r.HandleFunc("/api/v1/physio/session-notes/{id}", h.UpdateSessionNote).Methods("PUT")

	r.HandleFunc("/api/v1/physio/outcome-measures", h.ListOutcomeMeasures).Methods("GET")
}

// requireAPC validates that the authenticated clinician holds a current APC.
// Returns false and writes a 403 if the check fails. If hpiClient is nil the
// check is skipped (development/test mode).
func requireAPC(w http.ResponseWriter, r *http.Request, hpiClient *hpi.Client) bool {
	if hpiClient == nil {
		return true
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || !principal.Practitioner || principal.PractitionerID == "" {
		http.Error(w, "forbidden: authenticated principal is not a registered practitioner", http.StatusForbidden)
		return false
	}
	valid, err := hpiClient.ValidateAPC(r.Context(), principal.PractitionerID)
	if err != nil {
		http.Error(w, "forbidden: APC validation failed: "+err.Error(), http.StatusForbidden)
		return false
	}
	if !valid {
		http.Error(w, "forbidden: clinician does not hold a current Annual Practising Certificate", http.StatusForbidden)
		return false
	}
	return true
}

// checkConsent verifies that an active consent record exists for the given patient.
// Returns false and writes a 403 if consent is absent. Skipped when consentStore is nil.
func checkConsent(w http.ResponseWriter, r *http.Request, consentStore *consent.Store, patientNHI string) bool {
	if consentStore == nil || patientNHI == "" {
		return true
	}
	tenantID, _ := middleware.TenantFromContext(r.Context())
	ok, err := consentStore.Check(r.Context(), tenantID, patientNHI, consent.ConsentTypeAccess)
	if err != nil {
		http.Error(w, "forbidden: consent check failed: "+err.Error(), http.StatusForbidden)
		return false
	}
	if !ok {
		http.Error(w, "forbidden: no active consent for patient data access", http.StatusForbidden)
		return false
	}
	return true
}

// parsePagination reads limit and offset from query params with defaults.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		if l > 200 {
			l = 200
		}
		limit = l
	}
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
		offset = o
	}
	return
}

// CreateTreatmentPlan creates a new treatment plan.
func (h *PhysioHandler) CreateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var plan physio.TreatmentPlan
	if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plan.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	plan.CreatedAt = now
	plan.UpdatedAt = now

	if err := plan.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(plan)
}

// GetTreatmentPlan retrieves a treatment plan by ID.
func (h *PhysioHandler) GetTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: fetch from database; stub returns placeholder data.
	plan := physio.TreatmentPlan{
		ID:          id,
		PatientNHI:  "ABC1234",
		ClinicianID: "clin-001",
		Diagnosis:   "Low back pain",
		Status:      physio.PlanStatusActive,
	}

	if !checkConsent(w, r, h.consentStore, plan.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// ListTreatmentPlans lists treatment plans with filters.
func (h *PhysioHandler) ListTreatmentPlans(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	status := query.Get("status")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	plans := []physio.TreatmentPlan{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   plans,
		"limit":  limit,
		"offset": offset,
		"total":  len(plans),
		"filters": map[string]string{
			"patient_nhi":  patientNHI,
			"clinician_id": clinicianID,
			"status":       status,
		},
	})
}

// UpdateTreatmentPlan updates a treatment plan.
func (h *PhysioHandler) UpdateTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var plan physio.TreatmentPlan
	if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	plan.ID = id
	plan.UpdatedAt = time.Now().UnixMilli()

	if err := plan.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// DeleteTreatmentPlan deletes a treatment plan.
func (h *PhysioHandler) DeleteTreatmentPlan(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}
	_ = mux.Vars(r)["id"]
	w.WriteHeader(http.StatusNoContent)
}

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
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: fetch from database; stub returns placeholder data.
	note := physio.SessionNote{
		ID:              id,
		PatientNHI:      "ABC1234",
		ClinicianID:     "clin-001",
		SessionDate:     time.Now().UnixMilli(),
		SessionNumber:   1,
		Subjective:      "Patient reports improved pain",
		Objective:       "ROM improved",
		Assessment:      "Progressing well",
		Plan:            "Continue current program",
		DurationMinutes: 30,
	}

	if !checkConsent(w, r, h.consentStore, note.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// ListSessionNotes lists session notes with filters.
func (h *PhysioHandler) ListSessionNotes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	treatmentPlanID := query.Get("treatment_plan_id")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	notes := []physio.SessionNote{}

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

	vars := mux.Vars(r)
	id := vars["id"]

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

// ACCHandler handles ACC claim endpoints for allied health.
type ACCHandler struct {
	hpiClient    *hpi.Client
	consentStore *consent.Store
	pool         *pgxpool.Pool
}

// NewACCHandler creates a new ACC handler.
func NewACCHandler(hpiClient *hpi.Client, consentStore *consent.Store, pool *pgxpool.Pool) *ACCHandler {
	return &ACCHandler{hpiClient: hpiClient, consentStore: consentStore, pool: pool}
}

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

// RegisterRoutes registers ACC routes.
func (h *ACCHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/allied-health/acc/claims", h.CreateClaim).Methods("POST")
	r.HandleFunc("/api/v1/allied-health/acc/claims", h.ListClaims).Methods("GET")
	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}", h.GetClaim).Methods("GET")
	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}", h.UpdateClaim).Methods("PUT")

	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}/sessions", h.CreateSession).Methods("POST")
	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}/sessions", h.ListSessions).Methods("GET")

	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}/reviews", h.CreateReview).Methods("POST")
	r.HandleFunc("/api/v1/allied-health/acc/claims/{id}/reviews", h.ListReviews).Methods("GET")

	r.HandleFunc("/api/v1/allied-health/acc/charge-codes", h.ListChargeCodes).Methods("GET")
	r.HandleFunc("/api/v1/allied-health/acc/charge-codes/{profession}", h.GetChargeCodesByProfession).Methods("GET")
}

// CreateClaim creates a new ACC claim.
func (h *ACCHandler) CreateClaim(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	var claim acc.Claim
	if err := json.NewDecoder(r.Body).Decode(&claim); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	claim.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	claim.CreatedAt = now
	claim.UpdatedAt = now

	if err := claim.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(claim)
}

// GetClaim retrieves an ACC claim by ID.
func (h *ACCHandler) GetClaim(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// TODO: fetch from database; stub returns placeholder data.
	claim := acc.Claim{
		ID:               id,
		PatientNHI:       "ABC1234",
		ClinicianID:      "clin-001",
		ClaimType:        acc.ClaimTypePhysiotherapy,
		ACCNumber:        "ACC123456",
		Status:           acc.ClaimStatusAccepted,
		Diagnosis:        "Lumbar strain",
		BodyRegion:       "lumbar_spine",
		ApprovedSessions: 10,
		UsedSessions:     3,
	}

	if !checkConsent(w, r, h.consentStore, claim.PatientNHI) {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(claim)
}

// ListClaims lists ACC claims with filters.
func (h *ACCHandler) ListClaims(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	patientNHI := query.Get("patient_nhi")
	clinicianID := query.Get("clinician_id")
	claimType := query.Get("claim_type")
	status := query.Get("status")
	limit, offset := parsePagination(r)

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	claims := []acc.Claim{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":   claims,
		"limit":  limit,
		"offset": offset,
		"total":  len(claims),
		"filters": map[string]string{
			"patient_nhi":  patientNHI,
			"clinician_id": clinicianID,
			"claim_type":   claimType,
			"status":       status,
		},
	})
}

// UpdateClaim updates an ACC claim.
func (h *ACCHandler) UpdateClaim(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var claim acc.Claim
	if err := json.NewDecoder(r.Body).Decode(&claim); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	claim.ID = id
	claim.UpdatedAt = time.Now().UnixMilli()

	if err := claim.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(claim)
}

// CreateSession creates a new treatment session under a claim.
func (h *ACCHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	vars := mux.Vars(r)
	claimID := vars["id"]

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

var (
	errClaimNotFound  = fmt.Errorf("acc: claim not found")
	errClaimIneligible = fmt.Errorf("acc: claim not eligible for session")
)

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
	vars := mux.Vars(r)
	claimID := vars["id"]
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

// CreateReview creates a new review report.
func (h *ACCHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	vars := mux.Vars(r)
	claimID := vars["id"]

	var review acc.ReviewReport
	if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	review.ID = uuid.New().String()
	review.ClaimID = claimID
	now := time.Now().UnixMilli()
	review.CreatedAt = now
	review.UpdatedAt = now

	if err := review.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(review)
}

// ListReviews lists review reports for a claim.
func (h *ACCHandler) ListReviews(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	claimID := vars["id"]

	reviews := []acc.ReviewReport{}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":     reviews,
		"claim_id": claimID,
	})
}

// ListChargeCodes lists all ACC charge codes.
func (h *ACCHandler) ListChargeCodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(acc.StandardChargeCodes)
}

// GetChargeCodesByProfession returns charge codes for a profession.
func (h *ACCHandler) GetChargeCodesByProfession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	profession := vars["profession"]

	codes := acc.GetChargeCodesByProfession(profession)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(codes)
}
