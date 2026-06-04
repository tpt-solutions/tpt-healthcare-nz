// Package email defines the EmailProvider interface for transactional email
// delivery: appointment confirmations, breach notifications, subscription
// dispatch, and lab result notifications.
package email

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Attachment is a file attached to an outbound email.
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
}

// Message is an outbound email.
type Message struct {
	To          []string     `json:"to"`
	CC          []string     `json:"cc,omitempty"`
	From        string       `json:"from"`
	ReplyTo     string       `json:"reply_to,omitempty"`
	Subject     string       `json:"subject"`
	HTMLBody    string       `json:"html_body"`
	TextBody    string       `json:"text_body,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	// TemplateID is an optional provider-hosted template identifier.
	// When set, the provider renders the template with TemplateData.
	TemplateID   string         `json:"template_id,omitempty"`
	TemplateData map[string]any `json:"template_data,omitempty"`
}

// SendResult is returned by Send.
type SendResult struct {
	ExternalID string    `json:"external_id"`
	QueuedAt   time.Time `json:"queued_at"`
}

// HealthStatus reports connectivity to the email provider.
type HealthStatus struct {
	OK       bool          `json:"ok"`
	Provider string        `json:"provider"`
	Latency  time.Duration `json:"latency_ms"`
	Err      string        `json:"error,omitempty"`
}

// Error is an email provider application-level error.
type Error struct {
	Provider  string
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("email(%s): %s — %s", e.Provider, e.Code, e.Message)
}

// Provider is the interface all email backends must implement.
type Provider interface {
	Send(ctx context.Context, msg Message) (*SendResult, error)
	SendBulk(ctx context.Context, messages []Message) ([]SendResult, error)
	HealthCheck(ctx context.Context) (*HealthStatus, error)
}

// Factory is a constructor registered by each backend.
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
		panic(fmt.Sprintf("email: provider %q already registered", name))
	}
	registry[name] = factory
}

// New instantiates the provider named by viper key "email.provider".
func New(ctx context.Context, v *viper.Viper) (Provider, error) {
	name := v.GetString("email.provider")
	if name == "" {
		return nil, errors.New("email: email.provider config key is not set")
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
var ErrUnknownProvider = errors.New("email: unknown provider")
