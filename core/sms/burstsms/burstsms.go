// Package burstsms implements sms.Provider for the Burst SMS REST API.
// Burst SMS is an AU/NZ carrier with strong NZ healthcare sector coverage.
// Sandbox credentials: https://burstsms.com.au/api-documentation
package burstsms

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
	sms.Register("burstsms", func(ctx context.Context, v *viper.Viper) (sms.Provider, error) {
		return New(ctx, Config{
			APIKey:    v.GetString("sms.burstsms.api_key"),
			APISecret: v.GetString("sms.burstsms.api_secret"),
			Sender:    v.GetString("sms.burstsms.sender"),
		})
	})
}

const baseURL = "https://api.transmitsms.com"

// Config holds Burst SMS API credentials.
type Config struct {
	APIKey    string
	APISecret string
	Sender    string // sender name, max 11 alphanumeric chars
}

// Provider implements sms.Provider for Burst SMS.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.APIKey == "" || cfg.APISecret == "" {
		return nil, fmt.Errorf("burstsms: api_key and api_secret are required")
	}
	if cfg.Sender == "" {
		cfg.Sender = "TPTHealth"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "burstsms"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("burstsms marshal: %w", err)
	}

	var result []byte
	err = resilience.Do(ctx, p.breaker, "burstsms", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("burstsms request: %w", err)
		}
		req.SetBasicAuth(p.cfg.APIKey, p.cfg.APISecret)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("burstsms http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("burstsms read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &sms.Error{Provider: "burstsms", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) Send(ctx context.Context, msg sms.Message) (*sms.SendResult, error) {
	payload := map[string]any{
		"to":      msg.To,
		"from":    p.cfg.Sender,
		"message": msg.Body,
	}
	raw, err := p.post(ctx, "/send-sms.json", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		MessageID int64 `json:"message_id"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("burstsms send decode: %w", err)
	}
	return &sms.SendResult{
		ExternalID: fmt.Sprint(resp.MessageID),
		Status:     "queued",
		QueuedAt:   time.Now().UTC(),
	}, nil
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
	// Burst SMS delivery receipts are pushed via webhook; polling not supported in v1.
	// Status is approximated from the message detail endpoint.
	return &sms.DeliveryStatus{ExternalID: externalID, Delivered: false, FailureReason: "polling not supported; use webhook"}, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*sms.HealthStatus, error) {
	start := time.Now()
	_, err := p.post(ctx, "/get-balance.json", map[string]any{})
	latency := time.Since(start)
	if err != nil {
		return &sms.HealthStatus{OK: false, Provider: "burstsms", Latency: latency, Err: err.Error()}, nil
	}
	return &sms.HealthStatus{OK: true, Provider: "burstsms", Latency: latency}, nil
}
