// Package api — compulsory orders under the Mental Health (Compulsory Assessment
// and Treatment) Act 1992 (MHCAA 1992).
//
// Order types:
//   CAO           — Compulsory Assessment Order (s11): initial 5-day assessment.
//   CTO-inpatient — Compulsory Treatment Order, inpatient (s30): treatment required
//                   as an inpatient.
//   CTO-community — Compulsory Treatment Order, community (s29): conditions on living
//                   in the community.
//   SPO           — Special Patient Order (s34): for persons acquitted on insanity
//                   grounds or transferred from a penal institution.
//
// Status transitions:
//   active → suspended → active  (temporary suspension by RC)
//   active → revoked              (ended before expiry)
//   active → expired              (automatic on expiry_date)
//   active → appealed             (Mental Health Review Tribunal appeal filed)
//
// Mandatory review dates are enforced by business logic; a next_review_date
// column flags overdue reviews in the dashboard.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OrderType classifies the compulsory order under MHCAA 1992.
type OrderType string

const (
	OrderCAO           OrderType = "CAO"
	OrderCTOInpatient  OrderType = "CTO-inpatient"
	OrderCTOCommunity  OrderType = "CTO-community"
	OrderSPO           OrderType = "SPO"
)

// OrderStatus tracks the MHCAA 1992 order lifecycle.
type OrderStatus string

const (
	OrderActive    OrderStatus = "active"
	OrderSuspended OrderStatus = "suspended"
	OrderExpired   OrderStatus = "expired"
	OrderRevoked   OrderStatus = "revoked"
	OrderAppealed  OrderStatus = "appealed"
)

// CompulsoryOrder represents a MHCAA 1992 compulsory order record.
type CompulsoryOrder struct {
	ID                string      `json:"id"`
	PatientID         string      `json:"patientId"`
	PatientNHI        string      `json:"patientNhi"`
	TenantID          string      `json:"tenantId"`
	EpisodeID         string      `json:"episodeId,omitempty"`
	OrderType         OrderType   `json:"orderType"`
	Status            OrderStatus `json:"status"`
	ResponsibleHPI    string      `json:"responsibleHpi"`
	SecondOpinionHPI  string      `json:"secondOpinionHpi,omitempty"`
	LegalAuthority    string      `json:"legalAuthority,omitempty"`
	Conditions        string      `json:"conditions,omitempty"` // decrypted
	IssuedDate        string      `json:"issuedDate"`           // YYYY-MM-DD
	ExpiryDate        string      `json:"expiryDate"`           // YYYY-MM-DD
	FirstReviewDate   string      `json:"firstReviewDate"`      // YYYY-MM-DD
	LastReviewDate    string      `json:"lastReviewDate,omitempty"`
	NextReviewDate    string      `json:"nextReviewDate"`       // YYYY-MM-DD
	TribunalReference string      `json:"tribunalReference,omitempty"`
	ExtraSensitive    bool        `json:"extraSensitive"`
	CreatedAt         time.Time   `json:"createdAt"`
	UpdatedAt         time.Time   `json:"updatedAt"`
}

// orderCreateRequest is the body for POST /api/v1/orders.
type orderCreateRequest struct {
	PatientID        string    `json:"patientId"`
	PatientNHI       string    `json:"patientNhi"`
	EpisodeID        string    `json:"episodeId,omitempty"`
	OrderType        OrderType `json:"orderType"`
	ResponsibleHPI   string    `json:"responsibleHpi"`
	SecondOpinionHPI string    `json:"secondOpinionHpi,omitempty"`
	LegalAuthority   string    `json:"legalAuthority,omitempty"`
	Conditions       string    `json:"conditions,omitempty"`
	IssuedDate       string    `json:"issuedDate"`       // YYYY-MM-DD
	ExpiryDate       string    `json:"expiryDate"`       // YYYY-MM-DD
	FirstReviewDate  string    `json:"firstReviewDate"`  // YYYY-MM-DD
	NextReviewDate   string    `json:"nextReviewDate"`   // YYYY-MM-DD
}

// orderUpdateRequest is the body for PUT /api/v1/orders/{id}.
type orderUpdateRequest struct {
	ResponsibleHPI    string    `json:"responsibleHpi,omitempty"`
	SecondOpinionHPI  string    `json:"secondOpinionHpi,omitempty"`
	LegalAuthority    string    `json:"legalAuthority,omitempty"`
	Conditions        string    `json:"conditions,omitempty"`
	ExpiryDate        string    `json:"expiryDate,omitempty"`
	NextReviewDate    string    `json:"nextReviewDate,omitempty"`
	TribunalReference string    `json:"tribunalReference,omitempty"`
	Status            OrderStatus `json:"status,omitempty"`
}

// reviewRequest is the body for POST /api/v1/orders/{id}/review.
// Records a mandatory legal review of an active compulsory order.
type reviewRequest struct {
	ReviewedAt     string `json:"reviewedAt"`     // YYYY-MM-DD
	NextReviewDate string `json:"nextReviewDate"` // YYYY-MM-DD
	Outcome        string `json:"outcome"`        // "continued", "varied", "discharged"
	Notes          string `json:"notes,omitempty"`
}

