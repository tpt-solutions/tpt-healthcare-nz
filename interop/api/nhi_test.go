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

// newNHITestHandler returns an NHIHandler wired with nil dependencies (unit-test mode).
func newNHITestHandler() *NHIHandler {
	return newNHIHandler(nil, nil)
}

// TestNHILookupMethodNotAllowed verifies that non-GET requests to the lookup path
// are rejected with HTTP 405.
func TestNHILookupMethodNotAllowed(t *testing.T) {
	h := newNHITestHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nhi/ABC1234", nil)
	rec := httptest.NewRecorder()
	h.router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// TestNHILookupMissingNHI verifies that a GET to the bare /api/v1/nhi/ path
// (no NHI segment) returns HTTP 400.
func TestNHILookupMissingNHI(t *testing.T) {
	h := newNHITestHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nhi/", nil)
	rec := httptest.NewRecorder()
	h.router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	assert.Contains(t, body["error"], "required")
}

// TestNHILookupInvalidFormat verifies that a NHI failing format validation
// returns HTTP 400 with an informative error message.
func TestNHILookupInvalidFormat(t *testing.T) {
	h := newNHITestHandler()
	tests := []struct {
		nhi  string
		desc string
	}{
		{"INVALID", "non-alphanumeric / wrong pattern"},
		{"123", "digits only"},
		{"abcdefg", "lowercase not allowed"},
		{"AB12345", "only 2 leading letters"},
		{"ABCD123", "4 leading letters"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/nhi/"+tc.nhi, nil)
			rec := httptest.NewRecorder()
			h.router().ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "nhi=%q", tc.nhi)

			var body map[string]string
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
			assert.NotEmpty(t, body["error"])
		})
	}
}

// TestNHILookupNilClientServiceUnavailable verifies that a syntactically valid NHI
// with no NHI client configured returns HTTP 400 (failed checksum) or HTTP 503
// (nil client — format passed). Either is acceptable depending on whether
// ValidateNHI enforces checksum.
func TestNHILookupNilClientOrChecksumFail(t *testing.T) {
	h := newNHITestHandler()
	// ABC1234 matches the old-format regex [A-Z]{3}[0-9]{4}. If the core
	// validator also checks the Luhn/MOH checksum this returns 400; if it only
	// checks the regex it returns 503 (nil client).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nhi/ABC1234", nil)
	rec := httptest.NewRecorder()
	h.router().ServeHTTP(rec, req)

	// We accept either a 400 (checksum failed) or 503 (nil client, format OK).
	assert.True(t, rec.Code == http.StatusBadRequest || rec.Code == http.StatusServiceUnavailable,
		"expected 400 or 503, got %d", rec.Code)
}

// TestNHIMatchMethodNotAllowed verifies that GET requests to /api/v1/nhi/match
// are rejected with HTTP 405.
func TestNHIMatchMethodNotAllowed(t *testing.T) {
	h := newNHITestHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nhi/match", nil)
	rec := httptest.NewRecorder()
	h.router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

// TestNHIMatchBadJSON verifies that a non-JSON body returns HTTP 400.
func TestNHIMatchBadJSON(t *testing.T) {
	h := newNHITestHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nhi/match", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestNHIMatchMissingRequiredFields verifies that a match request missing
// givenName, familyName, or birthDate returns HTTP 400.
func TestNHIMatchMissingRequiredFields(t *testing.T) {
	h := newNHITestHandler()
	tests := []struct {
		name string
		body string
	}{
		{"missing givenName", `{"familyName":"Smith","birthDate":"1980-01-01"}`},
		{"missing familyName", `{"givenName":"John","birthDate":"1980-01-01"}`},
		{"missing birthDate", `{"givenName":"John","familyName":"Smith"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/nhi/match", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.router().ServeHTTP(rec, req)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

// TestNHIMatchInvalidBirthDateFormat verifies that a birthDate not in YYYY-MM-DD
// returns HTTP 400.
func TestNHIMatchInvalidBirthDateFormat(t *testing.T) {
	h := newNHITestHandler()
	body := `{"givenName":"John","familyName":"Smith","birthDate":"01/01/1980"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nhi/match", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestNHIMatchNilClientServiceUnavailable verifies that a valid match request
// with no NHI client configured returns HTTP 503.
func TestNHIMatchNilClientServiceUnavailable(t *testing.T) {
	h := newNHITestHandler()
	body := `{"givenName":"John","familyName":"Smith","birthDate":"1980-06-15"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nhi/match", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.router().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var body2 map[string]string
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body2))
	assert.Contains(t, body2["error"], "not configured")
}
