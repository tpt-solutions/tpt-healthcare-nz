package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// resultCallbackRequest is the JSON body for POST /api/v1/orders/result-callback.
// Pathology/radiology systems call this when a result is ready.
type resultCallbackRequest struct {
	PlacerOrderID string `json:"placerOrderId"` // matches hl7_placer_order_id
	ResultID      string `json:"resultId"`      // FHIR DiagnosticReport ID (optional)
	Status        string `json:"status"`        // completed or partial_result
}

// ResultCallback handles POST /api/v1/orders/result-callback.
// Updates order status when lab/radiology results arrive.
func (h *CPOEHandler) ResultCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	var req resultCallbackRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.PlacerOrderID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PLACER_ORDER_ID", Message: "placerOrderId is required"})
		return
	}

	// Look up the order by placer order ID.
	o, err := h.getOrderByPlacerID(ctx, req.PlacerOrderID, tenantID.String())
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "order not found for placer order ID"})
		return
	}

	// Skip if already in terminal status.
	if o.Status == OrderCompleted || o.Status == OrderCancelled {
		writeJSON(w, http.StatusOK, map[string]any{"status": "already_terminal", "order": o})
		return
	}

	// Determine target status.
	targetStatus := OrderCompleted
	if req.Status == "partial_result" {
		targetStatus = OrderPartialResult
	}

	o.Status = targetStatus
	if req.ResultID != "" {
		o.ResultID = &req.ResultID
	}
	now := time.Now()
	o.ResultAt = &now
	o.CompletedAt = &now

	updated, err := h.updateOrder(ctx, o)
	if err != nil {
		h.logger.Error("result callback update order", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update order"})
		return
	}

	h.logger.Info("order result received",
		slog.String("orderId", o.ID),
		slog.String("placerOrderId", req.PlacerOrderID),
		slog.String("status", string(targetStatus)),
	)

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: "system", Action: "result", ResourceType: "ClinicalOrder",
		ResourceID: o.ID, TenantID: tenantID,
		Details:    map[string]any{"placerOrderId": req.PlacerOrderID, "resultId": req.ResultID},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, updated)
}
