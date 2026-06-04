// Package mdns advertises the interop server on the local network via mDNS-SD.
//
// This allows clinic devices to discover the on-premise server automatically
// without hard-coding IP addresses. The service is registered as
// _tpt-interop._tcp.local., so browsers probing http://tpt-interop.local:PORT
// will resolve to this host on mDNS-capable operating systems (macOS, iOS,
// Linux with Avahi, Windows with Bonjour).
//
// Usage: call Advertise in a goroutine from the serve command, passing the
// server context so it shuts down cleanly on SIGINT/SIGTERM.
package mdns

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/grandcat/zeroconf"
)

const (
	instanceName = "tpt-interop"
	serviceType  = "_tpt-interop._tcp"
	domain       = "local."
)

// Advertise registers the interop server via mDNS-SD and blocks until ctx
// is cancelled, then de-registers the service cleanly.
func Advertise(ctx context.Context, logger *slog.Logger, port int) error {
	server, err := zeroconf.Register(
		instanceName,
		serviceType,
		domain,
		port,
		[]string{"version=1", "app=tpt-healthcare"},
		nil, // use all available interfaces
	)
	if err != nil {
		return fmt.Errorf("mdns: register service: %w", err)
	}

	logger.Info("mDNS advertisement active",
		"service", instanceName+"."+serviceType+"."+domain,
		"port", port,
	)

	<-ctx.Done()
	server.Shutdown()
	logger.Info("mDNS advertisement stopped")
	return nil
}
