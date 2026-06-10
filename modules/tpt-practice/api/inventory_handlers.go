package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/PhillipC05/tpt-healthcare/core/inventory"
)

// ============================================================
// Inventory handlers — delegates to inventory.Service + Repository
// ============================================================

func (s *Server) listStockItems(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	items, err := s.inventoryRepo.ListStockItems(r.Context(), tid)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []inventory.StockItem{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) createStockItem(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body inventory.StockItem
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.SKU == "" || body.Name == "" || body.Category == "" {
		http.Error(w, "sku, name, and category are required", http.StatusBadRequest)
		return
	}
	if body.Unit == "" {
		body.Unit = "unit"
	}
	body.TenantID = tid
	item, err := s.inventoryRepo.CreateStockItem(r.Context(), body)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) getStockItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	item, err := s.inventoryRepo.GetStockItem(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) receiveStock(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	tid, ok := tenantID(r)
	pid, pok := principalID(r)
	if !ok || !pok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Quantity int64  `json:"quantity"`
		Notes    string `json:"notes"`
	}
	if err := readJSON(r, &body); err != nil || body.Quantity <= 0 {
		http.Error(w, "quantity must be a positive integer", http.StatusBadRequest)
		return
	}
	mov, err := s.inventorySvc.Receive(r.Context(), id, tid, body.Quantity, pid, body.Notes)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, mov)
}

func (s *Server) consumeStock(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	tid, ok := tenantID(r)
	pid, pok := principalID(r)
	if !ok || !pok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Quantity     int64  `json:"quantity"`
		EncounterRef string `json:"encounter_ref"`
	}
	if err := readJSON(r, &body); err != nil || body.Quantity <= 0 {
		http.Error(w, "quantity must be a positive integer", http.StatusBadRequest)
		return
	}
	mov, err := s.inventorySvc.Consume(r.Context(), id, tid, body.Quantity, pid, body.EncounterRef)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, mov)
}

func (s *Server) listPurchaseOrders(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	pos, err := s.inventoryRepo.ListPurchaseOrders(r.Context(), tid)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if pos == nil {
		pos = []inventory.PurchaseOrder{}
	}
	writeJSON(w, http.StatusOK, pos)
}

func (s *Server) createPurchaseOrder(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body inventory.PurchaseOrder
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.SupplierName == "" {
		http.Error(w, "supplier_name is required", http.StatusBadRequest)
		return
	}
	if body.Status == "" {
		body.Status = inventory.POStatusDraft
	}
	body.TenantID = tid
	po, err := s.inventoryRepo.CreatePurchaseOrder(r.Context(), body)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, po)
}

func (s *Server) listColdChainBreaches(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	since := time.Now().UTC().AddDate(0, -3, 0) // default: last 90 days
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			since = t
		}
	}
	breaches, err := s.inventoryRepo.ListBreaches(r.Context(), tid, since)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if breaches == nil {
		breaches = []inventory.ColdChainLog{}
	}
	writeJSON(w, http.StatusOK, breaches)
}
