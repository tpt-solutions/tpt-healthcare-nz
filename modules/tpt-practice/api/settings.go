package api

import (
	"encoding/json"
	"net/http"
	"regexp"
)

// TenantSettings is the shape of the settings JSONB column on the tenants table.
type TenantSettings struct {
	Theme         string   `json:"theme"`
	CustomAccent  *string  `json:"customAccent"`
	ActiveModules []string `json:"activeModules"`
}

var (
	validThemes     = map[string]bool{"clinical": true, "indigo": true, "sage": true, "calm": true, "bloom": true, "amber": true, "ocean": true, "mono": true}
	hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

func defaultSettings() TenantSettings {
	return TenantSettings{Theme: "clinical", ActiveModules: []string{}}
}

// getSettings returns the practice theme + module config.
// Returns defaults without auth so ThemeProvider can apply the theme pre-login.
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		writeJSON(w, http.StatusOK, defaultSettings())
		return
	}

	var raw []byte
	err := s.cfg.Pool.QueryRow(r.Context(),
		`SELECT COALESCE(settings, '{}') FROM tenants WHERE id = $1`, tid,
	).Scan(&raw)
	if err != nil {
		writeJSON(w, http.StatusOK, defaultSettings())
		return
	}

	settings := defaultSettings()
	if err := json.Unmarshal(raw, &settings); err != nil {
		writeJSON(w, http.StatusOK, defaultSettings())
		return
	}
	if settings.Theme == "" || !validThemes[settings.Theme] {
		settings.Theme = "clinical"
	}
	writeJSON(w, http.StatusOK, settings)
}

// putSettings updates the practice theme + module config.
// Requires an authenticated practice_admin.
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req TenantSettings
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !validThemes[req.Theme] {
		http.Error(w, "invalid theme", http.StatusBadRequest)
		return
	}
	if req.CustomAccent != nil && !hexColorPattern.MatchString(*req.CustomAccent) {
		http.Error(w, "customAccent must be a 6-digit hex colour e.g. #0d9488", http.StatusBadRequest)
		return
	}
	if req.ActiveModules == nil {
		req.ActiveModules = []string{}
	}

	encoded, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_, err = s.cfg.Pool.Exec(r.Context(),
		`UPDATE tenants SET settings = $1, updated_at = now() WHERE id = $2`,
		encoded, tid,
	)
	if err != nil {
		s.cfg.Logger.Error("failed to update tenant settings", "tenant_id", tid, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, req)
}
