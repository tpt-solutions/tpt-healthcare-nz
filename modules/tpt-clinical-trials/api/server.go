// Package api implements the tpt-clinical-trials HTTP server.
// This module covers ICH E6(R3) GCP-compliant clinical trial management:
// study protocol library, participant enrolment and randomisation, scheduled
// study visits with CRF capture, and adverse event / SAE / SUSAR reporting
// to Medsafe under the Medicines Act 1981.
// It can be deployed independently to serve trial sites, sponsors, or CROs.
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
	trialsdb "github.com/PhillipC05/tpt-healthcare/modules/tpt-clinical-trials/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all runtime configuration for the clinical trials server.
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

// Server is the tpt-clinical-trials HTTP server.
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

// NewServer creates and wires the clinical trials server including DB, auth, and encryption.
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

	// Study protocol library
	pr := &protocolHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/trials/protocols", chain(http.HandlerFunc(pr.List)))
	mux.Handle("POST /api/v1/trials/protocols", chain(http.HandlerFunc(pr.Create)))
	mux.Handle("GET /api/v1/trials/protocols/{id}", chain(http.HandlerFunc(pr.Get)))
	mux.Handle("PUT /api/v1/trials/protocols/{id}", chain(http.HandlerFunc(pr.Update)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/activate", chain(http.HandlerFunc(pr.Activate)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/close", chain(http.HandlerFunc(pr.Close)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/arms", chain(http.HandlerFunc(pr.ListArms)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/arms", chain(http.HandlerFunc(pr.CreateArm)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/eligibility", chain(http.HandlerFunc(pr.GetEligibility)))
	mux.Handle("PUT /api/v1/trials/protocols/{id}/eligibility", chain(http.HandlerFunc(pr.UpdateEligibility)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/schedule", chain(http.HandlerFunc(pr.GetSchedule)))
	mux.Handle("PUT /api/v1/trials/protocols/{id}/schedule", chain(http.HandlerFunc(pr.UpdateSchedule)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/amendments", chain(http.HandlerFunc(pr.ListAmendments)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/amendments", chain(http.HandlerFunc(pr.CreateAmendment)))

	// Participant enrolment and randomisation
	pa := &participantHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/trials/protocols/{id}/participants", chain(http.HandlerFunc(pa.List)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/screen", chain(http.HandlerFunc(pa.Screen)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/participants/{participantId}", chain(http.HandlerFunc(pa.Get)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/enrol", chain(http.HandlerFunc(pa.Enrol)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/randomise", chain(http.HandlerFunc(pa.Randomise)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/withdraw", chain(http.HandlerFunc(pa.Withdraw)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/complete", chain(http.HandlerFunc(pa.Complete)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/participants/{participantId}/consent", chain(http.HandlerFunc(pa.GetConsent)))
	mux.Handle("PUT /api/v1/trials/protocols/{id}/participants/{participantId}/consent", chain(http.HandlerFunc(pa.UpdateConsent)))
	mux.Handle("GET /api/v1/trials/screening-log", chain(http.HandlerFunc(pa.ScreeningLog)))

	// Study visit scheduling and CRF capture
	vi := &visitHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/trials/protocols/{id}/participants/{participantId}/visits", chain(http.HandlerFunc(vi.List)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/visits", chain(http.HandlerFunc(vi.Create)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/participants/{participantId}/visits/{visitId}", chain(http.HandlerFunc(vi.Get)))
	mux.Handle("PUT /api/v1/trials/protocols/{id}/participants/{participantId}/visits/{visitId}", chain(http.HandlerFunc(vi.Update)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/visits/{visitId}/complete", chain(http.HandlerFunc(vi.Complete)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/participants/{participantId}/visits/{visitId}/crf", chain(http.HandlerFunc(vi.GetCRF)))
	mux.Handle("PUT /api/v1/trials/protocols/{id}/participants/{participantId}/visits/{visitId}/crf", chain(http.HandlerFunc(vi.SaveCRF)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/participants/{participantId}/deviations", chain(http.HandlerFunc(vi.ListDeviations)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/deviations", chain(http.HandlerFunc(vi.RecordDeviation)))

	// Adverse event and safety reporting
	ae := &adverseEventHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/trials/protocols/{id}/participants/{participantId}/adverse-events", chain(http.HandlerFunc(ae.List)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/adverse-events", chain(http.HandlerFunc(ae.Create)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/participants/{participantId}/adverse-events/{aeId}", chain(http.HandlerFunc(ae.Get)))
	mux.Handle("PUT /api/v1/trials/protocols/{id}/participants/{participantId}/adverse-events/{aeId}", chain(http.HandlerFunc(ae.Update)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/adverse-events/{aeId}/resolve", chain(http.HandlerFunc(ae.Resolve)))
	mux.Handle("GET /api/v1/trials/protocols/{id}/participants/{participantId}/adverse-events/{aeId}/sae", chain(http.HandlerFunc(ae.GetSAE)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/adverse-events/{aeId}/sae", chain(http.HandlerFunc(ae.ReportSAE)))
	mux.Handle("PUT /api/v1/trials/protocols/{id}/participants/{participantId}/adverse-events/{aeId}/sae", chain(http.HandlerFunc(ae.UpdateSAE)))
	mux.Handle("POST /api/v1/trials/protocols/{id}/participants/{participantId}/adverse-events/{aeId}/susar", chain(http.HandlerFunc(ae.ReportSUSAR)))
	mux.Handle("GET /api/v1/trials/safety-report", chain(http.HandlerFunc(ae.SafetyReport)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-clinical-trials",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-clinical-trials"})
}

// RunMigrations runs the clinical trials module's embedded SQL migrations.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := coredb.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(trialsdb.Migrations, pool)
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
