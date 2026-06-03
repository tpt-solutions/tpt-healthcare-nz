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

// ProductType represents the type of blood product.
type ProductType string

const (
	ProductTypeRBC          ProductType = "rbc"           // Red Blood Cells
	ProductTypePlatelets    ProductType = "platelets"     // Apheresis or pooled platelets
	ProductTypePlasma       ProductType = "plasma"        // Fresh Frozen Plasma
	ProductTypeCryo         ProductType = "cryo"          // Cryoprecipitate
	ProductTypeWholeBlood   ProductType = "whole-blood"   // Whole blood
)

// ProductStatus represents the current state in the blood product lifecycle.
type ProductStatus string

const (
	ProductStatusCollected    ProductStatus = "collected"
	ProductStatusTested       ProductStatus = "tested"
	ProductStatusStored       ProductStatus = "stored"
	ProductStatusCrossmatched ProductStatus = "crossmatched"
	ProductStatusIssued       ProductStatus = "issued"
	ProductStatusTransfused   ProductStatus = "transfused"
	ProductStatusDiscarded    ProductStatus = "discarded"
	ProductStatusQuarantined  ProductStatus = "quarantined"
)

// ProductStatusTransitions defines allowed status transitions.
var ProductStatusTransitions = map[ProductStatus][]ProductStatus{
	ProductStatusCollected:    {ProductStatusTested, ProductStatusDiscarded},
	ProductStatusTested:       {ProductStatusStored, ProductStatusQuarantined, ProductStatusDiscarded},
	ProductStatusStored:       {ProductStatusCrossmatched, ProductStatusQuarantined, ProductStatusDiscarded},
	ProductStatusCrossmatched: {ProductStatusIssued, ProductStatusStored, ProductStatusDiscarded},
	ProductStatusIssued:       {ProductStatusTransfused, ProductStatusDiscarded},
	ProductStatusQuarantined:  {ProductStatusTested, ProductStatusDiscarded},
}

