// Package api implements the tpt-community-health HTTP server.
// Covers home visit scheduling and documentation, district nursing care plans,
// and community outreach programme management for NZ community health practice.
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
	coredb "github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/db/migrate"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	communitydb "github.com/PhillipC05/tpt-healthcare/modules/tpt-community-health/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all runtime configuration for the community health server.
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

// Server is the tpt-community-health HTTP server.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	pool       *pgxpool.Pool
	enc        *encryption.Cipher
	auth       auth.Provider
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// NewServer creates and wires the community health server.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.TenantHeader == "" {
		cfg.TenantHeader = "X-Tenant-ID"
	}
	pool, err := coredb.Connect(context.Background(), cfg.DatabaseURL)
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
	s := &Server{
		cfg:        cfg,
		pool:       pool,
		enc:        enc,
		auth:       authProvider,
		hpiClient:  hpi.NewClient(cfg.RedisURL, cfg.Logger),
		auditTrail: audit.New(pool),
		logger:     cfg.Logger,
	}
	s.mux = s.buildRoutes()
	return s, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) buildRoutes() *http.ServeMux {
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

	deps := handlerDeps{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}

	// Home visits — scheduling and documentation
	hv := &homeVisitHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/community/home-visits", chain(http.HandlerFunc(hv.List)))
	mux.Handle("POST /api/v1/community/home-visits", chain(http.HandlerFunc(hv.Create)))
	mux.Handle("GET /api/v1/community/home-visits/{id}", chain(http.HandlerFunc(hv.Get)))
	mux.Handle("PUT /api/v1/community/home-visits/{id}", chain(http.HandlerFunc(hv.Update)))
	mux.Handle("POST /api/v1/community/home-visits/{id}/complete", chain(http.HandlerFunc(hv.Complete)))
	mux.Handle("POST /api/v1/community/home-visits/{id}/cancel", chain(http.HandlerFunc(hv.Cancel)))

	// District nursing care plans
	cp := &carePlanHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/community/district-nursing/care-plans", chain(http.HandlerFunc(cp.List)))
	mux.Handle("POST /api/v1/community/district-nursing/care-plans", chain(http.HandlerFunc(cp.Create)))
	mux.Handle("GET /api/v1/community/district-nursing/care-plans/{id}", chain(http.HandlerFunc(cp.Get)))
	mux.Handle("PUT /api/v1/community/district-nursing/care-plans/{id}", chain(http.HandlerFunc(cp.Update)))
	mux.Handle("POST /api/v1/community/district-nursing/care-plans/{id}/activate", chain(http.HandlerFunc(cp.Activate)))
	mux.Handle("POST /api/v1/community/district-nursing/care-plans/{id}/complete", chain(http.HandlerFunc(cp.Complete)))

	// District nursing visits (scoped under care plan)
	nv := &nursingVisitHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/community/district-nursing/care-plans/{id}/visits", chain(http.HandlerFunc(nv.ListForPlan)))
	mux.Handle("POST /api/v1/community/district-nursing/care-plans/{id}/visits", chain(http.HandlerFunc(nv.Create)))
	mux.Handle("GET /api/v1/community/district-nursing/visits/{id}", chain(http.HandlerFunc(nv.Get)))
	mux.Handle("PUT /api/v1/community/district-nursing/visits/{id}", chain(http.HandlerFunc(nv.Update)))
	mux.Handle("POST /api/v1/community/district-nursing/visits/{id}/complete", chain(http.HandlerFunc(nv.Complete)))

	// Outreach programmes
	op := &outreachProgrammeHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/community/outreach/programmes", chain(http.HandlerFunc(op.List)))
	mux.Handle("POST /api/v1/community/outreach/programmes", chain(http.HandlerFunc(op.Create)))
	mux.Handle("GET /api/v1/community/outreach/programmes/{id}", chain(http.HandlerFunc(op.Get)))
	mux.Handle("PUT /api/v1/community/outreach/programmes/{id}", chain(http.HandlerFunc(op.Update)))

	// Outreach events (scoped under programme)
	oe := &outreachEventHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/community/outreach/programmes/{id}/events", chain(http.HandlerFunc(oe.ListForProgramme)))
	mux.Handle("POST /api/v1/community/outreach/programmes/{id}/events", chain(http.HandlerFunc(oe.Create)))
	mux.Handle("GET /api/v1/community/outreach/events/{id}", chain(http.HandlerFunc(oe.Get)))
	mux.Handle("PUT /api/v1/community/outreach/events/{id}", chain(http.HandlerFunc(oe.Update)))
	mux.Handle("POST /api/v1/community/outreach/events/{id}/complete", chain(http.HandlerFunc(oe.Complete)))

	// Outreach encounters (patient contacts at outreach events)
	enc := &outreachEncounterHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/community/outreach/events/{id}/encounters", chain(http.HandlerFunc(enc.ListForEvent)))
	mux.Handle("POST /api/v1/community/outreach/events/{id}/encounters", chain(http.HandlerFunc(enc.Create)))
	mux.Handle("GET /api/v1/community/outreach/encounters/{id}", chain(http.HandlerFunc(enc.Get)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-community-health",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-community-health"})
}

// RunMigrations runs the module's embedded SQL migrations.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := coredb.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(communitydb.Migrations, pool)
	return r.Up(ctx)
}

// ValidateConnectivity checks that the database is reachable.
func ValidateConnectivity(ctx context.Context, cfg Config) error {
	pool, err := coredb.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer pool.Close()
	return pool.Ping(ctx)
}

// handlerDeps is a shared dependency bundle injected into all domain handlers.
type handlerDeps struct {
	pool       *pgxpool.Pool
	enc        *encryption.Cipher
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var errNotFound = errors.New("record not found")

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
