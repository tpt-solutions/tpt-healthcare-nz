package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TreatmentOrdersHandler handles all /api/v1/orders routes.
type TreatmentOrdersHandler struct {
	pool       *pgxpool.Pool
	enc        *encryption.Encryptor
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/orders.
// Query params: patient (internal ID), status, type.
func (h *TreatmentOrdersHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	q := r.URL.Query()
	patientFilter := q.Get("patient")
	statusFilter := q.Get("status")
	typeFilter := q.Get("type")

	records, err := h.listOrders(ctx, tenantID, patientFilter, statusFilter, typeFilter)
	if err != nil {
		h.logger.Error("list orders", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list orders"})
		return
	}

	responses := make([]CompulsoryOrder, 0, len(records))
	for _, rec := range records {
		o, err := h.decryptOrder(rec)
		if err != nil {
			h.logger.Error("decrypt order", slog.Any("error", err), slog.String("id", rec.ID))
			continue
		}
		if accessErr := checkMHAccess(ctx, h.pool, tenantID, o.PatientNHI, principal); accessErr != nil {
			continue
		}
		responses = append(responses, o)
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "CompulsoryOrder",
		ResourceID:   "list",
	})

	writeJSON(w, http.StatusOK, map[string]any{"orders": responses, "total": len(responses)})
}

// Get handles GET /api/v1/orders/{id}.
func (h *TreatmentOrdersHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "order ID is required"})
		return
	}

	rec, err := h.getOrderByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}

	o, err := h.decryptOrder(rec)
	if err != nil {
		h.logger.Error("decrypt order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt order"})
		return
	}

	if accessErr := checkMHAccess(ctx, h.pool, tenantID, o.PatientNHI, principal); accessErr != nil {
		writeJSON(w, http.StatusForbidden, apiError{Code: "ACCESS_DENIED", Message: accessErr.Error()})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "read",
		ResourceType: "CompulsoryOrder",
		ResourceID:   id,
		PatientNHI:   o.PatientNHI,
	})

	writeJSON(w, http.StatusOK, o)
}

// Create handles POST /api/v1/orders.
// CTOs require a second opinion HPI (endorsed psychiatrist).
// APC is validated for both the responsible clinician and the second opinion practitioner.
func (h *TreatmentOrdersHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req orderCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PatientID == "" && req.PatientNHI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PATIENT", Message: "patientId or patientNhi is required"})
		return
	}
	if req.ResponsibleHPI == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_RC", Message: "responsibleHpi (Responsible Clinician) is required"})
		return
	}
	if !validOrderType(req.OrderType) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_ORDER_TYPE", Message: fmt.Sprintf("unknown order type %q", req.OrderType)})
		return
	}
	if req.IssuedDate == "" || req.ExpiryDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DATES", Message: "issuedDate and expiryDate are required"})
		return
	}
	if req.FirstReviewDate == "" || req.NextReviewDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_REVIEW_DATES", Message: "firstReviewDate and nextReviewDate are required"})
		return
	}

	// CTOs require a second opinion from an endorsed psychiatrist (s30(4) MHCAA).
	if req.OrderType == OrderCTOInpatient || req.OrderType == OrderCTOCommunity {
		if req.SecondOpinionHPI == "" {
			writeJSON(w, http.StatusBadRequest, apiError{
				Code:    "MISSING_SECOND_OPINION",
				Message: "secondOpinionHpi is required for compulsory treatment orders (MHCAA s30(4))",
			})
			return
		}
	}

	// Validate APC for the Responsible Clinician.
	apcStatus, err := h.hpiClient.ValidateAPC(ctx, req.ResponsibleHPI)
	if err != nil {
		h.logger.Error("HPI APC check (RC)", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not verify responsible clinician APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusForbidden, apiError{Code: "INVALID_RC_APC", Message: "responsible clinician does not hold a current APC"})
		return
	}

	// Validate APC for the second opinion practitioner when required.
	if req.SecondOpinionHPI != "" {
		soStatus, err := h.hpiClient.ValidateAPC(ctx, req.SecondOpinionHPI)
		if err != nil {
			h.logger.Error("HPI APC check (second opinion)", slog.Any("error", err))
			writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not verify second opinion practitioner APC"})
			return
		}
		if !soStatus.Valid {
			writeJSON(w, http.StatusForbidden, apiError{Code: "INVALID_SO_APC", Message: "second opinion practitioner does not hold a current APC"})
			return
		}
	}

	rec, err := h.insertOrder(ctx, req, tenantID)
	if err != nil {
		h.logger.Error("insert order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create order"})
		return
	}

	o, err := h.decryptOrder(rec)
	if err != nil {
		h.logger.Error("decrypt order after insert", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt order"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "create",
		ResourceType: "CompulsoryOrder",
		ResourceID:   rec.ID,
		PatientNHI:   req.PatientNHI,
		Details: map[string]any{
			"orderType": string(req.OrderType),
			"issued":    req.IssuedDate,
			"expiry":    req.ExpiryDate,
		},
	})

	writeJSON(w, http.StatusCreated, o)
}

