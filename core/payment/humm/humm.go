// Package humm implements payment.Provider for Humm (formerly Flexigroup).
// Humm offers Buy Now Pay Later (BNPL) specifically targeting healthcare in
// AU/NZ under the "humm health" product — used for dental, optical, cosmetic
// and medical procedures where patients need payment plans.
// API docs: https://docs.shophumm.com.au/api/
package humm

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
	payment.Register("humm", func(ctx context.Context, v *viper.Viper) (payment.Provider, error) {
		return New(ctx, Config{
			MerchantID:  v.GetString("payment.humm.merchant_id"),
			APIKey:      v.GetString("payment.humm.api_key"),
			BaseURL:     v.GetString("payment.humm.base_url"),
			ReturnURL:   v.GetString("payment.humm.return_url"),
		})
	})
}

const defaultBaseURL = "https://integration-seller.shophumm.com/api/v1/nz"

// Config holds Humm API credentials.
type Config struct {
	MerchantID string
	APIKey     string
	BaseURL    string
	ReturnURL  string
}

// Provider implements payment.Provider for Humm.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.MerchantID == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("humm: merchant_id and api_key are required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "humm"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("humm marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "humm", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("humm request: %w", err)
		}
		req.Header.Set("x-merchant-id", p.cfg.MerchantID)
		req.Header.Set("x-api-key", p.cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("humm http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("humm read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &payment.Error{Provider: "humm", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
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
		"purchaseAmount":  float64(req.AmountCents) / 100,
		"merchantRef":     req.InvoiceID,
		"successUrl":      returnURL + "?status=success",
		"failUrl":         returnURL + "?status=failed",
		"cancelUrl":       returnURL + "?status=cancelled",
	}
	raw, err := p.post(ctx, "/payment/initiate", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Token      string `json:"token"`
		RedirectUrl string `json:"redirectUrl"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("humm initiate decode: %w", err)
	}
	return &payment.PaymentIntent{ExternalID: resp.Token, RedirectURL: resp.RedirectUrl}, nil
}

func (p *Provider) CapturePayment(ctx context.Context, intentID string) (*payment.PaymentResult, error) {
	raw, err := p.post(ctx, "/payment/status", map[string]any{"token": intentID})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Token  string `json:"token"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("humm capture decode: %w", err)
	}
	status := payment.PaymentFailed
	if resp.Status == "approved" {
		status = payment.PaymentSucceeded
	}
	return &payment.PaymentResult{ExternalID: resp.Token, Status: status, PaidAt: time.Now().UTC()}, nil
}

func (p *Provider) Refund(_ context.Context, _ string, _ int64) (*payment.RefundResult, error) {
	// Humm BNPL refunds are managed via the Humm merchant portal.
	return nil, &payment.Error{
		Provider: "humm",
		Code:     "NOT_SUPPORTED",
		Message:  "Humm BNPL refunds must be processed via the Humm merchant portal",
	}
}

func (p *Provider) HandleWebhook(_ context.Context, payload []byte, _ string) (*payment.WebhookEvent, error) {
	var evt struct {
		Token  string `json:"token"`
		Status string `json:"status"`
		Amount float64 `json:"amount"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("humm webhook decode: %w", err)
	}
	evtType := payment.WebhookPaymentFailed
	if evt.Status == "approved" {
		evtType = payment.WebhookPaymentSucceeded
	}
	return &payment.WebhookEvent{
		Type:        evtType,
		ExternalID:  evt.Token,
		AmountCents: int64(evt.Amount * 100),
		OccurredAt:  time.Now().UTC(),
	}, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*payment.HealthStatus, error) {
	start := time.Now()
	raw, err := p.post(ctx, "/merchant/info", map[string]any{})
	latency := time.Since(start)
	if err != nil {
		return &payment.HealthStatus{OK: false, Provider: "humm", Latency: latency, Err: err.Error()}, nil
	}
	var resp struct {
		MerchantName string `json:"merchantName"`
	}
	_ = json.Unmarshal(raw, &resp)
	return &payment.HealthStatus{OK: true, Provider: "humm", Latency: latency}, nil
}
