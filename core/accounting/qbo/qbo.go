// Package qbo implements accounting.Provider for QuickBooks Online API v3.
// OAuth2 with realm_id (company ID) per request.
// Sandbox: https://developer.intuit.com/app/developer/qbo/docs/develop/sandboxes
package qbo

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
	accounting.Register("qbo", func(ctx context.Context, v *viper.Viper) (accounting.Provider, error) {
		return New(ctx, Config{
			ClientID:     v.GetString("accounting.qbo.client_id"),
			ClientSecret: v.GetString("accounting.qbo.client_secret"),
			RealmID:      v.GetString("accounting.qbo.realm_id"),
			Sandbox:      v.GetBool("accounting.qbo.sandbox"),
		})
	})
}

// Config holds QuickBooks Online credentials.
type Config struct {
	ClientID     string
	ClientSecret string
	RealmID      string // QuickBooks company ID
	Sandbox      bool
}

// Provider implements accounting.Provider for QuickBooks Online.
type Provider struct {
	cfg     Config
	baseURL string
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("qbo: client_id and client_secret are required")
	}
	if cfg.RealmID == "" {
		return nil, fmt.Errorf("qbo: realm_id (QuickBooks company ID) is required")
	}
	base := "https://quickbooks.api.intuit.com"
	if cfg.Sandbox {
		base = "https://sandbox-quickbooks.api.intuit.com"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "qbo"})
	return &Provider{cfg: cfg, baseURL: base, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) url(path string) string {
	return fmt.Sprintf("%s/v3/company/%s%s?minorversion=70", p.baseURL, p.cfg.RealmID, path)
}

func (p *Provider) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("qbo marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	var result []byte
	err := resilience.Do(ctx, p.breaker, "qbo", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, method, p.url(path), bodyReader)
		if err != nil {
			return fmt.Errorf("qbo request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		// TODO: attach OAuth2 bearer token.

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("qbo http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("qbo read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &accounting.Error{
				Provider:  "qbo",
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
		"CustomerRef": map[string]any{"value": contactExternalID},
		"DocNumber":   inv.ID.String()[:8],
		"Line": billingLinesToQBO(inv.Lines),
	}
	raw, err := p.do(ctx, http.MethodPost, "/invoice", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Invoice struct {
			Id string `json:"Id"`
		} `json:"Invoice"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("qbo invoice decode: %w", err)
	}
	return &accounting.SyncInvoiceResult{ExternalID: resp.Invoice.Id}, nil
}

func (p *Provider) RecordPayment(ctx context.Context, payment accounting.Payment) error {
	payload := map[string]any{
		"TotalAmt": float64(payment.AmountCents) / 100,
		"CustomerRef": map[string]any{"value": payment.ExternalInvoiceID},
		"Line": []map[string]any{{
			"Amount": float64(payment.AmountCents) / 100,
			"LinkedTxn": []map[string]any{{
				"TxnId":   payment.ExternalInvoiceID,
				"TxnType": "Invoice",
			}},
		}},
	}
	_, err := p.do(ctx, http.MethodPost, "/payment", payload)
	return err
}

func (p *Provider) SyncContact(ctx context.Context, contact accounting.Contact) (*accounting.SyncContactResult, error) {
	payload := map[string]any{
		"DisplayName":  contact.Name,
		"PrimaryEmailAddr": map[string]any{"Address": contact.Email},
	}
	if contact.ExternalID != "" {
		payload["Id"] = contact.ExternalID
		payload["SyncToken"] = "0"
	}
	path := "/customer"
	raw, err := p.do(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Customer struct {
			Id string `json:"Id"`
		} `json:"Customer"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("qbo contact decode: %w", err)
	}
	return &accounting.SyncContactResult{ExternalID: resp.Customer.Id, Created: contact.ExternalID == ""}, nil
}

func (p *Provider) OutstandingBalance(ctx context.Context, externalContactID string) (int64, error) {
	raw, err := p.do(ctx, http.MethodGet, "/customer/"+externalContactID, nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Customer struct {
			Balance float64 `json:"Balance"`
		} `json:"Customer"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("qbo balance decode: %w", err)
	}
	return int64(resp.Customer.Balance * 100), nil
}

func (p *Provider) PostJournalEntry(ctx context.Context, entry accounting.JournalEntry) (string, error) {
	lines := make([]map[string]any, len(entry.Lines))
	for i, l := range entry.Lines {
		postingType := "Debit"
		amount := float64(l.DebitCents) / 100
		if l.CreditCents > 0 {
			postingType = "Credit"
			amount = float64(l.CreditCents) / 100
		}
		lines[i] = map[string]any{
			"JournalEntryLineDetail": map[string]any{
				"AccountRef":  map[string]any{"value": l.AccountCode},
				"PostingType": postingType,
			},
			"Amount":      amount,
			"Description": l.Description,
		}
	}
	payload := map[string]any{
		"TxnDate":   entry.Date.Format("2006-01-02"),
		"PrivateNote": entry.Reference,
		"Line":      lines,
	}
	raw, err := p.do(ctx, http.MethodPost, "/journalentry", payload)
	if err != nil {
		return "", err
	}
	var resp struct {
		JournalEntry struct {
			Id string `json:"Id"`
		} `json:"JournalEntry"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("qbo journal decode: %w", err)
	}
	return resp.JournalEntry.Id, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*accounting.HealthStatus, error) {
	start := time.Now()
	raw, err := p.do(ctx, http.MethodGet, "/companyinfo/"+p.cfg.RealmID, nil)
	latency := time.Since(start)
	if err != nil {
		return &accounting.HealthStatus{OK: false, Provider: "qbo", Latency: latency, Err: err.Error()}, nil
	}
	var resp struct {
		CompanyInfo struct {
			CompanyName string `json:"CompanyName"`
		} `json:"CompanyInfo"`
	}
	_ = json.Unmarshal(raw, &resp)
	return &accounting.HealthStatus{OK: true, Provider: "qbo", OrganisationName: resp.CompanyInfo.CompanyName, Latency: latency}, nil
}

func billingLinesToQBO(lines []billing.BillingLine) []map[string]any {
	result := make([]map[string]any, len(lines))
	for i, l := range lines {
		result[i] = map[string]any{
			"DetailType": "SalesItemLineDetail",
			"Amount":     float64(l.ServiceCode.UnitFee*int64(l.Quantity)) / 100,
			"SalesItemLineDetail": map[string]any{
				"Qty":       l.Quantity,
				"UnitPrice": float64(l.ServiceCode.UnitFee) / 100,
			},
			"Description": l.ServiceCode.Description,
		}
	}
	return result
}
