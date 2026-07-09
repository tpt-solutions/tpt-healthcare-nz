// Package api implements the tpt-massage HTTP server.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	coreAcc "github.com/PhillipC05/tpt-healthcare/core/acc"
	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/auth/auth0"
	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/db/migrate"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	massdb "github.com/PhillipC05/tpt-healthcare/modules/tpt-massage/db"
)

// Config holds all configuration for the tpt-massage server.
type Config struct {
	Host              string
	Port              int
	DatabaseURL       string
	RedisURL          string
	EncryptionKey     string
	Auth0Domain       string
	Auth0Audience     string
	TenantHeader      string
	ACCBaseURL        string
	ACCProviderNumber string // Practice ACC provider registration number
	HPIBaseURL        string
	Logger            *slog.Logger
}

// Server is the tpt-massage HTTP server.
type Server struct {
	cfg          Config
	mux          *http.ServeMux
	pool         db.Pool
	enc          *encryption.Cipher
	auth         auth.Provider
	hpiClient    *hpi.Client
	consentStore *consent.Store
	auditTrail   *audit.Trail
	accClient    *coreAcc.Client
	logger       *slog.Logger
}

// NewServer constructs and configures a Server.
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

	s := &Server{
		cfg:          cfg,
		pool:         pool,
		enc:          enc,
		auth:         authProvider,
		hpiClient:    hpi.NewClient(cfg.RedisURL, cfg.Logger),
		consentStore: consent.NewStore(pool),
		auditTrail:   audit.NewTrail(pool),
		logger:       cfg.Logger,
	}
	if cfg.ACCBaseURL != "" {
		s.accClient = coreAcc.New(cfg.ACCBaseURL, func(_ context.Context) (string, error) {
			return "", fmt.Errorf("ACC SMART on FHIR token acquisition not yet implemented")
		})
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

	mux.Handle("GET /api/v1/acc/claims", chain(http.HandlerFunc(s.handleListClaims)))
	mux.Handle("POST /api/v1/acc/claims", chain(http.HandlerFunc(s.handleCreateClaim)))
	mux.Handle("GET /api/v1/acc/claims/{claimId}", chain(http.HandlerFunc(s.handleGetClaim)))
	mux.Handle("PUT /api/v1/acc/claims/{claimId}", chain(http.HandlerFunc(s.handleUpdateClaim)))
	mux.Handle("POST /api/v1/acc/claims/{claimId}/submit", chain(http.HandlerFunc(s.handleSubmitClaim)))
	mux.Handle("GET /api/v1/acc/claims/{claimId}/po", chain(http.HandlerFunc(s.handleGetClaimPO)))
	mux.Handle("POST /api/v1/acc/claims/{claimId}/po/request", chain(http.HandlerFunc(s.handleRequestPOExtension)))

	mux.Handle("GET /api/v1/soap-notes/{patientNhi}", chain(http.HandlerFunc(s.handleListSOAPNotes)))
	mux.Handle("POST /api/v1/soap-notes", chain(http.HandlerFunc(s.handleCreateSOAPNote)))
	mux.Handle("GET /api/v1/soap-notes/{patientNhi}/{noteId}", chain(http.HandlerFunc(s.handleGetSOAPNote)))
	mux.Handle("PUT /api/v1/soap-notes/{patientNhi}/{noteId}", chain(http.HandlerFunc(s.handleUpdateSOAPNote)))

	mux.Handle("GET /api/v1/screening/{patientNhi}", chain(http.HandlerFunc(s.handleGetScreening)))
	mux.Handle("POST /api/v1/screening", chain(http.HandlerFunc(s.handleCreateScreening)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-massage",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "reason": "database not reachable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-massage"})
}

// RunMigrations runs database migrations.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(massdb.Migrations, pool)
	return r.Up(ctx)
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
	cfg.Logger.Info("connectivity validation complete")
	return nil
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("writeJSON encode error", slog.Any("error", err))
	}
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}