// revokeRequest is the body for POST /api/v1/orders/{id}/revoke.
type revokeRequest struct {
	Reason string `json:"reason"`
}

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

// ---------------------------------------------------------------------------
// Internal record type
// ---------------------------------------------------------------------------

type orderRecord struct {
	ID               string
	PatientID        string
	PatientNHI       string
	TenantID         string
	EpisodeID        string
	OrderType        string
	Status           string
	ResponsibleHPI   string
	SecondOpinionHPI string
	LegalAuthority   string
	ConditionsEnc    []byte
	IssuedDate       string
	ExpiryDate       string
	FirstReviewDate  string
	LastReviewDate   string
	NextReviewDate   string
	RevocationEnc    []byte
	TribunalReference string
	ExtraSensitive   bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ---------------------------------------------------------------------------
// Database helpers
// ---------------------------------------------------------------------------

func (h *TreatmentOrdersHandler) listOrders(
	ctx context.Context,
	tenantID uuid.UUID,
	patientFilter, statusFilter, typeFilter string,
) ([]orderRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, episode_id,
		        order_type, status, responsible_hpi, second_opinion_hpi,
		        legal_authority, conditions,
		        issued_date::text, expiry_date::text, first_review_date::text,
		        last_review_date::text, next_review_date::text,
		        revocation_reason, tribunal_reference, extra_sensitive,
		        created_at, updated_at
		 FROM compulsory_orders
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR status = $3)
		   AND ($4 = '' OR order_type = $4)
		 ORDER BY issued_date DESC
		 LIMIT 200`,
		tenantID, patientFilter, statusFilter, typeFilter,
	)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	var results []orderRecord
	for rows.Next() {
		rec, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (h *TreatmentOrdersHandler) getOrderByID(ctx context.Context, id string, tenantID uuid.UUID) (orderRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, episode_id,
		        order_type, status, responsible_hpi, second_opinion_hpi,
		        legal_authority, conditions,
		        issued_date::text, expiry_date::text, first_review_date::text,
		        last_review_date::text, next_review_date::text,
		        revocation_reason, tribunal_reference, extra_sensitive,
		        created_at, updated_at
		 FROM compulsory_orders
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanOrderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderRecord{}, errNotFound
		}
		return orderRecord{}, fmt.Errorf("get order by id: %w", err)
	}
	return rec, nil
}

func (h *TreatmentOrdersHandler) insertOrder(ctx context.Context, req orderCreateRequest, tenantID uuid.UUID) (orderRecord, error) {
	var condEnc []byte
	if req.Conditions != "" {
		var err error
		condEnc, err = h.enc.Encrypt([]byte(req.Conditions))
		if err != nil {
			return orderRecord{}, fmt.Errorf("encrypt conditions: %w", err)
		}
	}

	var episodeID *string
	if req.EpisodeID != "" {
		episodeID = &req.EpisodeID
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO compulsory_orders
		   (patient_id, patient_nhi, tenant_id, episode_id,
		    order_type, status, responsible_hpi, second_opinion_hpi,
		    legal_authority, conditions,
		    issued_date, expiry_date, first_review_date, next_review_date, extra_sensitive)
		 VALUES
		   ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, TRUE)
		 RETURNING id, patient_id, patient_nhi, tenant_id, episode_id,
		           order_type, status, responsible_hpi, second_opinion_hpi,
		           legal_authority, conditions,
		           issued_date::text, expiry_date::text, first_review_date::text,
		           last_review_date::text, next_review_date::text,
		           revocation_reason, tribunal_reference, extra_sensitive,
		           created_at, updated_at`,
		req.PatientID, req.PatientNHI, tenantID, episodeID,
		string(req.OrderType), string(OrderActive),
		req.ResponsibleHPI, req.SecondOpinionHPI,
		req.LegalAuthority, condEnc,
		req.IssuedDate, req.ExpiryDate, req.FirstReviewDate, req.NextReviewDate,
	)
	return scanOrderRow(row)
}

