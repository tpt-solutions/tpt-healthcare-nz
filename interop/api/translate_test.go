package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PhillipC05/tpt-healthcare/core/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newR4Handler returns an http.Handler for the FHIR R4 route with prefix stripped.
func newR4Handler(store repo.Store) http.Handler {
	h := newFHIRHandler(fhirVersionR4, store)
	return http.StripPrefix("/fhir/r4", h.router())
}

// TestR5MetadataFHIRVersion verifies the R5 capability statement reports fhirVersion 5.0.0.
func TestR5MetadataFHIRVersion(t *testing.T) {
	handler := newR5Handler(repo.NewMemoryStore())
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
	handler := newR4Handler(repo.NewMemoryStore())
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

// TestTranslatePatientR4toR5PreservesData verifies that translateResourceR4toR5
// maps a Patient's fields onto the R5 shape using the dedicated translator.
func TestTranslatePatientR4toR5PreservesData(t *testing.T) {
	input := map[string]any{
		"resourceType": "Patient",
		"id":           "test-1",
		"name":         []any{map[string]any{"family": "Smith", "given": []any{"John"}}},
		"birthDate":    "1980-01-01",
		"gender":       "male",
	}

	output := translateResourceR4toR5("Patient", input)

	assert.Equal(t, "Patient", output["resourceType"])
	assert.Equal(t, "test-1", output["id"])
	assert.Equal(t, "1980-01-01", output["birthDate"])
	assert.Equal(t, "male", output["gender"])
	names, ok := output["name"].([]any)
	require.True(t, ok)
	require.Len(t, names, 1)
	name := names[0].(map[string]any)
	assert.Equal(t, "Smith", name["family"])
}

// TestTranslateUnmappedResourcePassesThrough verifies that resource types
// without a dedicated translator are passed through unchanged (no data loss).
func TestTranslateUnmappedResourcePassesThrough(t *testing.T) {
	input := map[string]any{
		"resourceType": "Observation",
		"id":           "obs-42",
		"status":       "final",
		"code":         map[string]any{"text": "Blood pressure"},
	}

	toR5 := translateResourceR4toR5("Observation", input)
	for k, v := range input {
		assert.Equal(t, v, toR5[k], "unmapped resource type must pass through key %q unchanged", k)
	}

	toR4 := translateResourceR5toR4("Observation", toR5)
	for k, v := range input {
		assert.Equal(t, v, toR4[k], "round-trip must preserve key %q", k)
	}
}

// TestR4CreateReturns201 verifies that POST to the R4 handler creates a resource
// and returns HTTP 201 with a Location header, mirroring the R5 behaviour.
func TestR4CreateReturns201(t *testing.T) {
	handler := newR4Handler(repo.NewMemoryStore())
	body := `{"resourceType":"Patient","name":[{"family":"Tane","given":["Wiremu"]}]}`
	req := withTestTenant(httptest.NewRequest(http.MethodPost, "/fhir/r4/Patient", strings.NewReader(body)))
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
	handler := newR5Handler(repo.NewMemoryStore())
	req := withTestTenant(httptest.NewRequest(http.MethodGet, "/fhir/r5/Patient", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var bundle map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&bundle))
	assert.Equal(t, "Bundle", bundle["resourceType"])
	assert.Equal(t, "searchset", bundle["type"])
}

// TestFHIRDeleteReturns204 verifies that DELETE /{resourceType}/{id} returns
// HTTP 204 No Content for a resource that exists.
func TestFHIRDeleteReturns204(t *testing.T) {
	store := repo.NewMemoryStore()
	handler := newR5Handler(store)

	createBody := `{"resourceType":"Patient","name":[{"family":"ToDelete"}]}`
	createReq := withTestTenant(httptest.NewRequest(http.MethodPost, "/fhir/r5/Patient", strings.NewReader(createBody)))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(createRec.Body).Decode(&created))
	id := created["id"].(string)

	req := withTestTenant(httptest.NewRequest(http.MethodDelete, "/fhir/r5/Patient/"+id, nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// TestFHIRUpdateReturns200 verifies that PUT /{resourceType}/{id} returns HTTP 200
// with the updated resource echoed back once it has been created.
func TestFHIRUpdateReturns200(t *testing.T) {
	store := repo.NewMemoryStore()
	handler := newR5Handler(store)

	createBody := `{"resourceType":"Patient","id":"p1","name":[{"family":"Original"}]}`
	createReq := withTestTenant(httptest.NewRequest(http.MethodPost, "/fhir/r5/Patient", strings.NewReader(createBody)))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(createRec.Body).Decode(&created))
	id := created["id"].(string)

	body := `{"resourceType":"Patient","name":[{"family":"Updated"}]}`
	req := withTestTenant(httptest.NewRequest(http.MethodPut, "/fhir/r5/Patient/"+id, strings.NewReader(body)))
	req.Header.Set("Content-Type", "application/fhir+json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resource map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resource))
	assert.Equal(t, id, resource["id"])
	assert.Equal(t, "Patient", resource["resourceType"])
}

// TestFHIRPOSTWithIDReturnsError verifies that POST /{resourceType}/{id}
// (which is ambiguous in FHIR) is rejected.
func TestFHIRPOSTWithIDReturnsError(t *testing.T) {
	handler := newR5Handler(repo.NewMemoryStore())
	body := `{"resourceType":"Patient","id":"existing"}`
	req := withTestTenant(httptest.NewRequest(http.MethodPost, "/fhir/r5/Patient/existing", strings.NewReader(body)))
	req.Header.Set("Content-Type", "application/fhir+json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// TestFHIRCapabilityStatementStructure verifies the full structure of the R5
// capability statement including required rest resources.
func TestFHIRCapabilityStatementStructure(t *testing.T) {
	handler := newR5Handler(repo.NewMemoryStore())
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
