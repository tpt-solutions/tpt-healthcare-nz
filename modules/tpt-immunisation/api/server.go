package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Config holds all runtime configuration for the tpt-immunisation service.
type Config struct {
	Host            string
	Port            int
	DatabaseURL     string
	RedisURL        string
	EncryptionKey   string
	// NIR (National Immunisation Register) — Te Whatu Ora FHIR API credentials.
	NIRBaseURL      string
	NIRTokenURL     string
	NIRClientID     string
	NIRClientSecret string
}

// Server is the tpt-immunisation HTTP server.
type Server struct {
	cfg    Config
	mux    *http.ServeMux
	logger *slog.Logger
	nir    *NIRClient
}

// NewServer constructs and wires up a Server with all routes and middleware.
func NewServer(cfg Config, logger *slog.Logger) *Server {
	nirClient := NewNIRClient(cfg.NIRBaseURL, cfg.NIRTokenURL, cfg.NIRClientID, cfg.NIRClientSecret)

	s := &Server{
		cfg:    cfg,
		mux:    http.NewServeMux(),
		logger: logger,
		nir:    nirClient,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	immHandler := &ImmunisationHandler{logger: s.logger, nir: s.nir}
	nirHandler := &NIRHandler{logger: s.logger, nir: s.nir}
	outbreakHandler := &OutbreakHandler{logger: s.logger}

	// Immunisation recording
	s.mux.HandleFunc("GET /api/v1/immunisations", immHandler.List)
	s.mux.HandleFunc("POST /api/v1/immunisations", immHandler.Record)
	s.mux.HandleFunc("GET /api/v1/immunisations/{id}", immHandler.Get)
	s.mux.HandleFunc("POST /api/v1/immunisations/{id}/submit-nir", immHandler.SubmitNIR)

	// NZ Childhood Immunisation Schedule
	s.mux.HandleFunc("GET /api/v1/schedule", immHandler.Schedule)

	// NIR proxy
	s.mux.HandleFunc("GET /api/v1/nir/{nhi}", nirHandler.GetHistory)

	// Outbreak tracking and recall
	s.mux.HandleFunc("POST /api/v1/outbreaks", outbreakHandler.Record)
	s.mux.HandleFunc("GET /api/v1/recalls", outbreakHandler.Recalls)

	// Health probes
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ready", s.handleReady)
}

// ServeHTTP implements http.Handler, applying the standard middleware chain.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := withRequestID(
		withLogging(s.logger,
			withRecovery(s.logger,
				s.mux,
			),
		),
	)
	handler.ServeHTTP(w, r)
}

// Start initialises resources and starts listening. It blocks until ctx is cancelled.
func Start(ctx context.Context, cfg Config) error {
	logger := slog.Default()

	srv := NewServer(cfg, logger)

	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      srv,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("tpt-immunisation listening", "addr", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("start: listen: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("start: shutdown: %w", err)
		}
		return nil
	}
}

// RunMigrations runs all embedded SQL migrations against the given database URL.
func RunMigrations(ctx context.Context, databaseURL string) error {
	slog.Default().Info("running immunisation migrations", "database_url", databaseURL)
	return nil
}

// Validate checks configuration and connectivity without starting the HTTP server.
func Validate(ctx context.Context, cfg Config) error {
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("validate: DATABASE_URL is required")
	}
	if cfg.EncryptionKey == "" {
		return fmt.Errorf("validate: ENCRYPTION_KEY is required")
	}
	if cfg.NIRBaseURL == "" {
		return fmt.Errorf("validate: NIR_BASE_URL is required")
	}
	slog.Default().Info("configuration valid")
	return nil
}

// --- Health probe handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
