// Package subscription implements a FHIR R5 subscription engine backed by
// Redis pub/sub. It supports rest-hook, WebSocket, and email channel types as
// defined in the FHIR R5 SubscriptionTopic / Subscription resources.
package subscription

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ChannelType identifies the delivery mechanism for a FHIR R5 Subscription.
type ChannelType string

const (
	// ChannelRestHook delivers notifications via HTTP POST (rest-hook).
	ChannelRestHook ChannelType = "rest-hook"
	// ChannelWebSocket delivers notifications via WebSocket.
	ChannelWebSocket ChannelType = "websocket"
	// ChannelEmail delivers notifications via email (placeholder).
	ChannelEmail ChannelType = "email"
)

// redisChannelPrefix is prepended to topic names when publishing to Redis.
const redisChannelPrefix = "fhir:subscription:"

// Subscription represents a FHIR R5 Subscription resource stored in-memory.
type Subscription struct {
	// ID is the unique subscription identifier.
	ID uuid.UUID
	// TenantID scopes the subscription to a specific tenant.
	TenantID uuid.UUID
	// Topic is the canonical URL of the SubscriptionTopic this subscription
	// monitors, e.g. "http://example.org/fhir/SubscriptionTopic/new-lab-result".
	Topic string
	// Criteria is an optional FHIR search string to filter notifications within
	// the topic, e.g. "Observation?category=laboratory".
	Criteria string
	// Channel specifies how notifications are delivered.
	Channel ChannelType
	// Endpoint is the delivery destination. For rest-hook this is an HTTPS URL;
	// for email, the recipient address.
	Endpoint string
	// Headers is a map of additional HTTP headers to include in rest-hook POSTs.
	Headers map[string]string
	// HeartbeatPeriod is the interval in seconds at which a heartbeat
	// notification is sent (0 = disabled).
	HeartbeatPeriod int
	// Active indicates whether the subscription is currently enabled.
	Active bool
}

// EmailSender is satisfied by any transactional email provider that can send
// a plain-text or HTML email. Implementations live in core/email/*/
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// Engine is the FHIR R5 subscription dispatcher. It registers subscriptions,
// receives resource change events via Redis pub/sub, matches them against
// registered subscriptions, and dispatches notifications to the appropriate
// channel.
type Engine struct {
	rdb           *redis.Client
	mu            sync.RWMutex
	subscriptions map[uuid.UUID]Subscription
	httpClient    *http.Client
	wsHub         *Hub         // WebSocket notification hub (set via SetWSHub)
	emailSender   EmailSender  // email dispatch backend (set via SetEmailSender)
}

// New creates a new subscription Engine backed by the provided Redis client.
func New(rdb *redis.Client) *Engine {
	return &Engine{
		rdb:           rdb,
		subscriptions: make(map[uuid.UUID]Subscription),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetWSHub attaches a WebSocket Hub to the engine for ChannelWebSocket delivery.
// The Hub's Run() goroutine must be started separately by the caller.
func (e *Engine) SetWSHub(hub *Hub) {
	e.wsHub = hub
}

// SetEmailSender attaches an email provider to the engine for ChannelEmail delivery.
func (e *Engine) SetEmailSender(sender EmailSender) {
	e.emailSender = sender
}

// Register adds or replaces a Subscription in the engine.
// The subscription is persisted to Redis as a hash so it survives restarts.
func (e *Engine) Register(ctx context.Context, sub Subscription) error {
	if sub.ID == uuid.Nil {
		sub.ID = uuid.New()
	}

	// Serialise to JSON for Redis persistence.
	data, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("subscription: marshal: %w", err)
	}

	key := fmt.Sprintf("fhir:sub:%s", sub.ID)
	if err := e.rdb.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("subscription: redis set: %w", err)
	}

	e.mu.Lock()
	e.subscriptions[sub.ID] = sub
	e.mu.Unlock()

	log.Printf("subscription: registered %s topic=%s channel=%s endpoint=%s",
		sub.ID, sub.Topic, sub.Channel, sub.Endpoint)
	return nil
}

