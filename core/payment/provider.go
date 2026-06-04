// Package payment defines the PaymentProvider interface for collecting
// patient co-payments, private fees, and EFTPOS transactions.
// All amounts are NZD cents (int64) throughout.
package payment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// PaymentStatus classifies the outcome of a payment attempt.
type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentSucceeded PaymentStatus = "succeeded"
	PaymentFailed    PaymentStatus = "failed"
	PaymentCancelled PaymentStatus = "cancelled"
	PaymentRefunded  PaymentStatus = "refunded"
)

// PaymentRequest describes the payment to initiate.
type PaymentRequest struct {
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"` // always "NZD"
	InvoiceID   string `json:"invoice_id"`
	PatientNHI  string `json:"patient_nhi,omitempty"`
	Description string `json:"description"`
	// ReturnURL is used by hosted-page providers (Windcave, Paymark) to
	// redirect the patient after payment completion.
	ReturnURL string `json:"return_url,omitempty"`
}

// PaymentIntent is returned by CreatePaymentRequest.
type PaymentIntent struct {
	ExternalID  string `json:"external_id"`
	// RedirectURL is the provider-hosted payment page for the patient to visit.
	// Empty for server-side (non-redirect) integrations.
	RedirectURL string `json:"redirect_url,omitempty"`
}

// PaymentResult is returned by CapturePayment.
type PaymentResult struct {
	ExternalID  string        `json:"external_id"`
	Status      PaymentStatus `json:"status"`
	PaidAt      time.Time     `json:"paid_at"`
	Reference   string        `json:"reference,omitempty"`
	ReceiptURL  string        `json:"receipt_url,omitempty"`
}

// RefundResult is returned by Refund.
type RefundResult struct {
	ExternalID  string    `json:"external_id"`
	AmountCents int64     `json:"amount_cents"`
	RefundedAt  time.Time `json:"refunded_at"`
}

// WebhookEventType classifies incoming webhook events.
type WebhookEventType string

const (
	WebhookPaymentSucceeded WebhookEventType = "payment.succeeded"
	WebhookPaymentFailed    WebhookEventType = "payment.failed"
	WebhookRefundCompleted  WebhookEventType = "refund.completed"
)

// WebhookEvent is the normalised event delivered by a provider webhook.
type WebhookEvent struct {
	Type       WebhookEventType `json:"type"`
	ExternalID string           `json:"external_id"`
	AmountCents int64           `json:"amount_cents"`
	Reference  string           `json:"reference,omitempty"`
	OccurredAt time.Time        `json:"occurred_at"`
}

// HealthStatus reports connectivity to the payment provider.
type HealthStatus struct {
	OK       bool          `json:"ok"`
	Provider string        `json:"provider"`
	Latency  time.Duration `json:"latency_ms"`
	Err      string        `json:"error,omitempty"`
}

// Error is a payment provider application-level error.
type Error struct {
	Provider  string
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("payment(%s): %s — %s", e.Provider, e.Code, e.Message)
}

// Provider is the interface all payment backends must implement.
type Provider interface {
	// CreatePaymentRequest initiates a payment and returns a PaymentIntent.
	// For redirect-based providers, the patient is sent to RedirectURL.
	CreatePaymentRequest(ctx context.Context, req PaymentRequest) (*PaymentIntent, error)

	// CapturePayment confirms and captures a previously created payment intent.
	// For redirect providers this is called after the patient returns from RedirectURL.
	CapturePayment(ctx context.Context, intentID string) (*PaymentResult, error)

	// Refund issues a full or partial refund for a captured payment.
	// amountCents == 0 means full refund.
	Refund(ctx context.Context, intentID string, amountCents int64) (*RefundResult, error)

	// HandleWebhook parses and verifies an inbound provider webhook notification.
	// payload is the raw request body; signature is the provider's HMAC header value.
	HandleWebhook(ctx context.Context, payload []byte, signature string) (*WebhookEvent, error)

	// HealthCheck verifies connectivity and authentication.
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
		panic(fmt.Sprintf("payment: provider %q already registered", name))
	}
	registry[name] = factory
}

// New instantiates the provider named by viper key "payment.provider".
func New(ctx context.Context, v *viper.Viper) (Provider, error) {
	name := v.GetString("payment.provider")
	if name == "" {
		return nil, errors.New("payment: payment.provider config key is not set")
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
var ErrUnknownProvider = errors.New("payment: unknown provider")
