// Package fax defines the FaxProvider interface for structured document
// delivery in NZ healthcare. Healthlink EDI is the primary channel for
// referrals and lab results; eFax is the fallback for non-Healthlink recipients.
package fax

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// SendResult is returned by Send.
type SendResult struct {
	ExternalID string    `json:"external_id"`
	Status     string    `json:"status"`
	QueuedAt   time.Time `json:"queued_at"`
}

// FaxStatus is the current delivery state of a sent document.
type FaxStatus struct {
	ExternalID  string     `json:"external_id"`
	Delivered   bool       `json:"delivered"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	FailureReason string   `json:"failure_reason,omitempty"`
}

// HealthStatus reports connectivity to the fax provider.
type HealthStatus struct {
	OK       bool          `json:"ok"`
	Provider string        `json:"provider"`
	Latency  time.Duration `json:"latency_ms"`
	Err      string        `json:"error,omitempty"`
}

// Error is a fax provider application-level error.
type Error struct {
	Provider  string
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("fax(%s): %s — %s", e.Provider, e.Code, e.Message)
}

// Provider is the interface all fax/secure-messaging backends must implement.
type Provider interface {
	// Send delivers document to recipient. document is the raw file bytes;
	// contentType should be "application/pdf" or "text/hl7-v2".
	Send(ctx context.Context, to, subject string, document []byte, contentType string) (*SendResult, error)
	GetStatus(ctx context.Context, externalID string) (*FaxStatus, error)
	HealthCheck(ctx context.Context) (*HealthStatus, error)
}

// Factory is a constructor registered by each backend.
type Factory func(ctx context.Context, v *viper.Viper) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Factory)
)

// Register associates name with factory.
func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("fax: provider %q already registered", name))
	}
	registry[name] = factory
}

// New instantiates the provider named by viper key "fax.provider".
func New(ctx context.Context, v *viper.Viper) (Provider, error) {
	name := v.GetString("fax.provider")
	if name == "" {
		return nil, errors.New("fax: fax.provider config key is not set")
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
var ErrUnknownProvider = errors.New("fax: unknown provider")
