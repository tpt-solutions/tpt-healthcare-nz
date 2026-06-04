// Package ses implements email.Provider for AWS Simple Email Service (SES).
// Region ap-southeast-2 (Sydney) is preferred for NZ data residency.
// Sandbox: verify sender email in SES console; sandbox mode restricts recipients.
// API docs: https://docs.aws.amazon.com/ses/latest/APIReference/API_SendEmail.html
package ses

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/email"
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

func init() {
	email.Register("ses", func(ctx context.Context, v *viper.Viper) (email.Provider, error) {
		return New(ctx, Config{
			AccessKeyID:     v.GetString("email.ses.access_key_id"),
			SecretAccessKey: v.GetString("email.ses.secret_access_key"),
			Region:          v.GetString("email.ses.region"),
			FromAddress:     v.GetString("email.ses.from_address"),
			ConfigurationSet: v.GetString("email.ses.configuration_set"),
		})
	})
}

// Config holds AWS SES credentials.
type Config struct {
	AccessKeyID      string
	SecretAccessKey  string
	Region           string // default: ap-southeast-2
	FromAddress      string
	ConfigurationSet string // optional SES configuration set name
}

// Provider implements email.Provider for AWS SES.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("ses: access_key_id and secret_access_key are required")
	}
	if cfg.FromAddress == "" {
		return nil, fmt.Errorf("ses: from_address is required")
	}
	if cfg.Region == "" {
		cfg.Region = "ap-southeast-2"
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "ses"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}, breaker: reg}, nil
}

func (p *Provider) endpoint() string {
	return fmt.Sprintf("https://email.%s.amazonaws.com/v2/email/outbound-emails", p.cfg.Region)
}

func (p *Provider) Send(ctx context.Context, msg email.Message) (*email.SendResult, error) {
	from := p.cfg.FromAddress
	if msg.From != "" {
		from = msg.From
	}

	toAddrs := make([]string, len(msg.To))
	copy(toAddrs, msg.To)

	body := map[string]any{
		"FromEmailAddress": from,
		"Destination": map[string]any{
			"ToAddresses": toAddrs,
		},
		"Content": map[string]any{
			"Simple": map[string]any{
				"Subject": map[string]any{"Data": msg.Subject},
				"Body": map[string]any{
					"Text": map[string]any{"Data": msg.TextBody},
					"Html": map[string]any{"Data": msg.HTMLBody},
				},
			},
		},
	}
	if p.cfg.ConfigurationSet != "" {
		body["ConfigurationSetName"] = p.cfg.ConfigurationSet
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ses marshal: %w", err)
	}

	var result []byte
	err = resilience.Do(ctx, p.breaker, "ses", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := p.signedRequest(ctx, b)
		if err != nil {
			return err
		}
		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("ses http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("ses read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &email.Error{Provider: "ses", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(result), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var resp struct {
		MessageId string `json:"MessageId"`
	}
	_ = json.Unmarshal(result, &resp)
	return &email.SendResult{ExternalID: resp.MessageId, QueuedAt: time.Now().UTC()}, nil
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
	body := []byte(`{"MaxItems":1}`)
	req, _ := p.signedRequest(ctx, body)
	if req != nil {
		req.URL, _ = req.URL.Parse(fmt.Sprintf("https://email.%s.amazonaws.com/v2/email/identities?PageSize=1", p.cfg.Region))
		req.Method = http.MethodGet
		req.Body = nil
		req.ContentLength = 0
	}
	if req == nil {
		return &email.HealthStatus{OK: false, Provider: "ses", Err: "failed to build request"}, nil
	}
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &email.HealthStatus{OK: false, Provider: "ses", Latency: latency, Err: msg}, nil
	}
	if resp != nil {
		resp.Body.Close()
	}
	return &email.HealthStatus{OK: true, Provider: "ses", Latency: latency}, nil
}

// signedRequest builds an AWS SigV4-signed HTTP POST to the SES v2 endpoint.
func (p *Provider) signedRequest(ctx context.Context, body []byte) (*http.Request, error) {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	service := "ses"
	region := p.cfg.Region

	bodyHash := sha256Hex(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ses build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", req.Host)

	// Canonical request
	signedHeaders := "content-type;host;x-amz-date"
	canonical := strings.Join([]string{
		http.MethodPost,
		req.URL.Path,
		"",
		"content-type:" + req.Header.Get("Content-Type"),
		"host:" + req.URL.Host,
		"x-amz-date:" + amzDate,
		"",
		signedHeaders,
		bodyHash,
	}, "\n")

	// String to sign
	credScope := strings.Join([]string{dateStamp, region, service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{"AWS4-HMAC-SHA256", amzDate, credScope, sha256Hex([]byte(canonical))}, "\n")

	// Signing key
	signingKey := hmacSHA256(
		hmacSHA256(
			hmacSHA256(
				hmacSHA256([]byte("AWS4"+p.cfg.SecretAccessKey), []byte(dateStamp)),
				[]byte(region)),
			[]byte(service)),
		[]byte("aws4_request"))

	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
	auth := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		p.cfg.AccessKeyID, credScope, signedHeaders, signature,
	)
	req.Header.Set("Authorization", auth)
	return req, nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// _ suppresses unused import warning.
var _ = base64.StdEncoding
