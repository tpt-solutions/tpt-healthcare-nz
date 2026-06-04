package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// defaultShareTTL is the default validity period for a new sharing token.
const defaultShareTTL = 7 * 24 * time.Hour

// ImagingShare is the domain model for a study-sharing token.
type ImagingShare struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenantId"`
	ImagingStudyID string     `json:"imagingStudyId"`
	CreatedByHPI   string     `json:"createdByHpi"`
	RecipientEmail string     `json:"recipientEmail,omitempty"`
	RecipientNPI   string     `json:"recipientNpi,omitempty"`
	Token          string     `json:"token,omitempty"` // plaintext — only set at creation
	ExpiresAt      time.Time  `json:"expiresAt"`
	AccessedAt     *time.Time `json:"accessedAt,omitempty"`
	RevokedAt      *time.Time `json:"revokedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type shareCreateRequest struct {
	RecipientEmail string `json:"recipientEmail,omitempty"`
	RecipientNPI   string `json:"recipientNpi,omitempty"`
	TTLHours       int    `json:"ttlHours,omitempty"` // defaults to 168 (7 days)
}

// SharingHandler handles image-sharing token creation, access, and revocation.
type SharingHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	orthanc    *OrthancClient
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// CreateShare handles POST /api/v1/imaging-studies/{id}/share.
// Returns the new share including the plaintext token (only visible once).
func (h *SharingHandler) CreateShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	studyID := r.PathValue("id")
	var req shareCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	ttl := defaultShareTTL
	if req.TTLHours > 0 {
		ttl = time.Duration(req.TTLHours) * time.Hour
	}

	// Verify the study exists in this tenant.
	if _, err := h.getStudyByIDForSharing(ctx, studyID, tenantID); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "study not found"})
			return
		}
		h.logger.Error("get study for share", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve study"})
		return
	}

	plaintext, tokenHash, err := generateShareToken()
	if err != nil {
		h.logger.Error("generate share token", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "TOKEN_ERROR", Message: "failed to generate sharing token"})
		return
	}

	creatorHPI := principal.ID
	share, err := h.insertShare(ctx, studyID, tenantID, creatorHPI, tokenHash, req.RecipientEmail, req.RecipientNPI, ttl)
	if err != nil {
		h.logger.Error("insert imaging share", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create sharing token"})
		return
	}

	// Return the plaintext token — it is never stored.
	share.Token = plaintext

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "ImagingShare",
		ResourceID:   share.ID,
		TenantID:     tenantID,
		Metadata:     map[string]string{"study_id": studyID, "recipient": req.RecipientEmail},
	})

	writeJSON(w, http.StatusCreated, share)
}

// RevokeShare handles DELETE /api/v1/imaging-studies/{id}/shares/{shareId}.
func (h *SharingHandler) RevokeShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	studyID := r.PathValue("id")
	shareID := r.PathValue("shareId")

	if err := h.revokeShare(ctx, shareID, studyID, tenantID); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "share not found"})
			return
		}
		h.logger.Error("revoke imaging share", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "REVOKE_ERROR", Message: "failed to revoke share"})
		return
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "ImagingShare",
		ResourceID:   shareID,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "revoke", "study_id": studyID},
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// AccessShare handles GET /api/v1/share/{token} — public; no auth middleware.
// Returns study metadata and Orthanc DICOMweb endpoint for client-side viewing.
func (h *SharingHandler) AccessShare(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.PathValue("token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TOKEN", Message: "token is required"})
		return
	}

	sum := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(sum[:])

	share, err := h.getShareByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "share not found or expired"})
			return
		}
		h.logger.Error("get share by token", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to validate token"})
		return
	}

	if share.RevokedAt != nil {
		writeJSON(w, http.StatusGone, apiError{Code: "REVOKED", Message: "this share has been revoked"})
		return
	}
	if time.Now().UTC().After(share.ExpiresAt) {
		writeJSON(w, http.StatusGone, apiError{Code: "EXPIRED", Message: "this share link has expired"})
		return
	}

	// Fetch study metadata for display.
	study, err := h.getStudyForShare(ctx, share.ImagingStudyID)
	if err != nil {
		h.logger.Error("get study for share access", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve study"})
		return
	}

	// Record first access time.
	if share.AccessedAt == nil {
		if err := h.recordShareAccess(ctx, share.ID); err != nil {
			h.logger.Error("record share access", slog.Any("error", err))
		}
	}

	_ = h.auditTrail.Write(ctx, audit.Event{
		Actor:        audit.SystemActor("share-access"),
		Action:       audit.ActionRead,
		ResourceType: "ImagingStudy",
		ResourceID:   share.ImagingStudyID,
		TenantID:     share.TenantID,
		Metadata:     map[string]string{"share_id": share.ID, "action": "share-access"},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"share": map[string]any{
			"id":        share.ID,
			"expiresAt": share.ExpiresAt,
		},
		"study": study,
	})
}

// ---------------------------------------------------------------------------
// Token generation
// ---------------------------------------------------------------------------

// generateShareToken returns a hex-encoded random plaintext token and its
// SHA-256 hash for database storage.
func generateShareToken() (plaintext, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	plaintext = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(sum[:])
	return plaintext, hash, nil
}

// ---------------------------------------------------------------------------
// Database operations
// ---------------------------------------------------------------------------

func (h *SharingHandler) getStudyByIDForSharing(ctx context.Context, id, tenantID string) (ImagingStudy, error) {
	var s ImagingStudy
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, patient_nhi, study_instance_uid,
		        accession_number, modality, body_part, study_date, description,
		        referring_hpi, performing_hpi, status, num_series, num_instances,
		        created_at, updated_at
		 FROM imaging_studies WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&s.ID, &s.TenantID, &s.PatientNHI, &s.StudyInstanceUID,
		&s.AccessionNumber, &s.Modality, &s.BodyPart, &s.StudyDate, &s.Description,
		&s.ReferringHPI, &s.PerformingHPI, &s.Status, &s.NumSeries, &s.NumInstances,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return ImagingStudy{}, errNotFound
		}
		return ImagingStudy{}, fmt.Errorf("get study for sharing: %w", err)
	}
	return s, nil
}

// getStudyForShare fetches study metadata without requiring a tenantID (used
// for the public share-access endpoint once the token has been validated).
func (h *SharingHandler) getStudyForShare(ctx context.Context, id string) (ImagingStudy, error) {
	var s ImagingStudy
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, patient_nhi, study_instance_uid,
		        accession_number, modality, body_part, study_date, description,
		        referring_hpi, performing_hpi, status, num_series, num_instances,
		        created_at, updated_at
		 FROM imaging_studies WHERE id = @id`,
		db.NamedArgs{"id": id},
	).Scan(
		&s.ID, &s.TenantID, &s.PatientNHI, &s.StudyInstanceUID,
		&s.AccessionNumber, &s.Modality, &s.BodyPart, &s.StudyDate, &s.Description,
		&s.ReferringHPI, &s.PerformingHPI, &s.Status, &s.NumSeries, &s.NumInstances,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return ImagingStudy{}, errNotFound
		}
		return ImagingStudy{}, fmt.Errorf("get study for share access: %w", err)
	}
	return s, nil
}

