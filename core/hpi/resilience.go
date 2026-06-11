package hpi

import (
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

// UseResilience wraps the client's HTTP transport with circuit-breaking and
// exponential-backoff retry via the supplied registry.
func (c *Client) UseResilience(registry *resilience.Registry, cfg resilience.RetryConfig) {
	c.httpClient.Transport = resilience.NewTransport(
		c.httpClient.Transport, registry, "hpi", cfg)
}
