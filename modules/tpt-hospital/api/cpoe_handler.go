package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/hl7"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// CPOEHandler handles Computerised Provider Order Entry routes.
type CPOEHandler struct {
	pool       db.Pool
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
	hl7Client  *hl7.MLLPClient // optional; nil when no MLLP endpoint configured
}

// List handles GET /api/v1/admissions/{admissionId}/orders.
func (h *CPOEHandler) List(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	orderTypeFilter := r.URL.Query().Get("orderType")
	statusFilter := r.URL.Query().Get("status")

	orders, err := h.listOrders(ctx, admissionID, tenantID.String(), orderTypeFilter, statusFilter)
	if err != nil {
		h.logger.Error("list clinical orders", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list orders"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "ClinicalOrder",
		ResourceID: admissionID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"orders": orders, "total": len(orders)})
}

// Create handles POST /api/v1/admissions/{admissionId}/orders.
func (h *CPOEHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	var req createOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.OrderType == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ORDER_TYPE", Message: "orderType is required"})
		return
	}
	if req.OrderCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ORDER_CODE", Message: "orderCode is required"})
		return
	}
	if req.OrderText == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ORDER_TEXT", Message: "orderText is required"})
		return
	}

	// Validate the ordering clinician's APC.
	apcStatus, err := h.hpiClient.ValidateAPC(ctx, principal.ID)
	if err != nil {
		h.logger.Error("HPI APC validation for ordering", slog.Any("error", err))
		writeJSON(w, http.StatusBadGateway, apiError{Code: "HPI_ERROR", Message: "could not validate ordering clinician APC"})
		return
	}
	if !apcStatus.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Code: "INVALID_APC", Message: "ordering clinician does not hold a current Annual Practising Certificate"})
		return
	}

	order, err := h.insertOrder(ctx, admissionID, tenantID.String(), req, principal.ID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "ADMISSION_NOT_FOUND", Message: "admission not found"})
			return
		}
		h.logger.Error("create clinical order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create order"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "ClinicalOrder",
		ResourceID: order.ID, TenantID: tenantID,
		Details:    map[string]any{"orderType": req.OrderType, "orderCode": req.OrderCode},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, order)
}

// Get handles GET /api/v1/admissions/{admissionId}/orders/{orderId}.
func (h *CPOEHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	orderID := r.PathValue("orderId")
	order, err := h.getOrderByID(ctx, orderID, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get clinical order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "ClinicalOrder",
		ResourceID: orderID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, order)
}

// Update handles PUT /api/v1/admissions/{admissionId}/orders/{orderId}.
func (h *CPOEHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	orderID := r.PathValue("orderId")
	existing, err := h.getOrderByID(ctx, orderID, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get order for update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}
	if existing.Status == OrderCompleted || existing.Status == OrderCancelled {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: "cannot update a completed or cancelled order"})
		return
	}

	var req updateOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if req.Priority != "" {
		existing.Priority = OrderPriority(req.Priority)
	}
	if req.Comments != "" {
		existing.Comments = req.Comments
	}
	if req.ScheduledFor != nil {
		existing.ScheduledFor = req.ScheduledFor
	}

	updated, err := h.updateOrder(ctx, existing)
	if err != nil {
		h.logger.Error("update clinical order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update order"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "update", ResourceType: "ClinicalOrder",
		ResourceID: orderID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}

// Cancel handles POST /api/v1/admissions/{admissionId}/orders/{orderId}/cancel.
func (h *CPOEHandler) Cancel(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	orderID := r.PathValue("orderId")

	var req cancelOrderRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Reason == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_REASON", Message: "reason is required"})
		return
	}

	order, err := h.cancelOrder(ctx, orderID, admissionID, tenantID.String(), req.Reason)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found or already in terminal status"})
			return
		}
		h.logger.Error("cancel clinical order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CANCEL_ERROR", Message: "failed to cancel order"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "cancel", ResourceType: "ClinicalOrder",
		ResourceID: orderID, TenantID: tenantID,
		Details:    map[string]any{"reason": req.Reason},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, order)
}

// Complete handles POST /api/v1/admissions/{admissionId}/orders/{orderId}/complete.
func (h *CPOEHandler) Complete(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	orderID := r.PathValue("orderId")

	var req completeOrderRequest
	_ = decodeJSON(r, &req) // body is optional

	order, err := h.completeOrder(ctx, orderID, admissionID, tenantID.String(), req.ResultID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found or already in terminal status"})
			return
		}
		h.logger.Error("complete clinical order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "COMPLETE_ERROR", Message: "failed to complete order"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "complete", ResourceType: "ClinicalOrder",
		ResourceID: orderID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, order)
}

// Dispatch handles POST /api/v1/admissions/{admissionId}/orders/{orderId}/dispatch.
// It sends an HL7 ORM^O01 message to the configured lab/radiology MLLP endpoint.
func (h *CPOEHandler) Dispatch(w http.ResponseWriter, r *http.Request) {
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

	admissionID := r.PathValue("admissionId")
	orderID := r.PathValue("orderId")

	existing, err := h.getOrderByID(ctx, orderID, admissionID, tenantID.String())
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found"})
			return
		}
		h.logger.Error("get order for dispatch", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve order"})
		return
	}
	if existing.Status != OrderPending && existing.Status != OrderInProgress {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "order must be pending or in_progress to dispatch"})
		return
	}

	// Generate placer order number.
	placerID := fmt.Sprintf("ORDER-%s", existing.ID[:8])

	// If MLLP client is configured, send the HL7 ORM^O01 message.
	if h.hl7Client != nil {
		priority := "R"
		switch existing.Priority {
		case PriorityStat:
			priority = "S"
		case PriorityASAP:
			priority = "A"
		case PriorityPreOp:
			priority = "R"
		}

		ormMsg := hl7.BuildORM(hl7.ORMOrder{
			OrderControl:  "NW",
			PlacerOrderID: placerID,
			PatientID:     existing.PatientNHI,
			OrderCode:     existing.OrderCode,
			OrderText:     existing.OrderText,
			OrderStatus:   "pending",
			Priority:      priority,
			RequestedBy:   existing.OrderedBy,
			OrderDateTime: existing.OrderedAt,
		})

		if err := h.hl7Client.Send(ctx, ormMsg); err != nil {
			h.logger.Error("dispatch HL7 ORM", slog.Any("error", err))
			writeJSON(w, http.StatusBadGateway, apiError{Code: "HL7_ERROR", Message: fmt.Sprintf("failed to dispatch order to lab/radiology: %v", err)})
			return
		}
	}

	order, err := h.dispatchOrder(ctx, orderID, admissionID, tenantID.String(), placerID)
	if err != nil {
		h.logger.Error("dispatch clinical order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DISPATCH_ERROR", Message: "failed to dispatch order"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "dispatch", ResourceType: "ClinicalOrder",
		ResourceID: orderID, TenantID: tenantID,
		Details:    map[string]any{"placerOrderId": placerID},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, order)
}
