// Package healthlink implements fax.Provider for Healthlink EDI.
// Healthlink is the primary secure messaging network used in NZ healthcare
// for referrals, discharge summaries, lab results, and specialist reports.
// It requires HPI credentials and a Healthlink EDI account.
// Test environment: available via Healthlink's developer portal.
// API: Healthlink REST API (OAuth2 with HPI client credentials).
package healthlink

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
	fax.Register("healthlink", func(ctx context.Context, v *viper.Viper) (fax.Provider, error) {
		return New(ctx, Config{
			ClientID:     v.GetString("fax.healthlink.client_id"),
			ClientSecret: v.GetString("fax.healthlink.client_secret"),
			HPICPN:       v.GetString("fax.healthlink.hpi_cpn"),
			HPIFacilityID: v.GetString("fax.healthlink.hpi_facility_id"),
			BaseURL:      v.GetString("fax.healthlink.base_url"),
		})
	})
}

// Config holds Healthlink EDI credentials.
type Config struct {
	ClientID      string
	ClientSecret  string
	HPICPN        string // sending practitioner's HPI CPN
	HPIFacilityID string // sending facility's HPI organisation ID
	BaseURL       string // default: https://api.healthlink.net/v1
}

// Provider implements fax.Provider for Healthlink EDI.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("healthlink: client_id and client_secret are required")
	}
	if cfg.HPICPN == "" {
		return nil, fmt.Errorf("healthlink: hpi_cpn (sending practitioner CPN) is required")
	}
	if cfg.HPIFacilityID == "" {
		return nil, fmt.Errorf("healthlink: hpi_facility_id is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.healthlink.net/v1"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "healthlink"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("healthlink marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "healthlink", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("healthlink request: %w", err)
		}
		req.SetBasicAuth(p.cfg.ClientID, p.cfg.ClientSecret)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-HPI-CPN", p.cfg.HPICPN)
		req.Header.Set("X-HPI-Facility-ID", p.cfg.HPIFacilityID)

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("healthlink http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("healthlink read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &fax.Error{Provider: "healthlink", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) Send(ctx context.Context, to, subject string, document []byte, contentType string) (*fax.SendResult, error) {
	payload := map[string]any{
		"to":          to, // Healthlink EDI address, e.g. "DR_JONES@practice.healthlink.net"
		"subject":     subject,
		"contentType": contentType,
		"content":     base64.StdEncoding.EncodeToString(document),
		"senderCPN":   p.cfg.HPICPN,
		"senderFacilityID": p.cfg.HPIFacilityID,
	}
	raw, err := p.post(ctx, "/messages", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		MessageID string `json:"messageId"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("healthlink send decode: %w", err)
	}
	return &fax.SendResult{ExternalID: resp.MessageID, Status: resp.Status, QueuedAt: time.Now().UTC()}, nil
}

func (p *Provider) GetStatus(ctx context.Context, externalID string) (*fax.FaxStatus, error) {
	var result []byte
	err := resilience.Do(ctx, p.breaker, "healthlink", resilience.RetryConfig{MaxAttempts: 2}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/messages/"+externalID, nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth(p.cfg.ClientID, p.cfg.ClientSecret)
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
		DeliveredAt string `json:"deliveredAt"`
		ErrorMessage string `json:"errorMessage"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("healthlink status decode: %w", err)
	}
	fs := &fax.FaxStatus{ExternalID: externalID, Delivered: resp.Status == "delivered"}
	if fs.Delivered && resp.DeliveredAt != "" {
		t, _ := time.Parse(time.RFC3339, resp.DeliveredAt)
		fs.DeliveredAt = &t
	}
	if resp.Status == "failed" {
		fs.FailureReason = resp.ErrorMessage
	}
	return fs, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*fax.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/health", nil)
	if err != nil {
		return &fax.HealthStatus{OK: false, Provider: "healthlink", Err: err.Error()}, nil
	}
	req.SetBasicAuth(p.cfg.ClientID, p.cfg.ClientSecret)
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &fax.HealthStatus{OK: false, Provider: "healthlink", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &fax.HealthStatus{OK: true, Provider: "healthlink", Latency: latency}, nil
}
