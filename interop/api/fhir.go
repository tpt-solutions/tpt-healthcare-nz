package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
)

// fhirVersion identifies the FHIR version a handler is operating at.
type fhirVersion string

const (
	fhirVersionR5 fhirVersion = "R5"
	fhirVersionR4 fhirVersion = "R4"
)

// fhirContentType is the media type for FHIR JSON responses.
const fhirContentType = "application/fhir+json; charset=utf-8"

// resourceIDCounter is an atomic counter used to generate stub resource ids.
// Replace with github.com/google/uuid in production.
var resourceIDCounter int64

// FHIRHandler serves FHIR REST API requests for a specific version.
// It handles the standard FHIR CRUD + search operations and proxies R4
// requests through a thin R4↔R5 translation layer.
type FHIRHandler struct {
	version fhirVersion
}

// newFHIRHandler returns a FHIRHandler for the given version.
func newFHIRHandler(v fhirVersion) *FHIRHandler {
	return &FHIRHandler{version: v}
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
	resource := h.buildStubResource(resourceType, id)
	if h.version == fhirVersionR4 {
		resource = translateR5toR4(resource)
	}
	writeFHIRJSON(w, http.StatusOK, resource)
}

// handleCreate serves POST /{resourceType}.
func (h *FHIRHandler) handleCreate(w http.ResponseWriter, r *http.Request, resourceType string) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fhirError(w, http.StatusBadRequest, "invalid", "request body is not valid JSON: "+err.Error())
		return
	}
	if h.version == fhirVersionR4 {
		body = translateR4toR5(body)
	}
	newID := nextResourceID()
	body["id"] = newID
	body["resourceType"] = resourceType
	if h.version == fhirVersionR4 {
		body = translateR5toR4(body)
	}
	w.Header().Set("Location", fmt.Sprintf("%s/%s", resourceType, newID))
	writeFHIRJSON(w, http.StatusCreated, body)
}

// handleUpdate serves PUT /{resourceType}/{id}.
func (h *FHIRHandler) handleUpdate(w http.ResponseWriter, r *http.Request, resourceType, id string) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fhirError(w, http.StatusBadRequest, "invalid", "request body is not valid JSON: "+err.Error())
		return
	}
	if h.version == fhirVersionR4 {
		body = translateR4toR5(body)
	}
	body["id"] = id
	body["resourceType"] = resourceType
	if h.version == fhirVersionR4 {
		body = translateR5toR4(body)
	}
	writeFHIRJSON(w, http.StatusOK, body)
}

// handleDelete serves DELETE /{resourceType}/{id}.
func (h *FHIRHandler) handleDelete(w http.ResponseWriter, _ *http.Request, _, _ string) {
	w.WriteHeader(http.StatusNoContent)
}

// handleSearch serves GET /{resourceType} (search with query params).
func (h *FHIRHandler) handleSearch(w http.ResponseWriter, r *http.Request, _ string) {
	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "searchset",
		"total":        0,
		"entry":        []any{},
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
// R4 ↔ R5 translation stubs
// ---------------------------------------------------------------------------

// translateR4toR5 performs a minimal R4-to-R5 field translation. Replace this
// stub with a complete mapping for production use.
func translateR4toR5(r4 map[string]any) map[string]any {
	return shallowCopy(r4)
}

// translateR5toR4 performs a minimal R5-to-R4 field translation. Replace this
// stub with a complete mapping for production use.
func translateR5toR4(r5 map[string]any) map[string]any {
	return shallowCopy(r5)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildStubResource returns a minimal FHIR resource map for a given type and id.
func (h *FHIRHandler) buildStubResource(resourceType, id string) map[string]any {
	return map[string]any{
		"resourceType": resourceType,
		"id":           id,
		"meta": map[string]any{
			"versionId":   "1",
			"lastUpdated": "2026-01-01T00:00:00Z",
		},
	}
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

// nextResourceID returns a monotonically incrementing string id. Replace with
// github.com/google/uuid or a server-assigned strategy in production.
func nextResourceID() string {
	return fmt.Sprintf("%d", atomic.AddInt64(&resourceIDCounter, 1))
}

// shallowCopy returns a new map with the same top-level keys/values.
func shallowCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
