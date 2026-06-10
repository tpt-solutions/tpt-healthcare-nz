package nhi

import (
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

// UseResilience wraps the client's HTTP transport with circuit-breaking and
// exponential-backoff retry via the supplied registry. Call this immediately
// after New() before the client is used.
//
//	c := nhi.New(baseURL, tokenFunc)
//	c.UseResilience(registry, resilience.RetryConfig{MaxAttempts: 3})
func (c *Client) UseResilience(registry *resilience.Registry, cfg resilience.RetryConfig) {
	c.httpClient.Transport = resilience.NewTransport(
		c.httpClient.Transport, registry, "nhi", cfg)
}
