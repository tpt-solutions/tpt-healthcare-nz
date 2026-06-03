package events

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Common domain event type constants.
const (
	PatientCreated      = "patient.created"
	PatientUpdated      = "patient.updated"
	EncounterStarted    = "encounter.started"
	EncounterCompleted  = "encounter.completed"
	PrescriptionIssued  = "prescription.issued"
	ClaimSubmitted      = "claim.submitted"
)

// subscribeAllKey is the internal key used to register handlers that receive every event.
const subscribeAllKey = "*"

// Event is a domain event published on the Bus.
type Event struct {
	ID            uuid.UUID
	Type          string
	AggregateID   string
	AggregateType string
	TenantID      uuid.UUID
	Payload       any
	OccurredAt    time.Time
}

// Handler is a function that processes a domain event.
type Handler func(ctx context.Context, e Event) error

// Bus is an in-process domain event bus that dispatches events to registered handlers.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// New returns a new, ready-to-use Bus.
func New() *Bus {
	return &Bus{
		handlers: make(map[string][]Handler),
	}
}

// Subscribe registers a Handler to be called whenever an event of eventType is published.
func (b *Bus) Subscribe(eventType string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], h)
}

// SubscribeAll registers a Handler to be called for every published event, regardless of type.
func (b *Bus) SubscribeAll(h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[subscribeAllKey] = append(b.handlers[subscribeAllKey], h)
}

// Publish dispatches e to all matching handlers synchronously.
// All handlers are called even if some return errors; all errors are collected and returned
// as a single combined error.
func (b *Bus) Publish(ctx context.Context, e Event) error {
	b.mu.RLock()
	typed := make([]Handler, len(b.handlers[e.Type]))
	copy(typed, b.handlers[e.Type])
	all := make([]Handler, len(b.handlers[subscribeAllKey]))
	copy(all, b.handlers[subscribeAllKey])
	b.mu.RUnlock()

	var errs []string
	for _, h := range typed {
		if err := h(ctx, e); err != nil {
			errs = append(errs, err.Error())
		}
	}
	for _, h := range all {
		if err := h(ctx, e); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("event bus publish errors for %q: %s", e.Type, strings.Join(errs, "; "))
	}
	return nil
}

// PublishAsync dispatches e to all matching handlers concurrently in separate goroutines.
// Errors returned by handlers are silently discarded; callers that need error handling
// should use Publish instead or wrap handlers with their own error logic.
func (b *Bus) PublishAsync(ctx context.Context, e Event) {
	b.mu.RLock()
	typed := make([]Handler, len(b.handlers[e.Type]))
	copy(typed, b.handlers[e.Type])
	all := make([]Handler, len(b.handlers[subscribeAllKey]))
	copy(all, b.handlers[subscribeAllKey])
	b.mu.RUnlock()

	for _, h := range typed {
		h := h
		go h(ctx, e) //nolint:errcheck
	}
	for _, h := range all {
		h := h
		go h(ctx, e) //nolint:errcheck
	}
}