func (h *TreatmentOrdersHandler) updateOrder(ctx context.Context, rec orderRecord, condEnc []byte, tenantID uuid.UUID) (orderRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE compulsory_orders
		 SET responsible_hpi   = $1,
		     second_opinion_hpi = $2,
		     legal_authority    = $3,
		     conditions         = $4,
		     expiry_date        = $5::date,
		     next_review_date   = $6::date,
		     tribunal_reference = $7,
		     status             = $8,
		     updated_at         = now()
		 WHERE id = $9 AND tenant_id = $10
		 RETURNING id, patient_id, patient_nhi, tenant_id, episode_id,
		           order_type, status, responsible_hpi, second_opinion_hpi,
		           legal_authority, conditions,
		           issued_date::text, expiry_date::text, first_review_date::text,
		           last_review_date::text, next_review_date::text,
		           revocation_reason, tribunal_reference, extra_sensitive,
		           created_at, updated_at`,
		rec.ResponsibleHPI, rec.SecondOpinionHPI, rec.LegalAuthority,
		condEnc, rec.ExpiryDate, rec.NextReviewDate, rec.TribunalReference, rec.Status,
		rec.ID, tenantID,
	)
	updated, err := scanOrderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderRecord{}, errNotFound
		}
		return orderRecord{}, fmt.Errorf("update order: %w", err)
	}
	return updated, nil
}

func (h *TreatmentOrdersHandler) recordReview(
	ctx context.Context,
	id, reviewedAt, nextReviewDate, newStatus string,
	tenantID uuid.UUID,
) (orderRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE compulsory_orders
		 SET last_review_date = $1::date,
		     next_review_date = $2::date,
		     status           = $3,
		     updated_at       = now()
		 WHERE id = $4 AND tenant_id = $5
		 RETURNING id, patient_id, patient_nhi, tenant_id, episode_id,
		           order_type, status, responsible_hpi, second_opinion_hpi,
		           legal_authority, conditions,
		           issued_date::text, expiry_date::text, first_review_date::text,
		           last_review_date::text, next_review_date::text,
		           revocation_reason, tribunal_reference, extra_sensitive,
		           created_at, updated_at`,
		reviewedAt, nextReviewDate, newStatus, id, tenantID,
	)
	updated, err := scanOrderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderRecord{}, errNotFound
		}
		return orderRecord{}, fmt.Errorf("record review: %w", err)
	}
	return updated, nil
}

func (h *TreatmentOrdersHandler) revokeOrder(ctx context.Context, id string, reasonEnc []byte, tenantID uuid.UUID) (orderRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE compulsory_orders
		 SET status            = $1,
		     revocation_reason = $2,
		     updated_at        = now()
		 WHERE id = $3 AND tenant_id = $4
		 RETURNING id, patient_id, patient_nhi, tenant_id, episode_id,
		           order_type, status, responsible_hpi, second_opinion_hpi,
		           legal_authority, conditions,
		           issued_date::text, expiry_date::text, first_review_date::text,
		           last_review_date::text, next_review_date::text,
		           revocation_reason, tribunal_reference, extra_sensitive,
		           created_at, updated_at`,
		string(OrderRevoked), reasonEnc, id, tenantID,
	)
	updated, err := scanOrderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderRecord{}, errNotFound
		}
		return orderRecord{}, fmt.Errorf("revoke order: %w", err)
	}
	return updated, nil
}

func (h *TreatmentOrdersHandler) decryptOrder(rec orderRecord) (CompulsoryOrder, error) {
	var conditions string
	if len(rec.ConditionsEnc) > 0 {
		plain, err := h.enc.Decrypt(rec.ConditionsEnc)
		if err != nil {
			return CompulsoryOrder{}, fmt.Errorf("decrypt conditions: %w", err)
		}
		conditions = string(plain)
	}
	return CompulsoryOrder{
		ID:                rec.ID,
		PatientID:         rec.PatientID,
		PatientNHI:        rec.PatientNHI,
		TenantID:          rec.TenantID,
		EpisodeID:         rec.EpisodeID,
		OrderType:         OrderType(rec.OrderType),
		Status:            OrderStatus(rec.Status),
		ResponsibleHPI:    rec.ResponsibleHPI,
		SecondOpinionHPI:  rec.SecondOpinionHPI,
		LegalAuthority:    rec.LegalAuthority,
		Conditions:        conditions,
		IssuedDate:        rec.IssuedDate,
		ExpiryDate:        rec.ExpiryDate,
		FirstReviewDate:   rec.FirstReviewDate,
		LastReviewDate:    rec.LastReviewDate,
		NextReviewDate:    rec.NextReviewDate,
		TribunalReference: rec.TribunalReference,
		ExtraSensitive:    rec.ExtraSensitive,
		CreatedAt:         rec.CreatedAt,
		UpdatedAt:         rec.UpdatedAt,
	}, nil
}

func scanOrder(s rowScanner) (orderRecord, error) {
	return scanOrderRow(s)
}

func scanOrderRow(s rowScanner) (orderRecord, error) {
	var rec orderRecord
	if err := s.Scan(
		&rec.ID, &rec.PatientID, &rec.PatientNHI, &rec.TenantID, &rec.EpisodeID,
		&rec.OrderType, &rec.Status, &rec.ResponsibleHPI, &rec.SecondOpinionHPI,
		&rec.LegalAuthority, &rec.ConditionsEnc,
		&rec.IssuedDate, &rec.ExpiryDate, &rec.FirstReviewDate,
		&rec.LastReviewDate, &rec.NextReviewDate,
		&rec.RevocationEnc, &rec.TribunalReference, &rec.ExtraSensitive,
		&rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return orderRecord{}, err
	}
	return rec, nil
}

func validOrderType(t OrderType) bool {
	switch t {
	case OrderCAO, OrderCTOInpatient, OrderCTOCommunity, OrderSPO:
		return true
	}
	return false
}
