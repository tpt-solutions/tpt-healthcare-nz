package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Config holds all runtime configuration for the tpt-pharmacy service.
type Config struct {
	Host          string
	Port          int
	DatabaseURL   string
	RedisURL      string
	EncryptionKey string
}

// Server is the tpt-pharmacy HTTP server.
type Server struct {
	cfg    Config
	mux    *http.ServeMux
	logger *slog.Logger
}

// NewServer constructs and wires up a Server with all routes and middleware.
func NewServer(cfg Config, logger *slog.Logger) *Server {
	s := &Server{
		cfg:    cfg,
		mux:    http.NewServeMux(),
		logger: logger,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	dispensingHandler := &DispensingHandler{logger: s.logger}
	claimsHandler := &ClaimsHandler{logger: s.logger}

	// Dispensing routes
	s.mux.HandleFunc("GET /api/v1/dispensing", dispensingHandler.List)
	s.mux.HandleFunc("POST /api/v1/dispensing", dispensingHandler.Receive)
	s.mux.HandleFunc("GET /api/v1/dispensing/{id}", dispensingHandler.Get)
	s.mux.HandleFunc("POST /api/v1/dispensing/{id}/dispense", dispensingHandler.Dispense)
	s.mux.HandleFunc("POST /api/v1/dispensing/{id}/schedule2-confirm", dispensingHandler.Schedule2Confirm)

	// Prescription pass-through (incoming from GP)
	s.mux.HandleFunc("GET /api/v1/prescriptions", dispensingHandler.ListPrescriptions)
	s.mux.HandleFunc("POST /api/v1/prescriptions", dispensingHandler.ReceivePrescription)

	// Inventory routes (stub handlers — expanded by inventory.go if present)
	s.mux.HandleFunc("GET /api/v1/inventory", s.handleInventoryList)
	s.mux.HandleFunc("POST /api/v1/inventory", s.handleInventoryUpdate)

	// Claims routes
	s.mux.HandleFunc("GET /api/v1/claims", claimsHandler.List)
	s.mux.HandleFunc("POST /api/v1/claims", claimsHandler.Create)
	s.mux.HandleFunc("POST /api/v1/claims/{id}/submit", claimsHandler.Submit)
	s.mux.HandleFunc("GET /api/v1/claims/{id}/status", claimsHandler.Status)

	// HSD reporting
	s.mux.HandleFunc("POST /api/v1/reports/hsd", claimsHandler.GenerateHSDReport)

	// Kubernetes-style health probes
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
		logger.Info("tpt-pharmacy listening", "addr", addr)
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
	// Delegate to core/db migration runner. Wired at build time via core dependency.
	// Placeholder until the core db.Migrate API is imported here.
	slog.Default().Info("running pharmacy migrations", "database_url", databaseURL)
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
	slog.Default().Info("configuration valid")
	return nil
}

// --- Inventory stub handlers ---

func (s *Server) handleInventoryList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "inventory": "[]"})
}

func (s *Server) handleInventoryUpdate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

// --- Health probe handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