// BloodProduct represents a single unit of a blood product in inventory.
type BloodProduct struct {
	ID              string        `json:"id"`
	TenantID        string        `json:"tenantId"`
	ProductType     ProductType   `json:"productType"`
	ABO             string        `json:"abo"`    // A, B, AB, O
	RhD             string        `json:"rhd"`    // POSITIVE, NEGATIVE
	DonationID      string        `json:"donationId,omitempty"`
	DonorID         string        `json:"donorId,omitempty"`
	Status          ProductStatus `json:"status"`
	VolumeML        int           `json:"volumeMl"`
	CollectionDate  time.Time     `json:"collectionDate"`
	ExpiryDate      time.Time     `json:"expiryDate"`
	TestResults     []TestResult  `json:"testResults,omitempty"`
	StorageLocation string        `json:"storageLocation,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

// TestResult records a single infectious disease test on a blood product.
type TestResult struct {
	TestName   string    `json:"testName"`
	Result     string    `json:"result"` // reactive, non-reactive, pending
	PerformedAt time.Time `json:"performedAt"`
	PerformedBy string    `json:"performedBy"`
	Notes      string    `json:"notes,omitempty"`
}

// productCreateRequest is the body for POST /api/v1/inventory.
type productCreateRequest struct {
	ProductType    ProductType   `json:"productType"`
	ABO            string        `json:"abo"`
	RhD            string        `json:"rhd"`
	DonorID        string        `json:"donorId,omitempty"`
	VolumeML       int           `json:"volumeMl"`
	CollectionDate string        `json:"collectionDate"` // RFC3339
	ExpiryDate     string        `json:"expiryDate"`     // RFC3339
	StorageLocation string       `json:"storageLocation,omitempty"`
}

// statusUpdateRequest is the body for PUT /api/v1/inventory/{id}/status.
type statusUpdateRequest struct {
	NewStatus     ProductStatus `json:"newStatus"`
	StorageLocation string      `json:"storageLocation,omitempty"`
	Reason        string        `json:"reason,omitempty"`
}

// InventoryHandler handles all /api/v1/inventory routes.
type InventoryHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/inventory.
// Supports query parameters: productType, abo, rhd, status.
func (h *InventoryHandler) List(w http.ResponseWriter, r *http.Request) {
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
	productTypeFilter := q.Get("productType")
	aboFilter := q.Get("abo")
	rhdFilter := q.Get("rhd")
	statusFilter := q.Get("status")

	products, err := h.listProducts(ctx, tenantID, productTypeFilter, aboFilter, rhdFilter, statusFilter)
	if err != nil {
		h.logger.Error("list inventory", slog.Any("error", err), slog.String("tenant", tenantID))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list inventory"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "BloodProduct",
		ResourceID:   "list",
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"products": products,
		"total":    len(products),
	})
}

// Create handles POST /api/v1/inventory.
func (h *InventoryHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req productCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if req.ABO == "" || req.RhD == "" || req.ProductType == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "productType, abo, and rhd are required"})
		return
	}

	collectionDate, err := time.Parse(time.RFC3339, req.CollectionDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_DATE", Message: "collectionDate must be RFC3339 format"})
		return
	}
	expiryDate, err := time.Parse(time.RFC3339, req.ExpiryDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_DATE", Message: "expiryDate must be RFC3339 format"})
		return
	}

	if expiryDate.Before(time.Now()) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "EXPIRED", Message: "product is already expired"})
		return
	}

	product, err := h.insertProduct(ctx, req, collectionDate, expiryDate, tenantID)
	if err != nil {
		h.logger.Error("insert product", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create product"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "BloodProduct",
		ResourceID:   product.ID,
		TenantID:     tenantID,
		Metadata:     map[string]string{"productType": string(req.ProductType), "abo": req.ABO, "rhd": req.RhD},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, product)
}

// Get handles GET /api/v1/inventory/{id}.
func (h *InventoryHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "product ID is required"})
		return
	}

	product, err := h.getProductByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "product not found"})
			return
		}
		h.logger.Error("get product", slog.Any("error", err), slog.String("id", id))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve product"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "BloodProduct",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, product)
}

// UpdateStatus handles PUT /api/v1/inventory/{id}/status.
func (h *InventoryHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "product ID is required"})
		return
	}

	var req statusUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	existing, err := h.getProductByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "product not found"})
			return
		}
		h.logger.Error("get product for status update", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve product"})
		return
	}

	// Validate status transition.
	allowed, ok := ProductStatusTransitions[existing.Status]
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_TRANSITION", Message: fmt.Sprintf("no transitions defined from status %q", existing.Status)})
		return
	}
	transitionValid := false
	for _, s := range allowed {
		if s == req.NewStatus {
			transitionValid = true
			break
		}
	}
	if !transitionValid {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code:    "INVALID_TRANSITION",
			Message: fmt.Sprintf("cannot transition from %q to %q", existing.Status, req.NewStatus),
		})
		return
	}

	if existing.Status == ProductStatusTransfused || existing.Status == ProductStatusDiscarded {
		writeJSON(w, http.StatusConflict, apiError{Code: "TERMINAL_STATUS", Message: fmt.Sprintf("cannot change status of a %s product", existing.Status)})
		return
	}

	existing.Status = req.NewStatus
	if req.StorageLocation != "" {
		existing.StorageLocation = req.StorageLocation
	}

	updated, err := h.updateProduct(ctx, existing)
	if err != nil {
		h.logger.Error("update product status", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "UPDATE_ERROR", Message: "failed to update product status"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "BloodProduct",
		ResourceID:   id,
		TenantID:     tenantID,
		Metadata:     map[string]string{"newStatus": string(req.NewStatus), "reason": req.Reason},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, updated)
}

// ListExpiring handles GET /api/v1/inventory/expiring.
// Returns products expiring within the given number of days (default 7).
func (h *InventoryHandler) ListExpiring(w http.ResponseWriter, r *http.Request) {
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

	products, err := h.listExpiringProducts(ctx, tenantID)
	if err != nil {
		h.logger.Error("list expiring products", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list expiring products"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "BloodProduct",
		ResourceID:   "expiring",
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"products": products,
		"total":    len(products),
	})
}

// ListAvailable handles GET /api/v1/inventory/available.
// Returns products in "stored" status suitable for cross-matching.
// Supports query parameters: abo, rhd, productType.
func (h *InventoryHandler) ListAvailable(w http.ResponseWriter, r *http.Request) {
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
	aboFilter := q.Get("abo")
	rhdFilter := q.Get("rhd")
	productTypeFilter := q.Get("productType")

	products, err := h.listAvailableProducts(ctx, tenantID, aboFilter, rhdFilter, productTypeFilter)
	if err != nil {
		h.logger.Error("list available products", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list available products"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "BloodProduct",
		ResourceID:   "available",
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"products": products,
		"total":    len(products),
	})
}

// --- Database operations ---

func (h *InventoryHandler) listProducts(ctx context.Context, tenantID, productTypeFilter, aboFilter, rhdFilter, statusFilter string) ([]BloodProduct, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, product_type, abo, rhd, donation_id, donor_id,
		        status, volume_ml, collection_date, expiry_date,
		        test_results, storage_location, created_at, updated_at
		 FROM blood_products
		 WHERE tenant_id = @tenant_id
		   AND (@product_type_filter = '' OR product_type = @product_type_filter)
		   AND (@abo_filter    = '' OR abo = @abo_filter)
		   AND (@rhd_filter    = '' OR rhd = @rhd_filter)
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY expiry_date ASC
		 LIMIT 200`,
		db.NamedArgs{
			"tenant_id":          tenantID,
			"product_type_filter": productTypeFilter,
			"abo_filter":         aboFilter,
			"rhd_filter":         rhdFilter,
			"status_filter":      statusFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query blood products: %w", err)
	}
	defer rows.Close()

	var results []BloodProduct
	for rows.Next() {
		var p BloodProduct
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.ProductType, &p.ABO, &p.RhD, &p.DonationID, &p.DonorID,
			&p.Status, &p.VolumeML, &p.CollectionDate, &p.ExpiryDate,
			&p.TestResults, &p.StorageLocation, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan blood product row: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (h *InventoryHandler) listExpiringProducts(ctx context.Context, tenantID string) ([]BloodProduct, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, product_type, abo, rhd, donation_id, donor_id,
		        status, volume_ml, collection_date, expiry_date,
		        test_results, storage_location, created_at, updated_at
		 FROM blood_products
		 WHERE tenant_id = @tenant_id
		   AND status NOT IN ('transfused', 'discarded')
		   AND expiry_date BETWEEN now() AND now() + INTERVAL '7 days'
		 ORDER BY expiry_date ASC`,
		db.NamedArgs{"tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query expiring products: %w", err)
	}
	defer rows.Close()

	var results []BloodProduct
	for rows.Next() {
		var p BloodProduct
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.ProductType, &p.ABO, &p.RhD, &p.DonationID, &p.DonorID,
			&p.Status, &p.VolumeML, &p.CollectionDate, &p.ExpiryDate,
			&p.TestResults, &p.StorageLocation, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan expiring product row: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (h *InventoryHandler) listAvailableProducts(ctx context.Context, tenantID, aboFilter, rhdFilter, productTypeFilter string) ([]BloodProduct, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, tenant_id, product_type, abo, rhd, donation_id, donor_id,
		        status, volume_ml, collection_date, expiry_date,
		        test_results, storage_location, created_at, updated_at
		 FROM blood_products
		 WHERE tenant_id = @tenant_id
		   AND status = 'stored'
		   AND expiry_date > now()
		   AND (@abo_filter    = '' OR abo = @abo_filter)
		   AND (@rhd_filter    = '' OR rhd = @rhd_filter)
		   AND (@product_type_filter = '' OR product_type = @product_type_filter)
		 ORDER BY expiry_date ASC`,
		db.NamedArgs{
			"tenant_id":          tenantID,
			"abo_filter":         aboFilter,
			"rhd_filter":         rhdFilter,
			"product_type_filter": productTypeFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query available products: %w", err)
	}
	defer rows.Close()

	var results []BloodProduct
	for rows.Next() {
		var p BloodProduct
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.ProductType, &p.ABO, &p.RhD, &p.DonationID, &p.DonorID,
			&p.Status, &p.VolumeML, &p.CollectionDate, &p.ExpiryDate,
			&p.TestResults, &p.StorageLocation, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan available product row: %w", err)
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

func (h *InventoryHandler) getProductByID(ctx context.Context, id, tenantID string) (BloodProduct, error) {
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

func (h *InventoryHandler) insertProduct(ctx context.Context, req productCreateRequest, collectionDate, expiryDate time.Time, tenantID string) (BloodProduct, error) {
	var p BloodProduct
	err := h.pool.QueryRow(ctx,
		`INSERT INTO blood_products (product_type, abo, rhd, donor_id, status, volume_ml,
		                             collection_date, expiry_date, storage_location, tenant_id)
		 VALUES (@product_type, @abo, @rhd, @donor_id, 'collected', @volume_ml,
		         @collection_date, @expiry_date, @storage_location, @tenant_id)
		 RETURNING id, tenant_id, product_type, abo, rhd, donation_id, donor_id,
		           status, volume_ml, collection_date, expiry_date,
		           test_results, storage_location, created_at, updated_at`,
		db.NamedArgs{
			"product_type":     req.ProductType,
			"abo":              req.ABO,
			"rhd":              req.RhD,
			"donor_id":         req.DonorID,
			"volume_ml":        req.VolumeML,
			"collection_date":  collectionDate,
			"expiry_date":      expiryDate,
			"storage_location": req.StorageLocation,
			"tenant_id":        tenantID,
		},
	).Scan(
		&p.ID, &p.TenantID, &p.ProductType, &p.ABO, &p.RhD, &p.DonationID, &p.DonorID,
		&p.Status, &p.VolumeML, &p.CollectionDate, &p.ExpiryDate,
		&p.TestResults, &p.StorageLocation, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return BloodProduct{}, fmt.Errorf("insert blood product: %w", err)
	}
	return p, nil
}

func (h *InventoryHandler) updateProduct(ctx context.Context, p BloodProduct) (BloodProduct, error) {
	var updated BloodProduct
	err := h.pool.QueryRow(ctx,
		`UPDATE blood_products
		 SET status           = @status,
		     storage_location = @storage_location,
		     test_results     = @test_results,
		     updated_at       = now()
		 WHERE id = @id AND tenant_id = @tenant_id
		 RETURNING id, tenant_id, product_type, abo, rhd, donation_id, donor_id,
		           status, volume_ml, collection_date, expiry_date,
		           test_results, storage_location, created_at, updated_at`,
		db.NamedArgs{
			"status":           p.Status,
			"storage_location": p.StorageLocation,
			"test_results":     p.TestResults,
			"id":               p.ID,
			"tenant_id":        p.TenantID,
		},
	).Scan(
		&updated.ID, &updated.TenantID, &updated.ProductType, &updated.ABO, &updated.RhD, &updated.DonationID, &updated.DonorID,
		&updated.Status, &updated.VolumeML, &updated.CollectionDate, &updated.ExpiryDate,
		&updated.TestResults, &updated.StorageLocation, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return BloodProduct{}, errNotFound
		}
		return BloodProduct{}, fmt.Errorf("update blood product: %w", err)
	}
	return updated, nil
}