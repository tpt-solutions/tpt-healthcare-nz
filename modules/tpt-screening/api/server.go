// Package api implements the tpt-screening HTTP server.
// Covers national screening programme enrolments (NCSP, bowel, breast,
// newborn metabolic, newborn hearing, antenatal), recall notification
// workflows, and screening result capture with follow-up management.
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
	screeningdb "github.com/PhillipC05/tpt-healthcare/modules/tpt-screening/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all runtime configuration for the screening server.
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

// Server is the tpt-screening HTTP server.
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

// NewServer creates and wires the screening server.
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

	// Programme enrolments
	enrolments := &enrolmentHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/screening/enrolments", chain(http.HandlerFunc(enrolments.List)))
	mux.Handle("POST /api/v1/screening/enrolments", chain(http.HandlerFunc(enrolments.Create)))
	mux.Handle("GET /api/v1/screening/enrolments/{id}", chain(http.HandlerFunc(enrolments.Get)))
	mux.Handle("PUT /api/v1/screening/enrolments/{id}", chain(http.HandlerFunc(enrolments.Update)))
	mux.Handle("POST /api/v1/screening/enrolments/{id}/suspend", chain(http.HandlerFunc(enrolments.Suspend)))
	mux.Handle("POST /api/v1/screening/enrolments/{id}/withdraw", chain(http.HandlerFunc(enrolments.Withdraw)))

	// Recall notifications
	recalls := &recallHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/screening/recalls", chain(http.HandlerFunc(recalls.List)))
	mux.Handle("POST /api/v1/screening/recalls", chain(http.HandlerFunc(recalls.Create)))
	mux.Handle("GET /api/v1/screening/recalls/{id}", chain(http.HandlerFunc(recalls.Get)))
	mux.Handle("PUT /api/v1/screening/recalls/{id}", chain(http.HandlerFunc(recalls.Update)))
	mux.Handle("POST /api/v1/screening/recalls/{id}/send", chain(http.HandlerFunc(recalls.Send)))
	mux.Handle("POST /api/v1/screening/recalls/{id}/acknowledge", chain(http.HandlerFunc(recalls.Acknowledge)))
	mux.Handle("POST /api/v1/screening/recalls/{id}/complete", chain(http.HandlerFunc(recalls.Complete)))

	// Screening results
	results := &resultHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/screening/results", chain(http.HandlerFunc(results.List)))
	mux.Handle("POST /api/v1/screening/results", chain(http.HandlerFunc(results.Create)))
	mux.Handle("GET /api/v1/screening/results/{id}", chain(http.HandlerFunc(results.Get)))
	mux.Handle("PUT /api/v1/screening/results/{id}", chain(http.HandlerFunc(results.Update)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-screening",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-screening"})
}

// RunMigrations runs the module's embedded SQL migrations.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := coredb.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(screeningdb.Migrations, pool)
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
