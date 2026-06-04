// Package paymark implements payment.Provider for Paymark Click.
// Paymark is the legacy NZ switching network; Paymark Click is their online
// payment gateway used by DHB-funded services and older NZ integrations.
// API docs: https://developer.paymark.nz
package paymark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/payment"
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

func init() {
	payment.Register("paymark", func(ctx context.Context, v *viper.Viper) (payment.Provider, error) {
		return New(ctx, Config{
			MerchantID:    v.GetString("payment.paymark.merchant_id"),
			APIKey:        v.GetString("payment.paymark.api_key"),
			BaseURL:       v.GetString("payment.paymark.base_url"),
			ReturnURL:     v.GetString("payment.paymark.return_url"),
		})
	})
}

const defaultBaseURL = "https://api.paymark.nz/v1"

// Config holds Paymark Click credentials.
type Config struct {
	MerchantID string
	APIKey     string
	BaseURL    string
	ReturnURL  string
}

// Provider implements payment.Provider for Paymark Click.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.MerchantID == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("paymark: merchant_id and api_key are required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "paymark"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("paymark marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "paymark", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("paymark request: %w", err)
		}
		req.Header.Set("X-Merchant-ID", p.cfg.MerchantID)
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("paymark http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("paymark read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &payment.Error{Provider: "paymark", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) CreatePaymentRequest(ctx context.Context, req payment.PaymentRequest) (*payment.PaymentIntent, error) {
	returnURL := p.cfg.ReturnURL
	if req.ReturnURL != "" {
		returnURL = req.ReturnURL
	}
	payload := map[string]any{
		"amount":    req.AmountCents,
		"currency":  "NZD",
		"reference": req.InvoiceID,
		"returnUrl": returnURL,
	}
	raw, err := p.post(ctx, "/transactions", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		TransactionID string `json:"transactionId"`
		PaymentURL    string `json:"paymentUrl"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("paymark create decode: %w", err)
	}
	return &payment.PaymentIntent{ExternalID: resp.TransactionID, RedirectURL: resp.PaymentURL}, nil
}

func (p *Provider) CapturePayment(ctx context.Context, intentID string) (*payment.PaymentResult, error) {
	var result []byte
	err := resilience.Do(ctx, p.breaker, "paymark", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/transactions/"+intentID, nil)
		if err != nil {
			return err
		}
		req.Header.Set("X-Merchant-ID", p.cfg.MerchantID)
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
		resp, err := p.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		return err
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		TransactionID string `json:"transactionId"`
		Status        string `json:"status"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("paymark capture decode: %w", err)
	}
	status := payment.PaymentFailed
	if resp.Status == "approved" || resp.Status == "completed" {
		status = payment.PaymentSucceeded
	}
	return &payment.PaymentResult{ExternalID: resp.TransactionID, Status: status, PaidAt: time.Now().UTC()}, nil
}

func (p *Provider) Refund(ctx context.Context, intentID string, amountCents int64) (*payment.RefundResult, error) {
	payload := map[string]any{"transactionId": intentID}
	if amountCents > 0 {
		payload["amount"] = amountCents
	}
	raw, err := p.post(ctx, "/refunds", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		RefundID string `json:"refundId"`
	}
	_ = json.Unmarshal(raw, &resp)
	return &payment.RefundResult{ExternalID: resp.RefundID, AmountCents: amountCents, RefundedAt: time.Now().UTC()}, nil
}

func (p *Provider) HandleWebhook(_ context.Context, payload []byte, _ string) (*payment.WebhookEvent, error) {
	var evt struct {
		TransactionID string `json:"transactionId"`
		Status        string `json:"status"`
		Amount        int64  `json:"amount"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("paymark webhook decode: %w", err)
	}
	evtType := payment.WebhookPaymentFailed
	if evt.Status == "approved" || evt.Status == "completed" {
		evtType = payment.WebhookPaymentSucceeded
	}
	return &payment.WebhookEvent{Type: evtType, ExternalID: evt.TransactionID, AmountCents: evt.Amount, OccurredAt: time.Now().UTC()}, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*payment.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/health", nil)
	if err != nil {
		return &payment.HealthStatus{OK: false, Provider: "paymark", Err: err.Error()}, nil
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &payment.HealthStatus{OK: false, Provider: "paymark", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &payment.HealthStatus{OK: true, Provider: "paymark", Latency: latency}, nil
}
