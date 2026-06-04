// Package windcave implements payment.Provider for Windcave (formerly Payment Express).
// Windcave is the dominant payment gateway in NZ healthcare, supporting EFTPOS,
// Visa, Mastercard, and the DPS-hosted payment page.
// UAT environment: https://uat.windcave.com
// API docs: https://www.windcave.com/developer-documentation
package windcave

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
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
	payment.Register("windcave", func(ctx context.Context, v *viper.Viper) (payment.Provider, error) {
		return New(ctx, Config{
			UserID:    v.GetString("payment.windcave.user_id"),
			APIKey:    v.GetString("payment.windcave.api_key"),
			BaseURL:   v.GetString("payment.windcave.base_url"),
			ReturnURL: v.GetString("payment.windcave.return_url"),
		})
	})
}

// Config holds Windcave REST API credentials.
type Config struct {
	UserID    string
	APIKey    string
	BaseURL   string // UAT: https://uat.windcave.com/api/v1; Production: https://api.windcave.com/v1
	ReturnURL string // default redirect URL after payment completion
}

// Provider implements payment.Provider for Windcave.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.UserID == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("windcave: user_id and api_key are required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://uat.windcave.com/api/v1"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "windcave"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) auth() string {
	return base64.StdEncoding.EncodeToString([]byte(p.cfg.UserID + ":" + p.cfg.APIKey))
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("windcave marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "windcave", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("windcave request: %w", err)
		}
		req.Header.Set("Authorization", "Basic "+p.auth())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("windcave http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("windcave read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &payment.Error{Provider: "windcave", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
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
		"type":     "purchase",
		"amount":   fmt.Sprintf("%.2f", float64(req.AmountCents)/100),
		"currency": "NZD",
		"merchantReference": req.InvoiceID,
		"language": "en",
		"links": []map[string]any{
			{"href": returnURL, "rel": "return", "method": "REDIRECT"},
		},
	}
	raw, err := p.post(ctx, "/sessions", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ID    string `json:"id"`
		Links []struct {
			Href   string `json:"href"`
			Rel    string `json:"rel"`
		} `json:"links"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("windcave session decode: %w", err)
	}
	redirectURL := ""
	for _, l := range resp.Links {
		if l.Rel == "paymentLink" {
			redirectURL = l.Href
		}
	}
	return &payment.PaymentIntent{ExternalID: resp.ID, RedirectURL: redirectURL}, nil
}

func (p *Provider) CapturePayment(ctx context.Context, intentID string) (*payment.PaymentResult, error) {
	var result []byte
	err := resilience.Do(ctx, p.breaker, "windcave", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/sessions/"+intentID, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Basic "+p.auth())
		req.Header.Set("Accept", "application/json")
		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("windcave capture http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		return err
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		ID          string `json:"id"`
		Transactions []struct {
			ID          string `json:"id"`
			Authorised  bool   `json:"authorised"`
			Reco        string `json:"reco"`
			ResponseText string `json:"responseText"`
		} `json:"transactions"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("windcave capture decode: %w", err)
	}
	status := payment.PaymentFailed
	if len(resp.Transactions) > 0 && resp.Transactions[0].Authorised {
		status = payment.PaymentSucceeded
	}
	return &payment.PaymentResult{ExternalID: intentID, Status: status, PaidAt: time.Now().UTC()}, nil
}

func (p *Provider) Refund(ctx context.Context, intentID string, amountCents int64) (*payment.RefundResult, error) {
	payload := map[string]any{
		"sessionId": intentID,
	}
	if amountCents > 0 {
		payload["amount"] = fmt.Sprintf("%.2f", float64(amountCents)/100)
	}
	raw, err := p.post(ctx, "/transactions", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &resp)
	return &payment.RefundResult{ExternalID: resp.ID, AmountCents: amountCents, RefundedAt: time.Now().UTC()}, nil
}

func (p *Provider) HandleWebhook(_ context.Context, payload []byte, signature string) (*payment.WebhookEvent, error) {
	mac := hmac.New(sha256.New, []byte(p.cfg.APIKey))
	mac.Write(payload)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return nil, fmt.Errorf("windcave: webhook signature mismatch")
	}
	var evt struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Amount string `json:"amount"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("windcave webhook decode: %w", err)
	}
	evtType := payment.WebhookPaymentSucceeded
	if evt.Type == "failed" {
		evtType = payment.WebhookPaymentFailed
	}
	return &payment.WebhookEvent{
		Type:       evtType,
		ExternalID: evt.ID,
		OccurredAt: time.Now().UTC(),
	}, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*payment.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/sessions?limit=1", nil)
	if err != nil {
		return &payment.HealthStatus{OK: false, Provider: "windcave", Err: err.Error()}, nil
	}
	req.Header.Set("Authorization", "Basic "+p.auth())
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &payment.HealthStatus{OK: false, Provider: "windcave", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &payment.HealthStatus{OK: true, Provider: "windcave", Latency: latency}, nil
}