func (h *SharingHandler) insertShare(ctx context.Context, studyID, tenantID, createdByHPI, tokenHash, email, npi string, ttl time.Duration) (ImagingShare, error) {
	expiresAt := time.Now().UTC().Add(ttl)
	var share ImagingShare
	err := h.pool.QueryRow(ctx,
		`INSERT INTO imaging_share_tokens
		   (tenant_id, imaging_study_id, created_by_hpi,
		    recipient_email, recipient_npi, token_hash, expires_at)
		 VALUES
		   (@tenant_id, @imaging_study_id, @created_by_hpi,
		    @recipient_email, @recipient_npi, @token_hash, @expires_at)
		 RETURNING id, tenant_id, imaging_study_id, created_by_hpi,
		           recipient_email, recipient_npi,
		           expires_at, accessed_at, revoked_at, created_at`,
		db.NamedArgs{
			"tenant_id":       tenantID,
			"imaging_study_id": studyID,
			"created_by_hpi":  createdByHPI,
			"recipient_email": email,
			"recipient_npi":   npi,
			"token_hash":      tokenHash,
			"expires_at":      expiresAt,
		},
	).Scan(
		&share.ID, &share.TenantID, &share.ImagingStudyID, &share.CreatedByHPI,
		&share.RecipientEmail, &share.RecipientNPI,
		&share.ExpiresAt, &share.AccessedAt, &share.RevokedAt, &share.CreatedAt,
	)
	if err != nil {
		return ImagingShare{}, fmt.Errorf("insert imaging share: %w", err)
	}
	return share, nil
}

func (h *SharingHandler) getShareByTokenHash(ctx context.Context, tokenHash string) (ImagingShare, error) {
	var share ImagingShare
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, imaging_study_id, created_by_hpi,
		        recipient_email, recipient_npi,
		        expires_at, accessed_at, revoked_at, created_at
		 FROM imaging_share_tokens
		 WHERE token_hash = @token_hash`,
		db.NamedArgs{"token_hash": tokenHash},
	).Scan(
		&share.ID, &share.TenantID, &share.ImagingStudyID, &share.CreatedByHPI,
		&share.RecipientEmail, &share.RecipientNPI,
		&share.ExpiresAt, &share.AccessedAt, &share.RevokedAt, &share.CreatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return ImagingShare{}, errNotFound
		}
		return ImagingShare{}, fmt.Errorf("get share by token: %w", err)
	}
	return share, nil
}

func (h *SharingHandler) revokeShare(ctx context.Context, shareID, studyID, tenantID string) error {
	result, err := h.pool.Exec(ctx,
		`UPDATE imaging_share_tokens
		 SET revoked_at = now()
		 WHERE id = @id AND imaging_study_id = @study_id AND tenant_id = @tenant_id
		   AND revoked_at IS NULL`,
		db.NamedArgs{
			"id":       shareID,
			"study_id": studyID,
			"tenant_id": tenantID,
		},
	)
	if err != nil {
		return fmt.Errorf("revoke share: %w", err)
	}
	if result.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

func (h *SharingHandler) recordShareAccess(ctx context.Context, shareID string) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE imaging_share_tokens SET accessed_at = now() WHERE id = @id AND accessed_at IS NULL`,
		db.NamedArgs{"id": shareID},
	)
	return err
}
