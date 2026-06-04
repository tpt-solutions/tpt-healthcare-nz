// Package api implements the tpt-aged-care HTTP server and route handlers.
//
// This module covers four aged-care domains:
//   - interRAI assessments (HC, LTCF, CA, CHA, PAC instruments)
//   - NASC (Needs Assessment and Service Coordination) referrals and plans
//   - Funded hours management (MoH home support allocations and timesheets)
//   - Care plans for residential and home care workflows
//
// All handlers validate practitioner APC via core/hpi before write operations
// and emit audit events for every read/write, per HIPC Rule 10.
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
	"github.com/redis/go-redis/v9"
)

// Config holds all configuration for the tpt-aged-care server.
type Config struct {
	Host           string
	Port           int
	DatabaseURL    string
	RedisURL       string
	EncryptionKey  string // hex-encoded 32-byte AES key
	Auth0Domain    string
	Auth0Audience  string
	Auth0ClientID  string
	HPIBaseURL     string
	CORSOrigins    []string
	RateLimitRPS   float64
	RateLimitBurst int
	Logger         *slog.Logger
}

// Server is the tpt-aged-care HTTP server.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	pool       interface{ Ping(context.Context) error; Close() }
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
	interrai := &InterRAIHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	nasc := &NASCHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	funded := &FundedHoursHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	carePlans := &CarePlansHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
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

	// interRAI assessment routes.
	mux.Handle("GET /api/v1/interrai/assessments", chain(http.HandlerFunc(interrai.List)))
	mux.Handle("POST /api/v1/interrai/assessments", chain(http.HandlerFunc(interrai.Create)))
	mux.Handle("GET /api/v1/interrai/assessments/{id}", chain(http.HandlerFunc(interrai.Get)))
	mux.Handle("PUT /api/v1/interrai/assessments/{id}", chain(http.HandlerFunc(interrai.Update)))
	mux.Handle("POST /api/v1/interrai/assessments/{id}/submit", chain(http.HandlerFunc(interrai.Submit)))
	mux.Handle("GET /api/v1/interrai/assessments/{id}/caps", chain(http.HandlerFunc(interrai.GetCAPs)))

	// NASC routes.
	mux.Handle("GET /api/v1/nasc/referrals", chain(http.HandlerFunc(nasc.ListReferrals)))
	mux.Handle("POST /api/v1/nasc/referrals", chain(http.HandlerFunc(nasc.CreateReferral)))
	mux.Handle("GET /api/v1/nasc/referrals/{id}", chain(http.HandlerFunc(nasc.GetReferral)))
	mux.Handle("PUT /api/v1/nasc/referrals/{id}", chain(http.HandlerFunc(nasc.UpdateReferral)))
	mux.Handle("POST /api/v1/nasc/referrals/{id}/complete", chain(http.HandlerFunc(nasc.CompleteReferral)))
	mux.Handle("GET /api/v1/nasc/service-plans", chain(http.HandlerFunc(nasc.ListServicePlans)))
	mux.Handle("POST /api/v1/nasc/service-plans", chain(http.HandlerFunc(nasc.CreateServicePlan)))
	mux.Handle("GET /api/v1/nasc/service-plans/{id}", chain(http.HandlerFunc(nasc.GetServicePlan)))
	mux.Handle("PUT /api/v1/nasc/service-plans/{id}", chain(http.HandlerFunc(nasc.UpdateServicePlan)))
	mux.Handle("POST /api/v1/nasc/service-plans/{id}/review", chain(http.HandlerFunc(nasc.ReviewServicePlan)))

	// Funded hours routes.
	mux.Handle("GET /api/v1/funded-hours/allocations", chain(http.HandlerFunc(funded.ListAllocations)))
	mux.Handle("POST /api/v1/funded-hours/allocations", chain(http.HandlerFunc(funded.CreateAllocation)))
	mux.Handle("GET /api/v1/funded-hours/allocations/{id}", chain(http.HandlerFunc(funded.GetAllocation)))
	mux.Handle("PUT /api/v1/funded-hours/allocations/{id}", chain(http.HandlerFunc(funded.UpdateAllocation)))
	mux.Handle("GET /api/v1/funded-hours/timesheets", chain(http.HandlerFunc(funded.ListTimesheets)))
	mux.Handle("POST /api/v1/funded-hours/timesheets", chain(http.HandlerFunc(funded.CreateTimesheet)))
	mux.Handle("GET /api/v1/funded-hours/timesheets/{id}", chain(http.HandlerFunc(funded.GetTimesheet)))
	mux.Handle("PUT /api/v1/funded-hours/timesheets/{id}/approve", chain(http.HandlerFunc(funded.ApproveTimesheet)))
	mux.Handle("GET /api/v1/funded-hours/summary", chain(http.HandlerFunc(funded.GetSummary)))

	// Care plan routes.
	mux.Handle("GET /api/v1/care-plans", chain(http.HandlerFunc(carePlans.List)))
	mux.Handle("POST /api/v1/care-plans", chain(http.HandlerFunc(carePlans.Create)))
	mux.Handle("GET /api/v1/care-plans/{id}", chain(http.HandlerFunc(carePlans.Get)))
	mux.Handle("PUT /api/v1/care-plans/{id}", chain(http.HandlerFunc(carePlans.Update)))
	mux.Handle("POST /api/v1/care-plans/{id}/review", chain(http.HandlerFunc(carePlans.RecordReview)))
	mux.Handle("POST /api/v1/care-plans/{id}/goals", chain(http.HandlerFunc(carePlans.AddGoal)))
	mux.Handle("PUT /api/v1/care-plans/{id}/goals/{goalId}", chain(http.HandlerFunc(carePlans.UpdateGoal)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-aged-care",
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
		"service": "tpt-aged-care",
	})
}

// RunMigrations applies database migrations for the tpt-aged-care module.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := db.New(ctx, db.Config{DSN: databaseURL})
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, logger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// apiError is the standard error response envelope.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

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
