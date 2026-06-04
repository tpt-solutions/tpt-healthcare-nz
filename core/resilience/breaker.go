// Package resilience provides circuit breaker and retry primitives for
// external provider calls (accounting, payroll, SMS, email, etc.).
// Every provider backend should wrap outbound HTTP calls with Do().
package resilience

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// BreakerConfig holds thresholds for a single circuit breaker.
type BreakerConfig struct {
	// Name is the provider name used for logging and metrics (e.g. "xero", "twilio").
	Name string

	// MaxFailures is the number of consecutive failures before the circuit opens.
	// Default: 5.
	MaxFailures uint32

	// Timeout is how long the circuit stays open before moving to half-open.
	// Default: 30s.
	Timeout time.Duration

	// SuccessThreshold is the number of consecutive successes in half-open
	// required to close the circuit again.
	// Default: 2.
	SuccessThreshold uint32
}

func (c *BreakerConfig) withDefaults() BreakerConfig {
	if c.MaxFailures == 0 {
		c.MaxFailures = 5
	}
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
	}
	if c.SuccessThreshold == 0 {
		c.SuccessThreshold = 2
	}
	return *c
}

// ErrCircuitOpen is returned when the circuit breaker is in the open state.
type ErrCircuitOpen struct {
	Provider string
}

func (e *ErrCircuitOpen) Error() string {
	return fmt.Sprintf("resilience: circuit open for provider %q — too many recent failures", e.Provider)
}

// Registry maintains a named set of circuit breakers, one per provider.
// It is safe for concurrent use.
type Registry struct {
	mu       sync.RWMutex
	breakers map[string]*gobreaker.CircuitBreaker
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{breakers: make(map[string]*gobreaker.CircuitBreaker)}
}

// Register adds (or replaces) a breaker for the named provider.
func (r *Registry) Register(cfg BreakerConfig) {
	cfg = cfg.withDefaults()
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.SuccessThreshold,
		Interval:    0, // count window resets when circuit closes
		Timeout:     cfg.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cfg.MaxFailures
		},
	})
	r.mu.Lock()
	r.breakers[cfg.Name] = cb
	r.mu.Unlock()
}

// Do executes fn through the named circuit breaker.
// If the circuit is open it returns ErrCircuitOpen immediately without
// calling fn. If fn returns an error, the failure is counted.
func (r *Registry) Do(ctx context.Context, provider string, fn func() error) error {
	r.mu.RLock()
	cb, ok := r.breakers[provider]
	r.mu.RUnlock()

	if !ok {
		// No breaker configured — register with defaults and proceed.
		r.Register(BreakerConfig{Name: provider})
		return r.Do(ctx, provider, fn)
	}

	_, err := cb.Execute(func() (any, error) {
		// Honour context cancellation even inside the breaker.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, fn()
	})

	if err == gobreaker.ErrOpenState || err == gobreaker.ErrTooManyRequests {
		return &ErrCircuitOpen{Provider: provider}
	}
	return err
}

// State returns the current state of the named breaker ("closed", "half-open", "open").
// Returns "unknown" if no breaker is registered for the name.
func (r *Registry) State(provider string) string {
	r.mu.RLock()
	cb, ok := r.breakers[provider]
	r.mu.RUnlock()
	if !ok {
		return "unknown"
	}
	switch cb.State() {
	case gobreaker.StateClosed:
		return "closed"
	case gobreaker.StateHalfOpen:
		return "half-open"
	case gobreaker.StateOpen:
		return "open"
	default:
		return "unknown"
	}
}
