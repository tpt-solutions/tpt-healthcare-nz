// Package api implements the HTTP server for tpt-allied-health.
package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server holds the HTTP server and dependencies.
type Server struct {
	mux          *http.ServeMux
	auth         auth.Provider
	config       Config
	pool         *pgxpool.Pool
	auditTrail   *audit.Trail
	hpiClient    *hpi.Client
	consentStore *consent.Store
	logger       *slog.Logger
}

// Config holds server configuration.
type Config struct {
	Addr           string
	ReadTimeout    int
	WriteTimeout   int
	IdleTimeout    int
	AllowedOrigins []string
	Logger         *slog.Logger
}

// DefaultConfig returns default server configuration.
func DefaultConfig() Config {
	return Config{
		Addr:           ":8080",
		ReadTimeout:    15,
		WriteTimeout:   15,
		IdleTimeout:    60,
		AllowedOrigins: []string{"*"},
		Logger:         slog.Default(),
	}
}

// NewServer creates a new HTTP server.
func NewServer(pool *pgxpool.Pool, authProvider auth.Provider, auditTrail *audit.Trail, hpiClient *hpi.Client, consentStore *consent.Store, cfg Config) *Server {
	if cfg.Addr == "" {
		cfg = DefaultConfig()
	}
	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = []string{"*"}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	s := &Server{
		auth:         authProvider,
		config:       cfg,
		pool:         pool,
		auditTrail:   auditTrail,
		hpiClient:    hpiClient,
		consentStore: consentStore,
		logger:       cfg.Logger,
	}

	s.mux = s.buildRoutes()
	return s
}

// Handler returns the root HTTP handler with middleware applied.
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux
	if s.auditTrail != nil {
		h = middleware.AuditWrap(s.auditTrail)(h)
	}
	h = middleware.CORS(s.config.AllowedOrigins)(h)
	h = middleware.RateLimit(100, 200)(h)
	h = middleware.RecoveryMiddleware()(h)
	return h
}

// buildRoutes registers all routes.
func (s *Server) buildRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Public routes — no auth, no tenant extraction, no audit.
	mux.HandleFunc("/health", s.healthCheck)
	mux.HandleFunc("/ready", s.readinessCheck)

	// Protected route builder.
	p := func(h http.HandlerFunc) http.Handler {
		return auth.RequireAuth(s.auth)(middleware.TenantExtraction()(h))
	}

	physioHandler := NewPhysioHandler(s.hpiClient, s.consentStore, s.pool, s.logger)
	physioHandler.RegisterRoutes(mux, p)

	otHandler := NewOTHandler(s.hpiClient, s.consentStore)
	otHandler.RegisterRoutes(mux, p)

	speechHandler := NewSpeechHandler(s.hpiClient, s.consentStore)
	speechHandler.RegisterRoutes(mux, p)

	podiatryHandler := NewPodiatryHandler(s.hpiClient, s.consentStore)
	podiatryHandler.RegisterRoutes(mux, p)

	accHandler := NewACCHandler(s.hpiClient, s.consentStore, s.pool)
	accHandler.RegisterRoutes(mux, p)

	return mux
}

// healthCheck handles GET /health.
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy","service":"tpt-allied-health"}`))
}

// readinessCheck handles GET /ready.
func (s *Server) readinessCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"not ready","service":"tpt-allied-health","error":"database unavailable"}`))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready","service":"tpt-allied-health"}`))
}
