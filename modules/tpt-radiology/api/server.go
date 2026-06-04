// Package api implements the tpt-radiology HTTP server and route handlers.
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
	"github.com/PhillipC05/tpt-healthcare/core/subscription"

	"github.com/redis/go-redis/v9"
)

// ImagingStudyTopic is the canonical SubscriptionTopic URL for new imaging studies.
const ImagingStudyTopic = "https://standards.digital.health.nz/fhir/SubscriptionTopic/new-imaging-study"

// Config holds all configuration for the tpt-radiology server.
type Config struct {
	Host          string
	Port          int
	DatabaseURL   string
	RedisURL      string
	EncryptionKey string
	Auth0Domain   string
	Auth0Audience string
	TenantHeader  string
	// OrthancURL is the base URL of the Orthanc DICOM server (e.g. http://localhost:8042).
	OrthancURL string
	// OrthancAPIKey is used as a Bearer token when calling Orthanc. Leave empty if
	// Orthanc is configured without authentication (development only).
	OrthancAPIKey string
	Logger        *slog.Logger
}

// Server is the tpt-radiology HTTP server.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	pool       db.Pool
	enc        *encryption.Cipher
	auth       auth.Provider
	orthanc    *OrthancClient
	subEngine  *subscription.Engine
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
	if cfg.OrthancURL == "" {
		cfg.OrthancURL = "http://localhost:8042"
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

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
	subEng := subscription.New(rdb)

	orthancClient := NewOrthancClient(cfg.OrthancURL, cfg.OrthancAPIKey, cfg.Logger)
	trail := audit.NewTrail(pool)

	s := &Server{
		cfg:        cfg,
		pool:       pool,
		enc:        enc,
		auth:       authProvider,
		orthanc:    orthancClient,
		subEngine:  subEng,
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
	studies := &StudiesHandler{
		pool:       s.pool,
		enc:        s.enc,
		orthanc:    s.orthanc,
		subEngine:  s.subEngine,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	orders := &OrdersHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	reports := &ReportsHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	sharing := &SharingHandler{
		pool:       s.pool,
		enc:        s.enc,
		orthanc:    s.orthanc,
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

	// FHIR ImagingStudy resource routes.
	mux.Handle("GET /api/v1/imaging-studies", chain(http.HandlerFunc(studies.List)))
	mux.Handle("POST /api/v1/imaging-studies", chain(http.HandlerFunc(studies.Create)))
	mux.Handle("GET /api/v1/imaging-studies/{id}", chain(http.HandlerFunc(studies.Get)))
	mux.Handle("PUT /api/v1/imaging-studies/{id}", chain(http.HandlerFunc(studies.Update)))
	mux.Handle("POST /api/v1/imaging-studies/{id}/share", chain(http.HandlerFunc(sharing.CreateShare)))
	mux.Handle("DELETE /api/v1/imaging-studies/{id}/shares/{shareId}", chain(http.HandlerFunc(sharing.RevokeShare)))

	// DICOMweb proxy routes — QIDO-RS (query).
	mux.Handle("GET /api/v1/dicom-web/studies", chain(http.HandlerFunc(studies.QIDOStudies)))
	mux.Handle("GET /api/v1/dicom-web/studies/{study}/series", chain(http.HandlerFunc(studies.QIDOSeries)))
	mux.Handle("GET /api/v1/dicom-web/studies/{study}/series/{series}/instances", chain(http.HandlerFunc(studies.QIDOInstances)))

	// DICOMweb proxy routes — WADO-RS (retrieve).
	mux.Handle("GET /api/v1/dicom-web/studies/{study}", chain(http.HandlerFunc(studies.WADOStudy)))
	mux.Handle("GET /api/v1/dicom-web/studies/{study}/series/{series}/instances/{instance}", chain(http.HandlerFunc(studies.WADOInstance)))
	mux.Handle("GET /api/v1/dicom-web/studies/{study}/series/{series}/instances/{instance}/frames/{frame}", chain(http.HandlerFunc(studies.WADOFrame)))

	// DICOMweb proxy routes — STOW-RS (store).
	mux.Handle("POST /api/v1/dicom-web/studies", chain(http.HandlerFunc(studies.STOWStudy)))
	mux.Handle("POST /api/v1/dicom-web/studies/{study}", chain(http.HandlerFunc(studies.STOWStudy)))

	// RIS radiology order routes.
	mux.Handle("GET /api/v1/radiology-orders", chain(http.HandlerFunc(orders.List)))
	mux.Handle("POST /api/v1/radiology-orders", chain(http.HandlerFunc(orders.Create)))
	mux.Handle("GET /api/v1/radiology-orders/{id}", chain(http.HandlerFunc(orders.Get)))
	mux.Handle("PUT /api/v1/radiology-orders/{id}", chain(http.HandlerFunc(orders.Update)))
	mux.Handle("POST /api/v1/radiology-orders/{id}/schedule", chain(http.HandlerFunc(orders.Schedule)))
	mux.Handle("POST /api/v1/radiology-orders/{id}/complete", chain(http.HandlerFunc(orders.Complete)))
	mux.Handle("POST /api/v1/radiology-orders/{id}/cancel", chain(http.HandlerFunc(orders.Cancel)))

	// Radiology report routes.
	mux.Handle("GET /api/v1/radiology-reports", chain(http.HandlerFunc(reports.List)))
	mux.Handle("POST /api/v1/radiology-reports", chain(http.HandlerFunc(reports.Create)))
	mux.Handle("GET /api/v1/radiology-reports/{id}", chain(http.HandlerFunc(reports.Get)))
	mux.Handle("PUT /api/v1/radiology-reports/{id}", chain(http.HandlerFunc(reports.Update)))
	mux.Handle("POST /api/v1/radiology-reports/{id}/sign", chain(http.HandlerFunc(reports.Sign)))
	mux.Handle("POST /api/v1/radiology-reports/{id}/amend", chain(http.HandlerFunc(reports.Amend)))

	// Public share access — only Recovery middleware; token validates access.
	mux.Handle("GET /api/v1/share/{token}", middleware.Recovery(http.HandlerFunc(sharing.AccessShare), s.logger))

	return mux
}

// handleHealth responds to liveness probes.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-radiology",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// handleReady responds to readiness probes, checking database and Orthanc connectivity.
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
		"service": "tpt-radiology",
	})
}

// RunMigrations runs database migrations for the tpt-radiology module.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, logger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// ValidateConnectivity checks that the database and Orthanc are reachable.
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

	orthanc := NewOrthancClient(cfg.OrthancURL, cfg.OrthancAPIKey, cfg.Logger)
	if err := orthanc.Ping(ctx); err != nil {
		cfg.Logger.Warn("Orthanc not reachable (non-fatal in dev)", slog.Any("error", err))
	} else {
		cfg.Logger.Info("Orthanc connectivity OK")
	}

	cfg.Logger.Info("connectivity validation complete")
	return nil
}

// errNotFound is returned by data-access helpers when a record is not found.
var errNotFound = fmt.Errorf("not found")

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
