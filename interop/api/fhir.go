package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/fhir/r4"
	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
	"github.com/PhillipC05/tpt-healthcare/core/fhir/translate"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/repo"
	"github.com/google/uuid"
)

// fhirVersion identifies the FHIR version a handler is operating at.
type fhirVersion string

const (
	fhirVersionR5 fhirVersion = "R5"
	fhirVersionR4 fhirVersion = "R4"
)

// fhirContentType is the media type for FHIR JSON responses.
const fhirContentType = "application/fhir+json; charset=utf-8"

// FHIRHandler serves FHIR REST API requests for a specific version.
// It handles the standard FHIR CRUD + search operations against a real
// resource repository (core/repo) and proxies R4 requests through the R4↔R5
// translation layer (core/fhir/translate).
type FHIRHandler struct {
	version fhirVersion
	store   repo.Store
}

// newFHIRHandler returns a FHIRHandler for the given version backed by store.
// A nil store falls back to an in-memory, non-persistent store — useful for
// local development and tests where PostgreSQL is unavailable.
func newFHIRHandler(v fhirVersion, store repo.Store) *FHIRHandler {
	if store == nil {
		store = repo.NewMemoryStore()
	}
	return &FHIRHandler{version: v, store: store}
}

// router returns an http.ServeMux with all FHIR routes registered.
// The caller is expected to strip the version prefix before dispatching
// (e.g. http.StripPrefix("/fhir/r5", h.router())).
func (h *FHIRHandler) router() http.Handler {
	mux := http.NewServeMux()

	// Capability statement (no resource type segment).
	mux.HandleFunc("/metadata", h.handleCapabilityStatement)

	// Resource-level routes. The standard library ServeMux does not support
	// path parameters natively, so we use a single catch-all pattern and
	// dispatch by method + path shape inside the handler.
	mux.HandleFunc("/", h.dispatch)

	return mux
}

// dispatch routes requests to the appropriate CRUD or search handler based on
// the HTTP method and whether an id segment is present in the path.
//
// Supported shapes (after prefix strip):
//
//	GET    /{resourceType}/{id}   → read
//	POST   /{resourceType}        → create
//	PUT    /{resourceType}/{id}   → update
//	DELETE /{resourceType}/{id}   → delete
//	GET    /{resourceType}        → search (query params)
func (h *FHIRHandler) dispatch(w http.ResponseWriter, r *http.Request) {
	// Trim leading slash and split into at most 2 segments.
	path := strings.TrimPrefix(r.URL.Path, "/")
	segments := strings.SplitN(path, "/", 2)

	resourceType := segments[0]
	if resourceType == "" {
		fhirError(w, http.StatusBadRequest, "invalid", "missing resource type in path")
		return
	}

	hasID := len(segments) == 2 && segments[1] != ""
	id := ""
	if hasID {
		id = segments[1]
	}

	switch r.Method {
	case http.MethodGet:
		if hasID {
			h.handleRead(w, r, resourceType, id)
		} else {
			h.handleSearch(w, r, resourceType)
		}
	case http.MethodPost:
		if hasID {
			fhirError(w, http.StatusMethodNotAllowed, "not-supported", "POST with id not supported; use PUT to update")
			return
		}
		h.handleCreate(w, r, resourceType)
	case http.MethodPut:
		if !hasID {
			fhirError(w, http.StatusBadRequest, "invalid", "PUT requires a resource id in the path")
			return
		}
		h.handleUpdate(w, r, resourceType, id)
	case http.MethodDelete:
		if !hasID {
			fhirError(w, http.StatusBadRequest, "invalid", "DELETE requires a resource id in the path")
			return
		}
		h.handleDelete(w, r, resourceType, id)
	default:
		fhirError(w, http.StatusMethodNotAllowed, "not-supported",
			fmt.Sprintf("method %s not supported", r.Method))
	}
}

