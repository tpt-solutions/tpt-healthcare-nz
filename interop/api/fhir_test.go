package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newR5Handler returns an http.Handler for the FHIR R5 route, matching the
// prefix-stripping that registerRoutes applies in production.
func newR5Handler() http.Handler {
	h := newFHIRHandler(fhirVersionR5)
	return http.StripPrefix("/fhir/r5", h.router())
}

// TestFHIRCreate verifies that POST /fhir/r5/Patient creates a resource and
// returns HTTP 201 with the resource in the response body.
func TestFHIRCreate(t *testing.T) {
	handler := newR5Handler()

	body := `{"resourceType":"Patient","name":[{"family":"Smith","given":["John"]}]}`
	req := httptest.NewRequest(http.MethodPost, "/fhir/r5/Patient", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/fhir+json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code, "POST /fhir/r5/Patient should return 201 Created")

	var resource map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resource))
	assert.Equal(t, "Patient", resource["resourceType"], "response resourceType should be Patient")
	assert.NotEmpty(t, resource["id"], "created resource should have an id assigned")

	location := rec.Header().Get("Location")
	assert.NotEmpty(t, location, "response should include a Location header")
	assert.Contains(t, location, "Patient/", "Location should reference the Patient resource")
}

// TestFHIRRead verifies that GET /fhir/r5/Patient/{id} returns the resource
// with HTTP 200.
func TestFHIRRead(t *testing.T) {
	handler := newR5Handler()

	req := httptest.NewRequest(http.MethodGet, "/fhir/r5/Patient/abc-123", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "GET /fhir/r5/Patient/{id} should return 200 OK")

	var resource map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resource))
	assert.Equal(t, "Patient", resource["resourceType"])
	assert.Equal(t, "abc-123", resource["id"], "returned resource id should match the requested id")
}

// TestFHIRNotFound verifies that a request with a missing resource type
// (i.e. a bare "/" path after prefix strip) returns a FHIR OperationOutcome
// with HTTP 400, because the handler requires a non-empty resource type
// segment. This is the closest analogue to a 404 for the current stub
// implementation, which does not maintain a persistent store.
func TestFHIRNotFound(t *testing.T) {
	handler := newR5Handler()

	// Strip prefix leaves "/" which gives an empty resourceType segment.
	req := httptest.NewRequest(http.MethodGet, "/fhir/r5/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Expect a 4xx client error with an OperationOutcome body.
	assert.GreaterOrEqual(t, rec.Code, 400, "missing resource type should result in a 4xx response")
	assert.Less(t, rec.Code, 500, "error should be a client error, not a server error")

	var outcome map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&outcome))
	assert.Equal(t, "OperationOutcome", outcome["resourceType"],
		"error response should be a FHIR OperationOutcome")

	issues, ok := outcome["issue"].([]any)
	require.True(t, ok, "OperationOutcome should have an issue array")
	require.NotEmpty(t, issues, "OperationOutcome should contain at least one issue")
}

// TestFHIRMetadata verifies that GET /fhir/r5/metadata returns a
// CapabilityStatement with HTTP 200.
func TestFHIRMetadata(t *testing.T) {
	handler := newR5Handler()

	req := httptest.NewRequest(http.MethodGet, "/fhir/r5/metadata", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "GET /fhir/r5/metadata should return 200 OK")
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/fhir+json",
		"metadata response should use FHIR JSON content type")

	var cs map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&cs))
	assert.Equal(t, "CapabilityStatement", cs["resourceType"])
	assert.Equal(t, "5.0.0", cs["fhirVersion"], "R5 handler should report FHIR version 5.0.0")
	assert.Equal(t, "active", cs["status"])

	rest, ok := cs["rest"].([]any)
	require.True(t, ok, "CapabilityStatement should have a rest array")
	require.NotEmpty(t, rest, "rest array should not be empty")
}
