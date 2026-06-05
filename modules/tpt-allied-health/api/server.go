// Package api implements the HTTP server for tpt-allied-health.
package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

// Server holds the HTTP server and dependencies.
type Server struct {
	router  *mux.Router
	server  *http.Server
	auth    auth.Provider
	config  Config
}

// Config holds server configuration.
type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DefaultConfig returns default server configuration.
func DefaultConfig() Config {
	return Config{
		Addr:         ":8080",
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// NewServer creates a new HTTP server.
func NewServer(authProvider auth.Provider, cfg Config) *Server {
	if cfg.Addr == "" {
		cfg = DefaultConfig()
	}

	router := mux.NewRouter()

	s := &Server{
		router: router,
		auth:   authProvider,
		config: cfg,
	}

	s.setupRoutes()
	s.setupMiddleware()

	s.server = &http.Server{
		Addr:         cfg.Addr,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return s
}

// setupMiddleware configures global middleware.
func (s *Server) setupMiddleware() {
	// Request ID middleware
	s.router.Use(middleware.RequestID)

	// Logging middleware
	s.router.Use(middleware.Logging)

	// Recovery middleware
	s.router.Use(middleware.Recovery)

	// CORS middleware
	s.router.Use(middleware.CORS)

	// Tenant extraction middleware
	s.router.Use(middleware.TenantExtractor)

	// Auth middleware (applied to all routes except health)
	s.router.Use(middleware.Auth(s.auth))

	// Audit middleware
	s.router.Use(middleware.Audit)
}

// setupRoutes registers all API routes.
func (s *Server) setupRoutes() {
	// Health check endpoint (no auth required)
	s.router.HandleFunc("/health", s.healthCheck).Methods("GET")
	s.router.HandleFunc("/ready", s.readinessCheck).Methods("GET")

	// API v1 routes
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Physiotherapy routes
	physioHandler := NewPhysioHandler()
	physioHandler.RegisterRoutes(api)

	// Occupational Therapy routes
	otHandler := NewOTHandler()
	otHandler.RegisterRoutes(api)

	// Speech-Language Therapy routes
	speechHandler := NewSpeechHandler()
	speechHandler.RegisterRoutes(api)

	// Podiatry routes
	podiatryHandler := NewPodiatryHandler()
	podiatryHandler.RegisterRoutes(api)

	// ACC routes (shared across all professions)
	accHandler := NewACCHandler()
	accHandler.RegisterRoutes(api)
}

// healthCheck handles GET /health.
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy","service":"tpt-allied-health"}`))
}

// readinessCheck handles GET /ready.
func (s *Server) readinessCheck(w http.ResponseWriter, r *http.Request) {
	// In real implementation, check database connectivity, etc.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready","service":"tpt-allied-health"}`))
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	log.Info().Str("addr", s.config.Addr).Msg("Starting tpt-allied-health server")

	// Channel to listen for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Run server in a goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Info().Msg("Shutting down server...")

	// Graceful shutdown with timeout
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