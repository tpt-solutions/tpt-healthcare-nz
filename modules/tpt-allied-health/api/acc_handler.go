package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/acc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

// RegisterRoutes registers ACC routes.
func (h *ACCHandler) RegisterRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.Handler) {
	mux.Handle("POST /api/v1/allied-health/acc/claims", protect(h.CreateClaim))
	mux.Handle("GET /api/v1/allied-health/acc/claims", protect(h.ListClaims))
	mux.Handle("GET /api/v1/allied-health/acc/claims/{id}", protect(h.GetClaim))
	mux.Handle("PUT /api/v1/allied-health/acc/claims/{id}", protect(h.UpdateClaim))

	mux.Handle("POST /api/v1/allied-health/acc/claims/{id}/sessions", protect(h.CreateSession))
	mux.Handle("GET /api/v1/allied-health/acc/claims/{id}/sessions", protect(h.ListSessions))

	mux.Handle("POST /api/v1/allied-health/acc/claims/{id}/reviews", protect(h.CreateReview))
	mux.Handle("GET /api/v1/allied-health/acc/claims/{id}/reviews", protect(h.ListReviews))

	mux.Handle("GET /api/v1/allied-health/acc/charge-codes", protect(h.ListChargeCodes))
	mux.Handle("GET /api/v1/allied-health/acc/charge-codes/{profession}", protect(h.GetChargeCodesByProfession))
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
	claim.Status = acc.ClaimStatusDraft

	if err := claim.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO acc_claims (id, patient_nhi, clinician_id, practice_id,
		                         claim_type, acc_number, status, diagnosis,
		                         icd10_code, body_region, injury_mechanism,
		                         referrer, approved_sessions, used_sessions,
		                         start_date, expiry_date, clinical_notes, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
		claim.ID, claim.PatientNHI, claim.ClinicianID, claim.PracticeID,
		string(claim.ClaimType), claim.ACCNumber, string(claim.Status),
		claim.Diagnosis, claim.ICD10Code, claim.BodyRegion, claim.InjuryMechanism,
		claim.Referrer, claim.ApprovedSessions, claim.UsedSessions,
		claim.StartDate, claim.ExpiryDate, claim.ClinicalNotes, claim.CreatedAt, claim.UpdatedAt)
	if err != nil {
		http.Error(w, "failed to persist claim: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(claim)
}

// GetClaim retrieves an ACC claim by ID.
func (h *ACCHandler) GetClaim(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id path parameter is required", http.StatusBadRequest)
		return
	}

	var claim acc.Claim
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, patient_nhi, clinician_id, practice_id, claim_type, acc_number, status,
		        diagnosis, icd10_code, body_region, injury_mechanism, referrer,
		        approved_sessions, used_sessions, start_date, expiry_date,
		        last_treatment_date, next_review_date, clinical_notes, created_at, updated_at
		 FROM acc_claims WHERE id = $1`, id,
	).Scan(&claim.ID, &claim.PatientNHI, &claim.ClinicianID, &claim.PracticeID, &claim.ClaimType,
		&claim.ACCNumber, &claim.Status, &claim.Diagnosis, &claim.ICD10Code, &claim.BodyRegion,
		&claim.InjuryMechanism, &claim.Referrer, &claim.ApprovedSessions, &claim.UsedSessions,
		&claim.StartDate, &claim.ExpiryDate, &claim.LastTreatmentDate, &claim.NextReviewDate,
		&claim.ClinicalNotes, &claim.CreatedAt, &claim.UpdatedAt)
	if err != nil {
		if db.IsNoRows(err) {
			http.Error(w, "claim not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load claim: "+err.Error(), http.StatusInternalServerError)
		return
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

	if !checkConsent(w, r, h.consentStore, patientNHI) {
		return
	}

	sql := `SELECT id, patient_nhi, clinician_id, practice_id, claim_type, acc_number, status,
	               diagnosis, icd10_code, body_region, injury_mechanism, referrer,
	               approved_sessions, used_sessions, start_date, expiry_date,
	               last_treatment_date, next_review_date, clinical_notes, created_at, updated_at
	        FROM acc_claims WHERE 1=1`
	args := db.NamedArgs{}

	if patientNHI != "" {
		sql += " AND patient_nhi = @patient_nhi"
		args["patient_nhi"] = patientNHI
	}
	if clinicianID != "" {
		sql += " AND clinician_id = @clinician_id"
		args["clinician_id"] = clinicianID
	}
	if claimType != "" {
		sql += " AND claim_type = @claim_type"
		args["claim_type"] = claimType
	}
	if status != "" {
		sql += " AND status = @status"
		args["status"] = status
	}
	sql += " ORDER BY created_at DESC LIMIT 100"

	rows, err := h.pool.Query(r.Context(), sql, args)
	if err != nil {
		http.Error(w, "failed to list claims: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var claims []acc.Claim
	for rows.Next() {
		var c acc.Claim
		if err := rows.Scan(&c.ID, &c.PatientNHI, &c.ClinicianID, &c.PracticeID, &c.ClaimType,
			&c.ACCNumber, &c.Status, &c.Diagnosis, &c.ICD10Code, &c.BodyRegion,
			&c.InjuryMechanism, &c.Referrer, &c.ApprovedSessions, &c.UsedSessions,
			&c.StartDate, &c.ExpiryDate, &c.LastTreatmentDate, &c.NextReviewDate,
			&c.ClinicalNotes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		claims = append(claims, c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": claims,
		"total": len(claims),
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

	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "id path parameter is required", http.StatusBadRequest)
		return
	}

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

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE acc_claims
		 SET patient_nhi = $1, clinician_id = $2, practice_id = $3,
		     claim_type = $4, status = $5, diagnosis = $6,
		     icd10_code = $7, body_region = $8, injury_mechanism = $9,
		     referrer = $10, approved_sessions = $11, used_sessions = $12,
		     start_date = $13, expiry_date = $14,
		     last_treatment_date = $15, next_review_date = $16,
		     clinical_notes = $17, updated_at = $18
		 WHERE id = $19`,
		claim.PatientNHI, claim.ClinicianID, claim.PracticeID,
		string(claim.ClaimType), string(claim.Status), claim.Diagnosis,
		claim.ICD10Code, claim.BodyRegion, claim.InjuryMechanism,
		claim.Referrer, claim.ApprovedSessions, claim.UsedSessions,
		claim.StartDate, claim.ExpiryDate,
		claim.LastTreatmentDate, claim.NextReviewDate,
		claim.ClinicalNotes, claim.UpdatedAt, id)
	if err != nil {
		http.Error(w, "failed to update claim: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "claim not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(claim)
}

// CreateReview creates a review report for an ACC claim.
func (h *ACCHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	claimID := r.PathValue("id")
	if claimID == "" {
		http.Error(w, "claim id path parameter is required", http.StatusBadRequest)
		return
	}

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

	goalsAchieved, _ := json.Marshal(review.GoalsAchieved)
	goalsOngoing, _ := json.Marshal(review.GoalsOngoing)
	goalsNotAchieved, _ := json.Marshal(review.GoalsNotAchieved)

	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO acc_review_reports (id, claim_id, patient_nhi, clinician_id,
		                                 report_date, report_type, sessions_since_last_review,
		                                 progress_summary, current_status, goals_achieved,
		                                 goals_ongoing, goals_not_achieved, recommendation,
		                                 additional_sessions_requested, proposed_end_date, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		review.ID, claimID, review.PatientNHI, review.ClinicianID,
		review.ReportDate, string(review.ReportType), review.SessionsSinceLastReview,
		review.ProgressSummary, review.CurrentStatus, goalsAchieved,
		goalsOngoing, goalsNotAchieved, string(review.Recommendation),
		review.AdditionalSessionsRequested, review.ProposedEndDate, string(review.Status),
		review.CreatedAt, review.UpdatedAt)
	if err != nil {
		http.Error(w, "failed to persist review: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(review)
}

// ListReviews lists review reports for an ACC claim.
func (h *ACCHandler) ListReviews(w http.ResponseWriter, r *http.Request) {
	claimID := r.PathValue("id")
	if claimID == "" {
		http.Error(w, "claim id path parameter is required", http.StatusBadRequest)
		return
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, claim_id, patient_nhi, clinician_id, report_date, report_type,
		        sessions_since_last_review, progress_summary, current_status,
		        goals_achieved, goals_ongoing, goals_not_achieved, recommendation,
		        additional_sessions_requested, proposed_end_date, status, submitted_at, created_at, updated_at
		 FROM acc_review_reports
		 WHERE claim_id = $1
		 ORDER BY report_date DESC`, claimID)
	if err != nil {
		http.Error(w, "failed to list reviews: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var reviews []acc.ReviewReport
	for rows.Next() {
		var r acc.ReviewReport
		var goalsAchieved, goalsOngoing, goalsNotAchieved []byte
		if err := rows.Scan(&r.ID, &r.ClaimID, &r.PatientNHI, &r.ClinicianID, &r.ReportDate,
			&r.ReportType, &r.SessionsSinceLastReview, &r.ProgressSummary, &r.CurrentStatus,
			&goalsAchieved, &goalsOngoing, &goalsNotAchieved, &r.Recommendation,
			&r.AdditionalSessionsRequested, &r.ProposedEndDate, &r.Status,
			&r.SubmittedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
			continue
		}
		_ = json.Unmarshal(goalsAchieved, &r.GoalsAchieved)
		_ = json.Unmarshal(goalsOngoing, &r.GoalsOngoing)
		_ = json.Unmarshal(goalsNotAchieved, &r.GoalsNotAchieved)
		reviews = append(reviews, r)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":     reviews,
		"claim_id": claimID,
		"total":    len(reviews),
	})
}

// ListChargeCodes lists all ACC charge codes.
func (h *ACCHandler) ListChargeCodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(acc.StandardChargeCodes)
}

// GetChargeCodesByProfession returns charge codes for a profession.
func (h *ACCHandler) GetChargeCodesByProfession(w http.ResponseWriter, r *http.Request) {
	profession := r.PathValue("profession")
	codes := acc.GetChargeCodesByProfession(profession)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(codes)
}