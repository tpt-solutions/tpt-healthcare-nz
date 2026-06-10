// Package clicksend implements sms.Provider for the ClickSend REST API v3.
// ClickSend is widely used across the NZ and AU healthcare sector and supports
// two-way messaging, delivery receipts, and HIPAA-grade audit logging.
// API documentation: https://developers.clicksend.com/docs/rest/v3/
package clicksend

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

	"github.com/PhillipC05/tpt-healthcare/core/resilience"
	"github.com/PhillipC05/tpt-healthcare/core/sms"
)

func init() {
	sms.Register("clicksend", func(ctx context.Context, v *viper.Viper) (sms.Provider, error) {
		return New(ctx, Config{
			Username: v.GetString("sms.clicksend.username"),
			APIKey:   v.GetString("sms.clicksend.api_key"),
			Sender:   v.GetString("sms.clicksend.sender"),
		})
	})
}

const baseURL = "https://rest.clicksend.com/v3"

// Config holds ClickSend credentials.
// Username is your ClickSend account username (email).
// APIKey is generated under My Account → API Credentials.
type Config struct {
	Username string
	APIKey   string
	Sender   string // sender name or E.164 number; max 11 chars for alphanumeric
}

// Provider implements sms.Provider for ClickSend.
type Provider struct {
	cfg     Config
	auth    string // base64(username:api_key) for Basic Auth
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.Username == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("clicksend: username and api_key are required")
	}
	if cfg.Sender == "" {
		cfg.Sender = "TPTHealth"
	}
	auth := base64.StdEncoding.EncodeToString([]byte(cfg.Username + ":" + cfg.APIKey))
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "clicksend"})
	return &Provider{
		cfg:     cfg,
		auth:    auth,
		client:  &http.Client{Timeout: 15 * time.Second},
		breaker: reg,
	}, nil
}

func (p *Provider) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("clicksend marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	var result []byte
	err := resilience.Do(ctx, p.breaker, "clicksend", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("clicksend request: %w", err)
		}
		req.Header.Set("Authorization", "Basic "+p.auth)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("clicksend http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("clicksend read body: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &sms.Error{
				Provider:  "clicksend",
				Code:      fmt.Sprintf("HTTP%d", resp.StatusCode),
				Message:   string(result),
				Retryable: resp.StatusCode >= 500,
			}
		}
		return nil
	})
	return result, err
}

// clicksendMessage is the per-message payload in the ClickSend batch API.
type clicksendMessage struct {
	Body   string `json:"body"`
	To     string `json:"to"`
	From   string `json:"from,omitempty"`
	Source string `json:"source,omitempty"` // optional label e.g. "tpt-healthcare"
}

func (p *Provider) Send(ctx context.Context, msg sms.Message) (*sms.SendResult, error) {
	results, err := p.SendBulk(ctx, []sms.Message{msg})
	if err != nil {
		return nil, err
	}
	return &results[0], nil
}

func (p *Provider) SendBulk(ctx context.Context, messages []sms.Message) ([]sms.SendResult, error) {
	batch := make([]clicksendMessage, len(messages))
	for i, m := range messages {
		batch[i] = clicksendMessage{
			Body:   m.Body,
			To:     m.To,
			From:   p.cfg.Sender,
			Source: "tpt-healthcare",
		}
	}
	payload := map[string]any{"messages": batch}

	raw, err := p.do(ctx, http.MethodPost, "/sms/send", payload)
	if err != nil {
		return nil, err
	}

	// ClickSend wraps every response in { "http_code": 200, "response_code": "SUCCESS", "data": { "messages": [...] } }
	var envelope struct {
		Data struct {
			Messages []struct {
				MessageID    string `json:"message_id"`
				Status       string `json:"status"` // "SUCCESS" | "FAILED" etc.
				ErrorMessage string `json:"error_message,omitempty"`
			} `json:"messages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("clicksend decode send response: %w", err)
	}

	results := make([]sms.SendResult, len(envelope.Data.Messages))
	for i, m := range envelope.Data.Messages {
		status := "queued"
		if m.Status != "SUCCESS" {
			status = "failed"
		}
		results[i] = sms.SendResult{
			ExternalID: m.MessageID,
			Status:     status,
			QueuedAt:   time.Now().UTC(),
		}
	}
	// If the API returned fewer results than messages (shouldn't happen), pad with failures.
	for len(results) < len(messages) {
		results = append(results, sms.SendResult{Status: "failed"})
	}
	return results, nil
}

func (p *Provider) GetDeliveryStatus(ctx context.Context, externalID string) (*sms.DeliveryStatus, error) {
	raw, err := p.do(ctx, http.MethodGet, "/sms/receipts/"+externalID, nil)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data struct {
			Status    string `json:"status"` // "Delivered", "NotDelivered", "Pending"
			Timestamp int64  `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("clicksend decode delivery status: %w", err)
	}
	ds := &sms.DeliveryStatus{ExternalID: externalID}
	if envelope.Data.Status == "Delivered" {
		ds.Delivered = true
		if envelope.Data.Timestamp > 0 {
			t := time.Unix(envelope.Data.Timestamp, 0).UTC()
			ds.DeliveredAt = &t
		}
	} else if envelope.Data.Status == "NotDelivered" {
		ds.FailureReason = "delivery failed"
	}
	return ds, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*sms.HealthStatus, error) {
	start := time.Now()
	_, err := p.do(ctx, http.MethodGet, "/account", nil)
	latency := time.Since(start)
	if err != nil {
		return &sms.HealthStatus{OK: false, Provider: "clicksend", Latency: latency, Err: err.Error()}, nil
	}
	return &sms.HealthStatus{OK: true, Provider: "clicksend", Latency: latency}, nil
}
