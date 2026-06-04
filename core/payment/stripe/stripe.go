// Package stripe implements payment.Provider for Stripe (NZ entity).
// Used for patient portal online payments. Test mode available via test API keys.
// API docs: https://stripe.com/docs/api
package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/payment"
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

func init() {
	payment.Register("stripe", func(ctx context.Context, v *viper.Viper) (payment.Provider, error) {
		return New(ctx, Config{
			SecretKey:       v.GetString("payment.stripe.secret_key"),
			WebhookSecret:   v.GetString("payment.stripe.webhook_secret"),
		})
	})
}

const baseURL = "https://api.stripe.com/v1"

// Config holds Stripe credentials.
type Config struct {
	SecretKey     string
	WebhookSecret string
}

// Provider implements payment.Provider for Stripe.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.SecretKey == "" {
		return nil, fmt.Errorf("stripe: secret_key is required")
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "stripe"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, path string, form url.Values) ([]byte, error) {
	var result []byte
	err := resilience.Do(ctx, p.breaker, "stripe", resilience.RetryConfig{MaxAttempts: 3, IsRetryable: func(err error) bool {
		if pe, ok := err.(*payment.Error); ok {
			return pe.Retryable
		}
		return true
	}}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, strings.NewReader(form.Encode()))
		if err != nil {
			return fmt.Errorf("stripe request: %w", err)
		}
		req.SetBasicAuth(p.cfg.SecretKey, "")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Stripe-Version", "2024-06-20")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("stripe http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("stripe read: %w", err)
		}
		if resp.StatusCode >= 400 {
			var errResp struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal(result, &errResp)
			return &payment.Error{
				Provider:  "stripe",
				Code:      errResp.Error.Code,
				Message:   errResp.Error.Message,
				Retryable: resp.StatusCode >= 500 || resp.StatusCode == 429,
			}
		}
		return nil
	})
	return result, err
}

func (p *Provider) CreatePaymentRequest(ctx context.Context, req payment.PaymentRequest) (*payment.PaymentIntent, error) {
	form := url.Values{
		"amount":      {strconv.FormatInt(req.AmountCents, 10)},
		"currency":    {"nzd"},
		"description": {req.Description},
		"metadata[invoice_id]": {req.InvoiceID},
		"metadata[patient_nhi]": {req.PatientNHI},
	}
	raw, err := p.post(ctx, "/payment_intents", form)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ID           string `json:"id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("stripe intent decode: %w", err)
	}
	return &payment.PaymentIntent{ExternalID: resp.ID}, nil
}

func (p *Provider) CapturePayment(ctx context.Context, intentID string) (*payment.PaymentResult, error) {
	var result []byte
	err := resilience.Do(ctx, p.breaker, "stripe", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/payment_intents/"+intentID, nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth(p.cfg.SecretKey, "")
		req.Header.Set("Stripe-Version", "2024-06-20")
		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("stripe capture http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		return err
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("stripe capture decode: %w", err)
	}
	status := payment.PaymentFailed
	if resp.Status == "succeeded" {
		status = payment.PaymentSucceeded
	}
	return &payment.PaymentResult{ExternalID: resp.ID, Status: status, PaidAt: time.Now().UTC()}, nil
}

func (p *Provider) Refund(ctx context.Context, intentID string, amountCents int64) (*payment.RefundResult, error) {
	form := url.Values{"payment_intent": {intentID}}
	if amountCents > 0 {
		form.Set("amount", strconv.FormatInt(amountCents, 10))
	}
	raw, err := p.post(ctx, "/refunds", form)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ID     string `json:"id"`
		Amount int64  `json:"amount"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("stripe refund decode: %w", err)
	}
	return &payment.RefundResult{ExternalID: resp.ID, AmountCents: resp.Amount, RefundedAt: time.Now().UTC()}, nil
}

func (p *Provider) HandleWebhook(_ context.Context, payload []byte, signature string) (*payment.WebhookEvent, error) {
	// Stripe-Signature header: t=<timestamp>,v1=<sig>,v1=<sig2>,...
	parts := strings.Split(signature, ",")
	var ts, sig string
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			ts = kv[1]
		case "v1":
			if sig == "" {
				sig = kv[1]
			}
		}
	}
	mac := hmac.New(sha256.New, []byte(p.cfg.WebhookSecret))
	mac.Write([]byte(ts + "." + string(payload)))
	expected := fmt.Sprintf("%x", mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return nil, fmt.Errorf("stripe: webhook signature invalid")
	}

	var evt struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID     string `json:"id"`
				Amount int64  `json:"amount"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("stripe webhook decode: %w", err)
	}

	evtType := payment.WebhookPaymentSucceeded
	switch evt.Type {
	case "payment_intent.payment_failed":
		evtType = payment.WebhookPaymentFailed
	case "charge.refunded":
		evtType = payment.WebhookRefundCompleted
	}

	return &payment.WebhookEvent{
		Type:        evtType,
		ExternalID:  evt.Data.Object.ID,
		AmountCents: evt.Data.Object.Amount,
		OccurredAt:  time.Now().UTC(),
	}, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*payment.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/balance", nil)
	if err != nil {
		return &payment.HealthStatus{OK: false, Provider: "stripe", Err: err.Error()}, nil
	}
	req.SetBasicAuth(p.cfg.SecretKey, "")
	req.Header.Set("Stripe-Version", "2024-06-20")
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &payment.HealthStatus{OK: false, Provider: "stripe", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &payment.HealthStatus{OK: true, Provider: "stripe", Latency: latency}, nil
}
