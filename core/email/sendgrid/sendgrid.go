// Package sendgrid implements email.Provider for the SendGrid (Twilio) Mail Send API v3.
// Sandbox mode available via the SendGrid API settings.
// API docs: https://docs.sendgrid.com/api-reference/mail-send
package sendgrid

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
	email.Register("sendgrid", func(ctx context.Context, v *viper.Viper) (email.Provider, error) {
		return New(ctx, Config{
			APIKey:      v.GetString("email.sendgrid.api_key"),
			FromAddress: v.GetString("email.sendgrid.from_address"),
			FromName:    v.GetString("email.sendgrid.from_name"),
		})
	})
}

const baseURL = "https://api.sendgrid.com/v3"

// Config holds SendGrid credentials.
type Config struct {
	APIKey      string
	FromAddress string
	FromName    string
}

// Provider implements email.Provider for SendGrid.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("sendgrid: api_key is required")
	}
	if cfg.FromAddress == "" {
		return nil, fmt.Errorf("sendgrid: from_address is required")
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "sendgrid"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("sendgrid marshal: %w", err)
	}
	return resilience.Do(ctx, p.breaker, "sendgrid", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/mail/send", bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("sendgrid request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("sendgrid http: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			raw, _ := io.ReadAll(resp.Body)
			return &email.Error{Provider: "sendgrid", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(raw), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
}

func (p *Provider) Send(ctx context.Context, msg email.Message) (*email.SendResult, error) {
	payload := p.buildPayload(msg)
	if err := p.post(ctx, payload); err != nil {
		return nil, err
	}
	return &email.SendResult{QueuedAt: time.Now().UTC()}, nil
}

func (p *Provider) SendBulk(ctx context.Context, messages []email.Message) ([]email.SendResult, error) {
	results := make([]email.SendResult, 0, len(messages))
	for _, m := range messages {
		r, err := p.Send(ctx, m)
		if err != nil {
			results = append(results, email.SendResult{QueuedAt: time.Now().UTC()})
			continue
		}
		results = append(results, *r)
	}
	return results, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*email.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/user/credits", nil)
	if err != nil {
		return &email.HealthStatus{OK: false, Provider: "sendgrid", Err: err.Error()}, nil
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &email.HealthStatus{OK: false, Provider: "sendgrid", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &email.HealthStatus{OK: true, Provider: "sendgrid", Latency: latency}, nil
}

func (p *Provider) buildPayload(msg email.Message) map[string]any {
	from := map[string]any{"email": p.cfg.FromAddress}
	if p.cfg.FromName != "" {
		from["name"] = p.cfg.FromName
	}
	if msg.From != "" {
		from["email"] = msg.From
	}

	toList := make([]map[string]any, len(msg.To))
	for i, t := range msg.To {
		toList[i] = map[string]any{"email": t}
	}

	content := []map[string]any{}
	if msg.TextBody != "" {
		content = append(content, map[string]any{"type": "text/plain", "value": msg.TextBody})
	}
	if msg.HTMLBody != "" {
		content = append(content, map[string]any{"type": "text/html", "value": msg.HTMLBody})
	}

	payload := map[string]any{
		"from":             from,
		"personalizations": []map[string]any{{"to": toList}},
		"subject":          msg.Subject,
		"content":          content,
	}

	if len(msg.Attachments) > 0 {
		atts := make([]map[string]any, len(msg.Attachments))
		for i, a := range msg.Attachments {
			atts[i] = map[string]any{
				"filename":     a.Filename,
				"type":         a.ContentType,
				"content":      base64.StdEncoding.EncodeToString(a.Data),
				"disposition":  "attachment",
			}
		}
		payload["attachments"] = atts
	}

	if msg.TemplateID != "" {
		payload["template_id"] = msg.TemplateID
		payload["dynamic_template_data"] = msg.TemplateData
	}

	return payload
}
