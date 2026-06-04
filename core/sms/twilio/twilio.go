// Package twilio implements sms.Provider for the Twilio Messaging REST API.
// Global reliability; NZ numbers supported. Sandbox/test credentials available.
// API docs: https://www.twilio.com/docs/sms/api
package twilio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/resilience"
	"github.com/PhillipC05/tpt-healthcare/core/sms"
)

func init() {
	sms.Register("twilio", func(ctx context.Context, v *viper.Viper) (sms.Provider, error) {
		return New(ctx, Config{
			AccountSID:  v.GetString("sms.twilio.account_sid"),
			AuthToken:   v.GetString("sms.twilio.auth_token"),
			FromNumber:  v.GetString("sms.twilio.from_number"),
		})
	})
}

// Config holds Twilio credentials.
type Config struct {
	AccountSID string
	AuthToken  string
	FromNumber string // E.164, e.g. "+18005551234"
}

// Provider implements sms.Provider for Twilio.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.AccountSID == "" || cfg.AuthToken == "" {
		return nil, fmt.Errorf("twilio: account_sid and auth_token are required")
	}
	if cfg.FromNumber == "" {
		return nil, fmt.Errorf("twilio: from_number is required")
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "twilio"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}, breaker: reg}, nil
}

func (p *Provider) baseURL() string {
	return fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s", p.cfg.AccountSID)
}

func (p *Provider) post(ctx context.Context, path string, form url.Values) ([]byte, error) {
	var result []byte
	err := resilience.Do(ctx, p.breaker, "twilio", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL()+path, strings.NewReader(form.Encode()))
		if err != nil {
			return fmt.Errorf("twilio request: %w", err)
		}
		req.SetBasicAuth(p.cfg.AccountSID, p.cfg.AuthToken)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("twilio http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("twilio read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &sms.Error{
				Provider:  "twilio",
				Code:      fmt.Sprintf("HTTP%d", resp.StatusCode),
				Message:   string(result),
				Retryable: resp.StatusCode >= 500 || resp.StatusCode == 429,
			}
		}
		return nil
	})
	return result, err
}

func (p *Provider) Send(ctx context.Context, msg sms.Message) (*sms.SendResult, error) {
	form := url.Values{
		"To":   {msg.To},
		"From": {p.cfg.FromNumber},
		"Body": {msg.Body},
	}
	raw, err := p.post(ctx, "/Messages.json", form)
	if err != nil {
		return nil, err
	}
	var resp struct {
		SID         string `json:"sid"`
		Status      string `json:"status"`
		DateCreated string `json:"date_created"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("twilio send decode: %w", err)
	}
	t, _ := time.Parse(time.RFC1123Z, resp.DateCreated)
	return &sms.SendResult{ExternalID: resp.SID, Status: resp.Status, QueuedAt: t}, nil
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
	err := resilience.Do(ctx, p.breaker, "twilio", resilience.RetryConfig{MaxAttempts: 2}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL()+"/Messages/"+externalID+".json", nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth(p.cfg.AccountSID, p.cfg.AuthToken)
		req.Header.Set("Accept", "application/json")
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
		DateUpdated string `json:"date_updated"`
		ErrorMessage string `json:"error_message"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("twilio status decode: %w", err)
	}
	ds := &sms.DeliveryStatus{ExternalID: externalID, Delivered: resp.Status == "delivered"}
	if ds.Delivered {
		t, _ := time.Parse(time.RFC1123Z, resp.DateUpdated)
		ds.DeliveredAt = &t
	}
	if resp.Status == "failed" || resp.Status == "undelivered" {
		ds.FailureReason = resp.ErrorMessage
	}
	return ds, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*sms.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL()+".json", nil)
	if err != nil {
		return &sms.HealthStatus{OK: false, Provider: "twilio", Err: err.Error()}, nil
	}
	req.SetBasicAuth(p.cfg.AccountSID, p.cfg.AuthToken)
	req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &sms.HealthStatus{OK: false, Provider: "twilio", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &sms.HealthStatus{OK: true, Provider: "twilio", Latency: latency}, nil
}
