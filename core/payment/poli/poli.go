// Package poli implements payment.Provider for POLi Payments.
// POLi is a NZ/AU direct bank transfer payment method with no credit card fees.
// Widely used in NZ for online health payments — patients pay directly from their
// bank account. Zero chargebacks; confirmation is typically instant.
// Sandbox: https://poliapi.apac.paywithpoli.com (requires test merchant credentials)
// API docs: https://www.polipayments.com/developers
package poli

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

	"github.com/PhillipC05/tpt-healthcare/core/payment"
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

func init() {
	payment.Register("poli", func(ctx context.Context, v *viper.Viper) (payment.Provider, error) {
		return New(ctx, Config{
			MerchantCode: v.GetString("payment.poli.merchant_code"),
			AuthCode:     v.GetString("payment.poli.auth_code"),
			BaseURL:      v.GetString("payment.poli.base_url"),
		})
	})
}

const defaultBaseURL = "https://poliapi.apac.paywithpoli.com/api/v2"

// Config holds POLi credentials.
type Config struct {
	MerchantCode string
	AuthCode     string
	BaseURL      string
}

// Provider implements payment.Provider for POLi.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.MerchantCode == "" || cfg.AuthCode == "" {
		return nil, fmt.Errorf("poli: merchant_code and auth_code are required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "poli"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) auth() string {
	return base64.StdEncoding.EncodeToString([]byte(p.cfg.MerchantCode + ":" + p.cfg.AuthCode))
}

func (p *Provider) post(ctx context.Context, path string, body any) ([]byte, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("poli marshal: %w", err)
	}
	var result []byte
	err = resilience.Do(ctx, p.breaker, "poli", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+path, bytes.NewReader(b))
		if err != nil {
			return fmt.Errorf("poli request: %w", err)
		}
		req.Header.Set("Authorization", "Basic "+p.auth())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("poli http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("poli read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &payment.Error{Provider: "poli", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	return result, err
}

func (p *Provider) CreatePaymentRequest(ctx context.Context, req payment.PaymentRequest) (*payment.PaymentIntent, error) {
	payload := map[string]any{
		"Amount":           fmt.Sprintf("%.2f", float64(req.AmountCents)/100),
		"CurrencyCode":     "NZD",
		"MerchantReference": req.InvoiceID,
		"MerchantHomepageURL": p.cfg.BaseURL,
		"SuccessURL":        req.ReturnURL + "?status=success",
		"FailureURL":        req.ReturnURL + "?status=failure",
		"CancellationURL":   req.ReturnURL + "?status=cancelled",
		"NotificationURL":   req.ReturnURL + "/webhook",
	}
	raw, err := p.post(ctx, "/Transaction/Initiate", payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Token          string `json:"Token"`
		NavigateURL    string `json:"NavigateURL"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("poli initiate decode: %w", err)
	}
	return &payment.PaymentIntent{ExternalID: resp.Token, RedirectURL: resp.NavigateURL}, nil
}

func (p *Provider) CapturePayment(ctx context.Context, intentID string) (*payment.PaymentResult, error) {
	var result []byte
	err := resilience.Do(ctx, p.breaker, "poli", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/Transaction/GetTransaction?token="+intentID, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Basic "+p.auth())
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
		TransactionStatusCode int    `json:"TransactionStatusCode"`
		TransactionRefNo      string `json:"TransactionRefNo"`
		Amount                float64 `json:"Amount"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, fmt.Errorf("poli capture decode: %w", err)
	}
	// Status 2 = completed successfully in POLi
	status := payment.PaymentFailed
	if resp.TransactionStatusCode == 2 {
		status = payment.PaymentSucceeded
	}
	return &payment.PaymentResult{ExternalID: resp.TransactionRefNo, Status: status, PaidAt: time.Now().UTC()}, nil
}

func (p *Provider) Refund(_ context.Context, _ string, _ int64) (*payment.RefundResult, error) {
	// POLi bank transfer payments cannot be refunded via API.
	// Refunds must be initiated manually by the merchant through the POLi portal.
	return nil, &payment.Error{
		Provider: "poli",
		Code:     "NOT_SUPPORTED",
		Message:  "POLi bank transfers cannot be refunded via API — initiate via the POLi merchant portal",
	}
}

func (p *Provider) HandleWebhook(_ context.Context, payload []byte, _ string) (*payment.WebhookEvent, error) {
	var evt struct {
		TransactionStatusCode int     `json:"TransactionStatusCode"`
		TransactionRefNo      string  `json:"TransactionRefNo"`
		Amount                float64 `json:"Amount"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil, fmt.Errorf("poli webhook decode: %w", err)
	}
	evtType := payment.WebhookPaymentFailed
	if evt.TransactionStatusCode == 2 {
		evtType = payment.WebhookPaymentSucceeded
	}
	return &payment.WebhookEvent{
		Type:        evtType,
		ExternalID:  evt.TransactionRefNo,
		AmountCents: int64(evt.Amount * 100),
		OccurredAt:  time.Now().UTC(),
	}, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*payment.HealthStatus, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.BaseURL+"/Merchant/GetMerchantDetails", nil)
	if err != nil {
		return &payment.HealthStatus{OK: false, Provider: "poli", Err: err.Error()}, nil
	}
	req.Header.Set("Authorization", "Basic "+p.auth())
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &payment.HealthStatus{OK: false, Provider: "poli", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &payment.HealthStatus{OK: true, Provider: "poli", Latency: latency}, nil
}