// handleRead serves GET /{resourceType}/{id}.
func (h *FHIRHandler) handleRead(w http.ResponseWriter, r *http.Request, resourceType, id string) {
	tenantID, err := requireTenant(r)
	if err != nil {
		fhirError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}

	raw, meta, err := h.store.Read(r.Context(), tenantID.String(), resourceType, id)
	if err != nil {
		writeStoreError(w, resourceType, id, err)
		return
	}

	body, err := decodeResource(raw)
	if err != nil {
		fhirError(w, http.StatusInternalServerError, "processing", "stored resource is not valid JSON: "+err.Error())
		return
	}
	applyMeta(body, meta)

	if h.version == fhirVersionR4 {
		body = translateResourceR5toR4(resourceType, body)
	}
	writeFHIRJSON(w, http.StatusOK, body)
}

// handleCreate serves POST /{resourceType}.
func (h *FHIRHandler) handleCreate(w http.ResponseWriter, r *http.Request, resourceType string) {
	tenantID, err := requireTenant(r)
	if err != nil {
		fhirError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fhirError(w, http.StatusBadRequest, "invalid", "request body is not valid JSON: "+err.Error())
		return
	}
	if h.version == fhirVersionR4 {
		body = translateResourceR4toR5(resourceType, body)
	}

	newID := uuid.NewString()
	body["id"] = newID
	body["resourceType"] = resourceType
	delete(body, "meta")

	raw, err := json.Marshal(body)
	if err != nil {
		fhirError(w, http.StatusInternalServerError, "processing", "failed to serialise resource: "+err.Error())
		return
	}

	meta, err := h.store.Create(r.Context(), tenantID.String(), resourceType, newID, raw)
	if err != nil {
		fhirError(w, http.StatusInternalServerError, "processing", "failed to persist resource: "+err.Error())
		return
	}
	applyMeta(body, meta)

	if h.version == fhirVersionR4 {
		body = translateResourceR5toR4(resourceType, body)
	}
	w.Header().Set("Location", fmt.Sprintf("%s/%s", resourceType, meta.ResourceID))
	writeFHIRJSON(w, http.StatusCreated, body)
}

// handleUpdate serves PUT /{resourceType}/{id}.
func (h *FHIRHandler) handleUpdate(w http.ResponseWriter, r *http.Request, resourceType, id string) {
	tenantID, err := requireTenant(r)
	if err != nil {
		fhirError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fhirError(w, http.StatusBadRequest, "invalid", "request body is not valid JSON: "+err.Error())
		return
	}
	if h.version == fhirVersionR4 {
		body = translateResourceR4toR5(resourceType, body)
	}

	body["id"] = id
	body["resourceType"] = resourceType
	delete(body, "meta")

	raw, err := json.Marshal(body)
	if err != nil {
		fhirError(w, http.StatusInternalServerError, "processing", "failed to serialise resource: "+err.Error())
		return
	}

	meta, err := h.store.Update(r.Context(), tenantID.String(), resourceType, id, raw)
	if err != nil {
		writeStoreError(w, resourceType, id, err)
		return
	}
	applyMeta(body, meta)

	if h.version == fhirVersionR4 {
		body = translateResourceR5toR4(resourceType, body)
	}
	writeFHIRJSON(w, http.StatusOK, body)
}

