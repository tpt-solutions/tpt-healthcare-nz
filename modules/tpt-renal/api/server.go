// Package api implements the tpt-renal HTTP server.
// This module covers haemodialysis (HD), peritoneal dialysis (PD), renal
// patient management, and transplant waitlist. Deployable independently to
// serve hospital renal units and community dialysis centres.
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
	renaldb "github.com/PhillipC05/tpt-healthcare/modules/tpt-renal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all runtime configuration for the renal server.
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

// Server is the tpt-renal HTTP server.
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

// NewServer creates and wires the renal server including DB, auth, and encryption.
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
		auditTrail: audit.NewTrail(pool),
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

	// Renal patient registration and CKD staging
	pt := &patientHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/renal/patients", chain(http.HandlerFunc(pt.List)))
	mux.Handle("POST /api/v1/renal/patients", chain(http.HandlerFunc(pt.Register)))
	mux.Handle("GET /api/v1/renal/patients/{id}", chain(http.HandlerFunc(pt.Get)))
	mux.Handle("PUT /api/v1/renal/patients/{id}", chain(http.HandlerFunc(pt.Update)))

	// Haemodialysis session scheduling and charting (Kt/V, UFR, access)
	hd := &hdSessionHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/renal/patients/{id}/hd-sessions", chain(http.HandlerFunc(hd.List)))
	mux.Handle("POST /api/v1/renal/patients/{id}/hd-sessions", chain(http.HandlerFunc(hd.Create)))
	mux.Handle("GET /api/v1/renal/patients/{id}/hd-sessions/{sessionId}", chain(http.HandlerFunc(hd.Get)))
	mux.Handle("PUT /api/v1/renal/patients/{id}/hd-sessions/{sessionId}", chain(http.HandlerFunc(hd.Update)))
	mux.Handle("POST /api/v1/renal/patients/{id}/hd-sessions/{sessionId}/complete", chain(http.HandlerFunc(hd.Complete)))
	mux.Handle("POST /api/v1/renal/patients/{id}/hd-sessions/{sessionId}/abort", chain(http.HandlerFunc(hd.Abort)))

	// Peritoneal dialysis (APD/CAPD) episode management
	pd := &pdHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/renal/patients/{id}/pd-episodes", chain(http.HandlerFunc(pd.List)))
	mux.Handle("POST /api/v1/renal/patients/{id}/pd-episodes", chain(http.HandlerFunc(pd.Create)))
	mux.Handle("GET /api/v1/renal/patients/{id}/pd-episodes/{episodeId}", chain(http.HandlerFunc(pd.Get)))
	mux.Handle("PUT /api/v1/renal/patients/{id}/pd-episodes/{episodeId}", chain(http.HandlerFunc(pd.Update)))
	mux.Handle("GET /api/v1/renal/patients/{id}/pd-episodes/{episodeId}/exchanges", chain(http.HandlerFunc(pd.ListExchanges)))
	mux.Handle("POST /api/v1/renal/patients/{id}/pd-episodes/{episodeId}/exchanges", chain(http.HandlerFunc(pd.RecordExchange)))

	// Renal transplant waitlist management
	tx := &transplantHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/renal/patients/{id}/transplant-waitlist", chain(http.HandlerFunc(tx.List)))
	mux.Handle("POST /api/v1/renal/patients/{id}/transplant-waitlist", chain(http.HandlerFunc(tx.Register)))
	mux.Handle("GET /api/v1/renal/patients/{id}/transplant-waitlist/{entryId}", chain(http.HandlerFunc(tx.Get)))
	mux.Handle("PUT /api/v1/renal/patients/{id}/transplant-waitlist/{entryId}", chain(http.HandlerFunc(tx.Update)))
	mux.Handle("POST /api/v1/renal/patients/{id}/transplant-waitlist/{entryId}/transplant", chain(http.HandlerFunc(tx.Transplant)))
	mux.Handle("POST /api/v1/renal/patients/{id}/transplant-waitlist/{entryId}/hold", chain(http.HandlerFunc(tx.Hold)))
	mux.Handle("POST /api/v1/renal/patients/{id}/transplant-waitlist/{entryId}/remove", chain(http.HandlerFunc(tx.Remove)))

	// Fluid balance and dry-weight tracking
	fb := &fluidBalanceHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/renal/patients/{id}/fluid-balance", chain(http.HandlerFunc(fb.List)))
	mux.Handle("POST /api/v1/renal/patients/{id}/fluid-balance", chain(http.HandlerFunc(fb.Record)))
	mux.Handle("GET /api/v1/renal/patients/{id}/fluid-balance/{recordId}", chain(http.HandlerFunc(fb.Get)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-renal",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-renal"})
}

// RunMigrations runs the renal module's embedded SQL migrations.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := coredb.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(renaldb.Migrations, pool)
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

func notImplemented(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, apiError{
		Code:    "NOT_IMPLEMENTED",
		Message: "this endpoint is not yet implemented",
	})
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
