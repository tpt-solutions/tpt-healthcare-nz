package api

import (
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

// DonorsHandler handles all /api/v1/donors routes.
type DonorsHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/donors.
// Supports query parameters: status, bloodGroup, nhi.
func (h *DonorsHandler) List(w http.ResponseWriter, r *http.Request) {
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

	q := r.URL.Query()
	statusFilter := q.Get("status")
	bloodGroupFilter := q.Get("bloodGroup")
	nhiFilter := q.Get("nhi")

	donors, err := h.listDonors(ctx, tenantID, statusFilter, bloodGroupFilter, nhiFilter)
	if err != nil {
		h.logger.Error("list donors", slog.Any("error", err), slog.String("tenant", tenantID))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list donors"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Donor",
		ResourceID:   "list",
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"donors": donors,
		"total":  len(donors),
	})
}

// Create handles POST /api/v1/donors.
func (h *DonorsHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req donorCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if req.NHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "nhi is required"})
		return
	}
	if !ValidBloodGroups[req.BloodGroup] {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BLOOD_GROUP", Message: fmt.Sprintf("unknown blood group %q", req.BloodGroup)})
		return
	}

	rhd := "POSITIVE"
	if req.BloodGroup[len(req.BloodGroup)-1:] == "-" {
		rhd = "NEGATIVE"
	}

	donor, err := h.insertDonor(ctx, req.NHI, req.BloodGroup, rhd, tenantID)
	if err != nil {
		h.logger.Error("insert donor", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create donor"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Donor",
		ResourceID:   donor.ID,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, donor)
}

// Get handles GET /api/v1/donors/{id}.
func (h *DonorsHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "donor ID is required"})
		return
	}

	donor, err := h.getDonorByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "donor not found"})
			return
		}
		h.logger.Error("get donor", slog.Any("error", err), slog.String("id", id))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve donor"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Donor",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, donor)
}

// Update handles PUT /api/v1/donors/{id}.
func (h *DonorsHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "donor ID is required"})
		return
	}

	var req donorUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	existing, err := h.getDonorByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "donor not found"})
			return
		}
		h.logger.Error("get donor for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve donor"})
		return
	}

	if req.BloodGroup != nil {
		if !ValidBloodGroups[*req.BloodGroup] {
			writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BLOOD_GROUP", Message: fmt.Sprintf("unknown blood group %q", *req.BloodGroup)})
			return
		}
		rhd := "POSITIVE"
		if (*req.BloodGroup)[len(*req.BloodGroup)-1:] == "-" {
			rhd = "NEGATIVE"
		}
		existing.BloodGroup = *req.BloodGroup
		existing.RhD = rhd
	}
	if req.HaemoglobinGDL != nil {
		existing.HaemoglobinGDL = req.HaemoglobinGDL
	}

	updated, err := h.updateDonor(ctx, existing)
	if err != nil {
		h.logger.Error("update donor", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update donor"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Donor",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// Defer handles POST /api/v1/donors/{id}/defer.
func (h *DonorsHandler) Defer(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "donor ID is required"})
		return
	}

	var req donorDeferRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if _, ok := DeferralDuration[req.Reason]; !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_REASON", Message: fmt.Sprintf("unknown deferral reason: %q", req.Reason)})
		return
	}

	existing, err := h.getDonorByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "donor not found"})
			return
		}
		h.logger.Error("get donor for deferral", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve donor"})
		return
	}

	if existing.Status == DonorStatusPermanent {
		writeJSON(w, http.StatusConflict, apiError{Code: "PERMANENT_DEFERRAL", Message: "donor has a permanent deferral and cannot be deferred further"})
		return
	}

	duration := DeferralDuration[req.Reason]
	var endDate *time.Time
	now := time.Now().UTC()
	if duration > 0 {
		t := now.AddDate(0, 0, duration)
		endDate = &t
	} else if req.Reason == DeferralPermanent {
		existing.Status = DonorStatusPermanent
	}

	reasonStr := string(req.Reason)
	existing.DeferralReason = &reasonStr
	existing.DeferralEndDate = endDate
	if existing.Status != DonorStatusPermanent {
		existing.Status = DonorStatusDeferred
	}

	updated, err := h.updateDonor(ctx, existing)
	if err != nil {
		h.logger.Error("defer donor", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DEFER_ERROR", Message: "failed to defer donor"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Donor",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"reason": string(req.Reason), "details": req.Details},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// Reinstate handles POST /api/v1/donors/{id}/reinstate.
func (h *DonorsHandler) Reinstate(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "donor ID is required"})
		return
	}

	existing, err := h.getDonorByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "donor not found"})
			return
		}
		h.logger.Error("get donor for reinstate", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve donor"})
		return
	}

	if existing.Status == DonorStatusPermanent {
		writeJSON(w, http.StatusConflict, apiError{Code: "PERMANENT_DEFERRAL", Message: "cannot reinstate a permanently deferred donor"})
		return
	}

	existing.Status = DonorStatusActive
	existing.DeferralReason = nil
	existing.DeferralEndDate = nil

	updated, err := h.updateDonor(ctx, existing)
	if err != nil {
		h.logger.Error("reinstate donor", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "REINSTATE_ERROR", Message: "failed to reinstate donor"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Donor",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "reinstate"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// DonationHistory handles GET /api/v1/donors/{id}/donations.
func (h *DonorsHandler) DonationHistory(w http.ResponseWriter, r *http.Request) {
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

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "donor ID is required"})
		return
	}

	donations, err := h.getDonations(ctx, id, tenantID)
	if err != nil {
		h.logger.Error("get donation history", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "HISTORY_ERROR", Message: "failed to retrieve donation history"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Donation",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "donation-history"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"donations": donations,
		"total":     len(donations),
	})
}

// ListEligible handles GET /api/v1/donors/eligible.
// Returns donors who are currently eligible to donate (active status, no active deferral).
func (h *DonorsHandler) ListEligible(w http.ResponseWriter, r *http.Request) {
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

	q := r.URL.Query()
	bloodGroupFilter := q.Get("bloodGroup")

	donors, err := h.listEligibleDonors(ctx, tenantID, bloodGroupFilter)
	if err != nil {
		h.logger.Error("list eligible donors", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list eligible donors"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Donor",
		ResourceID:   "eligible",
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"donors": donors,
		"total":  len(donors),
	})
}
