package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	visionacc "github.com/PhillipC05/tpt-healthcare/modules/tpt-vision/internal/acc"
)

// ACCHandler handles ACC vision claim CRUD and submission operations.
type ACCHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListClaims returns all ACC vision claims.
func (h *ACCHandler) ListClaims(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("list ACC claims")
	writeJSON(w, http.StatusOK, map[string]any{
		"claims": []any{},
	})
}

// CreateClaim creates a new ACC vision claim.
func (h *ACCHandler) CreateClaim(w http.ResponseWriter, r *http.Request) {
	var claim visionacc.Claim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if err := claim.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error()})
		return
	}

	h.logger.Info("ACC claim created", slog.String("patient_nhi", claim.PatientNHI), slog.String("claim_type", string(claim.ClaimType)))
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":     "created",
		"patientNhi": claim.PatientNHI,
		"claimType":  claim.ClaimType,
	})
}

// GetClaim returns a specific ACC claim.
func (h *ACCHandler) GetClaim(w http.ResponseWriter, r *http.Request) {
	claimId := r.PathValue("claimId")
	if claimId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLAIM_ID", Message: "Claim ID is required"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"claimId": claimId,
	})
}

// UpdateClaim updates an existing ACC claim.
func (h *ACCHandler) UpdateClaim(w http.ResponseWriter, r *http.Request) {
	claimId := r.PathValue("claimId")
	if claimId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLAIM_ID", Message: "Claim ID is required"})
		return
	}

	var claim visionacc.Claim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	h.logger.Info("ACC claim updated", slog.String("claim_id", claimId))
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "updated",
		"claimId": claimId,
	})
}

// SubmitClaim submits an ACC claim for processing.
func (h *ACCHandler) SubmitClaim(w http.ResponseWriter, r *http.Request) {
	claimId := r.PathValue("claimId")
	if claimId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLAIM_ID", Message: "Claim ID is required"})
		return
	}

	h.logger.Info("ACC claim submitted", slog.String("claim_id", claimId))
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "submitted",
		"claimId": claimId,
	})
}

// CheckStatus returns the current status of an ACC claim.
func (h *ACCHandler) CheckStatus(w http.ResponseWriter, r *http.Request) {
	claimId := r.PathValue("claimId")
	if claimId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLAIM_ID", Message: "Claim ID is required"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"claimId": claimId,
		"status":  string(visionacc.StatusDraft),
	})
}

// ValidateClaim validates claim data without submitting.
func (h *ACCHandler) ValidateClaim(w http.ResponseWriter, r *http.Request) {
	var claim visionacc.Claim
	if err := decodeJSON(r, &claim); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if err := claim.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "valid",
		"message": "Claim data is valid for submission",
	})
}

// ProcedureCodes returns all known ACC vision procedure codes.
func (h *ACCHandler) ProcedureCodes(w http.ResponseWriter, r *http.Request) {
	codes := make([]map[string]string, 0, len(visionacc.ProcedureDescriptions))
	for code, desc := range visionacc.ProcedureDescriptions {
		codes = append(codes, map[string]string{
			"code":        string(code),
			"description": desc,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"procedureCodes": codes,
	})
}

// GetClaimFHIR returns an ACC claim as a FHIR R5 Claim resource.
func (h *ACCHandler) GetClaimFHIR(w http.ResponseWriter, r *http.Request) {
	claimId := r.PathValue("claimId")
	if claimId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CLAIM_ID", Message: "Claim ID is required"})
		return
	}

	// TODO: retrieve claim from DB
	// For now, return a placeholder
	writeJSON(w, http.StatusOK, map[string]any{
		"resourceType": "OperationOutcome",
		"issue": []map[string]any{
			{
				"severity": "information",
				"code":     "not-implemented",
				"details": map[string]any{
					"text": "FHIR endpoint - implement DB retrieval",
				},
			},
		},
	})
}
