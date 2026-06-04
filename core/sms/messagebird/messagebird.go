// Package messagebird implements sms.Provider for the MessageBird REST API.
// MessageBird has strong NZ coverage and is commonly used in NZ healthcare.
// Test credentials available at https://dashboard.messagebird.com/en/settings
package messagebird

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
	sms.Register("messagebird", func(ctx context.Context, v *viper.Viper) (sms.Provider, error) {
		return New(ctx, Config{
			AccessKey:  v.GetString("sms.messagebird.access_key"),
			Originator: v.GetString("sms.messagebird.originator"),
		})
	})
}

const baseURL = "https://rest.messagebird.com"

// Config holds MessageBird credentials.
type Config struct {
	AccessKey  string
	Originator string // Sender name or number (e.g. "TPTHealth" or "+6421234567")
}

// Provider implements sms.Provider for MessageBird.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.AccessKey == "" {
		return nil, fmt.Errorf("messagebird: access_key is required")
	}
	if cfg.Originator == "" {
		cfg.Originator = "TPTHealth"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "messagebird"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("messagebird marshal: %w", err)
	}

	var result []byte
	err = resilience.Do(ctx, p.breaker, "messagebird", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("messagebird request: %w", err)
		}
		req.Header.Set("Authorization", "AccessKey "+p.cfg.AccessKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("messagebird http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("messagebird read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &sms.Error{Provider: "messagebird", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) Send(ctx context.Context, msg sms.Message) (*sms.SendResult, error) {
	payload := map[string]any{
		"originator": p.cfg.Originator,
		"recipients": msg.To,
		"body":       msg.Body,
		"reference":  msg.Reference,
	}
	raw, err := p.post(ctx, "/messages", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ID        string `json:"id"`
		CreatedAt string `json:"createdDatetime"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("messagebird send decode: %w", err)
	}
	t, _ := time.Parse(time.RFC3339, resp.CreatedAt)
	return &sms.SendResult{ExternalID: resp.ID, Status: "queued", QueuedAt: t}, nil
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
	var result []byte
	err := resilience.Do(ctx, p.breaker, "messagebird", resilience.RetryConfig{MaxAttempts: 2}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/messages/"+externalID, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "AccessKey "+p.cfg.AccessKey)
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
		Recipients struct {
			Items []struct {
				Status    string `json:"status"`
				StatusDatetime string `json:"statusDatetime"`
			} `json:"items"`
		} `json:"recipients"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("messagebird status decode: %w", err)
	}
	ds := &sms.DeliveryStatus{ExternalID: externalID}
	if len(resp.Recipients.Items) > 0 {
		item := resp.Recipients.Items[0]
		ds.Delivered = item.Status == "delivered"
		if ds.Delivered {
			t, _ := time.Parse(time.RFC3339, item.StatusDatetime)
			ds.DeliveredAt = &t
		}
		if item.Status == "failed" || item.Status == "delivery_failed" {
			ds.FailureReason = item.Status
		}
	}
	return ds, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*sms.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/balance", nil)
	if err != nil {
		return &sms.HealthStatus{OK: false, Provider: "messagebird", Err: err.Error()}, nil
	}
	req.Header.Set("Authorization", "AccessKey "+p.cfg.AccessKey)
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &sms.HealthStatus{OK: false, Provider: "messagebird", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &sms.HealthStatus{OK: true, Provider: "messagebird", Latency: latency}, nil
}