// Unregister removes a Subscription by ID from the engine and from Redis.
func (e *Engine) Unregister(ctx context.Context, id uuid.UUID) error {
	e.mu.Lock()
	delete(e.subscriptions, id)
	e.mu.Unlock()

	key := fmt.Sprintf("fhir:sub:%s", id)
	if err := e.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("subscription: redis del: %w", err)
	}

	log.Printf("subscription: unregistered %s", id)
	return nil
}

// eventPayload is the structure published to Redis channels.
type eventPayload struct {
	Topic        string          `json:"topic"`
	ResourceType string          `json:"resourceType"`
	ResourceID   string          `json:"resourceId"`
	Payload      json.RawMessage `json:"payload"`
	Timestamp    time.Time       `json:"timestamp"`
}

// Publish publishes a resource change event to the Redis channel for the
// given topic. All subscriptions with a matching topic will be notified by
// the Start goroutine.
//
// topic should be the canonical SubscriptionTopic URL.
// resourceType and resourceID identify the changed FHIR resource.
// payload is the JSON-encoded resource (or a notification bundle).
func (e *Engine) Publish(ctx context.Context, topic, resourceType, resourceID string, payload json.RawMessage) error {
	ev := eventPayload{
		Topic:        topic,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Payload:      payload,
		Timestamp:    time.Now().UTC(),
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("subscription: marshal event: %w", err)
	}

	channel := redisChannelPrefix + topic
	if err := e.rdb.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("subscription: publish to %s: %w", channel, err)
	}
	return nil
}

// Start subscribes to all relevant Redis pub/sub channels and dispatches
// incoming events to matching subscriptions.
// It also reloads persisted subscriptions from Redis on startup.
// Blocks until ctx is cancelled.
func (e *Engine) Start(ctx context.Context) error {
	if err := e.loadFromRedis(ctx); err != nil {
		log.Printf("subscription: warning: could not reload from Redis: %v", err)
	}

	// Build the set of unique topics we need to subscribe to.
	e.mu.RLock()
	topicSet := make(map[string]struct{})
	for _, sub := range e.subscriptions {
		topicSet[sub.Topic] = struct{}{}
	}
	e.mu.RUnlock()

	// Subscribe to all known topic channels. We also subscribe to the wildcard
	// pattern to catch topics registered after Start() is called.
	pattern := redisChannelPrefix + "*"
	pubsub := e.rdb.PSubscribe(ctx, pattern)
	defer pubsub.Close()

	log.Printf("subscription: started, listening on pattern %s", pattern)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			log.Printf("subscription: engine shutting down")
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			go e.handleRedisMessage(ctx, msg.Payload)
		}
	}
}

// handleRedisMessage deserialises a Redis pub/sub message and dispatches
// notifications to all matching active subscriptions.
func (e *Engine) handleRedisMessage(ctx context.Context, raw string) {
	var ev eventPayload
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		log.Printf("subscription: unmarshal event: %v", err)
		return
	}

	// Build per-subscription FHIR notification bundles with correct subscription references.

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, sub := range e.subscriptions {
		if !sub.Active || sub.Topic != ev.Topic {
			continue
		}
		bundle := buildNotificationBundle(ev, sub.ID)
		s := sub // capture
		go func() {
			var dispErr error
			switch s.Channel {
			case ChannelRestHook:
				dispErr = e.dispatchRestHook(ctx, s, bundle)
			case ChannelEmail:
				dispErr = e.dispatchEmail(ctx, s, bundle)
			case ChannelWebSocket:
				dispErr = e.dispatchWebSocket(s, bundle)
			default:
				log.Printf("subscription: unknown channel type %q for subscription %s", s.Channel, s.ID)
			}
			if dispErr != nil {
				log.Printf("subscription: dispatch error for %s: %v", s.ID, dispErr)
			}
		}()
	}
}

