package resilience

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"time"
)

// RetryConfig controls the retry behaviour for a single call.
type RetryConfig struct {
	// MaxAttempts is the total number of attempts (including the first).
	// Default: 3.
	MaxAttempts int

	// BaseDelay is the initial backoff duration. Doubled on each attempt.
	// Default: 500ms.
	BaseDelay time.Duration

	// MaxDelay caps the per-attempt wait regardless of the backoff factor.
	// Default: 30s.
	MaxDelay time.Duration

	// JitterFactor adds up to JitterFactor*delay of random jitter to prevent
	// thundering-herd when many callers retry simultaneously.
	// Default: 0.2 (±20%).
	JitterFactor float64

	// IsRetryable decides whether a given error should be retried.
	// Defaults to retrying all non-nil errors.
	IsRetryable func(err error) bool
}

func (c RetryConfig) withDefaults() RetryConfig {
	if c.MaxAttempts == 0 {
		c.MaxAttempts = 3
	}
	if c.BaseDelay == 0 {
		c.BaseDelay = 500 * time.Millisecond
	}
	if c.MaxDelay == 0 {
		c.MaxDelay = 30 * time.Second
	}
	if c.JitterFactor == 0 {
		c.JitterFactor = 0.2
	}
	if c.IsRetryable == nil {
		c.IsRetryable = func(err error) bool { return err != nil }
	}
	return c
}

// Retry executes fn up to cfg.MaxAttempts times, pausing between attempts
// with exponential backoff + jitter. It stops and returns the last error if
// the context is cancelled, if the error is not retryable, or if all attempts
// are exhausted.
func Retry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	cfg = cfg.withDefaults()
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !cfg.IsRetryable(lastErr) {
			return lastErr
		}
		if attempt == cfg.MaxAttempts-1 {
			break
		}
		wait := backoff(cfg.BaseDelay, cfg.MaxDelay, cfg.JitterFactor, attempt)
		select {
		case <-ctx.Done():
			return errors.Join(lastErr, ctx.Err())
		case <-time.After(wait):
		}
	}
	return lastErr
}

// Do combines circuit breaking and retry: it calls the registry breaker for
// provider, and within that applies Retry with cfg. This is the primary entry
// point for provider backends.
//
// Typical usage inside a backend:
//
//	err := resilience.Do(ctx, registry, "xero", RetryConfig{MaxAttempts: 3}, func() error {
//	    return callXeroAPI(ctx, ...)
//	})
func Do(ctx context.Context, registry *Registry, provider string, cfg RetryConfig, fn func() error) error {
	return registry.Do(ctx, provider, func() error {
		return Retry(ctx, cfg, fn)
	})
}

// backoff returns the wait duration for attempt i (zero-indexed) using
// exponential backoff with jitter, capped at maxDelay.
func backoff(base, maxDelay time.Duration, jitterFactor float64, attempt int) time.Duration {
	exp := math.Pow(2, float64(attempt))
	d := time.Duration(float64(base) * exp)
	if d > maxDelay {
		d = maxDelay
	}
	// Add ±jitterFactor of jitter.
	jitter := time.Duration(float64(d) * jitterFactor * (rand.Float64()*2 - 1))
	d += jitter
	if d < 0 {
		d = 0
	}
	return d
}
