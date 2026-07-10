package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- Domain types ---

// ACCClaimStatus reflects the ACC claim lifecycle.
type ACCClaimStatus string

const (
	ACCClaimStatusPending  ACCClaimStatus = "pending"  // submitted, awaiting ACC assessment
	ACCClaimStatusActive   ACCClaimStatus = "active"   // accepted, ongoing treatment authorised
	ACCClaimStatusDeclined ACCClaimStatus = "declined" // assessed and declined
	ACCClaimStatusComplete ACCClaimStatus = "complete" // fully resolved and paid
	ACCClaimStatusDisputed ACCClaimStatus = "disputed" // under review or appeal
)

// ACCDiscipline identifies the treatment discipline for purchase order management.
// Each discipline has its own treatment session caps and fee schedules under the
// ACC Treatment Provider Schedule.
type ACCDiscipline string

const (
	ACCDisciplineGP             ACCDiscipline = "GP"
	ACCDisciplinePhysio         ACCDiscipline = "PHYSIO"
	ACCDisciplineAcupuncture    ACCDiscipline = "ACUPUNCTURE"
	ACCDisciplineChiropractic   ACCDiscipline = "CHIROPRACTIC"
	ACCDisciplineMassage        ACCDiscipline = "MASSAGE"
	ACCDisciplineOsteopathy     ACCDiscipline = "OSTEOPATHY"
	ACCDisciplineCounselling    ACCDiscipline = "COUNSELLING"
	ACCDisciplineRehabilitation ACCDiscipline = "REHABILITATION"
)

