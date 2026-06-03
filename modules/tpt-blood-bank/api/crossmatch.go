package api

import (
	"context"
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

// CrossmatchStatus represents the current state of a cross-match request.
type CrossmatchStatus string

const (
	CrossmatchStatusPending  CrossmatchStatus = "pending"
	CrossmatchStatusMatched  CrossmatchStatus = "matched"
	CrossmatchStatusIssued   CrossmatchStatus = "issued"
	CrossmatchStatusTransfused CrossmatchStatus = "transfused"
	CrossmatchStatusCancelled CrossmatchStatus = "cancelled"
	CrossmatchStatusIncompatible CrossmatchStatus = "incompatible"
)

// Crossmatch represents a cross-match request linking a patient to blood products.
type Crossmatch struct {
	ID              string           `json:"id"`
	TenantID        string           `json:"tenantId"`
	PatientID       string           `json:"patientId"`
	PatientNHI      string           `json:"patientNhi"`
	PatientABO      string           `json:"patientAbo"`
	PatientRhD      string           `json:"patientRhd"`
	AntibodyScreen  string           `json:"antibodyScreen"`  // negative, positive
	ProductUnitIDs  []string         `json:"productUnitIds"`  // IDs of matched product units
	Status          CrossmatchStatus `json:"status"`
	Compatibility   string           `json:"compatibility"`    // compatible, incompatible, emergency-release
	RequestedBy     string           `json:"requestedBy"`
	IssuedBy        *string          `json:"issuedBy,omitempty"`
	TransfusedBy    *string          `json:"transfusedBy,omitempty"`
	EmergencyReason *string          `json:"emergencyReason,omitempty"`
	Notes           string           `json:"notes,omitempty"`
	RequestedAt     time.Time        `json:"requestedAt"`
	IssuedAt        *time.Time       `json:"issuedAt,omitempty"`
	TransfusedAt    *time.Time       `json:"transfusedAt,omitempty"`
	CancelledAt     *time.Time       `json:"cancelledAt,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	UpdatedAt       time.Time        `json:"updatedAt"`
}

// crossmatchCreateRequest is the body for POST /api/v1/crossmatches.
type crossmatchCreateRequest struct {
	PatientID        string   `json:"patientId"`
	PatientNHI       string   `json:"patientNhi"`
	PatientABO       string   `json:"patientAbo"`
	PatientRhD       string   `json:"patientRhd"`
	AntibodyScreen   string   `json:"antibodyScreen"`
	ProductUnitIDs   []string `json:"productUnitIds"`
	RequestedBy      string   `json:"requestedBy"`
	Notes            string   `json:"notes,omitempty"`
}

// crossmatchIssueRequest is the body for POST /api/v1/crossmatches/{id}/issue.
type crossmatchIssueRequest struct {
	IssuedBy string `json:"issuedBy"`
}

// crossmatchTransfuseRequest is the body for POST /api/v1/crossmatches/{id}/transfuse.
type crossmatchTransfuseRequest struct {
	TransfusedBy string `json:"transfusedBy"`
	Notes        string `json:"notes,omitempty"`
}

// crossmatchCancelRequest is the body for POST /api/v1/crossmatches/{id}/cancel.
type crossmatchCancelRequest struct {
	Reason string `json:"reason"`
}

// crossmatchEmergencyRequest is the body for POST /api/v1/crossmatches/{id}/emergency.
type crossmatchEmergencyRequest struct {
	ApprovedBy     string `json:"approvedBy"`
	ClinicalReason string `json:"clinicalReason"`
}

// ABOCompatibilityTable defines which ABO donor types are compatible with each recipient type.
// Key = patient ABO, Value = list of compatible donor ABOs.
var ABOCompatibilityTable = map[string][]string{
	"O":  {"O"},
	"A":  {"A", "O"},
	"B":  {"B", "O"},
	"AB": {"A", "B", "AB", "O"},
}

// RhDCompatible returns true if the donor RhD is compatible with the patient.
// RhD-negative patients must receive RhD-negative blood to avoid alloimmunisation.
// RhD-positive patients may receive either RhD-positive or RhD-negative blood.
func RhDCompatible(patientRhD, donorRhD string) bool {
	if patientRhD == "NEGATIVE" && donorRhD == "POSITIVE" {
		return false
	}
	return true
}

// CrossmatchHandler handles all /api/v1/crossmatches routes.
type CrossmatchHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/crossmatches.
// Supports query parameters: patientId, status.
func (h *CrossmatchHandler) List(w http.ResponseWriter, r *http.Request) {
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
	patientFilter := q.Get("patientId")
	statusFilter := q.Get("status")

	crossmatches, err := h.listCrossmatches(ctx, tenantID, patientFilter, statusFilter)
	if err != nil {
		h.logger.Error("list crossmatches", slog.Any("error", err), slog.String("tenant", tenantID))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list crossmatches"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Crossmatch",
		ResourceID:   "list",
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"crossmatches": crossmatches,
		"total":        len(crossmatches),
	})
}

// Create handles POST /api/v1/crossmatches.
func (h *CrossmatchHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req crossmatchCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "either patientId or patientNhi is required"})
		return
	}
	if req.RequestedBy == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "requestedBy is required"})
		return
	}
	if len(req.ProductUnitIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "at least one productUnitId is required"})
		return
	}

	if req.AntibodyScreen == "" {
		req.AntibodyScreen = "negative"
	}

	// Validate product compatibility.
	compatibility, err := h.validateProductCompatibility(ctx, req, tenantID)
	if err != nil {
		if errors.Is(err, errIncompatible) {
			writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "INCOMPATIBLE", Message: err.Error()})
			return
		}
		h.logger.Error("validate compatibility", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "VALIDATION_ERROR", Message: "compatibility validation failed"})
		return
	}

	xm, err := h.insertCrossmatch(ctx, req, compatibility, tenantID)
	if err != nil {
		h.logger.Error("insert crossmatch", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create crossmatch"})
		return
	}

	// Update matched products to crossmatched status.
	for _, unitID := range req.ProductUnitIDs {
		if err := h.updateProductStatus(ctx, unitID, ProductStatusCrossmatched, tenantID); err != nil {
			h.logger.Error("update product to crossmatched", slog.Any("error", err), slog.String("productId", unitID))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "PRODUCT_STATUS_ERROR", Message: "crossmatch created but product status update failed"})
			return
		}
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Crossmatch",
		ResourceID:   xm.ID,
		TenantID:     tenantID,
		Metadata: map[string]string{
			"patientId":       req.PatientID,
			"patientNhi":      req.PatientNHI,
			"productCount":    fmt.Sprintf("%d", len(req.ProductUnitIDs)),
			"compatibility":   compatibility,
		},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, xm)
}

// Get handles GET /api/v1/crossmatches/{id}.
func (h *CrossmatchHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "crossmatch ID is required"})
		return
	}

	xm, err := h.getCrossmatchByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "crossmatch not found"})
			return
		}
		h.logger.Error("get crossmatch", slog.Any("error", err), slog.String("id", id))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve crossmatch"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "Crossmatch",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, xm)
}

// Issue handles POST /api/v1/crossmatches/{id}/issue.
// Transitions the crossmatch to "issued" and the products to "issued".
func (h *CrossmatchHandler) Issue(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "crossmatch ID is required"})
		return
	}

	var req crossmatchIssueRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.IssuedBy == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "issuedBy is required"})
		return
	}

	xm, err := h.getCrossmatchByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "crossmatch not found"})
			return
		}
		h.logger.Error("get crossmatch for issue", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve crossmatch"})
		return
	}

	if xm.Status != CrossmatchStatusMatched {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: fmt.Sprintf("cannot issue crossmatch in status %q", xm.Status)})
		return
	}

	now := time.Now().UTC()
	xm.Status = CrossmatchStatusIssued
	xm.IssuedBy = &req.IssuedBy
	xm.IssuedAt = &now

	updated, err := h.updateCrossmatch(ctx, xm)
	if err != nil {
		h.logger.Error("issue crossmatch", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to issue crossmatch"})
		return
	}

	// Update products to issued.
	for _, unitID := range xm.ProductUnitIDs {
		if err := h.updateProductStatus(ctx, unitID, ProductStatusIssued, tenantID); err != nil {
			h.logger.Error("update product to issued", slog.Any("error", err), slog.String("productId", unitID))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "PRODUCT_STATUS_ERROR", Message: "crossmatch issued but product status update failed"})
			return
		}
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Crossmatch",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "issue", "issuedBy": req.IssuedBy},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// Transfuse handles POST /api/v1/crossmatches/{id}/transfuse.
func (h *CrossmatchHandler) Transfuse(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "crossmatch ID is required"})
		return
	}

	var req crossmatchTransfuseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.TransfusedBy == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "transfusedBy is required"})
		return
	}

	xm, err := h.getCrossmatchByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "crossmatch not found"})
			return
		}
		h.logger.Error("get crossmatch for transfusion", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve crossmatch"})
		return
	}

	if xm.Status != CrossmatchStatusIssued {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: fmt.Sprintf("cannot record transfusion for crossmatch in status %q", xm.Status)})
		return
	}

	now := time.Now().UTC()
	xm.Status = CrossmatchStatusTransfused
	xm.TransfusedBy = &req.TransfusedBy
	xm.TransfusedAt = &now
	if req.Notes != "" {
		xm.Notes = req.Notes
	}

	updated, err := h.updateCrossmatch(ctx, xm)
	if err != nil {
		h.logger.Error("record transfusion", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to record transfusion"})
		return
	}

	// Update products to transfused.
	for _, unitID := range xm.ProductUnitIDs {
		if err := h.updateProductStatus(ctx, unitID, ProductStatusTransfused, tenantID); err != nil {
			h.logger.Error("update product to transfused", slog.Any("error", err), slog.String("productId", unitID))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "PRODUCT_STATUS_ERROR", Message: "transfusion recorded but product status update failed"})
			return
		}
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Crossmatch",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "transfuse", "transfusedBy": req.TransfusedBy},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// Cancel handles POST /api/v1/crossmatches/{id}/cancel.
func (h *CrossmatchHandler) Cancel(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "crossmatch ID is required"})
		return
	}

	var req crossmatchCancelRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	xm, err := h.getCrossmatchByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "crossmatch not found"})
			return
		}
		h.logger.Error("get crossmatch for cancel", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve crossmatch"})
		return
	}

	if xm.Status == CrossmatchStatusTransfused || xm.Status == CrossmatchStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: fmt.Sprintf("cannot cancel crossmatch in status %q", xm.Status)})
		return
	}

	now := time.Now().UTC()
	xm.Status = CrossmatchStatusCancelled
	xm.CancelledAt = &now
	if req.Reason != "" {
		xm.Notes = req.Reason
	}

	updated, err := h.updateCrossmatch(ctx, xm)
	if err != nil {
		h.logger.Error("cancel crossmatch", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to cancel crossmatch"})
		return
	}

	// Return products to stored status.
	for _, unitID := range xm.ProductUnitIDs {
		if err := h.updateProductStatus(ctx, unitID, ProductStatusStored, tenantID); err != nil {
			h.logger.Error("return product to stored", slog.Any("error", err), slog.String("productId", unitID))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "PRODUCT_STATUS_ERROR", Message: "crossmatch cancelled but product status update failed"})
			return
		}
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Crossmatch",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "cancel", "reason": req.Reason},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// EmergencyRelease handles POST /api/v1/crossmatches/{id}/emergency.
// Bypasses full crossmatching for life-threatening situations — always uses O-negative.
func (h *CrossmatchHandler) EmergencyRelease(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "crossmatch ID is required"})
		return
	}

	var req crossmatchEmergencyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ApprovedBy == "" || req.ClinicalReason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "approvedBy and clinicalReason are required for emergency release"})
		return
	}

	xm, err := h.getCrossmatchByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "crossmatch not found"})
			return
		}
		h.logger.Error("get crossmatch for emergency release", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve crossmatch"})
		return
	}

	if xm.Status == CrossmatchStatusTransfused || xm.Status == CrossmatchStatusCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: fmt.Sprintf("cannot emergency release crossmatch in status %q", xm.Status)})
		return
	}

	// Emergency release: force compatibility flag and issue immediately.
	now := time.Now().UTC()
	xm.Status = CrossmatchStatusIssued
	xm.Compatibility = "emergency-release"
	xm.EmergencyReason = &req.ClinicalReason
	xm.IssuedBy = &req.ApprovedBy
	xm.IssuedAt = &now
	xm.Notes = fmt.Sprintf("EMERGENCY RELEASE approved by %s. Reason: %s", req.ApprovedBy, req.ClinicalReason)

	updated, err := h.updateCrossmatch(ctx, xm)
	if err != nil {
		h.logger.Error("emergency release crossmatch", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to process emergency release"})
		return
	}

	// Issue products regardless of prior status.
	for _, unitID := range xm.ProductUnitIDs {
		if err := h.updateProductStatus(ctx, unitID, ProductStatusIssued, tenantID); err != nil {
			h.logger.Error("emergency issue product", slog.Any("error", err), slog.String("productId", unitID))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "PRODUCT_STATUS_ERROR", Message: "emergency release recorded but product status update failed"})
			return
		}
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "Crossmatch",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata: map[string]string{
			"action":         "emergency-release",
			"approvedBy":     req.ApprovedBy,
			"clinicalReason": req.ClinicalReason,
		},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// errIncompatible is a sentinel for ABO/RhD incompatibility.
var errIncompatible = errors.New("blood product is not ABO/RhD compatible with patient")

// validateProductCompatibility checks all selected products for ABO/RhD compatibility with the patient.
func (h *CrossmatchHandler) validateProductCompatibility(ctx context.Context, req crossmatchCreateRequest, tenantID string) (string, error) {
	compatibleABOs, ok := ABOCompatibilityTable[req.PatientABO]
	if !ok {
		return "", fmt.Errorf("unknown patient ABO group: %q", req.PatientABO)
	}

	for _, unitID := range req.ProductUnitIDs {
		product, err := h.getProductByID(ctx, unitID, tenantID)
		if err != nil {
			return "", fmt.Errorf("get product %s: %w", unitID, err)
		}

		// Only stored products are available for crossmatching.
		if product.Status != ProductStatusStored {
			return "", fmt.Errorf("%w: product %s is not available for crossmatch (status: %s)", errIncompatible, unitID, product.Status)
		}

		// Check ABO compatibility.
		aboCompatible := false
		for _, compatABO := range compatibleABOs {
			if product.ABO == compatABO {
				aboCompatible = true
				break
			}
		}
		if !aboCompatible {
			return "", fmt.Errorf("%w: product %s (ABO %s) incompatible with patient ABO %s", errIncompatible, unitID, product.ABO, req.PatientABO)
		}

		// Check RhD compatibility.
		if !RhDCompatible(req.PatientRhD, product.RhD) {
			return "", fmt.Errorf("%w: product %s (RhD %s) incompatible with patient RhD %s", errIncompatible, unitID, product.RhD, req.PatientRhD)
		}
	}

	if req.AntibodyScreen == "positive" {
		return "caution-antibody-screen-positive", nil
	}

	return "compatible", nil
}

// getProductByID retrieves a single blood product for crossmatch compatibility checks.
func (h *CrossmatchHandler) getProductByID(ctx context.Context, id, tenantID string) (BloodProduct, error) {
	var p BloodProduct
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, product_type, abo, rhd, donation_id, donor_id,
		        status, volume_ml, collection_date, expiry_date,
		        test_results, storage_location, created_at, updated_at
		 FROM blood_products
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&p.ID, &p.TenantID, &p.ProductType, &p.ABO, &p.RhD, &p.DonationID, &p.DonorID,
		&p.Status, &p.VolumeML, &p.CollectionDate, &p.ExpiryDate,
		&p.TestResults, &p.StorageLocation, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return BloodProduct{}, errNotFound
		}
		return BloodProduct{}, fmt.Errorf("get product by id: %w", err)
	}
	return p, nil
}

// updateProductStatus updates a single blood product's status.
func (h *CrossmatchHandler) updateProductStatus(ctx context.Context, productID string, status ProductStatus, tenantID string) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE blood_products
		 SET status = @status, updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{
			"status":    status,
			"id":        productID,
			"tenant_id": tenantID,
		},
	)
	if err != nil {
		return fmt.Errorf("update product %s status to %s: %w", productID, status, err)
	}
	return nil
}

// --- Database operations ---

func (h *CrossmatchHandler) listCrossmatches(ctx context.Context, tenantID, patientFilter, statusFilter string) ([]Crossmatch, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, patient_id, patient_nhi, patient_abo, patient_rhd,
		        antibody_screen, product_unit_ids, status, compatibility,
		        requested_by, issued_by, transfused_by,
		        emergency_reason, notes,
		        requested_at, issued_at, transfused_at, cancelled_at,
		        created_at, updated_at
		 FROM crossmatches
		 WHERE tenant_id = @tenant_id
		   AND (@patient_filter = '' OR patient_id = @patient_filter)
		   AND (@status_filter  = '' OR status      = @status_filter)
		 ORDER BY requested_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":     tenantID,
			"patient_filter": patientFilter,
			"status_filter":  statusFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query crossmatches: %w", err)
	}
	defer rows.Close()

	var results []Crossmatch
	for rows.Next() {
		xm, err := scanCrossmatch(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, xm)
	}
	return results, rows.Err()
}

func (h *CrossmatchHandler) getCrossmatchByID(ctx context.Context, id, tenantID string) (Crossmatch, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, patient_id, patient_nhi, patient_abo, patient_rhd,
		        antibody_screen, product_unit_ids, status, compatibility,
		        requested_by, issued_by, transfused_by,
		        emergency_reason, notes,
		        requested_at, issued_at, transfused_at, cancelled_at,
		        created_at, updated_at
		 FROM crossmatches
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	)
	xm, err := scanCrossmatch(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Crossmatch{}, errNotFound
		}
		return Crossmatch{}, fmt.Errorf("get crossmatch by id: %w", err)
	}
	return xm, nil
}

func (h *CrossmatchHandler) insertCrossmatch(ctx context.Context, req crossmatchCreateRequest, compatibility, tenantID string) (Crossmatch, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO crossmatches
		   (patient_id, patient_nhi, patient_abo, patient_rhd,
		    antibody_screen, product_unit_ids, status, compatibility,
		    requested_by, notes, tenant_id, requested_at)
		 VALUES
		   (@patient_id, @patient_nhi, @patient_abo, @patient_rhd,
		    @antibody_screen, @product_unit_ids, 'matched', @compatibility,
		    @requested_by, @notes, @tenant_id, now())
		 RETURNING id, tenant_id, patient_id, patient_nhi, patient_abo, patient_rhd,
		           antibody_screen, product_unit_ids, status, compatibility,
		           requested_by, issued_by, transfused_by,
		           emergency_reason, notes,
		           requested_at, issued_at, transfused_at, cancelled_at,
		           created_at, updated_at`,
		db.NamedArgs{
			"patient_id":       req.PatientID,
			"patient_nhi":      req.PatientNHI,
			"patient_abo":      req.PatientABO,
			"patient_rhd":      req.PatientRhD,
			"antibody_screen":  req.AntibodyScreen,
			"product_unit_ids": req.ProductUnitIDs,
			"compatibility":    compatibility,
			"requested_by":     req.RequestedBy,
			"notes":            req.Notes,
			"tenant_id":        tenantID,
		},
	)
	xm, err := scanCrossmatch(row)
	if err != nil {
		return Crossmatch{}, fmt.Errorf("insert crossmatch: %w", err)
	}
	return xm, nil
}

