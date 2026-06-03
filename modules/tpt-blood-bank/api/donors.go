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

// BloodGroup represents an ABO/RhD blood type.
type BloodGroup string

const (
	BloodGroupAPos  BloodGroup = "A+"
	BloodGroupANeg  BloodGroup = "A-"
	BloodGroupBPos  BloodGroup = "B+"
	BloodGroupBNeg  BloodGroup = "B-"
	BloodGroupABPos BloodGroup = "AB+"
	BloodGroupABNeg BloodGroup = "AB-"
	BloodGroupOPos  BloodGroup = "O+"
	BloodGroupONeg  BloodGroup = "O-"
)

// ValidBloodGroups contains all recognised NZ blood group values.
var ValidBloodGroups = map[BloodGroup]bool{
	BloodGroupAPos:  true,
	BloodGroupANeg:  true,
	BloodGroupBPos:  true,
	BloodGroupBNeg:  true,
	BloodGroupABPos: true,
	BloodGroupABNeg: true,
	BloodGroupOPos:  true,
	BloodGroupONeg:  true,
}

// DeferralReason describes why a donor is temporarily or permanently deferred.
type DeferralReason string

const (
	DeferralLowHaemoglobin   DeferralReason = "low-haemoglobin"
	DeferralRecentTravel     DeferralReason = "recent-travel"
	DeferralMedicalCondition DeferralReason = "medical-condition"
	DeferralMedication       DeferralReason = "medication"
	DeferralTattooPiercing   DeferralReason = "tattoo-piercing"
	DeferralUnderweight      DeferralReason = "underweight"
	DeferralBehaviouralRisk  DeferralReason = "behavioural-risk"
	DeferralPermanent        DeferralReason = "permanent"
)

// DeferralDuration maps reasons to standard deferral periods (in days).
// Zero means permanent deferral.
var DeferralDuration = map[DeferralReason]int{
	DeferralLowHaemoglobin:   180,
	DeferralRecentTravel:     120,
	DeferralMedicalCondition: 0, // assessed case-by-case
	DeferralMedication:       0, // assessed case-by-case
	DeferralTattooPiercing:   120,
	DeferralUnderweight:      0, // permanent until weight gain confirmed
	DeferralBehaviouralRisk:  365,
	DeferralPermanent:        0, // permanent
}

// DonorStatus represents a donor's current eligibility state.
type DonorStatus string

const (
	DonorStatusActive    DonorStatus = "active"
	DonorStatusDeferred  DonorStatus = "deferred"
	DonorStatusPermanent DonorStatus = "permanent"
	DonorStatusInactive  DonorStatus = "inactive"
)

