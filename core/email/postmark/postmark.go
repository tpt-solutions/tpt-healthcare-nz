// Package postmark implements email.Provider for the Postmark API.
// Postmark has excellent NZ/AU delivery and a generous sandbox mode.
// API docs: https://postmarkapp.com/developer/api/overview
package postmark

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

	"github.com/PhillipC05/tpt-healthcare/core/email"
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

func init() {
	email.Register("postmark", func(ctx context.Context, v *viper.Viper) (email.Provider, error) {
		return New(ctx, Config{
			ServerToken: v.GetString("email.postmark.server_token"),
			FromAddress: v.GetString("email.postmark.from_address"),
			MessageStream: v.GetString("email.postmark.message_stream"),
		})
	})
}

const baseURL = "https://api.postmarkapp.com"

// Config holds Postmark API credentials.
type Config struct {
	ServerToken   string
	FromAddress   string
	MessageStream string // defaults to "outbound"
}

// Provider implements email.Provider for Postmark.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.ServerToken == "" {
		return nil, fmt.Errorf("postmark: server_token is required")
	}
	if cfg.FromAddress == "" {
		return nil, fmt.Errorf("postmark: from_address is required")
	}
	if cfg.MessageStream == "" {
		cfg.MessageStream = "outbound"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "postmark"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("postmark marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "postmark", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("postmark request: %w", err)
		}
		req.Header.Set("X-Postmark-Server-Token", p.cfg.ServerToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("postmark http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("postmark read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &email.Error{Provider: "postmark", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) Send(ctx context.Context, msg email.Message) (*email.SendResult, error) {
	payload := p.buildPayload(msg)
	raw, err := p.post(ctx, "/email", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		MessageID string `json:"MessageID"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("postmark send decode: %w", err)
	}
	return &email.SendResult{ExternalID: resp.MessageID, QueuedAt: time.Now().UTC()}, nil
}

func (p *Provider) SendBulk(ctx context.Context, messages []email.Message) ([]email.SendResult, error) {
	payloads := make([]map[string]any, len(messages))
	for i, m := range messages {
		payloads[i] = p.buildPayload(m)
	}
	raw, err := p.post(ctx, "/email/batch", payloads)
	if err != nil {
		return nil, err
	}
	var resp []struct {
		MessageID string `json:"MessageID"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("postmark batch decode: %w", err)
	}
	results := make([]email.SendResult, len(resp))
	for i, r := range resp {
		results[i] = email.SendResult{ExternalID: r.MessageID, QueuedAt: time.Now().UTC()}
	}
	return results, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*email.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/server", nil)
	if err != nil {
		return &email.HealthStatus{OK: false, Provider: "postmark", Err: err.Error()}, nil
	}
	req.Header.Set("X-Postmark-Server-Token", p.cfg.ServerToken)
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &email.HealthStatus{OK: false, Provider: "postmark", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &email.HealthStatus{OK: true, Provider: "postmark", Latency: latency}, nil
}

func (p *Provider) buildPayload(msg email.Message) map[string]any {
	from := p.cfg.FromAddress
	if msg.From != "" {
		from = msg.From
	}
	payload := map[string]any{
		"From":          from,
		"To":            joinAddresses(msg.To),
		"Subject":       msg.Subject,
		"TextBody":      msg.TextBody,
		"HtmlBody":      msg.HTMLBody,
		"MessageStream": p.cfg.MessageStream,
	}
	if len(msg.CC) > 0 {
		payload["Cc"] = joinAddresses(msg.CC)
	}
	if len(msg.Attachments) > 0 {
		atts := make([]map[string]any, len(msg.Attachments))
		for i, a := range msg.Attachments {
			atts[i] = map[string]any{
				"Name":        a.Filename,
				"Content":     base64.StdEncoding.EncodeToString(a.Data),
				"ContentType": a.ContentType,
			}
		}
		payload["Attachments"] = atts
	}
	if msg.TemplateID != "" {
		payload["TemplateAlias"] = msg.TemplateID
		payload["TemplateModel"] = msg.TemplateData
		delete(payload, "TextBody")
		delete(payload, "HtmlBody")
	}
	return payload
}

func joinAddresses(addrs []string) string {
	result := ""
	for i, a := range addrs {
		if i > 0 {
			result += ", "
		}
		result += a
	}
	return result
}