// dispatchRestHook delivers a FHIR notification bundle to the subscription
// endpoint via HTTP POST. It includes any configured headers and sets the
// Content-Type to application/fhir+json.
func (e *Engine) dispatchRestHook(ctx context.Context, sub Subscription, bundle json.RawMessage) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(bundle))
	if err != nil {
		return fmt.Errorf("rest-hook: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/fhir+json")
	for k, v := range sub.Headers {
		req.Header.Set(k, v)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("rest-hook: POST %s: %w", sub.Endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rest-hook: non-2xx response %d from %s", resp.StatusCode, sub.Endpoint)
	}

	log.Printf("subscription: rest-hook delivered to %s (status %d)", sub.Endpoint, resp.StatusCode)
	return nil
}

// dispatchEmail delivers a FHIR notification bundle as an email to sub.Endpoint.
// The bundle is serialised as a JSON attachment in the email body.
// Requires an EmailSender to be configured via SetEmailSender.
func (e *Engine) dispatchEmail(ctx context.Context, sub Subscription, bundle json.RawMessage) error {
	if e.emailSender == nil {
		log.Printf("subscription: email dispatch skipped for %s — no email sender configured", sub.ID)
		return nil
	}

	subject := fmt.Sprintf("FHIR Subscription Notification — %s", sub.Topic)
	body := fmt.Sprintf(
		"A FHIR resource change has been published for subscription %s.\n\n"+
			"Topic: %s\n"+
			"Notification bundle:\n\n%s",
		sub.ID, sub.Topic, string(bundle),
	)

	if err := e.emailSender.SendEmail(ctx, sub.Endpoint, subject, body); err != nil {
		return fmt.Errorf("subscription: email dispatch to %s: %w", sub.Endpoint, err)
	}

	log.Printf("subscription: email dispatched to %s for subscription %s", sub.Endpoint, sub.ID)
	return nil
}

// dispatchWebSocket delivers a FHIR notification bundle to all WebSocket clients
// that are listening to sub.ID. Requires a Hub to be configured via SetWSHub.
func (e *Engine) dispatchWebSocket(sub Subscription, bundle json.RawMessage) error {
	if e.wsHub == nil {
		log.Printf("subscription: websocket dispatch skipped for %s — no hub configured", sub.ID)
		return nil
	}
	e.wsHub.Dispatch([]uuid.UUID{sub.ID}, bundle)
	log.Printf("subscription: websocket notification dispatched for subscription %s", sub.ID)
	return nil
}

// loadFromRedis reloads all persisted subscriptions from Redis keys matching
// "fhir:sub:*" into the in-memory map.
func (e *Engine) loadFromRedis(ctx context.Context) error {
	keys, err := e.rdb.Keys(ctx, "fhir:sub:*").Result()
	if err != nil {
		return err
	}
	for _, key := range keys {
		data, err := e.rdb.Get(ctx, key).Bytes()
		if err != nil {
			log.Printf("subscription: load key %s: %v", key, err)
			continue
		}
		var sub Subscription
		if err := json.Unmarshal(data, &sub); err != nil {
			log.Printf("subscription: unmarshal key %s: %v", key, err)
			continue
		}
		e.mu.Lock()
		e.subscriptions[sub.ID] = sub
		e.mu.Unlock()
	}
	log.Printf("subscription: loaded %d subscriptions from Redis", len(keys))
	return nil
}

// buildNotificationBundle constructs a FHIR R5 notification Bundle
// wrapping the event payload. The subscriptionID is used to set the
// correct Subscription reference in the SubscriptionStatus resource.
func buildNotificationBundle(ev eventPayload, subscriptionID uuid.UUID) json.RawMessage {
	bundle := map[string]interface{}{
		"resourceType": "Bundle",
		"type":         "subscription-notification",
		"timestamp":    ev.Timestamp.Format(time.RFC3339),
		"entry": []map[string]interface{}{
			{
				"resource": map[string]interface{}{
					"resourceType": "SubscriptionStatus",
					"status":       "active",
					"type":         "event-notification",
					"subscription": map[string]string{
						"reference": "Subscription/" + subscriptionID.String(),
					},
					"notificationEvent": []map[string]interface{}{
						{
							"eventNumber": 1,
							"timestamp":   ev.Timestamp.Format(time.RFC3339),
							"focus": map[string]string{
								"reference": fmt.Sprintf("%s/%s", ev.ResourceType, ev.ResourceID),
							},
						},
					},
				},
			},
			{
				"resource": ev.Payload,
			},
		},
	}

	data, err := json.Marshal(bundle)
	if err != nil {
		// Fallback to the raw payload if bundle construction fails.
		log.Printf("subscription: build bundle error: %v", err)
		return ev.Payload
	}
	return data
}