// Donor is the domain model for a blood donor.
type Donor struct {
	ID              string       `json:"id"`
	TenantID        string       `json:"tenantId"`
	NHI             string       `json:"nhi,omitempty"`
	BloodGroup      BloodGroup   `json:"bloodGroup"`
	RhD             string       `json:"rhd"` // "POSITIVE" or "NEGATIVE"
	Status          DonorStatus  `json:"status"`
	DeferralReason  *string      `json:"deferralReason,omitempty"`
	DeferralEndDate *time.Time   `json:"deferralEndDate,omitempty"`
	TotalDonations  int          `json:"totalDonations"`
	LastDonationAt  *time.Time   `json:"lastDonationAt,omitempty"`
	HaemoglobinGDL  *float64     `json:"haemoglobinGdl,omitempty"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

// DonationRecord tracks a single donation event.
type DonationRecord struct {
	ID            string    `json:"id"`
	DonorID       string    `json:"donorId"`
	ProductUnitID string    `json:"productUnitId"`
	VolumeML      int       `json:"volumeMl"`
	DonationType  string    `json:"donationType"` // whole-blood, apheresis-platelets, apheresis-plasma
	CollectedAt   time.Time `json:"collectedAt"`
	CreatedAt     time.Time `json:"createdAt"`
}

// donorCreateRequest is the body for POST /api/v1/donors.
type donorCreateRequest struct {
	NHI        string     `json:"nhi"`
	BloodGroup BloodGroup `json:"bloodGroup"`
}

// donorUpdateRequest is the body for PUT /api/v1/donors/{id}.
type donorUpdateRequest struct {
	BloodGroup     *BloodGroup `json:"bloodGroup,omitempty"`
	HaemoglobinGDL *float64    `json:"haemoglobinGdl,omitempty"`
}

// donorDeferRequest is the body for POST /api/v1/donors/{id}/defer.
type donorDeferRequest struct {
	Reason  DeferralReason `json:"reason"`
	Details string         `json:"details,omitempty"`
}

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

// --- Database operations ---

func (h *DonorsHandler) listDonors(ctx context.Context, tenantID, statusFilter, bloodGroupFilter, nhiFilter string) ([]Donor, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, nhi, blood_group, rhd, status,
		        deferral_reason, deferral_end_date,
		        total_donations, last_donation_at, haemoglobin_gdl,
		        created_at, updated_at
		 FROM donors
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter     = '' OR status        = @status_filter)
		   AND (@blood_group_filter = '' OR blood_group  = @blood_group_filter)
		   AND (@nhi_filter        = '' OR nhi           = @nhi_filter)
		 ORDER BY created_at DESC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":         tenantID,
			"status_filter":     statusFilter,
			"blood_group_filter": bloodGroupFilter,
			"nhi_filter":        nhiFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query donors: %w", err)
	}
	defer rows.Close()

	var results []Donor
	for rows.Next() {
		var d Donor
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.NHI, &d.BloodGroup, &d.RhD, &d.Status,
			&d.DeferralReason, &d.DeferralEndDate,
			&d.TotalDonations, &d.LastDonationAt, &d.HaemoglobinGDL,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan donor row: %w", err)
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

func (h *DonorsHandler) listEligibleDonors(ctx context.Context, tenantID, bloodGroupFilter string) ([]Donor, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, nhi, blood_group, rhd, status,
		        deferral_reason, deferral_end_date,
		        total_donations, last_donation_at, haemoglobin_gdl,
		        created_at, updated_at
		 FROM donors
		 WHERE tenant_id = @tenant_id
		   AND status = 'active'
		   AND (deferral_end_date IS NULL OR deferral_end_date < now())
		   AND (@blood_group_filter = '' OR blood_group = @blood_group_filter)
		 ORDER BY last_donation_at ASC NULLS FIRST
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":         tenantID,
			"blood_group_filter": bloodGroupFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query eligible donors: %w", err)
	}
	defer rows.Close()

	var results []Donor
	for rows.Next() {
		var d Donor
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.NHI, &d.BloodGroup, &d.RhD, &d.Status,
			&d.DeferralReason, &d.DeferralEndDate,
			&d.TotalDonations, &d.LastDonationAt, &d.HaemoglobinGDL,
			&d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan eligible donor row: %w", err)
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

func (h *DonorsHandler) getDonorByID(ctx context.Context, id, tenantID string) (Donor, error) {
	var d Donor
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, nhi, blood_group, rhd, status,
		        deferral_reason, deferral_end_date,
		        total_donations, last_donation_at, haemoglobin_gdl,
		        created_at, updated_at
		 FROM donors
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan(
		&d.ID, &d.TenantID, &d.NHI, &d.BloodGroup, &d.RhD, &d.Status,
		&d.DeferralReason, &d.DeferralEndDate,
		&d.TotalDonations, &d.LastDonationAt, &d.HaemoglobinGDL,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Donor{}, errNotFound
		}
		return Donor{}, fmt.Errorf("get donor by id: %w", err)
	}
	return d, nil
}

func (h *DonorsHandler) insertDonor(ctx context.Context, nhi string, bloodGroup BloodGroup, rhd, tenantID string) (Donor, error) {
	var d Donor
	err := h.pool.QueryRow(ctx,
		`INSERT INTO donors (nhi, blood_group, rhd, status, tenant_id)
		 VALUES (@nhi, @blood_group, @rhd, 'active', @tenant_id)
		 RETURNING id, tenant_id, nhi, blood_group, rhd, status,
		           deferral_reason, deferral_end_date,
		           total_donations, last_donation_at, haemoglobin_gdl,
		           created_at, updated_at`,
		db.NamedArgs{
			"nhi":         nhi,
			"blood_group": bloodGroup,
			"rhd":         rhd,
			"tenant_id":   tenantID,
		},
	).Scan(
		&d.ID, &d.TenantID, &d.NHI, &d.BloodGroup, &d.RhD, &d.Status,
		&d.DeferralReason, &d.DeferralEndDate,
		&d.TotalDonations, &d.LastDonationAt, &d.HaemoglobinGDL,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return Donor{}, fmt.Errorf("insert donor: %w", err)
	}
	return d, nil
}

func (h *DonorsHandler) updateDonor(ctx context.Context, d Donor) (Donor, error) {
	var updated Donor
	err := h.pool.QueryRow(ctx,
		`UPDATE donors
		 SET blood_group       = @blood_group,
		     rhd               = @rhd,
		     status            = @status,
		     deferral_reason   = @deferral_reason,
		     deferral_end_date = @deferral_end_date,
		     haemoglobin_gdl   = @haemoglobin_gdl,
		     updated_at        = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, tenant_id, nhi, blood_group, rhd, status,
		           deferral_reason, deferral_end_date,
		           total_donations, last_donation_at, haemoglobin_gdl,
		           created_at, updated_at`,
		db.NamedArgs{
			"blood_group":       d.BloodGroup,
			"rhd":               d.RhD,
			"status":            d.Status,
			"deferral_reason":   d.DeferralReason,
			"deferral_end_date": d.DeferralEndDate,
			"haemoglobin_gdl":   d.HaemoglobinGDL,
			"id":                d.ID,
			"tenant_id":         d.TenantID,
		},
	).Scan(
		&updated.ID, &updated.TenantID, &updated.NHI, &updated.BloodGroup, &updated.RhD, &updated.Status,
		&updated.DeferralReason, &updated.DeferralEndDate,
		&updated.TotalDonations, &updated.LastDonationAt, &updated.HaemoglobinGDL,
		&updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return Donor{}, errNotFound
		}
		return Donor{}, fmt.Errorf("update donor: %w", err)
	}
	return updated, nil
}

func (h *DonorsHandler) getDonations(ctx context.Context, donorID, tenantID string) ([]DonationRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, donor_id, product_unit_id, volume_ml, donation_type, collected_at, created_at
		 FROM donations
		 WHERE donor_id = @donor_id
		   AND tenant_id = @tenant_id
		 ORDER BY collected_at DESC
		 LIMIT 100`,
		db.NamedArgs{"donor_id": donorID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query donations: %w", err)
	}
	defer rows.Close()

	var results []DonationRecord
	for rows.Next() {
		var rec DonationRecord
		if err := rows.Scan(&rec.ID, &rec.DonorID, &rec.ProductUnitID, &rec.VolumeML, &rec.DonationType, &rec.CollectedAt, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan donation row: %w", err)
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}