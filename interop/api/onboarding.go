package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/tenant"
	"github.com/google/uuid"
)

// onboardingHandler serves the public self-registration and network-admin
// application management endpoints.
type onboardingHandler struct {
	store tenant.Store
}

func newOnboardingHandler(store tenant.Store) *onboardingHandler {
	return &onboardingHandler{store: store}
}

// publicRouter returns routes that require no auth and no X-Tenant-ID.
func (h *onboardingHandler) publicRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/onboarding/apply", h.handleApply)
	return mux
}

// adminRouter returns routes that require auth + network_admin role.
// Callers must wrap with RequireAuth → RequireRole("network_admin").
func (h *onboardingHandler) adminRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/applications", h.handleListApplications)
	mux.HandleFunc("POST /api/v1/admin/applications/{id}/approve", h.handleApprove)
	mux.HandleFunc("POST /api/v1/admin/applications/{id}/reject", h.handleReject)
	mux.HandleFunc("GET /api/v1/admin/tenants", h.handleListTenants)
	return mux
}

// handleApply accepts a clinic self-registration application. Public — no auth required.
//
// POST /api/v1/onboarding/apply
func (h *onboardingHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	var req tenant.SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if msgs := validateSubmit(req); len(msgs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"errors": msgs,
		})
		return
	}

	app, err := h.store.Submit(r.Context(), req)
	if err != nil {
		http.Error(w, "failed to submit application", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

// handleListApplications returns applications, optionally filtered by status.
//
// GET /api/v1/admin/applications?status=pending
func (h *onboardingHandler) handleListApplications(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	apps, err := h.store.ListApplications(r.Context(), status)
	if err != nil {
		http.Error(w, "failed to list applications", http.StatusInternalServerError)
		return
	}
	if apps == nil {
		apps = []*tenant.Application{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"applications": apps})
}

// handleApprove approves a pending application and provisions the tenant.
//
// POST /api/v1/admin/applications/{id}/approve
func (h *onboardingHandler) handleApprove(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDSegment(w, r, "id")
	if !ok {
		return
	}

	var req tenant.ReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength != 0 {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	principal, _ := auth.PrincipalFromContext(r.Context())
	reviewerID := ""
	if principal != nil {
		reviewerID = principal.ID
	}

	t, err := h.store.Approve(r.Context(), id, reviewerID, req.Notes)
	if err != nil {
		if strings.Contains(err.Error(), "not found or not pending") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "failed to approve application", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// handleReject rejects a pending application.
//
// POST /api/v1/admin/applications/{id}/reject
func (h *onboardingHandler) handleReject(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDSegment(w, r, "id")
	if !ok {
		return
	}

	var req tenant.ReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && r.ContentLength != 0 {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	principal, _ := auth.PrincipalFromContext(r.Context())
	reviewerID := ""
	if principal != nil {
		reviewerID = principal.ID
	}

	if err := h.store.Reject(r.Context(), id, reviewerID, req.Notes); err != nil {
		if strings.Contains(err.Error(), "not found or not pending") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "failed to reject application", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListTenants returns all active tenants on the network.
//
// GET /api/v1/admin/tenants
func (h *onboardingHandler) handleListTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.store.ListTenants(r.Context())
	if err != nil {
		http.Error(w, "failed to list tenants", http.StatusInternalServerError)
		return
	}
	if tenants == nil {
		tenants = []*tenant.Tenant{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenants": tenants})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validateSubmit(req tenant.SubmitRequest) []string {
	var msgs []string
	if strings.TrimSpace(req.PracticeName) == "" {
		msgs = append(msgs, "practice_name is required")
	}
	if strings.TrimSpace(req.HPIFacilityID) == "" {
		msgs = append(msgs, "hpi_facility_id is required")
	}
	if strings.TrimSpace(req.ContactName) == "" {
		msgs = append(msgs, "contact_name is required")
	}
	if strings.TrimSpace(req.ContactEmail) == "" {
		msgs = append(msgs, "contact_email is required")
	}
	return msgs
}

func parseUUIDSegment(w http.ResponseWriter, r *http.Request, segment string) (uuid.UUID, bool) {
	raw := r.PathValue(segment)
	id, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "invalid "+segment+": must be a UUID", http.StatusBadRequest)
		return uuid.UUID{}, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
