package events

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeEvent builds a minimal Event for test use.
func makeEvent(eventType string) Event {
	return Event{
		ID:            uuid.New(),
		Type:          eventType,
		AggregateID:   "agg-001",
		AggregateType: "Patient",
		TenantID:      uuid.New(),
		Payload:       map[string]string{"nhi": "ZAC5361"},
		OccurredAt:    time.Now().UTC(),
	}
}

func TestPublish(t *testing.T) {
	bus := New()
	ctx := context.Background()

	var received Event
	var called bool

	bus.Subscribe(PatientCreated, func(_ context.Context, e Event) error {
		received = e
		called = true
		return nil
	})

	evt := makeEvent(PatientCreated)
	err := bus.Publish(ctx, evt)
	require.NoError(t, err, "Publish should not return an error when handler succeeds")

	assert.True(t, called, "subscribed handler should have been called")
	assert.Equal(t, evt.ID, received.ID, "handler should receive the exact event published")
	assert.Equal(t, PatientCreated, received.Type)
}

func TestSubscribeAll(t *testing.T) {
	bus := New()
	ctx := context.Background()

	var receivedTypes []string
	var mu sync.Mutex

	bus.SubscribeAll(func(_ context.Context, e Event) error {
		mu.Lock()
		receivedTypes = append(receivedTypes, e.Type)
		mu.Unlock()
		return nil
	})

	events := []Event{
		makeEvent(PatientCreated),
		makeEvent(EncounterStarted),
		makeEvent(PrescriptionIssued),
	}

	for _, e := range events {
		require.NoError(t, bus.Publish(ctx, e))
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, receivedTypes, 3, "SubscribeAll handler should receive every published event")
	assert.Contains(t, receivedTypes, PatientCreated)
	assert.Contains(t, receivedTypes, EncounterStarted)
	assert.Contains(t, receivedTypes, PrescriptionIssued)
}

func TestPublishMultipleHandlers(t *testing.T) {
	bus := New()
	ctx := context.Background()

	var count int64

	increment := func(_ context.Context, _ Event) error {
		atomic.AddInt64(&count, 1)
		return nil
	}

	bus.Subscribe(ClaimSubmitted, increment)
	bus.Subscribe(ClaimSubmitted, increment)

	evt := makeEvent(ClaimSubmitted)
	err := bus.Publish(ctx, evt)
	require.NoError(t, err)

	assert.Equal(t, int64(2), atomic.LoadInt64(&count),
		"both handlers registered for the same event type should be called")
}

func TestPublishAsync(t *testing.T) {
	bus := New()
	ctx := context.Background()

	// This handler deliberately sleeps to simulate slow processing.
	// PublishAsync must return before the handler finishes.
	ready := make(chan struct{})
	bus.Subscribe(EncounterCompleted, func(_ context.Context, _ Event) error {
		close(ready)
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	evt := makeEvent(EncounterCompleted)

	start := time.Now()
	bus.PublishAsync(ctx, evt)
	elapsed := time.Since(start)

	// PublishAsync dispatches to goroutines and must return immediately — well
	// under the handler's 100ms sleep.
	assert.Less(t, elapsed, 50*time.Millisecond,
		"PublishAsync should return immediately without waiting for handlers")

	// Wait for the goroutine to start (proves it was actually dispatched).
	select {
	case <-ready:
		// handler was invoked asynchronously
	case <-time.After(500 * time.Millisecond):
		t.Fatal("async handler was never invoked within 500ms")
	}
}
