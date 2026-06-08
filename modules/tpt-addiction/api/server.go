// Package api implements the tpt-addiction HTTP server and route handlers.
// All resources carry extra_sensitive = true. Before returning any record the
// handler checks addiction-clinician role or active addiction_consents record.
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
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// Config holds all configuration for the tpt-addiction server.
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

// Server is the tpt-addiction HTTP server.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	pool       db.Pool
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

// Handler returns the root HTTP handler for the server.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) buildRoutes() *http.ServeMux {
	methadone := &MethadoneHandler{
		pool:       s.pool,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	counselling := &CounsellingHandler{
		pool:       s.pool,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	prescribing := &PrescribingHandler{
		pool:       s.pool,
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
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// -- Methadone programme routes --
	mux.Handle("GET /api/v1/methadone/programmes", chain(http.HandlerFunc(methadone.ListProgrammes)))
	mux.Handle("POST /api/v1/methadone/programmes", chain(http.HandlerFunc(methadone.CreateProgramme)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}", chain(http.HandlerFunc(methadone.GetProgramme)))
	mux.Handle("PUT /api/v1/methadone/programmes/{programmeId}", chain(http.HandlerFunc(methadone.UpdateProgramme)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}/doses", chain(http.HandlerFunc(methadone.ListDoses)))
	mux.Handle("POST /api/v1/methadone/programmes/{programmeId}/doses", chain(http.HandlerFunc(methadone.RecordDose)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}/doses/{doseId}", chain(http.HandlerFunc(methadone.GetDose)))
	mux.Handle("POST /api/v1/methadone/programmes/{programmeId}/take-home", chain(http.HandlerFunc(methadone.ApproveTakeHome)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}/take-home", chain(http.HandlerFunc(methadone.ListTakeHomeHistory)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}/urine-screens", chain(http.HandlerFunc(methadone.ListUrineScreens)))
	mux.Handle("POST /api/v1/methadone/programmes/{programmeId}/urine-screens", chain(http.HandlerFunc(methadone.RecordUrineScreen)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}/urine-screens/{screenId}", chain(http.HandlerFunc(methadone.GetUrineScreen)))

	// -- Addiction counselling routes --
	mux.Handle("GET /api/v1/counselling/sessions", chain(http.HandlerFunc(counselling.ListSessions)))
	mux.Handle("POST /api/v1/counselling/sessions", chain(http.HandlerFunc(counselling.CreateSession)))
	mux.Handle("GET /api/v1/counselling/sessions/{sessionId}", chain(http.HandlerFunc(counselling.GetSession)))
	mux.Handle("PUT /api/v1/counselling/sessions/{sessionId}", chain(http.HandlerFunc(counselling.UpdateSession)))
	mux.Handle("GET /api/v1/counselling/group-sessions", chain(http.HandlerFunc(counselling.ListGroupSessions)))
	mux.Handle("POST /api/v1/counselling/group-sessions", chain(http.HandlerFunc(counselling.CreateGroupSession)))
	mux.Handle("GET /api/v1/counselling/treatment-plans", chain(http.HandlerFunc(counselling.ListTreatmentPlans)))
	mux.Handle("POST /api/v1/counselling/treatment-plans", chain(http.HandlerFunc(counselling.CreateTreatmentPlan)))
	mux.Handle("GET /api/v1/counselling/treatment-plans/{planId}", chain(http.HandlerFunc(counselling.GetTreatmentPlan)))
	mux.Handle("PUT /api/v1/counselling/treatment-plans/{planId}", chain(http.HandlerFunc(counselling.UpdateTreatmentPlan)))
	mux.Handle("POST /api/v1/counselling/treatment-plans/{planId}/goals", chain(http.HandlerFunc(counselling.AddGoal)))
	mux.Handle("POST /api/v1/counselling/treatment-plans/{planId}/relapses", chain(http.HandlerFunc(counselling.RecordRelapse)))

	// -- OST Prescribing routes --
	mux.Handle("GET /api/v1/ost/prescriptions", chain(http.HandlerFunc(prescribing.ListPrescriptions)))
	mux.Handle("POST /api/v1/ost/prescriptions", chain(http.HandlerFunc(prescribing.CreatePrescription)))
	mux.Handle("GET /api/v1/ost/prescriptions/{prescriptionId}", chain(http.HandlerFunc(prescribing.GetPrescription)))
	mux.Handle("PUT /api/v1/ost/prescriptions/{prescriptionId}", chain(http.HandlerFunc(prescribing.UpdatePrescription)))
	mux.Handle("POST /api/v1/ost/prescriptions/{prescriptionId}/adjust", chain(http.HandlerFunc(prescribing.AdjustDose)))
	mux.Handle("GET /api/v1/ost/prescriptions/{prescriptionId}/adjustments", chain(http.HandlerFunc(prescribing.ListAdjustments)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-addiction",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "reason": "database not reachable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-addiction"})
}

// RunMigrations runs database migrations for the tpt-addiction module.
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
