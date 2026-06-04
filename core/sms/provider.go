// Package sms defines the SMSProvider interface for outbound SMS delivery.
// Used for appointment reminders, queue notifications, and cold-chain alerts.
// When the circuit is open the platform falls back to push notifications.
package sms

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Message is an outbound SMS to be delivered.
type Message struct {
	To        string `json:"to"`        // E.164 format, e.g. "+6421234567"
	Body      string `json:"body"`
	Reference string `json:"reference"` // internal reference for idempotency tracking
}

// SendResult is returned by Send and SendBulk for a single message.
type SendResult struct {
	ExternalID string    `json:"external_id"`
	Status     string    `json:"status"`
	QueuedAt   time.Time `json:"queued_at"`
}

// DeliveryStatus is the current delivery state of a previously sent message.
type DeliveryStatus struct {
	ExternalID string    `json:"external_id"`
	Delivered  bool      `json:"delivered"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
}

// HealthStatus reports connectivity to the SMS provider.
type HealthStatus struct {
	OK       bool          `json:"ok"`
	Provider string        `json:"provider"`
	Latency  time.Duration `json:"latency_ms"`
	Err      string        `json:"error,omitempty"`
}

// Error is an SMS provider application-level error.
type Error struct {
	Provider  string
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("sms(%s): %s — %s", e.Provider, e.Code, e.Message)
}

// Provider is the interface all SMS backends must implement.
type Provider interface {
	Send(ctx context.Context, msg Message) (*SendResult, error)
	SendBulk(ctx context.Context, messages []Message) ([]SendResult, error)
	GetDeliveryStatus(ctx context.Context, externalID string) (*DeliveryStatus, error)
	HealthCheck(ctx context.Context) (*HealthStatus, error)
}

// Factory is a constructor function registered by each backend.
type Factory func(ctx context.Context, v *viper.Viper) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Factory)
)

// Register associates name with factory. Called from each backend's init().
func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("sms: provider %q already registered", name))
	}
	registry[name] = factory
}

// New instantiates the provider named by viper key "sms.provider".
func New(ctx context.Context, v *viper.Viper) (Provider, error) {
	name := v.GetString("sms.provider")
	if name == "" {
		return nil, errors.New("sms: sms.provider config key is not set")
	}
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, name)
	}
	return factory(ctx, v)
}

// ErrUnknownProvider is returned when no registered backend matches.
var ErrUnknownProvider = errors.New("sms: unknown provider")
