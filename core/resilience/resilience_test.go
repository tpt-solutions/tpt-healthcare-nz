package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackoff(t *testing.T) {
	base := 100 * time.Millisecond
	maxDelay := 5 * time.Second

	t.Run("attempt 0 equals base", func(t *testing.T) {
		d := backoff(base, maxDelay, 0, 0)
		assert.Equal(t, base, d)
	})

	t.Run("exponential growth", func(t *testing.T) {
		d0 := backoff(base, maxDelay, 0, 0)
		d1 := backoff(base, maxDelay, 0, 1)
		d2 := backoff(base, maxDelay, 0, 2)
		assert.Equal(t, 2*base, d1)
		assert.Equal(t, 4*base, d2)
		assert.True(t, d0 < d1)
		assert.True(t, d1 < d2)
	})

	t.Run("capped at maxDelay", func(t *testing.T) {
		d := backoff(base, maxDelay, 0, 10)
		assert.LessOrEqual(t, d, maxDelay)
	})
}

func TestRetryConfig_WithDefaults(t *testing.T) {
	t.Run("zero-value gets defaults", func(t *testing.T) {
		cfg := RetryConfig{}.withDefaults()
		assert.Equal(t, 3, cfg.MaxAttempts)
		assert.Equal(t, 500*time.Millisecond, cfg.BaseDelay)
		assert.Equal(t, 30*time.Second, cfg.MaxDelay)
		assert.Equal(t, 0.2, cfg.JitterFactor)
		assert.NotNil(t, cfg.IsRetryable)
	})

	t.Run("partial keeps provided", func(t *testing.T) {
		cfg := RetryConfig{MaxAttempts: 5, BaseDelay: time.Second}.withDefaults()
		assert.Equal(t, 5, cfg.MaxAttempts)
		assert.Equal(t, time.Second, cfg.BaseDelay)
		assert.Equal(t, 30*time.Second, cfg.MaxDelay)
	})
}

func TestRetry_Success(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond}, func() error {
		attempts++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetry_FailThenSucceed(t *testing.T) {
	var attempts atomic.Int32
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond}, func() error {
		n := attempts.Add(1)
		if n < 3 {
			return errors.New("transient")
		}
		return nil
	})
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, int(attempts.Load()), 2)
}

func TestRetry_AllFail(t *testing.T) {
	err := Retry(context.Background(), RetryConfig{MaxAttempts: 2, BaseDelay: time.Millisecond}, func() error {
		return errors.New("permanent")
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permanent")
}

func TestRetry_NonRetryable(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   time.Millisecond,
		IsRetryable: func(err error) bool { return false },
	}, func() error {
		attempts++
		return errors.New("non-retryable")
	})
	assert.Error(t, err)
	assert.Equal(t, 1, attempts)
}

func TestRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Retry(ctx, RetryConfig{MaxAttempts: 3, BaseDelay: time.Second}, func() error {
		return errors.New("fail")
	})
	assert.Error(t, err)
}

func TestErrCircuitOpen_Error(t *testing.T) {
	err := &ErrCircuitOpen{Provider: "test"}
	assert.Contains(t, err.Error(), "test")
	assert.Contains(t, err.Error(), "circuit open")
}

func TestBreakerConfig_WithDefaults(t *testing.T) {
	cfg := BreakerConfig{}
	result := cfg.withDefaults()
	assert.Equal(t, uint32(5), result.MaxFailures)
	assert.Equal(t, 30*time.Second, result.Timeout)
	assert.Equal(t, uint32(2), result.SuccessThreshold)
}

func TestRegistry_Do_Success(t *testing.T) {
	r := NewRegistry()
	r.Register(BreakerConfig{Name: "test", MaxFailures: 3})

	err := r.Do(context.Background(), "test", func() error { return nil })
	assert.NoError(t, err)
	assert.Equal(t, "closed", r.State("test"))
}

func TestRegistry_Do_CircuitOpens(t *testing.T) {
	r := NewRegistry()
	r.Register(BreakerConfig{Name: "test", MaxFailures: 2, Timeout: time.Hour})

	for i := 0; i < 2; i++ {
		_ = r.Do(context.Background(), "test", func() error { return errors.New("fail") })
	}

	err := r.Do(context.Background(), "test", func() error { return nil })
	assert.Error(t, err)
	var cerr *ErrCircuitOpen
	assert.True(t, errors.As(err, &cerr))
}

func TestRegistry_State_Unknown(t *testing.T) {
	r := NewRegistry()
	assert.Equal(t, "unknown", r.State("nonexistent"))
}

func TestRegistry_Do_Unregistered(t *testing.T) {
	r := NewRegistry()
	err := r.Do(context.Background(), "auto-register", func() error { return nil })
	assert.NoError(t, err)
	assert.NotEqual(t, "unknown", r.State("auto-register"))
}
