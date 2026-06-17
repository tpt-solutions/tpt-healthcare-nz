package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-allied-health/internal/acc"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
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
	id := mux.Vars(r)["id"]

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

	id := mux.Vars(r)["id"]

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

// CreateReview creates a new review report.
func (h *ACCHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	if !requireAPC(w, r, h.hpiClient) {
		return
	}

	claimID := mux.Vars(r)["id"]

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
	claimID := mux.Vars(r)["id"]

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
	profession := mux.Vars(r)["profession"]
	codes := acc.GetChargeCodesByProfession(profession)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(codes)
}
