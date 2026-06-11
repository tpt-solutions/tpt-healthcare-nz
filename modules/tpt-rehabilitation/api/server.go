// Package api implements the tpt-rehabilitation HTTP server.
// Covers inpatient rehabilitation admissions, therapy goal setting (STG/LTG),
// FIM scoring, community rehabilitation episodes, ACC rehabilitation plans,
// and NASC referrals for discharge planning.
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
	rehabdb "github.com/PhillipC05/tpt-healthcare/modules/tpt-rehabilitation/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all runtime configuration for the rehabilitation server.
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

// Server is the tpt-rehabilitation HTTP server.
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

// NewServer creates and wires the rehabilitation server.
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
		h = middleware.AuditWrap(s.auditTrail)(h)
		h = auth.RequireAuth(s.auth)(h)
		h = middleware.TenantExtraction()(h)
		h = middleware.CORS([]string{"*"})(h)
		h = middleware.RateLimit(10, 30)(h)
		h = middleware.RecoveryMiddleware()(h)
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

	// Inpatient rehabilitation admissions and functional assessment
	adm := &admissionHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/rehab/admissions", chain(http.HandlerFunc(adm.List)))
	mux.Handle("POST /api/v1/rehab/admissions", chain(http.HandlerFunc(adm.Create)))
	mux.Handle("GET /api/v1/rehab/admissions/{id}", chain(http.HandlerFunc(adm.Get)))
	mux.Handle("PUT /api/v1/rehab/admissions/{id}", chain(http.HandlerFunc(adm.Update)))
	mux.Handle("POST /api/v1/rehab/admissions/{id}/discharge", chain(http.HandlerFunc(adm.Discharge)))

	// Therapy goal setting — STG/LTG with discipline tracking
	goals := &goalsHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/rehab/admissions/{id}/goals", chain(http.HandlerFunc(goals.List)))
	mux.Handle("POST /api/v1/rehab/admissions/{id}/goals", chain(http.HandlerFunc(goals.Create)))
	mux.Handle("GET /api/v1/rehab/admissions/{id}/goals/{goalId}", chain(http.HandlerFunc(goals.Get)))
	mux.Handle("PUT /api/v1/rehab/admissions/{id}/goals/{goalId}", chain(http.HandlerFunc(goals.Update)))

	// FIM (Functional Independence Measure) scoring
	fim := &fimHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/rehab/admissions/{id}/fim", chain(http.HandlerFunc(fim.List)))
	mux.Handle("POST /api/v1/rehab/admissions/{id}/fim", chain(http.HandlerFunc(fim.Create)))
	mux.Handle("GET /api/v1/rehab/admissions/{id}/fim/{assessmentId}", chain(http.HandlerFunc(fim.Get)))

	// Community rehabilitation episodes (post-discharge follow-up)
	com := &communityHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/rehab/community", chain(http.HandlerFunc(com.List)))
	mux.Handle("POST /api/v1/rehab/community", chain(http.HandlerFunc(com.Create)))
	mux.Handle("GET /api/v1/rehab/community/{id}", chain(http.HandlerFunc(com.Get)))
	mux.Handle("PUT /api/v1/rehab/community/{id}", chain(http.HandlerFunc(com.Update)))
	mux.Handle("POST /api/v1/rehab/community/{id}/complete", chain(http.HandlerFunc(com.Complete)))

	// ACC rehabilitation plan management
	acc := &accHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/rehab/acc-plans", chain(http.HandlerFunc(acc.List)))
	mux.Handle("POST /api/v1/rehab/acc-plans", chain(http.HandlerFunc(acc.Create)))
	mux.Handle("GET /api/v1/rehab/acc-plans/{id}", chain(http.HandlerFunc(acc.Get)))
	mux.Handle("PUT /api/v1/rehab/acc-plans/{id}", chain(http.HandlerFunc(acc.Update)))
	mux.Handle("POST /api/v1/rehab/acc-plans/{id}/submit", chain(http.HandlerFunc(acc.Submit)))
	mux.Handle("POST /api/v1/rehab/acc-plans/{id}/approve", chain(http.HandlerFunc(acc.Approve)))

	// Discharge planning and NASC referrals
	nasc := &nascHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/rehab/nasc", chain(http.HandlerFunc(nasc.List)))
	mux.Handle("POST /api/v1/rehab/nasc", chain(http.HandlerFunc(nasc.Create)))
	mux.Handle("GET /api/v1/rehab/nasc/{id}", chain(http.HandlerFunc(nasc.Get)))
	mux.Handle("PUT /api/v1/rehab/nasc/{id}", chain(http.HandlerFunc(nasc.Update)))
	mux.Handle("POST /api/v1/rehab/nasc/{id}/submit", chain(http.HandlerFunc(nasc.Submit)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-rehabilitation",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-rehabilitation"})
}

// RunMigrations runs the module's embedded SQL migrations.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := coredb.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(rehabdb.Migrations, pool)
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
