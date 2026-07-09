// Package api implements the tpt-palliative HTTP server.
// Covers hospice patient management, advance care planning, and WHO analgesic ladder pain protocols.
// All palliative resources carry extra_sensitive = true for enhanced HIPC protection.
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
	palldb "github.com/PhillipC05/tpt-healthcare/modules/tpt-palliative/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all runtime configuration for the palliative server.
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

// Server is the tpt-palliative HTTP server.
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

// NewServer constructs and configures a Server.
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

// Handler returns the root HTTP handler for the server.
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

	// Hospice / palliative patient routes
	hospice := &hospiceHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/palliative/patients", chain(http.HandlerFunc(hospice.ListPatients)))
	mux.Handle("POST /api/v1/palliative/patients", chain(http.HandlerFunc(hospice.CreatePatient)))
	mux.Handle("GET /api/v1/palliative/patients/{patientId}", chain(http.HandlerFunc(hospice.GetPatient)))
	mux.Handle("PUT /api/v1/palliative/patients/{patientId}", chain(http.HandlerFunc(hospice.UpdatePatient)))
	mux.Handle("GET /api/v1/palliative/patients/{patientId}/visits", chain(http.HandlerFunc(hospice.ListVisits)))
	mux.Handle("POST /api/v1/palliative/patients/{patientId}/visits", chain(http.HandlerFunc(hospice.RecordVisit)))
	mux.Handle("GET /api/v1/palliative/patients/{patientId}/goals-of-care", chain(http.HandlerFunc(hospice.ListGoalsOfCare)))
	mux.Handle("POST /api/v1/palliative/patients/{patientId}/goals-of-care", chain(http.HandlerFunc(hospice.AddGoalOfCare)))

	// Advance Care Planning routes
	acp := &acpHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/palliative/acp-plans", chain(http.HandlerFunc(acp.ListPlans)))
	mux.Handle("POST /api/v1/palliative/acp-plans", chain(http.HandlerFunc(acp.CreatePlan)))
	mux.Handle("GET /api/v1/palliative/acp-plans/{planId}", chain(http.HandlerFunc(acp.GetPlan)))
	mux.Handle("PUT /api/v1/palliative/acp-plans/{planId}", chain(http.HandlerFunc(acp.UpdatePlan)))
	mux.Handle("GET /api/v1/palliative/acp-plans/{planId}/decisions", chain(http.HandlerFunc(acp.ListDecisions)))
	mux.Handle("POST /api/v1/palliative/acp-plans/{planId}/decisions", chain(http.HandlerFunc(acp.AddDecision)))

	// Pain protocol routes
	pain := &painHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/palliative/pain-assessments", chain(http.HandlerFunc(pain.ListAssessments)))
	mux.Handle("POST /api/v1/palliative/pain-assessments", chain(http.HandlerFunc(pain.CreateAssessment)))
	mux.Handle("GET /api/v1/palliative/pain-assessments/{assessmentId}", chain(http.HandlerFunc(pain.GetAssessment)))
	mux.Handle("GET /api/v1/palliative/pain-protocols", chain(http.HandlerFunc(pain.ListProtocols)))
	mux.Handle("POST /api/v1/palliative/pain-protocols", chain(http.HandlerFunc(pain.CreateProtocol)))
	mux.Handle("GET /api/v1/palliative/pain-protocols/{protocolId}", chain(http.HandlerFunc(pain.GetProtocol)))
	mux.Handle("PUT /api/v1/palliative/pain-protocols/{protocolId}", chain(http.HandlerFunc(pain.UpdateProtocol)))
	mux.Handle("POST /api/v1/palliative/pain-protocols/{protocolId}/outcome", chain(http.HandlerFunc(pain.RecordOutcome)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-palliative",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-palliative"})
}

// RunMigrations runs the module's embedded SQL migrations.
func RunMigrations(ctx context.Context, databaseURL string) error {
	pool, err := coredb.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(palldb.Migrations, pool)
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

func genUUID() string {
	return uuid.New().String()
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