// handleDelete serves DELETE /{resourceType}/{id}.
func (h *FHIRHandler) handleDelete(w http.ResponseWriter, r *http.Request, resourceType, id string) {
	tenantID, err := requireTenant(r)
	if err != nil {
		fhirError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}

	if err := h.store.Delete(r.Context(), tenantID.String(), resourceType, id); err != nil {
		writeStoreError(w, resourceType, id, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSearch serves GET /{resourceType} (search with query params).
func (h *FHIRHandler) handleSearch(w http.ResponseWriter, r *http.Request, resourceType string) {
	tenantID, err := requireTenant(r)
	if err != nil {
		fhirError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}

	query := r.URL.Query()
	count := 20
	if v := query.Get("_count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			count = n
		}
	}
	offset := 0
	if v := query.Get("_offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	filterParams := make(map[string][]string)
	for key, values := range query {
		if key == "_count" || key == "_offset" {
			continue
		}
		filterParams[key] = values
	}

	result, err := h.store.Search(r.Context(), repo.SearchParams{
		ResourceType: resourceType,
		Params:       filterParams,
		TenantID:     tenantID,
		Count:        count,
		Offset:       offset,
	})
	if err != nil {
		fhirError(w, http.StatusInternalServerError, "processing", "search failed: "+err.Error())
		return
	}

	entries := make([]any, 0, len(result.Resources))
	for _, raw := range result.Resources {
		resource, err := decodeResource(raw)
		if err != nil {
			continue
		}
		if h.version == fhirVersionR4 {
			resource = translateResourceR5toR4(resourceType, resource)
		}
		entries = append(entries, map[string]any{
			"fullUrl":  fmt.Sprintf("%s/%v", resourceType, resource["id"]),
			"resource": resource,
		})
	}

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "searchset",
		"total":        result.Total,
		"entry":        entries,
		"link": []any{
			map[string]any{
				"relation": "self",
				"url":      r.URL.String(),
			},
		},
	}
	writeFHIRJSON(w, http.StatusOK, bundle)
}

// handleCapabilityStatement serves GET /metadata.
func (h *FHIRHandler) handleCapabilityStatement(w http.ResponseWriter, _ *http.Request) {
	ver := "5.0.0"
	if h.version == fhirVersionR4 {
		ver = "4.0.1"
	}

	cs := map[string]any{
		"resourceType": "CapabilityStatement",
		"status":       "active",
		"kind":         "instance",
		"fhirVersion":  ver,
		"format":       []string{"application/fhir+json"},
		"software": map[string]any{
			"name":    "TPT Healthcare NZ Interop",
			"version": "0.1.0",
		},
		"rest": []any{
			map[string]any{
				"mode": "server",
				"resource": []any{
					fhirResourceCapability("Patient"),
					fhirResourceCapability("Observation"),
					fhirResourceCapability("Condition"),
					fhirResourceCapability("MedicationRequest"),
					fhirResourceCapability("Encounter"),
					fhirResourceCapability("AllergyIntolerance"),
					fhirResourceCapability("DiagnosticReport"),
					fhirResourceCapability("ServiceRequest"),
					fhirResourceCapability("Subscription"),
				},
			},
		},
	}
	writeFHIRJSON(w, http.StatusOK, cs)
}

// ---------------------------------------------------------------------------
// R4 ↔ R5 translation
// ---------------------------------------------------------------------------
//
// Dedicated field-level translators exist in core/fhir/translate for the
// resource types most commonly exchanged with NZ national services (Patient,
// Practitioner). For resource types without a dedicated translator, R4 and R5
// JSON shapes are structurally compatible for the fields this API round-trips
// (both are JSON objects sharing the same core FHIR data types), so the
// untranslated fields are passed through unchanged rather than dropped.

// translateResourceR4toR5 converts an R4 resource map to its R5 equivalent.
func translateResourceR4toR5(resourceType string, body map[string]any) map[string]any {
	switch resourceType {
	case "Patient":
		var r4p r4.Patient
		if !remarshal(body, &r4p) {
			return shallowCopy(body)
		}
		out, ok := marshalToMap(translate.PatientR4ToR5(&r4p))
		if !ok {
			return shallowCopy(body)
		}
		return out
	case "Practitioner":
		var r4p r4.Practitioner
		if !remarshal(body, &r4p) {
			return shallowCopy(body)
		}
		out, ok := marshalToMap(translate.PractitionerR4ToR5(&r4p))
		if !ok {
			return shallowCopy(body)
		}
		return out
	default:
		return shallowCopy(body)
	}
}

// translateResourceR5toR4 converts an R5 resource map to its R4 equivalent.
func translateResourceR5toR4(resourceType string, body map[string]any) map[string]any {
	switch resourceType {
	case "Patient":
		var r5p r5.Patient
		if !remarshal(body, &r5p) {
			return shallowCopy(body)
		}
		out, ok := marshalToMap(translate.PatientR5ToR4(&r5p))
		if !ok {
			return shallowCopy(body)
		}
		return out
	case "Practitioner":
		var r5p r5.Practitioner
		if !remarshal(body, &r5p) {
			return shallowCopy(body)
		}
		out, ok := marshalToMap(translate.PractitionerR5ToR4(&r5p))
		if !ok {
			return shallowCopy(body)
		}
		return out
	default:
		return shallowCopy(body)
	}
}

// remarshal round-trips src through JSON into dst. Returns false on failure.
func remarshal(src map[string]any, dst any) bool {
	raw, err := json.Marshal(src)
	if err != nil {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

// marshalToMap round-trips src through JSON into a map[string]any.
func marshalToMap(src any) (map[string]any, bool) {
	raw, err := json.Marshal(src)
	if err != nil {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return out, true
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// requireTenant extracts the tenant UUID placed in the request context by
// middleware.TenantExtraction.
func requireTenant(r *http.Request) (uuid.UUID, error) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		return uuid.Nil, errors.New("missing tenant context")
	}
	return tenantID, nil
}

// decodeResource unmarshals a stored resource's raw JSON into a map.
func decodeResource(raw json.RawMessage) (map[string]any, error) {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	return body, nil
}

// applyMeta overwrites the id and meta fields of body with server-assigned
// values from meta, ensuring API responses always reflect the store's record
// of the resource's identity and version regardless of client-supplied data.
func applyMeta(body map[string]any, meta *repo.ResourceMeta) {
	body["id"] = meta.ResourceID
	body["meta"] = map[string]any{
		"versionId":   meta.VersionID,
		"lastUpdated": meta.LastUpdated.UTC().Format(time.RFC3339),
	}
}

// writeStoreError translates a repo.Store error into the appropriate FHIR
// OperationOutcome HTTP response.
func writeStoreError(w http.ResponseWriter, resourceType, id string, err error) {
	if errors.Is(err, repo.ErrNotFound) {
		fhirError(w, http.StatusNotFound, "not-found", fmt.Sprintf("%s/%s not found", resourceType, id))
		return
	}
	fhirError(w, http.StatusInternalServerError, "processing", err.Error())
}

// fhirResourceCapability returns a minimal CapabilityStatement.rest.resource entry.
func fhirResourceCapability(resourceType string) map[string]any {
	return map[string]any{
		"type": resourceType,
		"interaction": []any{
			map[string]any{"code": "read"},
			map[string]any{"code": "create"},
			map[string]any{"code": "update"},
			map[string]any{"code": "delete"},
			map[string]any{"code": "search-type"},
		},
	}
}

// writeFHIRJSON writes a FHIR JSON response with the given HTTP status code.
func writeFHIRJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", fhirContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// fhirError writes a FHIR OperationOutcome JSON response.
func fhirError(w http.ResponseWriter, status int, code, details string) {
	outcome := map[string]any{
		"resourceType": "OperationOutcome",
		"issue": []any{
			map[string]any{
				"severity":    issueSeverity(status),
				"code":        code,
				"diagnostics": details,
			},
		},
	}
	writeFHIRJSON(w, status, outcome)
}

// issueSeverity maps an HTTP status code to a FHIR issue severity string.
func issueSeverity(status int) string {
	switch {
	case status >= 500:
		return "fatal"
	case status >= 400:
		return "error"
	default:
		return "information"
	}
}

// shallowCopy returns a new map with the same top-level keys/values.
func shallowCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