func (h *CrossmatchHandler) updateCrossmatch(ctx context.Context, xm Crossmatch) (Crossmatch, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE crossmatches
		 SET status          = @status,
		     compatibility   = @compatibility,
		     issued_by       = @issued_by,
		     transfused_by   = @transfused_by,
		     emergency_reason = @emergency_reason,
		     notes           = @notes,
		     issued_at       = @issued_at,
		     transfused_at   = @transfused_at,
		     cancelled_at    = @cancelled_at,
		     updated_at      = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, tenant_id, patient_id, patient_nhi, patient_abo, patient_rhd,
		           antibody_screen, product_unit_ids, status, compatibility,
		           requested_by, issued_by, transfused_by,
		           emergency_reason, notes,
		           requested_at, issued_at, transfused_at, cancelled_at,
		           created_at, updated_at`,
		db.NamedArgs{
			"status":           xm.Status,
			"compatibility":    xm.Compatibility,
			"issued_by":        xm.IssuedBy,
			"transfused_by":    xm.TransfusedBy,
			"emergency_reason": xm.EmergencyReason,
			"notes":            xm.Notes,
			"issued_at":        xm.IssuedAt,
			"transfused_at":    xm.TransfusedAt,
			"cancelled_at":     xm.CancelledAt,
			"id":               xm.ID,
			"tenant_id":        xm.TenantID,
		},
	)
	updated, err := scanCrossmatch(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Crossmatch{}, errNotFound
		}
		return Crossmatch{}, fmt.Errorf("update crossmatch: %w", err)
	}
	return updated, nil
}

// dbRow is satisfied by both pgx.Row and pgx.Rows.
type dbRow interface {
	Scan(dest ...any) error
}

// scanCrossmatch scans a single Crossmatch from a row (pgx.Row or pgx.Rows).
func scanCrossmatch(row dbRow) (Crossmatch, error) {
	var xm Crossmatch
	if err := row.Scan(
		&xm.ID, &xm.TenantID, &xm.PatientID, &xm.PatientNHI, &xm.PatientABO, &xm.PatientRhD,
		&xm.AntibodyScreen, &xm.ProductUnitIDs, &xm.Status, &xm.Compatibility,
		&xm.RequestedBy, &xm.IssuedBy, &xm.TransfusedBy,
		&xm.EmergencyReason, &xm.Notes,
		&xm.RequestedAt, &xm.IssuedAt, &xm.TransfusedAt, &xm.CancelledAt,
		&xm.CreatedAt, &xm.UpdatedAt,
	); err != nil {
		return Crossmatch{}, err
	}
	return xm, nil
}