package resilience

import (
	"fmt"
	"net/http"
)

// Transport is an http.RoundTripper that applies a circuit breaker and
// exponential-backoff retry to every outbound HTTP call. It is intended to
// be set as the Transport on an *http.Client so that all requests from that
// client automatically benefit from resilience without modifying individual
// call sites.
//
// Retry policy: network errors and 5xx responses are retried; 4xx responses
// are not (they represent caller errors that retrying cannot fix).
//
// Usage:
//
//	client.Transport = resilience.NewTransport(registry, "nhi",
//	    resilience.RetryConfig{MaxAttempts: 3})
type Transport struct {
	inner    http.RoundTripper
	registry *Registry
	provider string
	cfg      RetryConfig
}

// NewTransport wraps inner (or http.DefaultTransport if inner is nil) with a
// circuit-breaking, retrying transport for the named provider.
func NewTransport(inner http.RoundTripper, registry *Registry, provider string, cfg RetryConfig) *Transport {
	if inner == nil {
		inner = http.DefaultTransport
	}
	cfg = cfg.withDefaults()
	cfg.IsRetryable = func(err error) bool {
		// Retry on non-nil errors (network failures are wrapped errors, not *httpStatusError).
		return err != nil
	}
	return &Transport{
		inner:    inner,
		registry: registry,
		provider: provider,
		cfg:      cfg,
	}
}

// RoundTrip executes the request through the circuit breaker and retry loop.
// Bodies are reset between retries using req.GetBody (set automatically by
// http.NewRequestWithContext when the body is a *bytes.Reader or *strings.Reader).
// Requests without GetBody (e.g. streaming bodies) are not retried on 5xx.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	attempt := 0
	err := Do(req.Context(), t.registry, t.provider, t.cfg, func() error {
		// On retries, reset the request body if GetBody is available.
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return fmt.Errorf("reset request body: %w", err)
			}
			req.Body = body
		}
		attempt++

		var doErr error
		resp, doErr = t.inner.RoundTrip(req)
		if doErr != nil {
			return doErr
		}
		// Treat 5xx as retryable transient failures.
		if resp.StatusCode >= 500 {
			status := resp.StatusCode
			_ = resp.Body.Close()
			resp = nil
			return &httpStatusError{status: status}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// httpStatusError carries an HTTP status code as an error, used internally by
// Transport to signal retryable 5xx responses.
type httpStatusError struct{ status int }

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("HTTP %d", e.status)
}
