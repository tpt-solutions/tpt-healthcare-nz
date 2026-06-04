package health

import (
	"context"
	"sync"
	"time"
)

// Checker is the interface each provider must satisfy to participate in
// the health aggregator. It mirrors the HealthCheck method on every provider
// interface in this codebase.
type Checker interface {
	HealthCheck(ctx context.Context) (*CheckResult, error)
}

// CheckResult is the normalised result returned by any provider's HealthCheck.
// It is structurally equivalent to the per-provider HealthStatus types so
// that the aggregator does not need to import each provider package.
type CheckResult struct {
	OK               bool
	ProviderName     string
	OrganisationName string
	Latency          time.Duration
	Err              string
}

// registration holds a Checker and its metadata.
type registration struct {
	providerType ProviderType
	checker      Checker
}

// Registry holds all registered provider health checkers.
// It is safe for concurrent use.
type Registry struct {
	mu      sync.RWMutex
	entries []registration
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds a checker for a provider. Multiple providers of the same type
// may be registered (e.g. all SMS backends for monitoring).
func (r *Registry) Register(providerType ProviderType, checker Checker) {
	r.mu.Lock()
	r.entries = append(r.entries, registration{providerType: providerType, checker: checker})
	r.mu.Unlock()
}

// CheckAll runs HealthCheck on every registered provider concurrently and
// returns a Report. Individual check failures are captured in the Status
// rather than propagating as errors.
func (r *Registry) CheckAll(ctx context.Context) *Report {
	r.mu.RLock()
	entries := make([]registration, len(r.entries))
	copy(entries, r.entries)
	r.mu.RUnlock()

	type result struct {
		s Status
	}
	ch := make(chan result, len(entries))
	for _, e := range entries {
		e := e
		go func() {
			s := runCheck(ctx, e)
			ch <- result{s: s}
		}()
	}

	statuses := make([]Status, 0, len(entries))
	for range entries {
		r := <-ch
		statuses = append(statuses, r.s)
	}

	allOK := true
	for _, s := range statuses {
		if !s.OK {
			allOK = false
			break
		}
	}
	return &Report{
		AllOK:       allOK,
		Providers:   statuses,
		GeneratedAt: time.Now().UTC(),
	}
}

func runCheck(ctx context.Context, e registration) Status {
	start := time.Now()
	res, err := e.checker.HealthCheck(ctx)
	latency := time.Since(start)

	s := Status{
		ProviderType:  e.providerType,
		LastCheckedAt: time.Now().UTC(),
		LatencyMs:     latency.Milliseconds(),
	}
	if err != nil {
		s.OK = false
		s.ErrorText = err.Error()
		return s
	}
	s.OK = res.OK
	s.ProviderName = res.ProviderName
	s.OrganisationName = res.OrganisationName
	s.ErrorText = res.Err
	return s
}
