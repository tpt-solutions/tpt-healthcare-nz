package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	corenhi "github.com/PhillipC05/tpt-healthcare/core/nhi"
)

// nhiAuditTrailer is the subset of audit.Trail used by NHI handlers.
type nhiAuditTrailer interface {
	Record(ctx context.Context, e audit.Event) error
}

// NHIHandler handles NHI patient lookup and matching endpoints.
type NHIHandler struct {
	client     *corenhi.Client
	auditTrail nhiAuditTrailer
}

// newNHIHandler returns an NHIHandler. Both client and auditTrail may be nil
// (e.g. in tests), in which case the corresponding features are skipped.
func newNHIHandler(client *corenhi.Client, trail nhiAuditTrailer) *NHIHandler {
	return &NHIHandler{client: client, auditTrail: trail}
}

// router returns a mux with all NHI routes registered.
// Routes are relative to the /api/v1/nhi prefix.
func (h *NHIHandler) router() http.Handler {
	mux := http.NewServeMux()

	// POST /api/v1/nhi/match must be registered before the catch-all so that
	// ServeMux selects it for the exact path.
	mux.HandleFunc("/api/v1/nhi/match", h.handleMatch)

	// GET /api/v1/nhi/{nhi} — the trailing slash catch-all dispatches all
	// other /api/v1/nhi/... paths here.
	mux.HandleFunc("/api/v1/nhi/", h.handleLookup)

	return mux
}

// handleLookup serves GET /api/v1/nhi/{nhi}.
// It validates the NHI format, calls the NHI client, records an audit event,
// and returns the patient JSON on success.
func (h *NHIHandler) handleLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, fmt.Sprintf("method %s not allowed", r.Method))
		return
	}

	// Extract the NHI from the path: /api/v1/nhi/{nhi}
	nhi := strings.TrimPrefix(r.URL.Path, "/api/v1/nhi/")
	nhi = strings.TrimSuffix(nhi, "/")
	if nhi == "" {
		writeJSONError(w, http.StatusBadRequest, "NHI number is required in the path")
		return
	}

	if !corenhi.ValidateNHI(nhi) {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid NHI format or checksum: %q", nhi))
		return
	}

	if h.client == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "NHI client not configured")
		return
	}

	patient, err := h.client.GetPatient(r.Context(), nhi)
	if err != nil {
		if isNotFound(err) {
			h.recordAudit(r, nhi, "read", "not found")
			writeJSONError(w, http.StatusNotFound, fmt.Sprintf("patient with NHI %s not found", nhi))
			return
		}
		h.recordAudit(r, nhi, "read", "error: "+err.Error())
		writeJSONError(w, http.StatusBadGateway, "NHI lookup failed: "+err.Error())
		return
	}

	h.recordAudit(r, nhi, "read", "success")

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(patient)
}

// handleMatch serves POST /api/v1/nhi/match.
// It matches a patient by demographics via the FHIR $match operation.
func (h *NHIHandler) handleMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, fmt.Sprintf("method %s not allowed", r.Method))
		return
	}

	if h.client == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "NHI client not configured")
		return
	}

	var req struct {
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
		BirthDate  string `json:"birthDate"` // YYYY-MM-DD
		Gender     string `json:"gender"`
		Address    string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "request body is not valid JSON: "+err.Error())
		return
	}

	if req.FamilyName == "" || req.GivenName == "" || req.BirthDate == "" {
		writeJSONError(w, http.StatusBadRequest, "givenName, familyName, and birthDate are required")
		return
	}

	birthDate, err := time.Parse("2006-01-02", req.BirthDate)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "birthDate must be in YYYY-MM-DD format")
		return
	}

	params := corenhi.MatchParams{
		GivenName:  req.GivenName,
		FamilyName: req.FamilyName,
		BirthDate:  birthDate,
		Gender:     req.Gender,
		Address:    req.Address,
	}

	patients, err := h.client.MatchPatient(r.Context(), params)
	if err != nil {
		h.recordAudit(r, "", "match", "error: "+err.Error())
		writeJSONError(w, http.StatusBadGateway, "NHI match failed: "+err.Error())
		return
	}

	h.recordAudit(r, "", "match", fmt.Sprintf("returned %d candidates", len(patients)))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"total":    len(patients),
		"patients": patients,
	})
}

// recordAudit writes an audit event for an NHI operation. Errors are silently
// dropped (audit failure must not break the request path).
func (h *NHIHandler) recordAudit(r *http.Request, nhi, action, detail string) {
	if h.auditTrail == nil {
		return
	}

	tenantID, _ := middleware.TenantFromContext(r.Context())
	principal, _ := auth.PrincipalFromContext(r.Context())

	principalID := ""
	if principal != nil {
		principalID = principal.ID
	}

	e := audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principalID,
		Action:       action,
		ResourceType: "Patient",
		PatientNHI:   strings.ToUpper(strings.TrimSpace(nhi)),
		IPAddress:    r.RemoteAddr,
		UserAgent:    r.UserAgent(),
		OccurredAt:   time.Now(),
		Details: map[string]any{
			"path":   r.URL.Path,
			"detail": detail,
		},
	}
	_ = h.auditTrail.Record(r.Context(), e)
}

// isNotFound reports whether err indicates a 404 / not-found response from the
// NHI service. The core NHI client embeds the HTTP status code in the error
// message as "HTTP 404".
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, errNotFound) || strings.Contains(err.Error(), "HTTP 404")
}

// errNotFound is a sentinel used in tests.
var errNotFound = errors.New("not found")

// writeJSONError writes a simple JSON error response.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