// ACCClaim represents a cross-module ACC claim tracked by the billing service.
type ACCClaim struct {
	ID                  string         `json:"id"`
	TenantID            string         `json:"tenantId"`
	SourceModule        string         `json:"sourceModule"` // e.g. "tpt-doctor", "tpt-acupuncture"
	ACCClaimNumber      string         `json:"accClaimNumber,omitempty"`
	PurchaseOrderNumber string         `json:"purchaseOrderNumber,omitempty"`
	PatientNHI          string         `json:"patientNhi"` // encrypted at rest; returned masked to non-privileged callers
	ProviderHPI         string         `json:"providerHpi"`
	DateOfAccident      time.Time      `json:"dateOfAccident"`
	InjuryDescription   string         `json:"injuryDescription"`
	DiagnosisCodes      []string       `json:"diagnosisCodes"`
	Discipline          ACCDiscipline  `json:"discipline"`
	Status              ACCClaimStatus `json:"status"`
	LodgedAt            *time.Time     `json:"lodgedAt,omitempty"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
}

// CreateACCClaimRequest is the body for POST /api/v1/acc/claims.
type CreateACCClaimRequest struct {
	TenantID          string        `json:"tenantId"`
	SourceModule      string        `json:"sourceModule"`
	PatientNHI        string        `json:"patientNhi"`
	ProviderHPI       string        `json:"providerHpi"`
	DateOfAccident    time.Time     `json:"dateOfAccident"`
	InjuryDescription string        `json:"injuryDescription"`
	DiagnosisCodes    []string      `json:"diagnosisCodes"`
	Discipline        ACCDiscipline `json:"discipline"`
}

// ACCPurchaseOrderStatus reflects the purchase order lifecycle.
type ACCPurchaseOrderStatus string

const (
	ACCPOStatusActive    ACCPurchaseOrderStatus = "active"
	ACCPOStatusExhausted ACCPurchaseOrderStatus = "exhausted"
	ACCPOStatusCancelled ACCPurchaseOrderStatus = "cancelled"
)

// ACCPurchaseOrder tracks the session cap and consumption for an ACC-authorised
// treatment programme. ACC issues a purchase order (PO) per claim per discipline
// authorising a maximum number of treatment sessions.
type ACCPurchaseOrder struct {
	ID               string                 `json:"id"`
	TenantID         string                 `json:"tenantId"`
	ClaimID          string                 `json:"claimId"`
	PONumber         string                 `json:"poNumber"`
	Discipline       ACCDiscipline          `json:"discipline"`
	MaxSessions      int                    `json:"maxSessions"`
	UsedSessions     int                    `json:"usedSessions"`
	RemainingSession int                    `json:"remainingSessions"`
	FeePerSessionNZD float64                `json:"feePerSessionNzd"`
	Status           ACCPurchaseOrderStatus `json:"status"`
	ExpiryDate       *time.Time             `json:"expiryDate,omitempty"`
	CreatedAt        time.Time              `json:"createdAt"`
	UpdatedAt        time.Time              `json:"updatedAt"`
}

// ConsumeSessionRequest is the body for POST /api/v1/acc/purchase-orders/{id}/consume.
type ConsumeSessionRequest struct {
	// ServiceDate is the date on which the treatment session was delivered.
	ServiceDate time.Time `json:"serviceDate"`
	// ProviderHPI is the HPI CPN of the treating practitioner for this session.
	ProviderHPI string `json:"providerHpi"`
	// Notes holds any clinical notes relevant to ACC documentation requirements.
	Notes string `json:"notes,omitempty"`
}

// ACCHandler handles all /api/v1/acc/* routes.
type ACCHandler struct {
	pool   *pgxpool.Pool
	enc    *encryption.Encryptor
	logger *slog.Logger
}

// List handles GET /api/v1/acc/claims — list ACC claims with optional filters.
//
// Query parameters:
//   - status: filter by ACCClaimStatus
//   - source_module: filter by originating module (e.g. "tpt-acupuncture")
//   - tenant_id: filter by tenant
func (h *ACCHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	sourceModule := r.URL.Query().Get("source_module")
	tenantID := r.URL.Query().Get("tenant_id")

	h.logger.Info("list ACC claims",
		"status", status,
		"source_module", sourceModule,
		"tenant_id", tenantID,
		"request_id", r.Context().Value(requestIDKey),
	)

	q := newQueryDB(h.pool, h.enc)
	claims, err := q.listACCClaims(r.Context(), tenantID, status, sourceModule)
	if err != nil {
		h.logger.Error("list ACC claims", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list ACC claims")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"claims": claims,
		"total":  len(claims),
	})
}

// Create handles POST /api/v1/acc/claims — register a new ACC claim from any module.
//
// This endpoint is called by tpt-doctor, tpt-acupuncture, tpt-chiropractic,
// tpt-massage, tpt-physio, tpt-rehabilitation, etc. when a clinical module
// lodges an ACC claim. The billing service becomes the single source of truth
// for ACC claim lifecycle and purchase order management across all modules.
func (h *ACCHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateACCClaimRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("create ACC claim: decode: %v", err))
		return
	}

	if req.PatientNHI == "" {
		writeError(w, http.StatusUnprocessableEntity, "patientNhi is required")
		return
	}
	if req.ProviderHPI == "" {
		writeError(w, http.StatusUnprocessableEntity, "providerHpi is required")
		return
	}
	if req.InjuryDescription == "" {
		writeError(w, http.StatusUnprocessableEntity, "injuryDescription is required")
		return
	}
	if len(req.DiagnosisCodes) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "at least one diagnosisCode is required")
		return
	}
	if req.Discipline == "" {
		writeError(w, http.StatusUnprocessableEntity, "discipline is required")
		return
	}

	now := time.Now().UTC()
	claim := ACCClaim{
		ID:                fmt.Sprintf("acc-%d", now.UnixNano()),
		TenantID:          req.TenantID,
		SourceModule:      req.SourceModule,
		PatientNHI:        req.PatientNHI,
		ProviderHPI:       req.ProviderHPI,
		DateOfAccident:    req.DateOfAccident,
		InjuryDescription: req.InjuryDescription,
		DiagnosisCodes:    req.DiagnosisCodes,
		Discipline:        req.Discipline,
		Status:            ACCClaimStatusPending,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	q := newQueryDB(h.pool, h.enc)
	if err := q.insertACCClaim(r.Context(), claim); err != nil {
		h.logger.Error("insert ACC claim", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create ACC claim")
		return
	}

	h.logger.Info("ACC claim created",
		"claim_id", claim.ID,
		"source_module", claim.SourceModule,
		"discipline", claim.Discipline,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, claim)
}

// Get handles GET /api/v1/acc/claims/{id}.
func (h *ACCHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	h.logger.Info("get ACC claim", "claim_id", id, "request_id", r.Context().Value(requestIDKey))

	q := newQueryDB(h.pool, h.enc)
	claim, err := q.getACCClaim(r.Context(), id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "claim not found")
			return
		}
		h.logger.Error("get ACC claim", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get ACC claim")
		return
	}

	writeJSON(w, http.StatusOK, claim)
}

// Submit handles POST /api/v1/acc/claims/{id}/submit — lodge the claim with ACC.
//
// ACC claims are submitted as FHIR R5 Claim resources to the ACC FHIR endpoint,
// authenticated via SMART on FHIR bearer tokens. On success ACC issues a purchase
// order for the authorised treatment sessions.
func (h *ACCHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	q := newQueryDB(h.pool, h.enc)
	claim, err := q.getACCClaim(r.Context(), id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "claim not found")
			return
		}
		h.logger.Error("get ACC claim for submit", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load ACC claim")
		return
	}
	if string(claim.Status) != string(ACCClaimStatusPending) {
		writeError(w, http.StatusConflict, "ACC claim must be in pending status to submit")
		return
	}

	// In production:
	//   2. Construct FHIR R5 Claim resource (core/fhir/r5).
	//   3. POST to ACC FHIR endpoint (cfg.ACCBaseURL) with mTLS + SMART bearer token.
	//   4. Parse ClaimResponse: extract ACC claim number and PO number.
	//   5. Insert purchase order into billing_acc_purchase_orders.
	//   6. Write AuditEvent.

	// For now, update status to active with placeholder claim/PO numbers.
	claimNumber := fmt.Sprintf("ACC-%s", id)
	poNumber := fmt.Sprintf("PO-%s", id)

	if err := q.updateACCClaimStatus(r.Context(), id, ACCClaimStatusActive, claimNumber, poNumber); err != nil {
		h.logger.Error("update ACC claim status", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to submit ACC claim")
		return
	}

	now := time.Now().UTC()

	h.logger.Info("ACC claim submitted",
		"claim_id", id,
		"lodged_at", now,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"claimId":  id,
		"status":   string(ACCClaimStatusActive),
		"lodgedAt": now,
	})
}

// Status handles GET /api/v1/acc/claims/{id}/status — poll ACC for current claim status.
func (h *ACCHandler) Status(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	q := newQueryDB(h.pool, h.enc)
	claim, err := q.getACCClaim(r.Context(), id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "claim not found")
			return
		}
		h.logger.Error("get ACC claim status", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to check ACC claim status")
		return
	}

	h.logger.Info("ACC claim status check", "claim_id", id, "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"claimId":   id,
		"status":    string(claim.Status),
		"checkedAt": time.Now().UTC(),
	})
}

// ListPurchaseOrders handles GET /api/v1/acc/purchase-orders.
//
// Query parameters:
//   - claim_id: filter by ACC claim
//   - status: filter by ACCPurchaseOrderStatus
//   - discipline: filter by ACCDiscipline
func (h *ACCHandler) ListPurchaseOrders(w http.ResponseWriter, r *http.Request) {
	claimID := r.URL.Query().Get("claim_id")
	status := r.URL.Query().Get("status")
	discipline := r.URL.Query().Get("discipline")

	h.logger.Info("list ACC purchase orders",
		"claim_id", claimID,
		"status", status,
		"discipline", discipline,
		"request_id", r.Context().Value(requestIDKey),
	)

	q := newQueryDB(h.pool, h.enc)
	pos, err := q.listACCPurchaseOrders(r.Context(), claimID, status, discipline)
	if err != nil {
		h.logger.Error("list ACC purchase orders", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list purchase orders")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"purchaseOrders": pos,
		"total":          len(pos),
	})
}

// GetPurchaseOrder handles GET /api/v1/acc/purchase-orders/{id}.
func (h *ACCHandler) GetPurchaseOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	h.logger.Info("get ACC purchase order", "po_id", id, "request_id", r.Context().Value(requestIDKey))

	q := newQueryDB(h.pool, h.enc)
	po, err := q.getACCPurchaseOrder(r.Context(), id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "purchase order not found")
			return
		}
		h.logger.Error("get ACC purchase order", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get purchase order")
		return
	}

	writeJSON(w, http.StatusOK, po)
}

// ConsumePurchaseOrderSession handles POST /api/v1/acc/purchase-orders/{id}/consume.
//
// Records a treatment session against a purchase order, decrementing the remaining
// session count. Called by clinical modules each time an ACC-covered treatment session
// is delivered. Returns an error if the PO is exhausted or expired.
func (h *ACCHandler) ConsumePurchaseOrderSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	var req ConsumeSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("consume session: decode: %v", err))
		return
	}

	if req.ProviderHPI == "" {
		writeError(w, http.StatusUnprocessableEntity, "providerHpi is required")
		return
	}

	q := newQueryDB(h.pool, h.enc)
	po, err := q.consumeACCPurchaseOrderSession(r.Context(), id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "purchase order not found")
			return
		}
		h.logger.Error("consume ACC PO session", "error", err)
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	h.logger.Info("ACC PO session consumed",
		"po_id", id,
		"service_date", req.ServiceDate,
		"provider_hpi", req.ProviderHPI,
		"used_sessions", po.UsedSessions,
		"remaining_sessions", po.RemainingSession,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusOK, po)
}
