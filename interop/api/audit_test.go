package api

import (
	"context"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedAuditTrail is an in-memory nhiAuditTrailer that captures audit events
// for assertion in tests.
type recordedAuditTrail struct {
	mu     sync.Mutex
	events []audit.Event
}

func (r *recordedAuditTrail) Record(_ context.Context, e audit.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

func (r *recordedAuditTrail) captured() []audit.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]audit.Event, len(r.events))
	copy(out, r.events)
	return out
}

// TestAuditNilTrailIsSafe verifies that recordAudit does not panic or error
// when the NHI handler was constructed with a nil audit trail.
func TestAuditNilTrailIsSafe(t *testing.T) {
	h := newNHIHandler(nil, nil) // nil trail
	req := httptest.NewRequest("GET", "/api/v1/nhi/ABC1234", nil)

	// Must not panic.
	assert.NotPanics(t, func() {
		h.recordAudit(req, "ABC1234", "read", "test")
	})
}

// TestAuditRecordIsCalledOnLookupNotFound verifies that the NHI handler records
// an audit event when the NHI is not found (nil client returns 503 or 400, but
// audit is attempted before the client check for not-found paths via recordAudit).
//
// This test exercises recordAudit directly rather than routing through the HTTP
// handler so it remains deterministic regardless of whether ValidateNHI also
// checks checksums.
func TestAuditRecordIsCalledDirectly(t *testing.T) {
	trail := &recordedAuditTrail{}
	h := newNHIHandler(nil, trail)

	req := httptest.NewRequest("GET", "/api/v1/nhi/ZBB4540", nil)

	h.recordAudit(req, "ZBB4540", "read", "success")

	events := trail.captured()
	require.Len(t, events, 1, "exactly one audit event should have been recorded")

	e := events[0]
	assert.Equal(t, "ZBB4540", e.PatientNHI)
	assert.Equal(t, "read", e.Action)
	assert.Equal(t, "Patient", e.ResourceType)
	assert.WithinDuration(t, time.Now(), e.OccurredAt, 5*time.Second, "audit event timestamp should be recent")
}

// TestAuditEventFieldsFromMatchHandler verifies that a match audit event
// records the expected details map including the path.
func TestAuditEventPathIsRecorded(t *testing.T) {
	trail := &recordedAuditTrail{}
	h := newNHIHandler(nil, trail)

	req := httptest.NewRequest("POST", "/api/v1/nhi/match", nil)
	h.recordAudit(req, "", "match", "returned 0 candidates")

	events := trail.captured()
	require.Len(t, events, 1)

	e := events[0]
	details, ok := e.Details.(map[string]any)
	require.True(t, ok, "Details should be a map[string]any")
	assert.Equal(t, "/api/v1/nhi/match", details["path"])
	assert.Equal(t, "returned 0 candidates", details["detail"])
}

// TestAuditNHIIsUppercased verifies that the PatientNHI field is normalised to
// uppercase in the recorded event.
func TestAuditNHIIsUppercased(t *testing.T) {
	trail := &recordedAuditTrail{}
	h := newNHIHandler(nil, trail)

	req := httptest.NewRequest("GET", "/api/v1/nhi/abc1234", nil)
	h.recordAudit(req, "abc1234", "read", "test")

	events := trail.captured()
	require.Len(t, events, 1)
	assert.Equal(t, "ABC1234", events[0].PatientNHI, "NHI should be uppercased in audit event")
}
