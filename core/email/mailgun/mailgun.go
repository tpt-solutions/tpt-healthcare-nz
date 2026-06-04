// Package mailgun implements email.Provider for the Mailgun REST API.
// Use the EU or US region endpoint depending on data residency requirements.
// API docs: https://documentation.mailgun.com/en/latest/api-sending.html
package mailgun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/email"
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

func init() {
	email.Register("mailgun", func(ctx context.Context, v *viper.Viper) (email.Provider, error) {
		return New(ctx, Config{
			APIKey:      v.GetString("email.mailgun.api_key"),
			Domain:      v.GetString("email.mailgun.domain"),
			FromAddress: v.GetString("email.mailgun.from_address"),
			Region:      v.GetString("email.mailgun.region"),
		})
	})
}

// Config holds Mailgun credentials.
type Config struct {
	APIKey      string
	Domain      string
	FromAddress string
	Region      string // "us" (default) or "eu"
}

// Provider implements email.Provider for Mailgun.
type Provider struct {
	cfg     Config
	baseURL string
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.APIKey == "" || cfg.Domain == "" {
		return nil, fmt.Errorf("mailgun: api_key and domain are required")
	}
	if cfg.FromAddress == "" {
		return nil, fmt.Errorf("mailgun: from_address is required")
	}
	base := "https://api.mailgun.net/v3"
	if cfg.Region == "eu" {
		base = "https://api.eu.mailgun.net/v3"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "mailgun"})
	return &Provider{cfg: cfg, baseURL: base, client: &http.Client{Timeout: 15 * time.Second}, breaker: reg}, nil
}

func (p *Provider) Send(ctx context.Context, msg email.Message) (*email.SendResult, error) {
	from := p.cfg.FromAddress
	if msg.From != "" {
		from = msg.From
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("from", from)
	for _, to := range msg.To {
		_ = w.WriteField("to", to)
	}
	_ = w.WriteField("subject", msg.Subject)
	if msg.TextBody != "" {
		_ = w.WriteField("text", msg.TextBody)
	}
	if msg.HTMLBody != "" {
		_ = w.WriteField("html", msg.HTMLBody)
	}
	for _, att := range msg.Attachments {
		part, _ := w.CreateFormFile("attachment", att.Filename)
		_, _ = part.Write(att.Data)
	}
	w.Close()

	var result []byte
	err := resilience.Do(ctx, p.breaker, "mailgun", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			fmt.Sprintf("%s/%s/messages", p.baseURL, p.cfg.Domain), bytes.NewReader(body.Bytes()))
		if err != nil {
			return fmt.Errorf("mailgun request: %w", err)
		}
		req.SetBasicAuth("api", p.cfg.APIKey)
		req.Header.Set("Content-Type", w.FormDataContentType())

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("mailgun http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("mailgun read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &email.Error{Provider: "mailgun", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(result, &resp)
	return &email.SendResult{ExternalID: resp.ID, QueuedAt: time.Now().UTC()}, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/%s", p.baseURL, p.cfg.Domain), nil)
	if err != nil {
		return &email.HealthStatus{OK: false, Provider: "mailgun", Err: err.Error()}, nil
	}
	req.SetBasicAuth("api", p.cfg.APIKey)
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &email.HealthStatus{OK: false, Provider: "mailgun", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &email.HealthStatus{OK: true, Provider: "mailgun", Latency: latency}, nil
}
