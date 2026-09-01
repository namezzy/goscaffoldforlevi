package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/example/go-enterprise/internal/config"
)

// MetricsServer serves the Prometheus exposition on its own listener.
type MetricsServer struct {
	srv      *http.Server
	listener net.Listener
}

// NewMetricsServer opens the metrics listener, or returns (nil, "", nil) when
// metrics are disabled.
//
// The endpoint gets its own port rather than a route on the public router,
// because the three candidate placements are not equivalent:
//
//   - Public port. One misconfigured ingress rule from the internet. The
//     exposition describes the system in detail — every route that exists, how
//     many users have registered, how many logins are failing right now — and
//     that is reconnaissance handed over for free. It also sits behind the
//     public router's middleware, so a scrape consumes the rate limit budget
//     and, during an incident, the scraper is throttled at the moment the
//     numbers are needed.
//   - Admin port, alongside operator endpoints. Better, but it couples the
//     scrape to whatever authentication the admin surface grows, and every
//     Prometheus deployment then needs credentials to collect a counter.
//   - Its own internal port, bound to a private interface. The scrape path has
//     no shared middleware, no authentication to expire, and no route through
//     the public listener. Network policy is the access control, which is the
//     one control that is already required for a Prometheus deployment.
//
// The third is the default here and the recommendation for production: bind it
// to the pod or host network only, and let the scraper reach it by network
// policy. config.Load enforces the one mistake this cannot survive — binding
// metrics to the same port as the public API in production.
//
// The listener is opened here rather than inside Serve so that a busy port
// fails startup, where an orchestrator will see it. Discovered in a goroutine
// it is a log line in a process that reports itself healthy and exports
// nothing, which is the worst of the three outcomes: monitoring that is absent
// but believed present.
func NewMetricsServer(ctx context.Context, cfg config.Metrics, m *Metrics) (*MetricsServer, string, error) {
	if !cfg.Enabled {
		return nil, "", nil
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", cfg.Addr)
	if err != nil {
		return nil, "", fmt.Errorf("listen on metrics address %s: %w", cfg.Addr, err)
	}

	// A bare mux, not the application router. Nothing but the exposition is
	// served: an unauthenticated listener that answers other paths is a
	// standing invitation to mount a debug handler on it, and net/http/pprof
	// registers itself into whatever mux is at hand.
	mux := http.NewServeMux()
	mux.Handle(cfg.Path, m.Handler())

	return &MetricsServer{
		srv: &http.Server{
			Handler: mux,
			// A scrape is a local call; the timeouts exist so that a stuck
			// scraper cannot accumulate connections on the process it is
			// meant to be observing.
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		listener: listener,
	}, listener.Addr().String(), nil
}

// Serve blocks until the server is shut down.
func (s *MetricsServer) Serve() error {
	if err := s.srv.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown stops the server, waiting for an in-flight scrape.
func (s *MetricsServer) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
