// Package eway implements payment.Provider for eWAY (Rapid API).
// eWAY is a popular AU/NZ payment gateway with a transparent pricing model.
// Sandbox endpoint: https://api.sandbox.ewaypayments.com
// API docs: https://eway.io/api-v3/
package eway

import (
	"bytes"
	"context"
	"encoding/base64"
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
	payment.Register("eway", func(ctx context.Context, v *viper.Viper) (payment.Provider, error) {
		return New(ctx, Config{
			APIKey:    v.GetString("payment.eway.api_key"),
			APIPassword: v.GetString("payment.eway.api_password"),
			BaseURL:   v.GetString("payment.eway.base_url"),
			ReturnURL: v.GetString("payment.eway.return_url"),
		})
	})
}

// Config holds eWAY credentials.
type Config struct {
	APIKey      string
	APIPassword string
	BaseURL     string // default: sandbox
	ReturnURL   string
}

// Provider implements payment.Provider for eWAY.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.APIKey == "" || cfg.APIPassword == "" {
		return nil, fmt.Errorf("eway: api_key and api_password are required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.sandbox.ewaypayments.com"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "eway"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) auth() string {
	return base64.StdEncoding.EncodeToString([]byte(p.cfg.APIKey + ":" + p.cfg.APIPassword))
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("eway marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "eway", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("eway request: %w", err)
		}
		req.Header.Set("Authorization", "Basic "+p.auth())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("eway http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("eway read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &payment.Error{Provider: "eway", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
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
		"Payment": map[string]any{
			"TotalAmount":    req.AmountCents,
			"CurrencyCode":   "NZD",
			"InvoiceReference": req.InvoiceID,
			"InvoiceDescription": req.Description,
		},
		"RedirectUrl":   returnURL,
		"TransactionType": "Purchase",
	}
	raw, err := p.post(ctx, "/AccessCodesShared", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		AccessCode  string `json:"AccessCode"`
		SharedPaymentUrl string `json:"SharedPaymentUrl"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("eway create decode: %w", err)
	}
	return &payment.PaymentIntent{ExternalID: resp.AccessCode, RedirectURL: resp.SharedPaymentUrl}, nil
}

func (p *Provider) CapturePayment(ctx context.Context, intentID string) (*payment.PaymentResult, error) {
	raw, err := p.post(ctx, "/AccessCode/"+intentID, nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		AccessCode       string `json:"AccessCode"`
		TransactionStatus bool  `json:"TransactionStatus"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("eway capture decode: %w", err)
	}
	status := payment.PaymentFailed
	if resp.TransactionStatus {
		status = payment.PaymentSucceeded
	}
	return &payment.PaymentResult{ExternalID: resp.AccessCode, Status: status, PaidAt: time.Now().UTC()}, nil
}

func (p *Provider) Refund(ctx context.Context, intentID string, amountCents int64) (*payment.RefundResult, error) {
	payload := map[string]any{
		"Refund": map[string]any{
			"TransactionID": intentID,
			"TotalAmount":   amountCents,
		},
	}
	raw, err := p.post(ctx, "/Transaction/"+intentID+"/Refund", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		TransactionID string `json:"TransactionID"`
	}
	_ = json.Unmarshal(raw, &resp)
	return &payment.RefundResult{ExternalID: resp.TransactionID, AmountCents: amountCents, RefundedAt: time.Now().UTC()}, nil
}

func (p *Provider) HandleWebhook(_ context.Context, payload []byte, _ string) (*payment.WebhookEvent, error) {
	var evt struct {
		Event         string `json:"Event"`
		TransactionID string `json:"TransactionID"`
		Amount        int64  `json:"Amount"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("eway webhook decode: %w", err)
	}
	evtType := payment.WebhookPaymentSucceeded
	if evt.Event == "TransactionFailed" {
		evtType = payment.WebhookPaymentFailed
	}
	return &payment.WebhookEvent{Type: evtType, ExternalID: evt.TransactionID, AmountCents: evt.Amount, OccurredAt: time.Now().UTC()}, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*payment.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/Info", nil)
	if err != nil {
		return &payment.HealthStatus{OK: false, Provider: "eway", Err: err.Error()}, nil
	}
	req.Header.Set("Authorization", "Basic "+p.auth())
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &payment.HealthStatus{OK: false, Provider: "eway", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &payment.HealthStatus{OK: true, Provider: "eway", Latency: latency}, nil
}
