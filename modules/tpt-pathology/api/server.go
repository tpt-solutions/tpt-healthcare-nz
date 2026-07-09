// Package api implements the tpt-pathology HTTP server and MLLP listener.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/auth/auth0"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/db/migrate"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hl7"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/subscription"
	pathologydb "github.com/PhillipC05/tpt-healthcare/modules/tpt-pathology/db"

	"github.com/redis/go-redis/v9"
)

// DiagnosticReportTopic is the canonical SubscriptionTopic URL for new lab results.
const DiagnosticReportTopic = "https://standards.digital.health.nz/fhir/SubscriptionTopic/new-diagnostic-report"

// Config holds all configuration for the tpt-pathology server.
type Config struct {
	Host          string
	Port          int
	DatabaseURL   string
	RedisURL      string
	EncryptionKey string
	Auth0Domain   string
	Auth0Audience string
	TenantHeader  string
	// MLLPAddr is the TCP address for the MLLP listener (e.g. ":2575").
	// Defaults to ":2575" if empty.
	MLLPAddr string
	Logger   *slog.Logger
}

// Server is the tpt-pathology HTTP + MLLP server.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	pool       db.Pool
	enc        *encryption.Cipher
	auth       auth.Provider
	subEngine  *subscription.Engine
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// NewServer constructs and configures a Server, wiring all dependencies.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.TenantHeader == "" {
		cfg.TenantHeader = "X-Tenant-ID"
	}
	if cfg.MLLPAddr == "" {
		cfg.MLLPAddr = ":2575"
	}

	pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	enc, err := encryption.NewCipher(cfg.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("init encryption cipher: %w", err)
	}

	authProvider, err := auth0.NewProvider(cfg.Auth0Domain, cfg.Auth0Audience)
	if err != nil {
		return nil, fmt.Errorf("init auth0 provider: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
	subEng := subscription.New(rdb)

	trail := audit.NewTrail(pool)

	s := &Server{
		cfg:        cfg,
		pool:       pool,
		enc:        enc,
		auth:       authProvider,
		subEngine:  subEng,
		auditTrail: trail,
		logger:     cfg.Logger,
	}

	s.mux = s.buildRoutes()
	return s, nil
}

// Handler returns the root HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// StartMLLP starts the MLLP TCP listener and blocks until ctx is cancelled.
// It is intended to run in its own goroutine alongside the HTTP server.
func (s *Server) StartMLLP(ctx context.Context) error {
	conv := &mllpConverter{
		pool:       s.pool,
		enc:        s.enc,
		subEngine:  s.subEngine,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	mllpSrv := hl7.NewMLLPServer(s.cfg.MLLPAddr, conv.handleMessage)
	s.logger.Info("tpt-pathology MLLP listener starting", slog.String("addr", s.cfg.MLLPAddr))
	return mllpSrv.Start(ctx)
}

// buildRoutes registers all HTTP routes and applies the middleware chain.
func (s *Server) buildRoutes() *http.ServeMux {
	reports := &ReportsHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	specimens := &SpecimensHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	tests := &TestsHandler{
		pool:   s.pool,
		logger: s.logger,
	}

	chain := func(h http.Handler) http.Handler {
		h = middleware.AuditWrap(s.auditTrail)(h)
		h = auth.RequireAuth(s.auth)(h)
		h = middleware.TenantExtraction()(h)
		h = middleware.CORS([]string{"*"})(h)
		h = middleware.RateLimit(10, 30)(h)
		h = middleware.RecoveryMiddleware()(h)
		return h
	}

	mux := http.NewServeMux()

	// Health and readiness probes — no auth required.
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// Diagnostic report routes.
	mux.Handle("GET /api/v1/reports", chain(http.HandlerFunc(reports.List)))
	mux.Handle("GET /api/v1/reports/{id}", chain(http.HandlerFunc(reports.Get)))
	mux.Handle("GET /api/v1/reports/{id}/observations", chain(http.HandlerFunc(reports.GetObservations)))

	// Specimen tracking routes.
	mux.Handle("GET /api/v1/specimens", chain(http.HandlerFunc(specimens.List)))
	mux.Handle("GET /api/v1/specimens/{id}", chain(http.HandlerFunc(specimens.Get)))
	mux.Handle("POST /api/v1/specimens", chain(http.HandlerFunc(specimens.Create)))
	mux.Handle("PUT /api/v1/specimens/{id}/status", chain(http.HandlerFunc(specimens.UpdateStatus)))

	// Test catalog and reference range routes.
	mux.Handle("GET /api/v1/tests", chain(http.HandlerFunc(tests.List)))
	mux.Handle("GET /api/v1/tests/{loinc}", chain(http.HandlerFunc(tests.Get)))
	mux.Handle("POST /api/v1/tests", chain(http.HandlerFunc(tests.Create)))
	mux.Handle("GET /api/v1/tests/{loinc}/reference-ranges", chain(http.HandlerFunc(tests.ListReferenceRanges)))
	mux.Handle("POST /api/v1/tests/{loinc}/reference-ranges", chain(http.HandlerFunc(tests.CreateReferenceRange)))

	return mux
}

// handleHealth responds to liveness probes.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-pathology",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// handleReady responds to readiness probes, checking database connectivity.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		s.logger.Error("readiness check failed", slog.Any("error", err))
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
			"reason": "database not reachable",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ready",
		"service": "tpt-pathology",
	})
}

// RunMigrations runs database migrations for the tpt-pathology module.
func RunMigrations(ctx context.Context, databaseURL string) error {
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(pathologydb.Migrations, pool)
	return r.Up(ctx)
}

// ValidateConnectivity checks that the database is reachable.
func ValidateConnectivity(ctx context.Context, cfg Config) error {
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	cfg.Logger.Info("database connectivity OK")
	cfg.Logger.Info("connectivity validation complete")
	return nil
}

// errNotFound is returned by data-access helpers when a record is not found.
var errNotFound = fmt.Errorf("not found")

// apiError is the standard error response envelope.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// writeJSON serialises v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON encode error", slog.Any("error", err))
	}
}

// decodeJSON reads and decodes a JSON request body into v.
func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}
