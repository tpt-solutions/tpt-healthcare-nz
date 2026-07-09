// Package api implements the tpt-blood-bank HTTP server and route handlers.
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
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// Config holds all configuration for the tpt-blood-bank server.
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

// Server is the tpt-blood-bank HTTP server.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	pool       db.Pool
	enc        *encryption.Cipher
	auth       auth.Provider
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

	trail := audit.NewTrail(pool)

	s := &Server{
		cfg:        cfg,
		pool:       pool,
		enc:        enc,
		auth:       authProvider,
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

// buildRoutes registers all routes and applies the middleware chain.
func (s *Server) buildRoutes() *http.ServeMux {
	donors := &DonorsHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	inventory := &InventoryHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	crossmatch := &CrossmatchHandler{
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

	// Health and readiness probes — no auth required.
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// Donor routes.
	mux.Handle("GET /api/v1/donors", chain(http.HandlerFunc(donors.List)))
	mux.Handle("POST /api/v1/donors", chain(http.HandlerFunc(donors.Create)))
	mux.Handle("GET /api/v1/donors/{id}", chain(http.HandlerFunc(donors.Get)))
	mux.Handle("PUT /api/v1/donors/{id}", chain(http.HandlerFunc(donors.Update)))
	mux.Handle("POST /api/v1/donors/{id}/defer", chain(http.HandlerFunc(donors.Defer)))
	mux.Handle("POST /api/v1/donors/{id}/reinstate", chain(http.HandlerFunc(donors.Reinstate)))
	mux.Handle("GET /api/v1/donors/{id}/donations", chain(http.HandlerFunc(donors.DonationHistory)))
	mux.Handle("GET /api/v1/donors/eligible", chain(http.HandlerFunc(donors.ListEligible)))

	// Inventory routes.
	mux.Handle("GET /api/v1/inventory", chain(http.HandlerFunc(inventory.List)))
	mux.Handle("POST /api/v1/inventory", chain(http.HandlerFunc(inventory.Create)))
	mux.Handle("GET /api/v1/inventory/{id}", chain(http.HandlerFunc(inventory.Get)))
	mux.Handle("PUT /api/v1/inventory/{id}/status", chain(http.HandlerFunc(inventory.UpdateStatus)))
	mux.Handle("GET /api/v1/inventory/expiring", chain(http.HandlerFunc(inventory.ListExpiring)))
	mux.Handle("GET /api/v1/inventory/available", chain(http.HandlerFunc(inventory.ListAvailable)))

	// Cross-match routes.
	mux.Handle("GET /api/v1/crossmatches", chain(http.HandlerFunc(crossmatch.List)))
	mux.Handle("POST /api/v1/crossmatches", chain(http.HandlerFunc(crossmatch.Create)))
	mux.Handle("GET /api/v1/crossmatches/{id}", chain(http.HandlerFunc(crossmatch.Get)))
	mux.Handle("POST /api/v1/crossmatches/{id}/issue", chain(http.HandlerFunc(crossmatch.Issue)))
	mux.Handle("POST /api/v1/crossmatches/{id}/transfuse", chain(http.HandlerFunc(crossmatch.Transfuse)))
	mux.Handle("POST /api/v1/crossmatches/{id}/cancel", chain(http.HandlerFunc(crossmatch.Cancel)))
	mux.Handle("POST /api/v1/crossmatches/{id}/emergency", chain(http.HandlerFunc(crossmatch.EmergencyRelease)))

	return mux
}

// handleHealth responds to liveness probes.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-blood-bank",
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
		"service": "tpt-blood-bank",
	})
}

// RunMigrations runs database migrations for the tpt-blood-bank module.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, "modules/tpt-blood-bank/db/migrate"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
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