// Update handles PUT /api/v1/orders/{id}.
// Allows updating the responsible clinician, conditions, expiry, and review dates.
func (h *TreatmentOrdersHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "order ID is required"})
		return
	}

	var req orderUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	existing, err := h.getOrderByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get order for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}

	if existing.Status == string(OrderRevoked) || existing.Status == string(OrderExpired) {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot update a revoked or expired order"})
		return
	}

	if req.ResponsibleHPI != "" {
		existing.ResponsibleHPI = req.ResponsibleHPI
	}
	if req.SecondOpinionHPI != "" {
		existing.SecondOpinionHPI = req.SecondOpinionHPI
	}
	if req.LegalAuthority != "" {
		existing.LegalAuthority = req.LegalAuthority
	}
	if req.ExpiryDate != "" {
		existing.ExpiryDate = req.ExpiryDate
	}
	if req.NextReviewDate != "" {
		existing.NextReviewDate = req.NextReviewDate
	}
	if req.TribunalReference != "" {
		existing.TribunalReference = req.TribunalReference
	}
	if req.Status != "" {
		existing.Status = string(req.Status)
	}

	var condEnc []byte
	if req.Conditions != "" {
		condEnc, err = h.enc.Encrypt([]byte(req.Conditions))
		if err != nil {
			h.logger.Error("encrypt conditions", slog.Any("error", err))
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt conditions"})
			return
		}
	} else {
		condEnc = existing.ConditionsEnc
	}

	updated, err := h.updateOrder(ctx, existing, condEnc, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("update order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update order"})
		return
	}

	o, err := h.decryptOrder(updated)
	if err != nil {
		h.logger.Error("decrypt order after update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt order"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "CompulsoryOrder",
		ResourceID:   id,
	})

	writeJSON(w, http.StatusOK, o)
}

// RecordReview handles POST /api/v1/orders/{id}/review.
// Records a mandatory legal review and advances the next_review_date.
func (h *TreatmentOrdersHandler) RecordReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "order ID is required"})
		return
	}

	var req reviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ReviewedAt == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_DATE", Message: "reviewedAt (YYYY-MM-DD) is required"})
		return
	}
	if req.NextReviewDate == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NEXT_DATE", Message: "nextReviewDate (YYYY-MM-DD) is required"})
		return
	}

	existing, err := h.getOrderByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get order for review", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}

	if existing.Status != string(OrderActive) && existing.Status != string(OrderSuspended) {
		writeJSON(w, http.StatusConflict, apiError{Code: "NOT_REVIEWABLE", Message: "only active or suspended orders can be reviewed"})
		return
	}

	// If the review outcome is "discharged" the order is revoked.
	newStatus := existing.Status
	if req.Outcome == "discharged" {
		newStatus = string(OrderRevoked)
	}

	updated, err := h.recordReview(ctx, id, req.ReviewedAt, req.NextReviewDate, newStatus, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("record review", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "REVIEW_ERROR", Message: "failed to record review"})
		return
	}

	o, err := h.decryptOrder(updated)
	if err != nil {
		h.logger.Error("decrypt order after review", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt order"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "CompulsoryOrder",
		ResourceID:   id,
		Details: map[string]any{
			"action":         "review",
			"reviewedAt":     req.ReviewedAt,
			"outcome":        req.Outcome,
			"nextReviewDate": req.NextReviewDate,
		},
	})

	writeJSON(w, http.StatusOK, o)
}

// Revoke handles POST /api/v1/orders/{id}/revoke.
// Terminates a compulsory order before its expiry date.
func (h *TreatmentOrdersHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "order ID is required"})
		return
	}

	var req revokeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Reason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_REASON", Message: "reason for revocation is required"})
		return
	}

	existing, err := h.getOrderByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get order for revoke", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}

	if existing.Status == string(OrderRevoked) || existing.Status == string(OrderExpired) {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_TERMINAL", Message: "order is already revoked or expired"})
		return
	}

	reasonEnc, err := h.enc.Encrypt([]byte(req.Reason))
	if err != nil {
		h.logger.Error("encrypt revocation reason", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt reason"})
		return
	}

	updated, err := h.revokeOrder(ctx, id, reasonEnc, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("revoke order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "REVOKE_ERROR", Message: "failed to revoke order"})
		return
	}

	o, err := h.decryptOrder(updated)
	if err != nil {
		h.logger.Error("decrypt order after revoke", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt order"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       "update",
		ResourceType: "CompulsoryOrder",
		ResourceID:   id,
		PatientNHI:   o.PatientNHI,
		Details:      map[string]any{"action": "revoke"},
	})

	writeJSON(w, http.StatusOK, o)
}
