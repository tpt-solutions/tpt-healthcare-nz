// Package api implements HTTP handlers for allied health services.
package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/gorilla/mux"
)

// PhysioHandler handles physiotherapy API endpoints.
type PhysioHandler struct {
	hpiClient    *hpi.Client
	consentStore *consent.Store
	pool         db.Pool
	logger       *slog.Logger
}

// NewPhysioHandler creates a new physio handler.
func NewPhysioHandler(hpiClient *hpi.Client, consentStore *consent.Store, pool db.Pool, logger *slog.Logger) *PhysioHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PhysioHandler{hpiClient: hpiClient, consentStore: consentStore, pool: pool, logger: logger}
}

// nullStr returns nil when s is empty so nullable DB columns store NULL.
func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// RegisterRoutes registers physio routes.
func (h *PhysioHandler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/api/v1/physio/treatment-plans", h.CreateTreatmentPlan).Methods("POST")
	r.HandleFunc("/api/v1/physio/treatment-plans", h.ListTreatmentPlans).Methods("GET")
	r.HandleFunc("/api/v1/physio/treatment-plans/{id}", h.GetTreatmentPlan).Methods("GET")
	r.HandleFunc("/api/v1/physio/treatment-plans/{id}", h.UpdateTreatmentPlan).Methods("PUT")
	r.HandleFunc("/api/v1/physio/treatment-plans/{id}", h.DeleteTreatmentPlan).Methods("DELETE")

	r.HandleFunc("/api/v1/physio/session-notes", h.CreateSessionNote).Methods("POST")
	r.HandleFunc("/api/v1/physio/session-notes", h.ListSessionNotes).Methods("GET")
	r.HandleFunc("/api/v1/physio/session-notes/{id}", h.GetSessionNote).Methods("GET")
	r.HandleFunc("/api/v1/physio/session-notes/{id}", h.UpdateSessionNote).Methods("PUT")

	r.HandleFunc("/api/v1/physio/outcome-measures", h.ListOutcomeMeasures).Methods("GET")
}

// requireAPC validates that the authenticated clinician holds a current APC.
// Returns false and writes a 403 if the check fails. If hpiClient is nil the
// check is skipped (development/test mode).
func requireAPC(w http.ResponseWriter, r *http.Request, hpiClient *hpi.Client) bool {
	if hpiClient == nil {
		return true
	}
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok || !principal.Practitioner || principal.PractitionerID == "" {
		http.Error(w, "forbidden: authenticated principal is not a registered practitioner", http.StatusForbidden)
		return false
	}
	apcStatus, err := hpiClient.ValidateAPC(r.Context(), principal.PractitionerID)
	if err != nil {
		http.Error(w, "forbidden: APC validation failed: "+err.Error(), http.StatusForbidden)
		return false
	}
	if !apcStatus.Valid {
		http.Error(w, "forbidden: clinician does not hold a current Annual Practising Certificate", http.StatusForbidden)
		return false
	}
	return true
}

// checkConsent verifies that an active consent record exists for the given patient.
// Returns false and writes a 403 if consent is absent. Skipped when consentStore is nil.
func checkConsent(w http.ResponseWriter, r *http.Request, consentStore *consent.Store, patientNHI string) bool {
	if consentStore == nil || patientNHI == "" {
		return true
	}
	tenantID, _ := middleware.TenantFromContext(r.Context())
	ok, err := consentStore.Check(r.Context(), tenantID, patientNHI, consent.ConsentTypeAccess)
	if err != nil {
		http.Error(w, "forbidden: consent check failed: "+err.Error(), http.StatusForbidden)
		return false
	}
	if !ok {
		http.Error(w, "forbidden: no active consent for patient data access", http.StatusForbidden)
		return false
	}
	return true
}

// parsePagination reads limit and offset from query params with defaults.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		if l > 200 {
			l = 200
		}
		limit = l
	}
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
		offset = o
	}
	return
}
