package subscription

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/PhillipC05/tpt-healthcare/core/events"
)

// topicBase is the canonical base URI for tpt-healthcare FHIR SubscriptionTopics.
const topicBase = "https://standards.digital.health.nz/fhir/SubscriptionTopic/"

// domainTopicMap maps domain event types to FHIR SubscriptionTopic canonical URLs.
var domainTopicMap = map[string]struct {
	topic        string
	resourceType string
}{
	events.PatientCreated:      {topicBase + "patient-created", "Patient"},
	events.PatientUpdated:      {topicBase + "patient-updated", "Patient"},
	events.EncounterStarted:    {topicBase + "encounter-started", "Encounter"},
	events.EncounterCompleted:  {topicBase + "encounter-completed", "Encounter"},
	events.PrescriptionIssued:  {topicBase + "prescription-issued", "MedicationRequest"},
	events.ClaimSubmitted:      {topicBase + "claim-submitted", "Claim"},
	// Queue events
	"queue.entry.checked_in":       {topicBase + "queue-entry-checked-in", "QueueEntry"},
	"queue.entry.called":           {topicBase + "queue-entry-called", "QueueEntry"},
	"queue.entry.location_updated": {topicBase + "queue-entry-location-updated", "QueueEntry"},
	"queue.entry.done":             {topicBase + "queue-entry-done", "QueueEntry"},
	"queue.entry.skipped":          {topicBase + "queue-entry-skipped", "QueueEntry"},
	"queue.entry.left":             {topicBase + "queue-entry-left", "QueueEntry"},
}

// WireEventBus subscribes to all events on bus and forwards them to the
// subscription Engine so FHIR R5 subscriptions receive the notifications.
// Call once during server startup.
func WireEventBus(bus *events.Bus, engine *Engine, logger *slog.Logger) {
	bus.SubscribeAll(func(ctx context.Context, ev events.Event) error {
		meta, ok := domainTopicMap[ev.Type]
		if !ok {
			return nil // no FHIR topic mapped for this event type
		}

		payload, err := toRawMessage(ev.Payload)
		if err != nil {
			logger.WarnContext(ctx, "subscription bridge: marshal payload",
				slog.String("eventType", ev.Type),
				slog.String("error", err.Error()),
			)
			return nil // non-fatal: don't block the event bus
		}

		if err := engine.Publish(ctx, meta.topic, meta.resourceType, ev.AggregateID, payload); err != nil {
			logger.WarnContext(ctx, "subscription bridge: engine publish failed",
				slog.String("topic", meta.topic),
				slog.String("error", err.Error()),
			)
		}
		return nil
	})
}

// toRawMessage converts an arbitrary value to json.RawMessage.
func toRawMessage(v any) (json.RawMessage, error) {
	if raw, ok := v.(json.RawMessage); ok {
		return raw, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}
