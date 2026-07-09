package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/PhillipC05/tpt-healthcare/core/servicelines"
)

// serviceLineProfileResponse is the JSON shape returned for both the GET and
// PUT service-line profile endpoints: the tenant's raw selection plus the
// resolved module/ward-type/triage-scale/formulary defaults it implies.
type serviceLineProfileResponse struct {
	TenantID        uuid.UUID         `json:"tenant_id"`
	ServiceLines    []string          `json:"service_lines"`
	UpdatedAt       time.Time         `json:"updated_at"`
	Modules         []string          `json:"modules"`
	WardTypes       []string          `json:"ward_types"`
	FormularySubset []string          `json:"formulary_subset"`
	TriageScales    map[string]string `json:"triage_scales"`
}

// listServiceLineCatalogue returns the full service-line catalogue so the
// onboarding wizard / settings UI can present the available options.
// No auth required, matching getSettings (ThemeProvider-style pre-login use).
func (s *Server) listServiceLineCatalogue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, servicelines.All())
}

// getServiceLineProfile returns the tenant's currently selected service
// lines plus the ward-type, triage-scale, and formulary defaults they imply.
func (s *Server) getServiceLineProfile(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	profile, err := s.serviceLinesStore.GetProfile(r.Context(), tid)
	if err != nil {
		s.cfg.Logger.Error("failed to load service-line profile", "tenant_id", tid, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, serviceLineProfileResponse{
		TenantID:        profile.TenantID,
		ServiceLines:    profile.ServiceLines,
		UpdatedAt:       profile.UpdatedAt,
		Modules:         servicelines.ResolveModules(profile.ServiceLines),
		WardTypes:       servicelines.ResolveWardTypes(profile.ServiceLines),
		FormularySubset: servicelines.ResolveFormularySubset(profile.ServiceLines),
		TriageScales:    servicelines.ResolveTriageScales(profile.ServiceLines),
	})
}

// putServiceLineProfile sets the tenant's selected service lines. Selecting
// a service line additively enables the modules it implies (see
// core/servicelines.ResolveModules); modules a practice admin enabled
// manually are preserved even if the corresponding service line is later
// deselected.
func (s *Server) putServiceLineProfile(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ServiceLines []string `json:"service_lines"`
	}
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := servicelines.ValidateIDs(req.ServiceLines); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	profile, err := s.serviceLinesStore.SetProfile(r.Context(), tid, req.ServiceLines)
	if err != nil {
		s.cfg.Logger.Error("failed to set service-line profile", "tenant_id", tid, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, serviceLineProfileResponse{
		TenantID:        profile.TenantID,
		ServiceLines:    profile.ServiceLines,
		UpdatedAt:       profile.UpdatedAt,
		Modules:         servicelines.ResolveModules(profile.ServiceLines),
		WardTypes:       servicelines.ResolveWardTypes(profile.ServiceLines),
		FormularySubset: servicelines.ResolveFormularySubset(profile.ServiceLines),
		TriageScales:    servicelines.ResolveTriageScales(profile.ServiceLines),
	})
}
