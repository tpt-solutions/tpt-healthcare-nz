// Package api implements the tpt-oncology HTTP server.
// This module covers chemotherapy protocols, treatment cycles, immunotherapy,
// CTCAE toxicity grading, radiation referrals, and palliative oncology.
// It can be deployed independently to serve oncology centres outside of a
// full hospital deployment.
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
	oncologydb "github.com/PhillipC05/tpt-healthcare/modules/tpt-oncology/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all runtime configuration for the oncology server.
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

// Server is the tpt-oncology HTTP server.
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

// NewServer creates and wires the oncology server including DB, auth, and encryption.
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

	// Oncology patient registration and tumour board referrals
	pt := &patientHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/oncology/patients", chain(http.HandlerFunc(pt.List)))
	mux.Handle("POST /api/v1/oncology/patients", chain(http.HandlerFunc(pt.Register)))
	mux.Handle("GET /api/v1/oncology/patients/{id}", chain(http.HandlerFunc(pt.Get)))
	mux.Handle("PUT /api/v1/oncology/patients/{id}", chain(http.HandlerFunc(pt.Update)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/tumour-board-referrals", chain(http.HandlerFunc(pt.ListTumourBoardReferrals)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/tumour-board-referrals", chain(http.HandlerFunc(pt.CreateTumourBoardReferral)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/tumour-board-referrals/{referralId}", chain(http.HandlerFunc(pt.GetTumourBoardReferral)))

	// Chemotherapy protocol library
	pr := &protocolHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/oncology/protocols", chain(http.HandlerFunc(pr.List)))
	mux.Handle("POST /api/v1/oncology/protocols", chain(http.HandlerFunc(pr.Create)))
	mux.Handle("GET /api/v1/oncology/protocols/{id}", chain(http.HandlerFunc(pr.Get)))
	mux.Handle("PUT /api/v1/oncology/protocols/{id}", chain(http.HandlerFunc(pr.Update)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/protocols", chain(http.HandlerFunc(pr.ListForPatient)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/protocols", chain(http.HandlerFunc(pr.AssignToPatient)))

	// Treatment cycle scheduling and administration
	cy := &cycleHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/oncology/patients/{id}/cycles", chain(http.HandlerFunc(cy.List)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/cycles", chain(http.HandlerFunc(cy.Create)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/cycles/{cycleId}", chain(http.HandlerFunc(cy.Get)))
	mux.Handle("PUT /api/v1/oncology/patients/{id}/cycles/{cycleId}", chain(http.HandlerFunc(cy.Update)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/cycles/{cycleId}/complete", chain(http.HandlerFunc(cy.Complete)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/cycles/{cycleId}/administrations", chain(http.HandlerFunc(cy.ListAdministrations)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/cycles/{cycleId}/administrations", chain(http.HandlerFunc(cy.RecordAdministration)))

	// Immunotherapy and targeted therapy
	it := &immunotherapyHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/oncology/patients/{id}/immunotherapy", chain(http.HandlerFunc(it.List)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/immunotherapy", chain(http.HandlerFunc(it.Create)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/immunotherapy/{episodeId}", chain(http.HandlerFunc(it.Get)))
	mux.Handle("PUT /api/v1/oncology/patients/{id}/immunotherapy/{episodeId}", chain(http.HandlerFunc(it.Update)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/immunotherapy/{episodeId}/hold", chain(http.HandlerFunc(it.Hold)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/immunotherapy/{episodeId}/resume", chain(http.HandlerFunc(it.Resume)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/immunotherapy/{episodeId}/discontinue", chain(http.HandlerFunc(it.Discontinue)))

	// CTCAE toxicity grading
	tx := &toxicityHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/oncology/patients/{id}/toxicity", chain(http.HandlerFunc(tx.List)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/toxicity", chain(http.HandlerFunc(tx.Create)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/toxicity/{assessmentId}", chain(http.HandlerFunc(tx.Get)))
	mux.Handle("PUT /api/v1/oncology/patients/{id}/toxicity/{assessmentId}", chain(http.HandlerFunc(tx.Update)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/toxicity/{assessmentId}/events", chain(http.HandlerFunc(tx.ListEvents)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/toxicity/{assessmentId}/events", chain(http.HandlerFunc(tx.AddEvent)))

	// Radiation therapy referrals
	rt := &radiationHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/oncology/patients/{id}/radiation-referrals", chain(http.HandlerFunc(rt.List)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/radiation-referrals", chain(http.HandlerFunc(rt.Create)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/radiation-referrals/{referralId}", chain(http.HandlerFunc(rt.Get)))
	mux.Handle("PUT /api/v1/oncology/patients/{id}/radiation-referrals/{referralId}", chain(http.HandlerFunc(rt.Update)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/radiation-referrals/{referralId}/fractions", chain(http.HandlerFunc(rt.ListFractions)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/radiation-referrals/{referralId}/fractions", chain(http.HandlerFunc(rt.RecordFraction)))

	// Palliative oncology pathways
	pa := &palliativeHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/oncology/patients/{id}/palliative", chain(http.HandlerFunc(pa.List)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/palliative", chain(http.HandlerFunc(pa.Create)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/palliative/{planId}", chain(http.HandlerFunc(pa.Get)))
	mux.Handle("PUT /api/v1/oncology/patients/{id}/palliative/{planId}", chain(http.HandlerFunc(pa.Update)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/palliative/{planId}/goals", chain(http.HandlerFunc(pa.ListGoals)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/palliative/{planId}/goals", chain(http.HandlerFunc(pa.AddGoal)))
	mux.Handle("PUT /api/v1/oncology/patients/{id}/palliative/{planId}/goals/{goalId}", chain(http.HandlerFunc(pa.UpdateGoal)))
	mux.Handle("GET /api/v1/oncology/patients/{id}/palliative/{planId}/symptoms", chain(http.HandlerFunc(pa.ListSymptoms)))
	mux.Handle("POST /api/v1/oncology/patients/{id}/palliative/{planId}/symptoms", chain(http.HandlerFunc(pa.RecordSymptom)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-oncology",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-oncology"})
}

// RunMigrations runs the oncology module's embedded SQL migrations.
func RunMigrations(ctx context.Context, databaseURL string) error {
	pool, err := coredb.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(oncologydb.Migrations, pool)
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
