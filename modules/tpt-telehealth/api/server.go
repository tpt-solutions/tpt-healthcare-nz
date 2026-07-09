// Package api implements the tpt-telehealth HTTP server and route handlers.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/auth/auth0"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/video"

	// Register all video provider backends via their init() functions.
	_ "github.com/PhillipC05/tpt-healthcare/core/video/jitsi"
	_ "github.com/PhillipC05/tpt-healthcare/core/video/teams"
	_ "github.com/PhillipC05/tpt-healthcare/core/video/zoom"

	"github.com/spf13/viper"
)

// errNotFound is the sentinel returned by DB helpers when no row matches.
var errNotFound = errors.New("not found")

// Config holds all configuration for the tpt-telehealth server.
type Config struct {
	Host          string
	Port          int
	DatabaseURL   string
	RedisURL      string
	EncryptionKey string
	Auth0Domain   string
	Auth0Audience string
	TenantHeader  string
	Logger        *slog.Logger
}

// Server is the tpt-telehealth HTTP server.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	pool       db.Pool
	enc        *encryption.Cipher
	auth       auth.Provider
	hpiClient  *hpi.Client
	video      video.Provider
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// NewServer constructs and wires all dependencies.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.TenantHeader == "" {
		cfg.TenantHeader = "X-Tenant-ID"
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

	hpiClient := hpi.NewClient(cfg.RedisURL, cfg.Logger)
	trail := audit.NewTrail(pool)

	videoProvider, err := video.New(context.Background(), viper.GetViper())
	if err != nil {
		return nil, fmt.Errorf("init video provider: %w", err)
	}

	s := &Server{
		cfg:        cfg,
		pool:       pool,
		enc:        enc,
		auth:       authProvider,
		hpiClient:  hpiClient,
		video:      videoProvider,
		auditTrail: trail,
		logger:     cfg.Logger,
	}

	s.mux = s.buildRoutes()
	return s, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// buildRoutes registers all routes and applies the middleware chain.
func (s *Server) buildRoutes() *http.ServeMux {
	sessions := &SessionsHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		video:      s.video,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	monitoring := &MonitoringHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}

	chain := func(h http.Handler) http.Handler {
		h = middleware.AuditWrap(h, s.auditTrail)
		h = middleware.Auth(h, s.auth)
		h = middleware.Tenant(h, s.cfg.TenantHeader)
		h = middleware.CORS(h)
		h = middleware.RateLimit(h)
		h = middleware.Recovery(h, s.logger)
		return h
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// Telehealth session routes.
	mux.Handle("GET /api/v1/sessions", chain(http.HandlerFunc(sessions.List)))
	mux.Handle("POST /api/v1/sessions", chain(http.HandlerFunc(sessions.Create)))
	mux.Handle("GET /api/v1/sessions/{id}", chain(http.HandlerFunc(sessions.Get)))
	mux.Handle("POST /api/v1/sessions/{id}/join", chain(http.HandlerFunc(sessions.Join)))
	mux.Handle("POST /api/v1/sessions/{id}/end", chain(http.HandlerFunc(sessions.End)))
	mux.Handle("GET /api/v1/sessions/{id}/recording", chain(http.HandlerFunc(sessions.Recording)))

	// Remote monitoring routes.
	mux.Handle("POST /api/v1/monitoring/devices", chain(http.HandlerFunc(monitoring.RegisterDevice)))
	mux.Handle("GET /api/v1/monitoring/devices", chain(http.HandlerFunc(monitoring.ListDevices)))
	mux.Handle("POST /api/v1/monitoring/observations", chain(http.HandlerFunc(monitoring.SubmitObservation)))
	mux.Handle("GET /api/v1/monitoring/observations", chain(http.HandlerFunc(monitoring.ListObservations)))
	mux.Handle("GET /api/v1/monitoring/observations/{id}", chain(http.HandlerFunc(monitoring.GetObservation)))
	mux.Handle("GET /api/v1/monitoring/patients/{nhi}/summary", chain(http.HandlerFunc(monitoring.PatientSummary)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-telehealth",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

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
		"service": "tpt-telehealth",
	})
}

// RunMigrations runs database migrations for the tpt-telehealth module.
func RunMigrations(ctx context.Context, databaseURL string) error {
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, "modules/tpt-telehealth/db/migrate"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// ValidateConnectivity checks that the database and Redis are reachable.
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

// apiError is the standard error response envelope.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}
