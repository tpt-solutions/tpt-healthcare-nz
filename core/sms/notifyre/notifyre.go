// Package notifyre implements sms.Provider for the Notifyre SMS REST API.
// Notifyre is an Australian/New Zealand provider with local number support,
// two-way messaging, and compliance-friendly audit logging. Used by several
// NZ general practice and specialist networks.
// API documentation: https://api.notifyre.com/docs
package notifyre

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
	sms.Register("notifyre", func(ctx context.Context, v *viper.Viper) (sms.Provider, error) {
		return New(ctx, Config{
			APIToken: v.GetString("sms.notifyre.api_token"),
			Sender:   v.GetString("sms.notifyre.sender"),
		})
	})
}

const baseURL = "https://api.notifyre.com/sms"

// Config holds Notifyre API credentials.
// APIToken is generated under Account → API Tokens in the Notifyre portal.
// Sender is a registered Notifyre sender ID or virtual mobile number (VMN).
type Config struct {
	APIToken string
	Sender   string
}

// Provider implements sms.Provider for Notifyre.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and returns a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("notifyre: api_token is required")
	}
	if cfg.Sender == "" {
		cfg.Sender = "TPTHealth"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "notifyre"})
	return &Provider{
		cfg:     cfg,
		client:  &http.Client{Timeout: 15 * time.Second},
		breaker: reg,
	}, nil
}

func (p *Provider) post(ctx context.Context, path string, payload any) ([]byte, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("notifyre marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "notifyre", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("notifyre request: %w", err)
		}
		req.Header.Set("x-api-token", p.cfg.APIToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("notifyre http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("notifyre read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &sms.Error{
				Provider:  "notifyre",
				Code:      fmt.Sprintf("HTTP%d", resp.StatusCode),
				Message:   string(result),
				Retryable: resp.StatusCode >= 500,
			}
		}
		return nil
	})
	return result, err
}

func (p *Provider) get(ctx context.Context, path string) ([]byte, error) {
	var result []byte
	err := resilience.Do(ctx, p.breaker, "notifyre", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
		if err != nil {
			return fmt.Errorf("notifyre request: %w", err)
		}
		req.Header.Set("x-api-token", p.cfg.APIToken)
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("notifyre http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("notifyre read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &sms.Error{
				Provider:  "notifyre",
				Code:      fmt.Sprintf("HTTP%d", resp.StatusCode),
				Message:   string(result),
				Retryable: resp.StatusCode >= 500,
			}
		}
		return nil
	})
	return result, err
}

func (p *Provider) Send(ctx context.Context, msg sms.Message) (*sms.SendResult, error) {
	results, err := p.SendBulk(ctx, []sms.Message{msg})
	if err != nil {
		return nil, err
	}
	return &results[0], nil
}

func (p *Provider) SendBulk(ctx context.Context, messages []sms.Message) ([]sms.SendResult, error) {
	// Notifyre accepts a recipients array in a single POST.
	type recipient struct {
		MobileNumber string `json:"mobileNumber"` // E.164 without leading +
		Reference    string `json:"reference,omitempty"`
	}
	recipients := make([]recipient, len(messages))
	// All messages in a bulk call share the same body; for varied bodies we
	// fall back to sequential sends.
	singleBody := messages[0].Body
	allSameBody := true
	for i, m := range messages {
		// Notifyre E.164: strip leading "+" (they accept both but strip is safer)
		to := m.To
		if len(to) > 0 && to[0] == '+' {
			to = to[1:]
		}
		recipients[i] = recipient{MobileNumber: to, Reference: m.Reference}
		if m.Body != singleBody {
			allSameBody = false
		}
	}

	if !allSameBody {
		// Varied bodies: send individually.
		results := make([]sms.SendResult, len(messages))
		for i, m := range messages {
			r, err := p.sendOne(ctx, m)
			if err != nil {
				results[i] = sms.SendResult{Status: "failed"}
				continue
			}
			results[i] = *r
		}
		return results, nil
	}

	payload := map[string]any{
		"message":    singleBody,
		"recipients": recipients,
		"from":       p.cfg.Sender,
	}
	raw, err := p.post(ctx, "/send", payload)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Messages []struct {
				MessageID string `json:"messageId"`
				Status    string `json:"status"` // "queued" | "failed"
			} `json:"messages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("notifyre decode send: %w", err)
	}
	results := make([]sms.SendResult, len(resp.Data.Messages))
	for i, m := range resp.Data.Messages {
		results[i] = sms.SendResult{
			ExternalID: m.MessageID,
			Status:     m.Status,
			QueuedAt:   time.Now().UTC(),
		}
	}
	for len(results) < len(messages) {
		results = append(results, sms.SendResult{Status: "failed"})
	}
	return results, nil
}

func (p *Provider) sendOne(ctx context.Context, msg sms.Message) (*sms.SendResult, error) {
	to := msg.To
	if len(to) > 0 && to[0] == '+' {
		to = to[1:]
	}
	payload := map[string]any{
		"message": msg.Body,
		"recipients": []map[string]any{
			{"mobileNumber": to, "reference": msg.Reference},
		},
		"from": p.cfg.Sender,
	}
	raw, err := p.post(ctx, "/send", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			Messages []struct {
				MessageID string `json:"messageId"`
				Status    string `json:"status"`
			} `json:"messages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Data.Messages) == 0 {
		return nil, fmt.Errorf("notifyre decode single send: %w", err)
	}
	m := resp.Data.Messages[0]
	return &sms.SendResult{ExternalID: m.MessageID, Status: m.Status, QueuedAt: time.Now().UTC()}, nil
}

func (p *Provider) GetDeliveryStatus(ctx context.Context, externalID string) (*sms.DeliveryStatus, error) {
	raw, err := p.get(ctx, "/messages/"+externalID)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			Status      string `json:"status"` // "delivered" | "failed" | "pending"
			DeliveredAt int64  `json:"deliveredAt,omitempty"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("notifyre decode delivery status: %w", err)
	}
	ds := &sms.DeliveryStatus{ExternalID: externalID}
	switch resp.Data.Status {
	case "delivered":
		ds.Delivered = true
		if resp.Data.DeliveredAt > 0 {
			t := time.Unix(resp.Data.DeliveredAt/1000, 0).UTC()
			ds.DeliveredAt = &t
		}
	case "failed":
		ds.FailureReason = "delivery failed"
	}
	return ds, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*sms.HealthStatus, error) {
	start := time.Now()
	// Notifyre doesn't have a dedicated ping endpoint; use a lightweight
	// GET against the account balance endpoint to verify credentials.
	_, err := p.get(ctx, "/balance")
	latency := time.Since(start)
	if err != nil {
		return &sms.HealthStatus{OK: false, Provider: "notifyre", Latency: latency, Err: err.Error()}, nil
	}
	return &sms.HealthStatus{OK: true, Provider: "notifyre", Latency: latency}, nil
}
