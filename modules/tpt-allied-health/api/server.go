// Package api implements the HTTP server for tpt-allied-health.
package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Server holds the HTTP server and dependencies.
type Server struct {
	router       *mux.Router
	server       *http.Server
	auth         auth.Provider
	config       Config
	pool         *pgxpool.Pool
	auditTrail   *audit.Trail
	hpiClient    *hpi.Client
	consentStore *consent.Store
}

// Config holds server configuration.
type Config struct {
	Addr           string
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	AllowedOrigins []string
}

// DefaultConfig returns default server configuration.
func DefaultConfig() Config {
	return Config{
		Addr:           ":8080",
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		AllowedOrigins: []string{"*"},
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

	router := mux.NewRouter()

	s := &Server{
		router:       router,
		auth:         authProvider,
		config:       cfg,
		pool:         pool,
		auditTrail:   auditTrail,
		hpiClient:    hpiClient,
		consentStore: consentStore,
	}

	s.setupMiddleware()
	s.setupRoutes()

	s.server = &http.Server{
		Addr:         cfg.Addr,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return s
}

// setupMiddleware configures global middleware (applied to all routes including health checks).
func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RecoveryMiddleware())
	s.router.Use(middleware.CORS(s.config.AllowedOrigins))
	s.router.Use(middleware.RateLimit(100, 200))
}

// setupRoutes registers all API routes.
func (s *Server) setupRoutes() {
	// Public routes — no auth, no tenant extraction, no audit.
	s.router.HandleFunc("/health", s.healthCheck).Methods("GET")
	s.router.HandleFunc("/ready", s.readinessCheck).Methods("GET")

	// Protected API routes. A subrouter with no path prefix is used purely
	// to scope middleware — auth, tenant extraction, and audit apply only here.
	protected := s.router.NewRoute().Subrouter()
	protected.Use(middleware.TenantExtraction())
	protected.Use(auth.RequireAuth(s.auth))
	protected.Use(middleware.AuditWrap(s.auditTrail))

	physioHandler := NewPhysioHandler(s.hpiClient, s.consentStore)
	physioHandler.RegisterRoutes(protected)

	otHandler := NewOTHandler(s.hpiClient, s.consentStore)
	otHandler.RegisterRoutes(protected)

	speechHandler := NewSpeechHandler(s.hpiClient, s.consentStore)
	speechHandler.RegisterRoutes(protected)

	podiatryHandler := NewPodiatryHandler(s.hpiClient, s.consentStore)
	podiatryHandler.RegisterRoutes(protected)

	accHandler := NewACCHandler(s.hpiClient, s.consentStore, s.pool)
	accHandler.RegisterRoutes(protected)
}

// healthCheck handles GET /health.
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy","service":"tpt-allied-health"}`))
}

// readinessCheck handles GET /ready.
func (s *Server) readinessCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"not ready","service":"tpt-allied-health","error":"database unavailable"}`))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready","service":"tpt-allied-health"}`))
}

// Start starts the HTTP server with graceful shutdown.
func (s *Server) Start() error {
	log.Info().Str("addr", s.config.Addr).Msg("Starting tpt-allied-health server")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	<-stop
	log.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
		return err
	}

	log.Info().Msg("Server exited gracefully")
	return nil
}

// Router returns the underlying router for testing.
func (s *Server) Router() *mux.Router {
	return s.router
}
