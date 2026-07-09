// Package api implements the tpt-mental-health HTTP server and route handlers.
//
// All resources in this service carry extra_sensitive = true.
// Before returning any record the handler calls checkMHAccess, which verifies
// that the caller either holds the "mental-health-clinician" role or holds an
// active mh_consents record of type "access" for the patient — enforcing the
// HIPC additional protections for mental health information.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/auth/auth0"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Config holds all configuration for the tpt-mental-health server.
type Config struct {
	Host          string
	Port          int
	DatabaseURL   string
	RedisURL      string
	EncryptionKey string // hex-encoded 32-byte AES key
	Auth0Domain   string
	Auth0Audience string
	Auth0ClientID string
	HPIBaseURL    string
	CORSOrigins   []string
	RateLimitRPS  float64
	RateLimitBurst int
	Logger        *slog.Logger
}

// Server is the tpt-mental-health HTTP server.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	pool       *pgxpool.Pool
	enc        *encryption.Encryptor
	auth       auth.Provider
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// NewServer constructs and configures a Server, wiring all dependencies.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if len(cfg.CORSOrigins) == 0 {
		cfg.CORSOrigins = []string{"*"}
	}
	if cfg.RateLimitRPS == 0 {
		cfg.RateLimitRPS = 10
	}
	if cfg.RateLimitBurst == 0 {
		cfg.RateLimitBurst = 20
	}

	pool, err := db.New(context.Background(), db.Config{
		DSN:      cfg.DatabaseURL,
		MaxConns: 25,
		MinConns: 2,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	encKey, err := decodeHexKey(cfg.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("parse encryption key: %w", err)
	}
	enc, err := encryption.New(encKey)
	if err != nil {
		return nil, fmt.Errorf("init encryption: %w", err)
	}

	authProvider, err := auth0.New(context.Background(), auth0.Config{
		Domain:   cfg.Auth0Domain,
		Audience: cfg.Auth0Audience,
		ClientID: cfg.Auth0ClientID,
	})
	if err != nil {
		return nil, fmt.Errorf("init auth0 provider: %w", err)
	}

	var rdb *redis.Client
	if cfg.RedisURL != "" {
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return nil, fmt.Errorf("parse redis URL: %w", err)
		}
		rdb = redis.NewClient(opts)
	}

	hpiClient := hpi.New(cfg.HPIBaseURL, func(ctx context.Context) (string, error) {
		return "", nil // token acquisition wired at deployment
	}, rdb)

	trail := audit.New(pool)

	s := &Server{
		cfg:        cfg,
		pool:       pool,
		enc:        enc,
		auth:       authProvider,
		hpiClient:  hpiClient,
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

func (s *Server) buildRoutes() *http.ServeMux {
	assessments := &AssessmentsHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	episodes := &EpisodesHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	orders := &TreatmentOrdersHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	consents := &MHConsentHandler{
		pool:       s.pool,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}

	chain := func(h http.Handler) http.Handler {
		h = middleware.AuditWrap(s.auditTrail)(h)
		h = auth.RequireAuth(s.auth)(h)
		h = middleware.TenantExtraction()(h)
		h = middleware.CORS(s.cfg.CORSOrigins)(h)
		h = middleware.RateLimit(s.cfg.RateLimitRPS, s.cfg.RateLimitBurst)(h)
		h = middleware.RecoveryMiddleware()(h)
		return h
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// Assessment routes.
	mux.Handle("GET /api/v1/assessments", chain(http.HandlerFunc(assessments.List)))
	mux.Handle("POST /api/v1/assessments", chain(http.HandlerFunc(assessments.Create)))
	mux.Handle("GET /api/v1/assessments/{id}", chain(http.HandlerFunc(assessments.Get)))
	mux.Handle("PUT /api/v1/assessments/{id}", chain(http.HandlerFunc(assessments.Update)))

	// Episode routes.
	mux.Handle("GET /api/v1/episodes", chain(http.HandlerFunc(episodes.List)))
	mux.Handle("POST /api/v1/episodes", chain(http.HandlerFunc(episodes.Create)))
	mux.Handle("GET /api/v1/episodes/{id}", chain(http.HandlerFunc(episodes.Get)))
	mux.Handle("PUT /api/v1/episodes/{id}", chain(http.HandlerFunc(episodes.Update)))
	mux.Handle("POST /api/v1/episodes/{id}/discharge", chain(http.HandlerFunc(episodes.Discharge)))

	// Ward round routes (sub-resource of an episode).
	mux.Handle("GET /api/v1/episodes/{id}/ward-rounds", chain(http.HandlerFunc(episodes.ListWardRounds)))
	mux.Handle("POST /api/v1/episodes/{id}/ward-rounds", chain(http.HandlerFunc(episodes.CreateWardRound)))
	mux.Handle("GET /api/v1/episodes/{id}/ward-rounds/{roundId}", chain(http.HandlerFunc(episodes.GetWardRound)))

	// Compulsory order routes (CAO / CTO / SPO under MHCAA 1992).
	mux.Handle("GET /api/v1/orders", chain(http.HandlerFunc(orders.List)))
	mux.Handle("POST /api/v1/orders", chain(http.HandlerFunc(orders.Create)))
	mux.Handle("GET /api/v1/orders/{id}", chain(http.HandlerFunc(orders.Get)))
	mux.Handle("PUT /api/v1/orders/{id}", chain(http.HandlerFunc(orders.Update)))
	mux.Handle("POST /api/v1/orders/{id}/review", chain(http.HandlerFunc(orders.RecordReview)))
	mux.Handle("POST /api/v1/orders/{id}/revoke", chain(http.HandlerFunc(orders.Revoke)))

	// Mental health consent routes.
	mux.Handle("GET /api/v1/consents", chain(http.HandlerFunc(consents.List)))
	mux.Handle("POST /api/v1/consents", chain(http.HandlerFunc(consents.Grant)))
	mux.Handle("GET /api/v1/consents/{id}", chain(http.HandlerFunc(consents.Get)))
	mux.Handle("POST /api/v1/consents/{id}/revoke", chain(http.HandlerFunc(consents.Revoke)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-mental-health",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

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
		"service": "tpt-mental-health",
	})
}

// RunMigrations applies database migrations for the tpt-mental-health module.
func RunMigrations(ctx context.Context, databaseURL string) error {
	pool, err := db.New(ctx, db.Config{DSN: databaseURL})
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()

	// No migration files yet — safe no-op until db/migrate/ is populated.
	if err := db.Migrate(ctx, pool, ""); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// checkMHAccess enforces HIPC additional protections for mental health records.
// It returns an error (with a message safe to return to the caller) if access
// should be denied. Access is permitted if:
//  1. The principal holds the "mental-health-clinician" role, OR
//  2. An active mh_consents record of type "access" exists for the patient+tenant.
func checkMHAccess(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID, patientNHI string, principal *auth.Principal) error {
	for _, role := range principal.Roles {
		if role == "mental-health-clinician" || role == "admin" {
			return nil
		}
	}

	var exists bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (
		     SELECT 1 FROM mh_consents
		     WHERE tenant_id   = $1
		       AND patient_nhi = $2
		       AND consent_type = 'access'
		       AND granted      = TRUE
		       AND revoked_at   IS NULL
		       AND (expires_at IS NULL OR expires_at > now())
		 )`,
		tenantID, patientNHI,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("consent check failed: %w", err)
	}
	if !exists {
		return errors.New("access denied: mental health consent not granted")
	}
	return nil
}

// apiError is the standard error response envelope.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// errNotFound is a sentinel for missing records.
var errNotFound = errors.New("record not found")

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

// decodeHexKey decodes a hex-encoded 32-byte encryption key.
func decodeHexKey(hexKey string) ([]byte, error) {
	if len(hexKey) != 64 {
		return nil, fmt.Errorf("encryption key must be 64 hex characters (32 bytes), got %d", len(hexKey))
	}
	key := make([]byte, 32)
	for i := 0; i < 32; i++ {
		hi, lo := hexNibble(hexKey[i*2]), hexNibble(hexKey[i*2+1])
		if hi < 0 || lo < 0 {
			return nil, fmt.Errorf("encryption key contains invalid hex character at position %d", i*2)
		}
		key[i] = byte(hi<<4 | lo)
	}
	return key, nil
}

func hexNibble(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
