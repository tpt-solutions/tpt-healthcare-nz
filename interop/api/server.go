// Package api implements the HTTP server and route handlers for the
// tpt-health-interop interoperability gateway.
package api

import (
	"context"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	coremigrate "github.com/PhillipC05/tpt-healthcare/core/db/migrate"
	"github.com/PhillipC05/tpt-healthcare/core/events"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/nhi"
	"github.com/PhillipC05/tpt-healthcare/core/push"
	corequeue "github.com/PhillipC05/tpt-healthcare/core/queue"
	"github.com/PhillipC05/tpt-healthcare/core/repo"
	"github.com/PhillipC05/tpt-healthcare/core/subscription"
	"github.com/PhillipC05/tpt-healthcare/core/tenant"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationsFS embeds the interop SQL migration files.
// The "all:" prefix ensures the directory is included even when it contains
// only non-Go files (e.g. during initial scaffolding).
//
//go:embed all:migrations
var migrationsFS embed.FS

// Config holds configuration for the HTTP server.
type Config struct {
	// Port is the TCP port to listen on.
	Port int
	// Host is the bind address (e.g. "0.0.0.0" or "127.0.0.1").
	Host string
	// CORSOrigins is the list of allowed CORS origins. Use ["*"] to allow all.
	CORSOrigins []string
	// RateLimit is the per-IP request rate (requests per second).
	RateLimit float64
}

// Server is the interop HTTP gateway. It wires together all route handlers
// and the shared middleware chain.
type Server struct {
	cfg            Config
	pool           *pgxpool.Pool
	auditTrail     *audit.Trail
	authProvider   auth.Provider
	nhiClient      *nhi.Client
	tenantStore    tenant.Store
	router         *http.ServeMux
	// Queue & real-time
	queueService   *corequeue.Service
	queueRepo      corequeue.Repository
	sseHub         *SSEHub
	reminderWorker *corequeue.ReminderWorker
	// Push notifications
	pushStore      *push.Store
	pushNotifier   *push.Notifier
	vapidPublicKey string
	// Event bus + subscription engine
	eventBus       *events.Bus
	subEngine      *subscription.Engine
	logger         *slog.Logger
	// FHIR resource repository and terminology store
	fhirStore repo.Store
	termStore TermStore
}

// New initialises a Server with the provided dependencies. Passing nil for
// optional dependencies (pool, auditTrail, authProvider, nhiClient) is
// allowed during testing; production callers should supply all four.
func New(cfg Config, opts ...ServerOption) *Server {
	s := &Server{
		cfg:    cfg,
		router: http.NewServeMux(),
		logger: slog.Default(),
	}
	for _, o := range opts {
		o(s)
	}
	// Wire the event bus to the subscription engine if both are present.
	if s.eventBus != nil && s.subEngine != nil {
		subscription.WireEventBus(s.eventBus, s.subEngine, s.logger)
	}
	// Initialise the SSE hub.
	s.sseHub = NewSSEHub(s.logger)
	s.registerRoutes()
	return s
}

// ServerOption is a functional option for Server construction.
type ServerOption func(*Server)

// WithPool attaches a pgxpool.Pool to the server.
func WithPool(p *pgxpool.Pool) ServerOption {
	return func(s *Server) { s.pool = p }
}

// WithAuditTrail attaches an audit.Trail to the server.
func WithAuditTrail(t *audit.Trail) ServerOption {
	return func(s *Server) { s.auditTrail = t }
}

// WithAuthProvider attaches an auth.Provider to the server.
func WithAuthProvider(p auth.Provider) ServerOption {
	return func(s *Server) { s.authProvider = p }
}

// WithNHIClient attaches a nhi.Client to the server.
func WithNHIClient(c *nhi.Client) ServerOption {
	return func(s *Server) { s.nhiClient = c }
}

// WithTenantStore attaches a tenant.Store to the server.
func WithTenantStore(ts tenant.Store) ServerOption {
	return func(s *Server) { s.tenantStore = ts }
}

// WithQueueService attaches a queue.Service and Repository to the server.
func WithQueueService(svc *corequeue.Service, repo corequeue.Repository) ServerOption {
	return func(s *Server) { s.queueService = svc; s.queueRepo = repo }
}

// WithPushStore attaches a push.Store and Notifier to the server.
func WithPushStore(store *push.Store, notifier *push.Notifier, vapidPublicKey string) ServerOption {
	return func(s *Server) {
		s.pushStore = store
		s.pushNotifier = notifier
		s.vapidPublicKey = vapidPublicKey
	}
}

// WithEventBus attaches the domain event bus.
func WithEventBus(bus *events.Bus) ServerOption {
	return func(s *Server) { s.eventBus = bus }
}

// WithSubscriptionEngine attaches the FHIR subscription engine.
func WithSubscriptionEngine(engine *subscription.Engine) ServerOption {
	return func(s *Server) { s.subEngine = engine }
}

// WithReminderWorker attaches the appointment reminder background worker.
func WithReminderWorker(w *corequeue.ReminderWorker) ServerOption {
	return func(s *Server) { s.reminderWorker = w }
}

// WithLogger sets the structured logger.
func WithLogger(l *slog.Logger) ServerOption {
	return func(s *Server) { s.logger = l }
}

// WithFHIRStore attaches a repo.Store used to persist FHIR resources.
// If not supplied, the server derives a PostgresStore from the attached
// pool (see WithPool), or falls back to an in-memory store if no pool is set.
func WithFHIRStore(store repo.Store) ServerOption {
	return func(s *Server) { s.fhirStore = store }
}

// WithTermStore attaches a TermStore used to serve terminology lookups.
// If not supplied, the terminology handler falls back to a no-op stub.
func WithTermStore(store TermStore) ServerOption {
	return func(s *Server) { s.termStore = store }
}

// Start begins listening for HTTP requests and blocks until ctx is cancelled,
// at which point it performs a graceful shutdown with a 30-second deadline.
func (s *Server) Start(ctx context.Context) error {
	addr := net.JoinHostPort(s.cfg.Host, fmt.Sprintf("%d", s.cfg.Port))
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.buildChain(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start listening before blocking on context cancellation so we can
	// surface bind errors immediately.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("server: listen %s: %w", addr, err)
	}

	// Start background workers.
	if s.reminderWorker != nil {
		go s.reminderWorker.Run(ctx)
	}
	if s.subEngine != nil {
		go func() {
			if err := s.subEngine.Start(ctx); err != nil {
				log.Printf("subscription engine: %v", err)
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("server: listening on %s", addr)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Println("server: shutting down gracefully…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server: shutdown: %w", err)
	}
	return <-errCh
}

// buildChain wraps the router with the standard middleware stack.
// Order (outermost → innermost):
//
//	RecoveryMiddleware → RateLimit → CORS → AuditWrap
//
// TenantExtraction is NOT applied globally; it is applied per route group in
// registerRoutes so that public endpoints (health, onboarding) can opt out.
// Auth is enforced per-route via withAuth / withAdminAuth helpers.
func (s *Server) buildChain() http.Handler {
	// Burst is set to 3× the per-second rate, minimum 10.
	burst := int(s.cfg.RateLimit * 3)
	if burst < 10 {
		burst = 10
	}

	var handler http.Handler = s.router

	// Innermost applied last (wraps outward).
	if s.auditTrail != nil {
		handler = middleware.AuditWrap(s.auditTrail)(handler)
	}
	handler = middleware.CORS(s.cfg.CORSOrigins)(handler)
	handler = middleware.RateLimit(s.cfg.RateLimit, burst)(handler)
	handler = middleware.RecoveryMiddleware()(handler)

	return handler
}

// withAuth wraps an http.Handler with auth.RequireAuth when an auth provider
// is configured, otherwise returns the handler unchanged (useful in tests).
func (s *Server) withAuth(h http.Handler) http.Handler {
	if s.authProvider == nil {
		return h
	}
	return auth.RequireAuth(s.authProvider)(h)
}

// withTenant wraps a handler with TenantExtraction middleware. Apply this to
// any route that requires a scoped tenant context (all clinical routes).
func (s *Server) withTenant(h http.Handler) http.Handler {
	return middleware.TenantExtraction()(h)
}

// withAdminAuth wraps a handler requiring auth + network_admin role. No tenant
// extraction — admin routes operate at the network level, not per-clinic.
func (s *Server) withAdminAuth(h http.Handler) http.Handler {
	if s.authProvider == nil {
		return h
	}
	return auth.RequireAuth(s.authProvider)(auth.RequireRole("network_admin")(h))
}

// registerRoutes mounts all route groups on the server's mux.
func (s *Server) registerRoutes() {
	// Health / readiness probes — no auth, no tenant required.
	s.router.HandleFunc("/health", s.handleHealth)
	s.router.HandleFunc("/ready", s.handleReady)

	// Clinic onboarding — public self-registration (no auth, no tenant).
	if s.tenantStore != nil {
		ob := newOnboardingHandler(s.tenantStore)
		s.router.Handle("/api/v1/onboarding/", ob.publicRouter())

		// Network-admin application management (auth + network_admin role, no tenant).
		s.router.Handle("/api/v1/admin/", s.withAdminAuth(ob.adminRouter()))
	}

	// FHIR resource repository: prefer an explicitly attached store, else
	// derive one from the DB pool, else fall back to an in-memory store.
	fhirStore := s.fhirStore
	if fhirStore == nil {
		if s.pool != nil {
			fhirStore = repo.NewPostgresStore(s.pool)
		} else {
			fhirStore = repo.NewMemoryStore()
		}
	}

	// FHIR R5 — tenant-scoped, auth required.
	fhirR5 := newFHIRHandler(fhirVersionR5, fhirStore)
	s.router.Handle("/fhir/r5/", s.withTenant(s.withAuth(http.StripPrefix("/fhir/r5", fhirR5.router()))))

	// FHIR R4 — tenant-scoped, auth required.
	fhirR4 := newFHIRHandler(fhirVersionR4, fhirStore)
	s.router.Handle("/fhir/r4/", s.withTenant(s.withAuth(http.StripPrefix("/fhir/r4", fhirR4.router()))))

	// NHI — tenant-scoped, auth required.
	nhiH := newNHIHandler(s.nhiClient, s.auditTrail)
	s.router.Handle("/api/v1/nhi/", s.withTenant(s.withAuth(nhiH.router())))

	// Terminology — tenant-scoped, auth required.
	termH := newTerminologyHandler(s.termStore)
	s.router.Handle("/api/v1/terminology/", s.withTenant(s.withAuth(termH.router())))

	// Subscriptions — tenant-scoped, auth required.
	subH := newSubscriptionHandler()
	s.router.Handle("/api/v1/subscriptions", s.withTenant(s.withAuth(subH.router())))
	s.router.Handle("/api/v1/subscriptions/", s.withTenant(s.withAuth(subH.router())))

	// Queue — tenant-scoped, auth required.
	if s.queueService != nil {
		s.router.Handle("POST /api/v1/queue",
			s.withTenant(s.withAuth(http.HandlerFunc(s.handleCreateQueue))))
		s.router.Handle("GET /api/v1/queue/{queueID}",
			s.withTenant(s.withAuth(http.HandlerFunc(s.handleQueueGetEntries))))
		s.router.Handle("POST /api/v1/queue/{queueID}/check-in",
			s.withTenant(s.withAuth(http.HandlerFunc(s.handleQueueCheckIn))))
		s.router.Handle("POST /api/v1/queue/{queueID}/call-next",
			s.withTenant(s.withAuth(http.HandlerFunc(s.handleQueueCallNext))))
		s.router.Handle("PATCH /api/v1/queue/{queueID}/entries/{entryID}",
			s.withTenant(s.withAuth(http.HandlerFunc(s.handleQueueUpdateEntry))))
		s.router.Handle("POST /api/v1/queue/{queueID}/entries/{entryID}/location",
			s.withTenant(s.withAuth(http.HandlerFunc(s.handleQueueUpdateLocation))))
		// SSE streams — auth required, no rate-limit buffering
		s.router.Handle("GET /api/v1/queue/{queueID}/stream",
			s.withTenant(s.withAuth(http.HandlerFunc(s.handleStaffStream))))
		s.router.Handle("GET /api/v1/queue/{queueID}/entries/{entryID}/stream",
			s.withTenant(s.withAuth(http.HandlerFunc(s.handlePatientStream))))
	}

	// Push notification subscriptions — auth required (patient-facing).
	if s.pushStore != nil {
		s.router.Handle("GET /api/v1/push/vapid-key",
			http.HandlerFunc(s.handleVAPIDKey))
		s.router.Handle("POST /api/v1/push/subscribe",
			s.withTenant(s.withAuth(http.HandlerFunc(s.handlePushSubscribe))))
		s.router.Handle("DELETE /api/v1/push/subscribe",
			s.withTenant(s.withAuth(http.HandlerFunc(s.handlePushUnsubscribe))))
	}
}

// handleHealth responds to liveness probe requests.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"ok"}`)
}

// handleReady responds to readiness probe requests. It checks the DB pool
// when one is available.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.pool != nil {
		if err := s.pool.Ping(r.Context()); err != nil {
			http.Error(w, `{"status":"not ready","detail":"db ping failed"}`, http.StatusServiceUnavailable)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"ready"}`)
}

// ---------------------------------------------------------------------------
// Migration runner helpers (used by the CLI migrate subcommand)
// ---------------------------------------------------------------------------

// MigrationRunner wraps core/db/migrate.Runner for the interop module.
type MigrationRunner struct {
	inner *coremigrate.Runner
}

// NewMigrationRunner opens a DB connection and returns a MigrationRunner.
func NewMigrationRunner(ctx context.Context, dsn string) (*MigrationRunner, error) {
	pool, err := db.New(ctx, db.Config{DSN: dsn, MaxConns: 2, MinConns: 1})
	if err != nil {
		return nil, fmt.Errorf("migration runner: connect: %w", err)
	}
	return &MigrationRunner{inner: coremigrate.New(migrationsFS, pool)}, nil
}

// Up runs all pending up migrations.
func (m *MigrationRunner) Up(ctx context.Context) error { return m.inner.Up(ctx) }

// Down rolls back n steps.
func (m *MigrationRunner) Down(ctx context.Context, steps int) error {
	return m.inner.Down(ctx, steps)
}

// ---------------------------------------------------------------------------
// Connectivity check helpers (used by the CLI validate subcommand)
// ---------------------------------------------------------------------------

// ConnectivityConfig holds the URLs needed for connectivity checks.
type ConnectivityConfig struct {
	DatabaseURL string
	RedisURL    string
	NHIBaseURL  string
}

// ConnectivityResult is the result of a single connectivity check.
type ConnectivityResult struct {
	Name   string
	OK     bool
	Detail string
}

// RunConnectivityChecks verifies reachability of all downstream services.
func RunConnectivityChecks(ctx context.Context, cfg ConnectivityConfig) []ConnectivityResult {
	results := make([]ConnectivityResult, 0, 3)

	// Database.
	dbResult := ConnectivityResult{Name: "PostgreSQL"}
	if cfg.DatabaseURL == "" {
		dbResult.Detail = "DATABASE_URL not configured"
	} else {
		pool, err := db.New(ctx, db.Config{DSN: cfg.DatabaseURL, MaxConns: 1, MinConns: 1})
		if err != nil {
			dbResult.Detail = err.Error()
		} else {
			pool.Close()
			dbResult.OK = true
			dbResult.Detail = "connected"
		}
	}
	results = append(results, dbResult)

	// Redis — simple TCP dial; no Redis client dependency required here.
	redisResult := ConnectivityResult{Name: "Redis"}
	if cfg.RedisURL == "" {
		redisResult.Detail = "REDIS_URL not configured"
	} else {
		dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", redisAddrFromURL(cfg.RedisURL))
		if err != nil {
			redisResult.Detail = err.Error()
		} else {
			conn.Close()
			redisResult.OK = true
			redisResult.Detail = "connected"
		}
	}
	results = append(results, redisResult)

	// NHI API — simple HTTP HEAD request.
	nhiResult := ConnectivityResult{Name: "NHI API"}
	if cfg.NHIBaseURL == "" {
		nhiResult.Detail = "NHI_BASE_URL not configured"
	} else {
		reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, cfg.NHIBaseURL+"/metadata", nil)
		if err != nil {
			nhiResult.Detail = fmt.Sprintf("build request: %v", err)
		} else {
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				nhiResult.Detail = err.Error()
			} else {
				resp.Body.Close()
				nhiResult.OK = resp.StatusCode < 500
				nhiResult.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
		}
	}
	results = append(results, nhiResult)

	return results
}

// redisAddrFromURL extracts "host:port" from a redis:// URL, falling back to
// the raw string when parsing fails (e.g. already a plain host:port).
func redisAddrFromURL(rawURL string) string {
	// Minimal parse: strip scheme.
	const scheme = "redis://"
	s := rawURL
	if len(s) > len(scheme) && s[:len(scheme)] == scheme {
		s = s[len(scheme):]
	}
	// Strip any path or credentials.
	if i := len(s); i > 0 {
		for j, c := range s {
			if c == '/' {
				s = s[:j]
				break
			}
		}
		if at := indexByte(s, '@'); at >= 0 {
			s = s[at+1:]
		}
	}
	// Default Redis port.
	if _, _, err := net.SplitHostPort(s); err != nil {
		s = s + ":6379"
	}
	return s
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
