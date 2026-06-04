// Package efax implements fax.Provider for eFax Corporate REST API.
// Used as a fallback for non-Healthlink recipients (private specialists,
// overseas providers, or older practices not on Healthlink).
// API docs: https://www.efax.com/api
package efax

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

	"github.com/PhillipC05/tpt-healthcare/core/fax"
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

func init() {
	fax.Register("efax", func(ctx context.Context, v *viper.Viper) (fax.Provider, error) {
		return New(ctx, Config{
			AccountID: v.GetString("fax.efax.account_id"),
			APIToken:  v.GetString("fax.efax.api_token"),
			FaxNumber: v.GetString("fax.efax.fax_number"),
			BaseURL:   v.GetString("fax.efax.base_url"),
		})
	})
}

const defaultBaseURL = "https://secure.efax.com/api/v1"

// Config holds eFax API credentials.
type Config struct {
	AccountID string
	APIToken  string
	FaxNumber string // outbound sender fax number
	BaseURL   string
}

// Provider implements fax.Provider for eFax.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.AccountID == "" || cfg.APIToken == "" {
		return nil, fmt.Errorf("efax: account_id and api_token are required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "efax"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("efax marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "efax", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("efax request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("efax http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("efax read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &fax.Error{Provider: "efax", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) Send(ctx context.Context, to, subject string, document []byte, contentType string) (*fax.SendResult, error) {
	ext := "pdf"
	if contentType == "text/hl7-v2" {
		ext = "txt"
	}
	payload := map[string]any{
		"fax_to":      to,
		"fax_from":    p.cfg.FaxNumber,
		"subject":     subject,
		"account_id":  p.cfg.AccountID,
		"files": []map[string]any{
			{
				"name":    "document." + ext,
				"content": base64.StdEncoding.EncodeToString(document),
			},
		},
	}
	raw, err := p.post(ctx, "/fax/outbox", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		FaxID  string `json:"fax_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("efax send decode: %w", err)
	}
	return &fax.SendResult{ExternalID: resp.FaxID, Status: resp.Status, QueuedAt: time.Now().UTC()}, nil
}

func (p *Provider) GetStatus(ctx context.Context, externalID string) (*fax.FaxStatus, error) {
	var result []byte
	err := resilience.Do(ctx, p.breaker, "efax", resilience.RetryConfig{MaxAttempts: 2}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/fax/outbox/"+externalID, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+p.cfg.APIToken)
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
		Status      string `json:"status"`
		CompletedAt string `json:"completed_at"`
		ErrorCode   string `json:"error_code"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("efax status decode: %w", err)
	}
	fs := &fax.FaxStatus{ExternalID: externalID, Delivered: resp.Status == "sent" || resp.Status == "delivered"}
	if fs.Delivered && resp.CompletedAt != "" {
		t, _ := time.Parse(time.RFC3339, resp.CompletedAt)
		fs.DeliveredAt = &t
	}
	if resp.Status == "failed" {
		fs.FailureReason = resp.ErrorCode
	}
	return fs, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*fax.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/account/"+p.cfg.AccountID, nil)
	if err != nil {
		return &fax.HealthStatus{OK: false, Provider: "efax", Err: err.Error()}, nil
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIToken)
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &fax.HealthStatus{OK: false, Provider: "efax", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &fax.HealthStatus{OK: true, Provider: "efax", Latency: latency}, nil
}
