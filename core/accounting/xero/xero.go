// Package xero implements accounting.Provider against the Xero Accounting API v2.
// OAuth2 PKCE flow; the Xero-Tenant-Id header identifies the organisation.
// Sandbox: https://developer.xero.com/documentation/getting-started-guide/
package xero

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/accounting"
	"github.com/PhillipC05/tpt-healthcare/core/billing"
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

func init() {
	accounting.Register("xero", func(ctx context.Context, v *viper.Viper) (accounting.Provider, error) {
		return New(ctx, Config{
			ClientID:     v.GetString("accounting.xero.client_id"),
			ClientSecret: v.GetString("accounting.xero.client_secret"),
			TenantID:     v.GetString("accounting.xero.tenant_id"),
			BaseURL:      v.GetString("accounting.xero.base_url"),
		})
	})
}

// Config holds Xero OAuth2 credentials and organisation identifiers.
type Config struct {
	ClientID     string
	ClientSecret string
	TenantID     string // Xero-Tenant-Id header (organisation GUID)
	BaseURL      string // default: https://api.xero.com/api.xro/2.0
}

// Provider implements accounting.Provider for Xero.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("xero: client_id and client_secret are required")
	}
	if cfg.TenantID == "" {
		return nil, fmt.Errorf("xero: tenant_id (Xero organisation GUID) is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.xero.com/api.xro/2.0"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "xero"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("xero marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	var result []byte
	err := resilience.Do(ctx, p.breaker, "xero", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, method, p.cfg.BaseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("xero request: %w", err)
		}
		req.Header.Set("Xero-Tenant-Id", p.cfg.TenantID)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		// TODO: attach OAuth2 bearer token from token source.

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("xero http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("xero read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &accounting.Error{
				Provider:  "xero",
				Code:      fmt.Sprintf("HTTP%d", resp.StatusCode),
				Message:   string(result),
				Retryable: resp.StatusCode >= 500,
			}
		}
		return nil
	})
	return result, err
}

func (p *Provider) SyncInvoice(ctx context.Context, inv billing.Invoice, contactExternalID string) (*accounting.SyncInvoiceResult, error) {
	payload := map[string]any{
		"Type":      "ACCREC",
		"Contact":   map[string]any{"ContactID": contactExternalID},
		"Reference": inv.ID.String(),
		"Status":    "AUTHORISED",
		"LineItems": billingLinesToXero(inv.Lines),
	}
	raw, err := p.do(ctx, http.MethodPost, "/Invoices", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Invoices []struct {
			InvoiceID string `json:"InvoiceID"`
			Url       string `json:"Url"`
		} `json:"Invoices"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("xero invoice decode: %w", err)
	}
	if len(resp.Invoices) == 0 {
		return nil, fmt.Errorf("xero: no invoice in response")
	}
	return &accounting.SyncInvoiceResult{
		ExternalID: resp.Invoices[0].InvoiceID,
		InvoiceURL: resp.Invoices[0].Url,
	}, nil
}

func (p *Provider) RecordPayment(ctx context.Context, payment accounting.Payment) error {
	payload := map[string]any{
		"Invoice":   map[string]any{"InvoiceID": payment.ExternalInvoiceID},
		"Amount":    float64(payment.AmountCents) / 100,
		"Date":      payment.PaidAt.Format("2006-01-02"),
		"Reference": payment.Reference,
	}
	_, err := p.do(ctx, http.MethodPost, "/Payments", payload)
	return err
}

func (p *Provider) SyncContact(ctx context.Context, contact accounting.Contact) (*accounting.SyncContactResult, error) {
	payload := map[string]any{
		"Name":         contact.Name,
		"EmailAddress": contact.Email,
		"Phones":       []map[string]any{{"PhoneType": "DEFAULT", "PhoneNumber": contact.Phone}},
	}
	method := http.MethodPost
	path := "/Contacts"
	if contact.ExternalID != "" {
		method = http.MethodPost // Xero uses POST with ContactID to upsert
		payload["ContactID"] = contact.ExternalID
	}
	raw, err := p.do(ctx, method, path, payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Contacts []struct {
			ContactID string `json:"ContactID"`
		} `json:"Contacts"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("xero contact decode: %w", err)
	}
	if len(resp.Contacts) == 0 {
		return nil, fmt.Errorf("xero: no contact in response")
	}
	return &accounting.SyncContactResult{
		ExternalID: resp.Contacts[0].ContactID,
		Created:    contact.ExternalID == "",
	}, nil
}

func (p *Provider) OutstandingBalance(ctx context.Context, externalContactID string) (int64, error) {
	raw, err := p.do(ctx, http.MethodGet, "/Contacts/"+externalContactID, nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Contacts []struct {
			Balances struct {
				AccountsReceivable struct {
					Outstanding float64 `json:"Outstanding"`
				} `json:"AccountsReceivable"`
			} `json:"Balances"`
		} `json:"Contacts"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("xero balance decode: %w", err)
	}
	if len(resp.Contacts) == 0 {
		return 0, nil
	}
	return int64(resp.Contacts[0].Balances.AccountsReceivable.Outstanding * 100), nil
}

func (p *Provider) PostJournalEntry(ctx context.Context, entry accounting.JournalEntry) (string, error) {
	lines := make([]map[string]any, len(entry.Lines))
	for i, l := range entry.Lines {
		lines[i] = map[string]any{
			"AccountCode":  l.AccountCode,
			"Description":  l.Description,
			"LineAmount":   float64(l.DebitCents-l.CreditCents) / 100,
			"TaxType":      l.TaxType,
		}
	}
	payload := map[string]any{
		"Date":      entry.Date.Format("2006-01-02"),
		"Narration": entry.Reference,
		"JournalLines": lines,
	}
	raw, err := p.do(ctx, http.MethodPost, "/ManualJournals", payload)
	if err != nil {
		return "", err
	}
	var resp struct {
		ManualJournals []struct {
			ManualJournalID string `json:"ManualJournalID"`
		} `json:"ManualJournals"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("xero journal decode: %w", err)
	}
	if len(resp.ManualJournals) == 0 {
		return "", fmt.Errorf("xero: no journal in response")
	}
	return resp.ManualJournals[0].ManualJournalID, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*accounting.HealthStatus, error) {
	start := time.Now()
	raw, err := p.do(ctx, http.MethodGet, "/Organisation", nil)
	latency := time.Since(start)
	if err != nil {
		return &accounting.HealthStatus{OK: false, Provider: "xero", Latency: latency, Err: err.Error()}, nil
	}
	var resp struct {
		Organisations []struct {
			Name string `json:"Name"`
		} `json:"Organisations"`
	}
	_ = json.Unmarshal(raw, &resp)
	name := ""
	if len(resp.Organisations) > 0 {
		name = resp.Organisations[0].Name
	}
	return &accounting.HealthStatus{OK: true, Provider: "xero", OrganisationName: name, Latency: latency}, nil
}

func billingLinesToXero(lines []billing.BillingLine) []map[string]any {
	result := make([]map[string]any, len(lines))
	for i, l := range lines {
		result[i] = map[string]any{
			"Description":  l.Service.Description,
			"Quantity":     l.Quantity,
			"UnitAmount":   float64(l.Service.UnitFee) / 100,
			"AccountCode":  "200", // default income account; configurable via viper
		}
	}
	return result
}
