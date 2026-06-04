// Package mailjet implements email.Provider for the Mailjet Send API v3.1.
// Mailjet offers a generous free tier and strong EU/AP deliverability.
// Sandbox: use test API credentials from the Mailjet dashboard.
// API docs: https://dev.mailjet.com/email/reference/send-emails/
package mailjet

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
	email.Register("mailjet", func(ctx context.Context, v *viper.Viper) (email.Provider, error) {
		return New(ctx, Config{
			APIKey:      v.GetString("email.mailjet.api_key"),
			SecretKey:   v.GetString("email.mailjet.secret_key"),
			FromAddress: v.GetString("email.mailjet.from_address"),
			FromName:    v.GetString("email.mailjet.from_name"),
		})
	})
}

const baseURL = "https://api.mailjet.com/v3.1"

// Config holds Mailjet credentials.
type Config struct {
	APIKey      string
	SecretKey   string
	FromAddress string
	FromName    string
}

// Provider implements email.Provider for Mailjet.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.APIKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("mailjet: api_key and secret_key are required")
	}
	if cfg.FromAddress == "" {
		return nil, fmt.Errorf("mailjet: from_address is required")
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "mailjet"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, payload any) ([]byte, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("mailjet marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "mailjet", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/send", bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("mailjet request: %w", err)
		}
		req.SetBasicAuth(p.cfg.APIKey, p.cfg.SecretKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("mailjet http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("mailjet read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &email.Error{Provider: "mailjet", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) Send(ctx context.Context, msg email.Message) (*email.SendResult, error) {
	from := p.cfg.FromAddress
	if msg.From != "" {
		from = msg.From
	}

	toList := make([]map[string]any, len(msg.To))
	for i, t := range msg.To {
		toList[i] = map[string]any{"Email": t}
	}

	message := map[string]any{
		"From":     map[string]any{"Email": from, "Name": p.cfg.FromName},
		"To":       toList,
		"Subject":  msg.Subject,
		"TextPart": msg.TextBody,
		"HTMLPart": msg.HTMLBody,
	}

	if len(msg.Attachments) > 0 {
		atts := make([]map[string]any, len(msg.Attachments))
		for i, a := range msg.Attachments {
			atts[i] = map[string]any{
				"Filename":    a.Filename,
				"ContentType": a.ContentType,
				"Base64Content": base64.StdEncoding.EncodeToString(a.Data),
			}
		}
		message["Attachments"] = atts
	}

	if msg.TemplateID != "" {
		message["TemplateID"] = msg.TemplateID
		message["TemplateLanguage"] = true
		message["Variables"] = msg.TemplateData
	}

	payload := map[string]any{"Messages": []map[string]any{message}}
	raw, err := p.post(ctx, payload)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Messages []struct {
			To []struct {
				MessageID string `json:"MessageID"`
			} `json:"To"`
		} `json:"Messages"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("mailjet send decode: %w", err)
	}
	id := ""
	if len(resp.Messages) > 0 && len(resp.Messages[0].To) > 0 {
		id = resp.Messages[0].To[0].MessageID
	}
	return &email.SendResult{ExternalID: id, QueuedAt: time.Now().UTC()}, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.mailjet.com/v3/REST/sender", nil)
	if err != nil {
		return &email.HealthStatus{OK: false, Provider: "mailjet", Err: err.Error()}, nil
	}
	req.SetBasicAuth(p.cfg.APIKey, p.cfg.SecretKey)
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &email.HealthStatus{OK: false, Provider: "mailjet", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &email.HealthStatus{OK: true, Provider: "mailjet", Latency: latency}, nil
}
