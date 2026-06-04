// Package azblob implements storage.Provider for Azure Blob Storage.
// Preferred region: australiaeast (Sydney) for NZ data residency.
// Authentication: storage account shared-key (HMAC-SHA256).
// API docs: https://learn.microsoft.com/en-us/rest/api/storageservices/
package azblob

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/resilience"
	"github.com/PhillipC05/tpt-healthcare/core/storage"
)

func init() {
	storage.Register("azblob", func(ctx context.Context, v *viper.Viper) (storage.Provider, error) {
		return New(ctx, Config{
			AccountName:  v.GetString("storage.azblob.account_name"),
			AccountKey:   v.GetString("storage.azblob.account_key"),
			Container:    v.GetString("storage.azblob.container"),
			BaseEndpoint: v.GetString("storage.azblob.base_endpoint"),
		})
	})
}

// Config holds Azure Blob Storage credentials.
type Config struct {
	AccountName  string
	AccountKey   string // base64-encoded shared key
	Container    string
	BaseEndpoint string // default: https://<account>.blob.core.windows.net
}

// Provider implements storage.Provider for Azure Blob Storage.
type Provider struct {
	cfg        Config
	accountKey []byte
	client     *http.Client
	breaker    *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.AccountName == "" || cfg.AccountKey == "" {
		return nil, fmt.Errorf("azblob: account_name and account_key are required")
	}
	if cfg.Container == "" {
		return nil, fmt.Errorf("azblob: container is required")
	}
	if cfg.BaseEndpoint == "" {
		cfg.BaseEndpoint = fmt.Sprintf("https://%s.blob.core.windows.net", cfg.AccountName)
	}
	key, err := base64.StdEncoding.DecodeString(cfg.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("azblob: invalid account_key (must be base64): %w", err)
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "azblob"})
	return &Provider{cfg: cfg, accountKey: key, client: &http.Client{Timeout: 60 * time.Second}, breaker: reg}, nil
}

func (p *Provider) blobURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", strings.TrimRight(p.cfg.BaseEndpoint, "/"), p.cfg.Container, key)
}

func (p *Provider) Upload(ctx context.Context, key string, r io.Reader, opts storage.UploadOptions) (*storage.UploadResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("azblob upload read: %w", err)
	}

	var etag string
	err = resilience.Do(ctx, p.breaker, "azblob", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, p.blobURL(key), strings.NewReader(string(data)))
		if err != nil {
			return fmt.Errorf("azblob request: %w", err)
		}
		req.ContentLength = int64(len(data))
		req.Header.Set("x-ms-blob-type", "BlockBlob")
		req.Header.Set("Content-Type", opts.ContentType)
		req.Header.Set("x-ms-date", time.Now().UTC().Format(http.TimeFormat))
		req.Header.Set("x-ms-version", "2020-12-06")
		if opts.Encrypted {
			req.Header.Set("x-ms-meta-encrypted", "true")
		}
		p.sign(req, int64(len(data)))

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("azblob put http: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return &storage.Error{Provider: "azblob", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(body), Retryable: resp.StatusCode >= 500}
		}
		etag = strings.Trim(resp.Header.Get("ETag"), `"`)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &storage.UploadResult{Key: key, ETag: etag, SizeBytes: int64(len(data)), UploadedAt: time.Now().UTC()}, nil
}

func (p *Provider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.blobURL(key), nil)
	if err != nil {
		return nil, fmt.Errorf("azblob download request: %w", err)
	}
	req.Header.Set("x-ms-date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("x-ms-version", "2020-12-06")
	p.sign(req, 0)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azblob get http: %w", err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &storage.Error{Provider: "azblob", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(body)}
	}
	return resp.Body, nil
}

func (p *Provider) Delete(ctx context.Context, key string) error {
	return resilience.Do(ctx, p.breaker, "azblob", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.blobURL(key), nil)
		if err != nil {
			return err
		}
		req.Header.Set("x-ms-date", time.Now().UTC().Format(http.TimeFormat))
		req.Header.Set("x-ms-version", "2020-12-06")
		p.sign(req, 0)
		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("azblob delete http: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 && resp.StatusCode != 404 {
			body, _ := io.ReadAll(resp.Body)
			return &storage.Error{Provider: "azblob", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(body), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
}

func (p *Provider) SignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	// Azure SAS token generation (service SAS, signed with account key).
	now := time.Now().UTC()
	expiry2 := now.Add(expiry)
	sv := "2020-12-06"
	sr := "b" // blob resource
	sp := "r" // read permission
	st := now.Format("2006-01-02T15:04:05Z")
	se := expiry2.Format("2006-01-02T15:04:05Z")

	stringToSign := strings.Join([]string{
		sp, st, se,
		fmt.Sprintf("/%s/%s/%s/%s", "blob", p.cfg.AccountName, p.cfg.Container, key),
		"", "", "", sv, sr, "", "", "", "", "b", "",
	}, "\n")

	mac := hmac.New(sha256.New, p.accountKey)
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	_ = ctx
	return fmt.Sprintf("%s?sv=%s&sr=%s&sp=%s&st=%s&se=%s&sig=%s",
		p.blobURL(key), sv, sr, sp, st, se, sig), nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*storage.HealthStatus, error) {
	start := time.Now()
	url := fmt.Sprintf("%s/%s?restype=container", strings.TrimRight(p.cfg.BaseEndpoint, "/"), p.cfg.Container)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &storage.HealthStatus{OK: false, Provider: "azblob", Err: err.Error()}, nil
	}
	req.Header.Set("x-ms-date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("x-ms-version", "2020-12-06")
	p.sign(req, 0)
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &storage.HealthStatus{OK: false, Provider: "azblob", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &storage.HealthStatus{OK: true, Provider: "azblob", Latency: latency}, nil
}

// sign adds the Azure Shared Key authorisation header to req.
func (p *Provider) sign(req *http.Request, contentLength int64) {
	date := req.Header.Get("x-ms-date")
	ver := req.Header.Get("x-ms-version")
	contentType := req.Header.Get("Content-Type")

	// Collect x-ms-* headers in sorted order.
	var xmsHeaders []string
	for k, vs := range req.Header {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "x-ms-") {
			xmsHeaders = append(xmsHeaders, lk+":"+strings.Join(vs, ","))
		}
	}
	// Simple sort for deterministic signing.
	for i := 0; i < len(xmsHeaders); i++ {
		for j := i + 1; j < len(xmsHeaders); j++ {
			if xmsHeaders[i] > xmsHeaders[j] {
				xmsHeaders[i], xmsHeaders[j] = xmsHeaders[j], xmsHeaders[i]
			}
		}
	}

	canonicalHeaders := strings.Join(xmsHeaders, "\n")
	canonicalResource := fmt.Sprintf("/%s/%s%s", p.cfg.AccountName, p.cfg.Container, req.URL.Path)
	if rq := req.URL.RawQuery; rq != "" {
		canonicalResource += "\n" + strings.ReplaceAll(rq, "&", "\n")
	}

	cl := ""
	if contentLength > 0 {
		cl = fmt.Sprint(contentLength)
	}

	stringToSign := strings.Join([]string{
		req.Method, "", "", cl, "", contentType,
		"", "", "", "", "", "",
		canonicalHeaders, canonicalResource,
	}, "\n")

	mac := hmac.New(sha256.New, p.accountKey)
	mac.Write([]byte(stringToSign))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req.Header.Set("Authorization", fmt.Sprintf("SharedKey %s:%s", p.cfg.AccountName, sig))
	_ = date
	_ = ver
}
