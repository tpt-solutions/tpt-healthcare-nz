package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-vision/internal/optical"
)

// OpticalHandler handles optical dispensing order CRUD operations.
type OpticalHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListOrders returns all dispensing orders for a patient.
func (h *OpticalHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	if patientNhi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "Patient NHI is required"})
		return
	}

	h.logger.Info("list dispensing orders", slog.String("patient_nhi", patientNhi))
	writeJSON(w, http.StatusOK, map[string]any{
		"patientNhi": patientNhi,
		"orders":     []any{},
	})
}

// CreateOrder creates a new optical dispensing order.
func (h *OpticalHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var order optical.DispensingOrder
	if err := decodeJSON(r, &order); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if err := order.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error()})
		return
	}

	h.logger.Info("dispensing order created", slog.String("patient_nhi", order.PatientNHI))
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":     "created",
		"patientNhi": order.PatientNHI,
		"orderDate":  order.OrderDate,
	})
}

// GetOrder returns a specific dispensing order.
func (h *OpticalHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	orderId := r.PathValue("orderId")

	if patientNhi == "" || orderId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and order ID are required"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"patientNhi": patientNhi,
		"orderId":    orderId,
	})
}

// UpdateOrder updates an existing dispensing order.
func (h *OpticalHandler) UpdateOrder(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	orderId := r.PathValue("orderId")

	if patientNhi == "" || orderId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and order ID are required"})
		return
	}

	var order optical.DispensingOrder
	if err := decodeJSON(r, &order); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	h.logger.Info("dispensing order updated", slog.String("patient_nhi", patientNhi), slog.String("order_id", orderId))
	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "updated",
		"patientNhi": patientNhi,
		"orderId":    orderId,
	})
}

// UpdateStatus updates the status of a dispensing order.
func (h *OpticalHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	orderId := r.PathValue("orderId")

	if patientNhi == "" || orderId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and order ID are required"})
		return
	}

	var req struct {
		Status optical.OrderStatus `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if req.Status == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_STATUS", Message: "Status is required"})
		return
	}

	h.logger.Info("dispensing order status updated",
		slog.String("patient_nhi", patientNhi),
		slog.String("order_id", orderId),
		slog.String("status", string(req.Status)))

	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "status_updated",
		"orderId":    orderId,
		"newStatus":  string(req.Status),
	})
}

// GetOrderFHIR returns a dispensing order as a FHIR R5 MedicationDispense resource.
func (h *OpticalHandler) GetOrderFHIR(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	orderId := r.PathValue("orderId")

	if patientNhi == "" || orderId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and order ID are required"})
		return
	}

	// TODO: retrieve order from DB
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
