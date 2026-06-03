package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/google/uuid"

	dentalacc "github.com/PhillipC05/tpt-healthcare/modules/tpt-dental/internal/acc"
)

// ACCHandler handles ACC dental claim operations.
type ACCHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListClaims returns all ACC dental claims, optionally filtered by patient or status.
func (h *ACCHandler) ListClaims(w http.ResponseWriter, r *http.Request) {
	// Simplified stub — real implementation queries DB.
	claims := []dentalacc.DentalClaim{}
	writeJSON(w, http.StatusOK, claims)
}

// CreateClaim creates a new ACC dental claim in draft status.
func (h *ACCHandler) CreateClaim(w http.ResponseWriter, r *http.Request) {
	var claim dentalacc.DentalClaim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	claim.ID = uuid.New().String()
	claim.Status = dentalacc.ClaimDraft
	claim.CreatedAt = time.Now().UTC()
	claim.UpdatedAt = claim.CreatedAt

	// Run validation.
	result := claim.Validate()
	if !result.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{
			Code:    "VALIDATION_FAILED",
			Message: "Claim failed validation",
			Details: result.Errors,
		})
		return
	}

	h.logger.Info("ACC dental claim created",
		slog.String("claim_id", claim.ID),
		slog.String("patient_nhi", claim.PatientNHI),
		slog.Int("teeth", len(claim.Teeth)))

	writeJSON(w, http.StatusCreated, claim)
}

// GetClaim returns details for a specific ACC dental claim.
func (h *ACCHandler) GetClaim(w http.ResponseWriter, r *http.Request) {
	claimID := r.PathValue("claimId")
	if claimID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_CLAIM_ID", Message: "Claim ID is required",
		})
		return
	}

	// Simplified stub — real implementation queries DB.
	writeJSON(w, http.StatusNotFound, apiError{
		Code: "NOT_FOUND", Message: fmt.Sprintf("Claim %s not found", claimID),
	})
}

// UpdateClaim updates fields on an existing draft claim.
func (h *ACCHandler) UpdateClaim(w http.ResponseWriter, r *http.Request) {
	claimID := r.PathValue("claimId")
	if claimID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_CLAIM_ID", Message: "Claim ID is required",
		})
		return
	}

	var claim dentalacc.DentalClaim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	claim.ID = claimID
	claim.UpdatedAt = time.Now().UTC()

	h.logger.Info("ACC dental claim updated",
		slog.String("claim_id", claimID))

	writeJSON(w, http.StatusOK, claim)
}

// SubmitClaim submits a draft claim to ACC for processing.
func (h *ACCHandler) SubmitClaim(w http.ResponseWriter, r *http.Request) {
	claimID := r.PathValue("claimId")
	if claimID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_CLAIM_ID", Message: "Claim ID is required",
		})
		return
	}

	// Simplified stub — real implementation loads from DB, validates,
	// submits via core/acc.Client, and records the ACC claim number.
	claim := &dentalacc.DentalClaim{
		ID:            claimID,
		Status:        dentalacc.ClaimSubmitted,
		ACCClaimNumber: "ACC-" + uuid.New().String()[:8],
		UpdatedAt:     time.Now().UTC(),
	}

	h.logger.Info("ACC dental claim submitted",
		slog.String("claim_id", claimID),
		slog.String("acc_claim_number", claim.ACCClaimNumber))

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "submitted",
		"claimId":         claimID,
		"accClaimNumber":  claim.ACCClaimNumber,
	})
}

// CheckStatus polls ACC for the current status of a submitted claim.
func (h *ACCHandler) CheckStatus(w http.ResponseWriter, r *http.Request) {
	claimID := r.PathValue("claimId")
	if claimID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_CLAIM_ID", Message: "Claim ID is required",
		})
		return
	}

	// Simplified stub — real implementation polls core/acc.Client.Poll().
	writeJSON(w, http.StatusOK, map[string]string{
		"claimId": claimID,
		"status":  "pending",
		"message": "Awaiting ACC adjudication",
	})
}

// ValidateClaim pre-validates a claim payload without saving it.
func (h *ACCHandler) ValidateClaim(w http.ResponseWriter, r *http.Request) {
	var claim dentalacc.DentalClaim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	result := claim.Validate()
	writeJSON(w, http.StatusOK, result)
}

// InjuryTypes returns the list of recognised ACC dental injury classifications.
func (h *ACCHandler) InjuryTypes(w http.ResponseWriter, r *http.Request) {
	types := dentalacc.DentalInjuryTypes()
	writeJSON(w, http.StatusOK, types)
}