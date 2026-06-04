// Package accounting defines the AccountingProvider interface and supporting
// types. External accounting systems (Xero, QuickBooks Online, FreshBooks)
// implement this interface; the platform calls it for invoice sync, payment
// recording, contact management, journal posting, and balance enquiry.
// All writes are queued via core/outbox for at-least-once delivery.
package accounting

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/billing"
)

// ContactRole classifies a contact as a patient debtor or a supplier creditor.
type ContactRole string

const (
	ContactDebtor   ContactRole = "debtor"
	ContactCreditor ContactRole = "creditor"
)

// Contact is the normalised representation of a person or organisation in an
// external accounting system.
type Contact struct {
	ExternalID  string         `json:"external_id,omitempty"`
	Role        ContactRole    `json:"role"`
	Name        string         `json:"name"`
	Email       string         `json:"email,omitempty"`
	Phone       string         `json:"phone,omitempty"`
	NHI         string         `json:"nhi,omitempty"`
	TaxNumber   string         `json:"tax_number,omitempty"`
	Address     ContactAddress `json:"address,omitempty"`
	UpdatedAt   time.Time      `json:"updated_at,omitempty"`
}

// ContactAddress holds normalised NZ address components.
type ContactAddress struct {
	Line1      string `json:"line1,omitempty"`
	Line2      string `json:"line2,omitempty"`
	Suburb     string `json:"suburb,omitempty"`
	City       string `json:"city,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Country    string `json:"country,omitempty"` // defaults to "NZ"
}

// JournalLine is one debit or credit line within a manual journal entry.
// All amounts are NZD cents. Exactly one of DebitCents/CreditCents is non-zero.
type JournalLine struct {
	AccountCode string `json:"account_code"`
	Description string `json:"description"`
	DebitCents  int64  `json:"debit_cents"`
	CreditCents int64  `json:"credit_cents"`
	TaxType     string `json:"tax_type,omitempty"` // e.g. "OUTPUT2", "NONE"
}

// JournalEntry represents a manual journal to be posted to the external system.
// Lines must balance (sum of debits == sum of credits).
type JournalEntry struct {
	ID          uuid.UUID     `json:"id"`
	TenantID    uuid.UUID     `json:"tenant_id"`
	Date        time.Time     `json:"date"`
	Reference   string        `json:"reference"`
	Lines       []JournalLine `json:"lines"`
	Attachments []string      `json:"attachments,omitempty"`
}

// Payment records a receipt of funds against a previously synced invoice.
type Payment struct {
	ExternalInvoiceID  string    `json:"external_invoice_id"`
	AmountCents        int64     `json:"amount_cents"`
	PaidAt             time.Time `json:"paid_at"`
	Reference          string    `json:"reference,omitempty"`
	PaymentAccountCode string    `json:"payment_account_code,omitempty"`
}

// SyncInvoiceResult is returned by SyncInvoice.
type SyncInvoiceResult struct {
	ExternalID string `json:"external_id"`
	InvoiceURL string `json:"invoice_url,omitempty"`
}

// SyncContactResult is returned by SyncContact.
type SyncContactResult struct {
	ExternalID string `json:"external_id"`
	Created    bool   `json:"created"`
}

// HealthStatus reports the result of a provider connectivity check.
type HealthStatus struct {
	OK               bool          `json:"ok"`
	Provider         string        `json:"provider"`
	OrganisationName string        `json:"organisation_name,omitempty"`
	Latency          time.Duration `json:"latency_ms"`
	Err              string        `json:"error,omitempty"`
}

// Error is returned when the external accounting system responds with an
// application-level error.
type Error struct {
	Provider  string
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("accounting(%s): %s — %s", e.Provider, e.Code, e.Message)
}

// Provider is the interface all accounting backends must implement.
// Context always carries the tenant UUID. All monetary amounts are NZD cents.
type Provider interface {
	SyncInvoice(ctx context.Context, inv billing.Invoice, contactExternalID string) (*SyncInvoiceResult, error)
	RecordPayment(ctx context.Context, payment Payment) error
	SyncContact(ctx context.Context, contact Contact) (*SyncContactResult, error)
	OutstandingBalance(ctx context.Context, externalContactID string) (int64, error)
	PostJournalEntry(ctx context.Context, entry JournalEntry) (externalJournalID string, err error)
	HealthCheck(ctx context.Context) (*HealthStatus, error)
}

// Factory is a constructor function registered by each backend.
type Factory func(ctx context.Context, v *viper.Viper) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Factory)
)

// Register associates name with factory. Called from each backend's init().
// Panics on duplicate registration (matches database/sql behaviour).
func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("accounting: provider %q already registered", name))
	}
	registry[name] = factory
}

// New instantiates the provider named by viper key "accounting.provider".
func New(ctx context.Context, v *viper.Viper) (Provider, error) {
	name := v.GetString("accounting.provider")
	if name == "" {
		return nil, errors.New("accounting: accounting.provider config key is not set")
	}
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, name)
	}
	return factory(ctx, v)
}

// ErrUnknownProvider is returned by New when no registered backend matches.
var ErrUnknownProvider = errors.New("accounting: unknown provider")
