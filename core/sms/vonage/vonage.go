// Package vonage implements sms.Provider for the Vonage (formerly Nexmo) SMS API.
// Sandbox: https://developer.vonage.com/en/getting-started/testing
package vonage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/resilience"
	"github.com/PhillipC05/tpt-healthcare/core/sms"
)

func init() {
	sms.Register("vonage", func(ctx context.Context, v *viper.Viper) (sms.Provider, error) {
		return New(ctx, Config{
			APIKey:    v.GetString("sms.vonage.api_key"),
			APISecret: v.GetString("sms.vonage.api_secret"),
			From:      v.GetString("sms.vonage.from"),
		})
	})
}

const baseURL = "https://rest.nexmo.com"

// Config holds Vonage API credentials.
type Config struct {
	APIKey    string
	APISecret string
	From      string // sender ID
}

// Provider implements sms.Provider for Vonage.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.APIKey == "" || cfg.APISecret == "" {
		return nil, fmt.Errorf("vonage: api_key and api_secret are required")
	}
	if cfg.From == "" {
		cfg.From = "TPTHealth"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "vonage"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("vonage marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "vonage", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("vonage request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("vonage http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("vonage read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &sms.Error{Provider: "vonage", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) Send(ctx context.Context, msg sms.Message) (*sms.SendResult, error) {
	payload := map[string]any{
		"api_key":    p.cfg.APIKey,
		"api_secret": p.cfg.APISecret,
		"from":       p.cfg.From,
		"to":         msg.To,
		"text":       msg.Body,
		"client-ref": msg.Reference,
	}
	raw, err := p.post(ctx, "/sms/json", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Messages []struct {
			MessageID string `json:"message-id"`
			Status    string `json:"status"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("vonage send decode: %w", err)
	}
	if len(resp.Messages) == 0 {
		return nil, fmt.Errorf("vonage: no message in response")
	}
	m := resp.Messages[0]
	if m.Status != "0" {
		return nil, &sms.Error{Provider: "vonage", Code: m.Status, Message: "send failed", Retryable: m.Status == "1"}
	}
	return &sms.SendResult{ExternalID: m.MessageID, Status: "queued", QueuedAt: time.Now().UTC()}, nil
}

func (p *Provider) SendBulk(ctx context.Context, messages []sms.Message) ([]sms.SendResult, error) {
	results := make([]sms.SendResult, 0, len(messages))
	for _, m := range messages {
		r, err := p.Send(ctx, m)
		if err != nil {
			results = append(results, sms.SendResult{Status: "failed"})
			continue
		}
		results = append(results, *r)
	}
	return results, nil
}

func (p *Provider) GetDeliveryStatus(ctx context.Context, externalID string) (*sms.DeliveryStatus, error) {
	// Vonage delivery receipts are webhook-based; this returns a best-effort status.
	return &sms.DeliveryStatus{ExternalID: externalID}, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*sms.HealthStatus, error) {
	start := time.Now()
	payload := map[string]any{
		"api_key":    p.cfg.APIKey,
		"api_secret": p.cfg.APISecret,
	}
	_, err := p.post(ctx, "/account/get-balance", payload)
	latency := time.Since(start)
	if err != nil {
		return &sms.HealthStatus{OK: false, Provider: "vonage", Latency: latency, Err: err.Error()}, nil
	}
	return &sms.HealthStatus{OK: true, Provider: "vonage", Latency: latency}, nil
}
