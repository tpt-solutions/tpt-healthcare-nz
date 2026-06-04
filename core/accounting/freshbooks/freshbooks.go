// Package freshbooks implements accounting.Provider for the FreshBooks API v1.
// OAuth2; account_id identifies the FreshBooks account.
// Sandbox: https://www.freshbooks.com/api/start
package freshbooks

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
	accounting.Register("freshbooks", func(ctx context.Context, v *viper.Viper) (accounting.Provider, error) {
		return New(ctx, Config{
			ClientID:     v.GetString("accounting.freshbooks.client_id"),
			ClientSecret: v.GetString("accounting.freshbooks.client_secret"),
			AccountID:    v.GetString("accounting.freshbooks.account_id"),
		})
	})
}

const baseURL = "https://api.freshbooks.com"

// Config holds FreshBooks credentials.
type Config struct {
	ClientID     string
	ClientSecret string
	AccountID    string
}

// Provider implements accounting.Provider for FreshBooks.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("freshbooks: client_id and client_secret are required")
	}
	if cfg.AccountID == "" {
		return nil, fmt.Errorf("freshbooks: account_id is required")
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "freshbooks"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("freshbooks marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	var result []byte
	err := resilience.Do(ctx, p.breaker, "freshbooks", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("freshbooks request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Api-Version", "alpha")
		// TODO: attach OAuth2 bearer token.

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("freshbooks http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("freshbooks read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &accounting.Error{
				Provider:  "freshbooks",
				Code:      fmt.Sprintf("HTTP%d", resp.StatusCode),
				Message:   string(result),
				Retryable: resp.StatusCode >= 500,
			}
		}
		return nil
	})
	return result, err
}

func (p *Provider) accountPath(sub string) string {
	return fmt.Sprintf("/accounting/account/%s%s", p.cfg.AccountID, sub)
}

func (p *Provider) SyncInvoice(ctx context.Context, inv billing.Invoice, contactExternalID string) (*accounting.SyncInvoiceResult, error) {
	lines := make([]map[string]any, len(inv.Lines))
	for i, l := range inv.Lines {
		lines[i] = map[string]any{
			"name":         l.Service.Description,
			"qty":          l.Quantity,
			"unit_cost":    map[string]any{"amount": fmt.Sprintf("%.2f", float64(l.Service.UnitFee)/100), "code": "NZD"},
		}
	}
	payload := map[string]any{
		"invoice": map[string]any{
			"customerid": contactExternalID,
			"po_number":  inv.ID.String()[:8],
			"lines":      lines,
		},
	}
	raw, err := p.do(ctx, http.MethodPost, p.accountPath("/invoices/invoices"), payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Response struct {
			Result struct {
				Invoice struct {
					ID int64 `json:"id"`
				} `json:"invoice"`
			} `json:"result"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("freshbooks invoice decode: %w", err)
	}
	return &accounting.SyncInvoiceResult{ExternalID: fmt.Sprint(resp.Response.Result.Invoice.ID)}, nil
}

func (p *Provider) RecordPayment(ctx context.Context, payment accounting.Payment) error {
	payload := map[string]any{
		"payment": map[string]any{
			"invoiceid": payment.ExternalInvoiceID,
			"amount":    map[string]any{"amount": fmt.Sprintf("%.2f", float64(payment.AmountCents)/100), "code": "NZD"},
			"date":      payment.PaidAt.Format("2006-01-02"),
			"note":      payment.Reference,
		},
	}
	_, err := p.do(ctx, http.MethodPost, p.accountPath("/payments/payments"), payload)
	return err
}

func (p *Provider) SyncContact(ctx context.Context, contact accounting.Contact) (*accounting.SyncContactResult, error) {
	payload := map[string]any{
		"client": map[string]any{
			"fname": contact.Name,
			"email": contact.Email,
		},
	}
	method := http.MethodPost
	path := p.accountPath("/clients/clients")
	if contact.ExternalID != "" {
		method = http.MethodPut
		path = p.accountPath("/clients/clients/" + contact.ExternalID)
	}
	raw, err := p.do(ctx, method, path, payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Response struct {
			Result struct {
				Client struct {
					ID int64 `json:"id"`
				} `json:"client"`
			} `json:"result"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("freshbooks contact decode: %w", err)
	}
	return &accounting.SyncContactResult{
		ExternalID: fmt.Sprint(resp.Response.Result.Client.ID),
		Created:    contact.ExternalID == "",
	}, nil
}

func (p *Provider) OutstandingBalance(ctx context.Context, externalContactID string) (int64, error) {
	raw, err := p.do(ctx, http.MethodGet, p.accountPath("/clients/clients/"+externalContactID), nil)
	if err != nil {
		return 0, err
	}
	var resp struct {
		Response struct {
			Result struct {
				Client struct {
					OutstandingBalance []struct {
						Amount struct {
							Amount string `json:"amount"`
						} `json:"amount"`
					} `json:"outstanding_balance"`
				} `json:"client"`
			} `json:"result"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("freshbooks balance decode: %w", err)
	}
	if len(resp.Response.Result.Client.OutstandingBalance) == 0 {
		return 0, nil
	}
	var cents float64
	fmt.Sscanf(resp.Response.Result.Client.OutstandingBalance[0].Amount.Amount, "%f", &cents)
	return int64(cents * 100), nil
}

func (p *Provider) PostJournalEntry(ctx context.Context, entry accounting.JournalEntry) (string, error) {
	// FreshBooks does not natively expose a journal entry API in its v1 REST API.
	// Journal entries should be handled via Xero or QBO for clients needing this feature.
	return "", &accounting.Error{
		Provider: "freshbooks",
		Code:     "NOT_SUPPORTED",
		Message:  "manual journal entries are not supported by the FreshBooks API; use Xero or QBO for GL journals",
	}
}

func (p *Provider) HealthCheck(ctx context.Context) (*accounting.HealthStatus, error) {
	start := time.Now()
	raw, err := p.do(ctx, http.MethodGet, "/auth/api/v1/users/me", nil)
	latency := time.Since(start)
	if err != nil {
		return &accounting.HealthStatus{OK: false, Provider: "freshbooks", Latency: latency, Err: err.Error()}, nil
	}
	var resp struct {
		Response struct {
			BusinessMemberships []struct {
				Business struct {
					Name string `json:"name"`
				} `json:"business"`
			} `json:"business_memberships"`
		} `json:"response"`
	}
	_ = json.Unmarshal(raw, &resp)
	name := ""
	if len(resp.Response.BusinessMemberships) > 0 {
		name = resp.Response.BusinessMemberships[0].Business.Name
	}
	return &accounting.HealthStatus{OK: true, Provider: "freshbooks", OrganisationName: name, Latency: latency}, nil
}
