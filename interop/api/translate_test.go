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

// newR4Handler returns an http.Handler for the FHIR R4 route with prefix stripped.
func newR4Handler() http.Handler {
	h := newFHIRHandler(fhirVersionR4)
	return http.StripPrefix("/fhir/r4", h.router())
}

// TestR5MetadataFHIRVersion verifies the R5 capability statement reports fhirVersion 5.0.0.
func TestR5MetadataFHIRVersion(t *testing.T) {
	handler := newR5Handler()
	req := httptest.NewRequest(http.MethodGet, "/fhir/r5/metadata", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var cs map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&cs))
	assert.Equal(t, "5.0.0", cs["fhirVersion"])
}

// TestR4MetadataFHIRVersion verifies the R4 capability statement reports fhirVersion 4.0.1.
func TestR4MetadataFHIRVersion(t *testing.T) {
	handler := newR4Handler()
	req := httptest.NewRequest(http.MethodGet, "/fhir/r4/metadata", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/fhir+json")
	var cs map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&cs))
	assert.Equal(t, "CapabilityStatement", cs["resourceType"])
	assert.Equal(t, "4.0.1", cs["fhirVersion"], "R4 handler must report FHIR version 4.0.1")
}

// TestTranslateR4toR5PreservesKeys verifies that translateR4toR5 returns a map
// containing all keys present in the input (no data is silently dropped).
func TestTranslateR4toR5PreservesKeys(t *testing.T) {
	input := map[string]any{
		"resourceType": "Patient",
		"id":           "test-1",
		"name":         []any{map[string]any{"family": "Smith", "given": []any{"John"}}},
		"birthDate":    "1980-01-01",
		"gender":       "male",
	}

	output := translateR4toR5(input)

	for k := range input {
		assert.Contains(t, output, k, "R4→R5 translation must preserve key %q", k)
		assert.Equal(t, input[k], output[k], "R4→R5 must not change value of key %q", k)
	}
}

// TestTranslateR5toR4PreservesKeys verifies that translateR5toR4 returns a map
// containing all keys present in the input.
func TestTranslateR5toR4PreservesKeys(t *testing.T) {
	input := map[string]any{
		"resourceType": "Patient",
		"id":           "test-2",
		"name":         []any{map[string]any{"family": "Doe", "given": []any{"Jane"}}},
		"birthDate":    "1990-06-15",
		"gender":       "female",
		"meta":         map[string]any{"versionId": "1"},
	}

	output := translateR5toR4(input)

	for k := range input {
		assert.Contains(t, output, k, "R5→R4 translation must preserve key %q", k)
	}
}

// TestR4R5RoundTrip verifies that translating R4→R5→R4 returns a map
// structurally equivalent to the original.
func TestR4R5RoundTrip(t *testing.T) {
	original := map[string]any{
		"resourceType": "Observation",
		"id":           "obs-42",
		"status":       "final",
		"code":         map[string]any{"text": "Blood pressure"},
	}

	roundTripped := translateR5toR4(translateR4toR5(original))

	for k, v := range original {
		assert.Equal(t, v, roundTripped[k], "round-trip must preserve key %q", k)
	}
}

// TestR4CreateReturns201 verifies that POST to the R4 handler creates a resource
// and returns HTTP 201 with a Location header, mirroring the R5 behaviour.
func TestR4CreateReturns201(t *testing.T) {
	handler := newR4Handler()
	body := `{"resourceType":"Patient","name":[{"family":"Tane","given":["Wiremu"]}]}`
	req := httptest.NewRequest(http.MethodPost, "/fhir/r4/Patient", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/fhir+json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Location"), "R4 create should set Location header")

	var resource map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resource))
	assert.Equal(t, "Patient", resource["resourceType"])
	assert.NotEmpty(t, resource["id"], "created resource must have an assigned id")
}

// TestFHIRSearchReturnsBundle verifies that GET /{resourceType} (search) returns
// a FHIR Bundle with type=searchset.
func TestFHIRSearchReturnsBundle(t *testing.T) {
	handler := newR5Handler()
	req := httptest.NewRequest(http.MethodGet, "/fhir/r5/Patient", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var bundle map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&bundle))
	assert.Equal(t, "Bundle", bundle["resourceType"])
	assert.Equal(t, "searchset", bundle["type"])
}

// TestFHIRDeleteReturns204 verifies that DELETE /{resourceType}/{id} returns
// HTTP 204 No Content.
func TestFHIRDeleteReturns204(t *testing.T) {
	handler := newR5Handler()
	req := httptest.NewRequest(http.MethodDelete, "/fhir/r5/Patient/to-delete", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// TestFHIRUpdateReturns200 verifies that PUT /{resourceType}/{id} returns HTTP 200
// with the updated resource echoed back.
func TestFHIRUpdateReturns200(t *testing.T) {
	handler := newR5Handler()
	body := `{"resourceType":"Patient","id":"p1","name":[{"family":"Updated"}]}`
	req := httptest.NewRequest(http.MethodPut, "/fhir/r5/Patient/p1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/fhir+json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resource map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resource))
	assert.Equal(t, "p1", resource["id"])
	assert.Equal(t, "Patient", resource["resourceType"])
}

// TestFHIRPOSTWithIDReturnsError verifies that POST /{resourceType}/{id}
// (which is ambiguous in FHIR) is rejected.
func TestFHIRPOSTWithIDReturnsError(t *testing.T) {
	handler := newR5Handler()
	body := `{"resourceType":"Patient","id":"existing"}`
	req := httptest.NewRequest(http.MethodPost, "/fhir/r5/Patient/existing", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/fhir+json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// TestFHIRCapabilityStatementStructure verifies the full structure of the R5
// capability statement including required rest resources.
func TestFHIRCapabilityStatementStructure(t *testing.T) {
	handler := newR5Handler()
	req := httptest.NewRequest(http.MethodGet, "/fhir/r5/metadata", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var cs map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&cs))

	assert.Equal(t, "active", cs["status"])
	assert.Equal(t, "instance", cs["kind"])

	rest, ok := cs["rest"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, rest)

	server, ok := rest[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "server", server["mode"])

	resources, ok := server["resource"].([]any)
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(resources), 5, "capability statement should advertise at least 5 resource types")
}
