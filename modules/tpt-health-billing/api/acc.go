package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
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

	// In production: query billing_acc_claims with the given filters, cursor pagination.
	writeJSON(w, http.StatusOK, map[string]any{
		"claims": []ACCClaim{},
		"total":  0,
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

	// In production:
	//   1. Validate NHI checksum via core/nhi.
	//   2. Validate provider APC via core/hpi (must be current and match discipline scope).
	//   3. Encrypt patientNhi with core/encryption before persisting.
	//   4. Persist to billing_acc_claims with status=pending.
	//   5. Write AuditEvent.

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

	// In production: load claim from billing_acc_claims by id.
	h.logger.Info("get ACC claim", "claim_id", id, "request_id", r.Context().Value(requestIDKey))

	writeError(w, http.StatusNotFound, "claim not found")
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

	// In production:
	//   1. Load claim, assert status == pending.
	//   2. Construct FHIR R5 Claim resource (core/fhir/r5).
	//      - Patient identifier: https://standards.digital.health.nz/ns/nhi-id
	//      - Provider identifier: https://standards.digital.health.nz/ns/hpi-person-id
	//      - Diagnosis system: http://hl7.org/fhir/sid/icd-10
	//   3. POST to ACC FHIR endpoint (cfg.ACCBaseURL) with mTLS + SMART bearer token.
	//   4. Parse ClaimResponse: extract ACC claim number and PO number.
	//   5. Update billing_acc_claims: status=active, acc_claim_number, lodged_at.
	//   6. Insert purchase order into billing_acc_purchase_orders.
	//   7. Write AuditEvent.

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

	// In production:
	//   1. Load claim from billing_acc_claims.
	//   2. If status is active: poll ACC FHIR ClaimResponse endpoint.
	//   3. Update local status if changed.
	//   4. Return current status.

	h.logger.Info("ACC claim status check", "claim_id", id, "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"claimId":   id,
		"status":    string(ACCClaimStatusActive),
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

	// In production: query billing_acc_purchase_orders with filters.
	writeJSON(w, http.StatusOK, map[string]any{
		"purchaseOrders": []ACCPurchaseOrder{},
		"total":          0,
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

	writeError(w, http.StatusNotFound, "purchase order not found")
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

	// In production:
	//   1. Load PO from billing_acc_purchase_orders, assert status=active.
	//   2. Assert used_sessions < max_sessions and (expiry_date is null OR service_date <= expiry_date).
	//   3. Increment used_sessions; if used_sessions == max_sessions set status=exhausted.
	//   4. Write AuditEvent.
	//   5. Return updated PO.

	h.logger.Info("ACC PO session consumed",
		"po_id", id,
		"service_date", req.ServiceDate,
		"provider_hpi", req.ProviderHPI,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"purchaseOrderId": id,
		"message":         "session recorded",
	})
}
