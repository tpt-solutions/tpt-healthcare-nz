// Package api implements the tpt-dental HTTP server and route handlers.
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

// Config holds all configuration for the tpt-dental server.
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

// Server is the tpt-dental HTTP server.
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
	chart := &ChartHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	proc := &ProcedureHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	acc := &ACCHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	codes := &CodeHandler{
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

	// Dental charting routes.
	mux.Handle("GET /api/v1/chart/{patientNhi}", chain(http.HandlerFunc(chart.GetChart)))
	mux.Handle("PUT /api/v1/chart/{patientNhi}", chain(http.HandlerFunc(chart.SaveChart)))
	mux.Handle("GET /api/v1/chart/{patientNhi}/tooth/{fdiCode}", chain(http.HandlerFunc(chart.GetTooth)))
	mux.Handle("PUT /api/v1/chart/{patientNhi}/tooth/{fdiCode}", chain(http.HandlerFunc(chart.UpdateTooth)))

	// FDI tooth reference data (read-only, no clinical data).
	mux.Handle("GET /api/v1/fdi/lookup/{fdiCode}", chain(http.HandlerFunc(codes.LookupTooth)))
	mux.Handle("GET /api/v1/fdi/surfaces", chain(http.HandlerFunc(codes.Surfaces)))
	mux.Handle("GET /api/v1/fdi/all", chain(http.HandlerFunc(codes.AllTeeth)))
	mux.Handle("GET /api/v1/fdi/all/deciduous", chain(http.HandlerFunc(codes.AllDeciduous)))

	// Procedure code lookup routes.
	mux.Handle("GET /api/v1/procedures", chain(http.HandlerFunc(proc.ListProcedures)))
	mux.Handle("GET /api/v1/procedures/dcnz/{code}", chain(http.HandlerFunc(proc.GetDCNZCode)))
	mux.Handle("GET /api/v1/procedures/acc/{code}", chain(http.HandlerFunc(proc.GetACCCode)))
	mux.Handle("GET /api/v1/procedures/category/{category}", chain(http.HandlerFunc(proc.ByCategory)))

	// Procedure treatment records (per patient visit).
	mux.Handle("GET /api/v1/procedures/records/{patientNhi}", chain(http.HandlerFunc(proc.ListRecords)))
	mux.Handle("POST /api/v1/procedures/records", chain(http.HandlerFunc(proc.CreateRecord)))
	mux.Handle("GET /api/v1/procedures/records/{patientNhi}/{recordId}", chain(http.HandlerFunc(proc.GetRecord)))
	mux.Handle("PUT /api/v1/procedures/records/{patientNhi}/{recordId}", chain(http.HandlerFunc(proc.UpdateRecord)))

	// ACC dental claim routes.
	mux.Handle("GET /api/v1/acc/claims", chain(http.HandlerFunc(acc.ListClaims)))
	mux.Handle("POST /api/v1/acc/claims", chain(http.HandlerFunc(acc.CreateClaim)))
	mux.Handle("GET /api/v1/acc/claims/{claimId}", chain(http.HandlerFunc(acc.GetClaim)))
	mux.Handle("PUT /api/v1/acc/claims/{claimId}", chain(http.HandlerFunc(acc.UpdateClaim)))
	mux.Handle("POST /api/v1/acc/claims/{claimId}/submit", chain(http.HandlerFunc(acc.SubmitClaim)))
	mux.Handle("GET /api/v1/acc/claims/{claimId}/status", chain(http.HandlerFunc(acc.CheckStatus)))
	mux.Handle("POST /api/v1/acc/claims/validate", chain(http.HandlerFunc(acc.ValidateClaim)))
	mux.Handle("GET /api/v1/acc/injury-types", chain(http.HandlerFunc(acc.InjuryTypes)))

	return mux
}

// handleHealth responds to liveness probes.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-dental",
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
		"service": "tpt-dental",
	})
}

// RunMigrations runs database migrations for the tpt-dental module.
func RunMigrations(ctx context.Context, databaseURL string) error {
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, "modules/tpt-dental/db/migrate"); err != nil {
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