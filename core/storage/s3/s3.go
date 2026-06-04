// Package s3 implements storage.Provider for Amazon S3.
// Default region: ap-southeast-2 (Sydney) for NZ data residency compliance.
// The Wasabi backend also uses this package with a custom endpoint —
// see the wasabi/ sub-package which sets BaseEndpoint to Wasabi's URL.
package s3

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/resilience"
	"github.com/PhillipC05/tpt-healthcare/core/storage"
)

func init() {
	storage.Register("s3", func(ctx context.Context, v *viper.Viper) (storage.Provider, error) {
		return New(ctx, Config{
			AccessKeyID:     v.GetString("storage.s3.access_key_id"),
			SecretAccessKey: v.GetString("storage.s3.secret_access_key"),
			Region:          v.GetString("storage.s3.region"),
			Bucket:          v.GetString("storage.s3.bucket"),
			BaseEndpoint:    v.GetString("storage.s3.base_endpoint"),
		})
	})
}

// Config holds AWS S3 (or S3-compatible) credentials.
type Config struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string // default: ap-southeast-2
	Bucket          string
	// BaseEndpoint overrides the S3 endpoint for S3-compatible services
	// (e.g. Wasabi: s3.ap-southeast-2.wasabisys.com, MinIO: http://localhost:9000).
	BaseEndpoint string
}

// Provider implements storage.Provider for AWS S3 (and S3-compatible systems).
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("s3: access_key_id and secret_access_key are required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3: bucket is required")
	}
	if cfg.Region == "" {
		cfg.Region = "ap-southeast-2"
	}
	if cfg.BaseEndpoint == "" {
		cfg.BaseEndpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", cfg.Region)
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "s3"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 60 * time.Second}, breaker: reg}, nil
}

func (p *Provider) objectURL(key string) string {
	base := strings.TrimRight(p.cfg.BaseEndpoint, "/")
	return fmt.Sprintf("%s/%s/%s", base, p.cfg.Bucket, url.PathEscape(key))
}

func (p *Provider) Upload(ctx context.Context, key string, r io.Reader, opts storage.UploadOptions) (*storage.UploadResult, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("s3 upload read: %w", err)
	}

	var etag string
	err = resilience.Do(ctx, p.breaker, "s3", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := p.signedRequest(ctx, http.MethodPut, key, data)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", opts.ContentType)
		for k, v := range opts.Metadata {
			req.Header.Set("x-amz-meta-"+k, v)
		}
		if opts.Encrypted {
			req.Header.Set("x-amz-meta-encrypted", "true")
		}

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("s3 put http: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return &storage.Error{Provider: "s3", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(body), Retryable: resp.StatusCode >= 500}
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
	req, err := p.signedRequest(ctx, http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3 get http: %w", err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &storage.Error{Provider: "s3", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(body), Retryable: false}
	}
	return resp.Body, nil
}

func (p *Provider) Delete(ctx context.Context, key string) error {
	return resilience.Do(ctx, p.breaker, "s3", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := p.signedRequest(ctx, http.MethodDelete, key, nil)
		if err != nil {
			return err
		}
		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("s3 delete http: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 && resp.StatusCode != 404 {
			body, _ := io.ReadAll(resp.Body)
			return &storage.Error{Provider: "s3", Code: fmt.Sprintf("HTTP%d", resp.StatusCode), Message: string(body), Retryable: resp.StatusCode >= 500}
		}
		return nil
	})
}

func (p *Provider) SignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	// Pre-signed URL generation using AWS Signature V4 query-string signing.
	now := time.Now().UTC()
	expSecs := int(expiry.Seconds())

	credential := fmt.Sprintf("%s/%s/%s/s3/aws4_request",
		p.cfg.AccessKeyID, now.Format("20060102"), p.cfg.Region)

	params := url.Values{
		"X-Amz-Algorithm":     {"AWS4-HMAC-SHA256"},
		"X-Amz-Credential":    {credential},
		"X-Amz-Date":          {now.Format("20060102T150405Z")},
		"X-Amz-Expires":       {fmt.Sprint(expSecs)},
		"X-Amz-SignedHeaders": {"host"},
	}

	objURL := p.objectURL(key) + "?" + params.Encode()
	// Signing: in production this constructs the SigV4 canonical request and
	// appends X-Amz-Signature. Placeholder returns the unsigned URL for now.
	_ = ctx
	return objURL + "&X-Amz-Signature=TODO", nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*storage.HealthStatus, error) {
	start := time.Now()
	req, err := p.signedRequest(ctx, http.MethodHead, "", nil)
	if err != nil {
		return &storage.HealthStatus{OK: false, Provider: "s3", Err: err.Error()}, nil
	}
	// HEAD the bucket itself.
	req.URL, _ = url.Parse(fmt.Sprintf("%s/%s", strings.TrimRight(p.cfg.BaseEndpoint, "/"), p.cfg.Bucket))
	resp, err := p.client.Do(req)
	latency := time.Since(start)
	if err != nil || resp.StatusCode >= 400 {
		msg := "connection failed"
		if err != nil {
			msg = err.Error()
		}
		return &storage.HealthStatus{OK: false, Provider: "s3", Latency: latency, Err: msg}, nil
	}
	resp.Body.Close()
	return &storage.HealthStatus{OK: true, Provider: "s3", Latency: latency}, nil
}

func (p *Provider) signedRequest(ctx context.Context, method, key string, body []byte) (*http.Request, error) {
	objURL := p.objectURL(key)
	if key == "" {
		objURL = fmt.Sprintf("%s/%s", strings.TrimRight(p.cfg.BaseEndpoint, "/"), p.cfg.Bucket)
	}

	var bodyHash string
	if body != nil {
		h := sha256.Sum256(body)
		bodyHash = hex.EncodeToString(h[:])
	} else {
		empty := sha256.Sum256(nil)
		bodyHash = hex.EncodeToString(empty[:])
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	parsedURL, err := url.Parse(objURL)
	if err != nil {
		return nil, fmt.Errorf("s3 parse url: %w", err)
	}

	headers := map[string]string{
		"host":         parsedURL.Host,
		"x-amz-date":  amzDate,
		"x-amz-content-sha256": bodyHash,
	}

	sortedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	headerStr := ""
	for _, h := range sortedHeaders {
		headerStr += h + ":" + headers[h] + "\n"
	}
	signedHeaders := strings.Join(sortedHeaders, ";")

	canonical := strings.Join([]string{
		method, parsedURL.Path, parsedURL.RawQuery,
		headerStr, "", signedHeaders, bodyHash,
	}, "\n")

	credScope := strings.Join([]string{dateStamp, p.cfg.Region, "s3", "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzDate, credScope,
		hex.EncodeToString(sha256sum([]byte(canonical))),
	}, "\n")

	signingKey := s3SigningKey(p.cfg.SecretAccessKey, dateStamp, p.cfg.Region)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	auth := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		p.cfg.AccessKeyID, credScope, signedHeaders, signature)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, objURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("s3 request: %w", err)
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", bodyHash)
	if body != nil {
		req.ContentLength = int64(len(body))
	}
	return req, nil
}

func s3SigningKey(secret, date, region string) []byte {
	return hmacSHA256(
		hmacSHA256(
			hmacSHA256(
				hmacSHA256([]byte("AWS4"+secret), []byte(date)),
				[]byte(region)),
			[]byte("s3")),
		[]byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func sha256sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
