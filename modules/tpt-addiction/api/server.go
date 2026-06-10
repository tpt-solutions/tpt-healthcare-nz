// Package api implements the tpt-addiction HTTP server.
// All resources carry extra_sensitive = true per HIPC Rule 10/11.
// The addiction-clinician role is required for all routes via RequireRole middleware.
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
	"github.com/PhillipC05/tpt-healthcare/core/primhd"
	addictiondb "github.com/PhillipC05/tpt-healthcare/modules/tpt-addiction/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds runtime configuration for the tpt-addiction server.
type Config struct {
	Host          string
	Port          int
	DatabaseURL   string
	RedisURL      string
	EncryptionKey string
	Auth0Domain   string
	Auth0Audience string
	TenantHeader  string
	// PRIMHDBaseURL is the root URL for the PRIMHD outcomes reporting API.
	// Leave empty to disable PRIMHD reporting.
	PRIMHDBaseURL string
	// PRIMHDToken is the bearer token for PRIMHD API requests.
	PRIMHDToken string
	Logger      *slog.Logger
}

// Server is the tpt-addiction HTTP server.
type Server struct {
	cfg          Config
	mux          *http.ServeMux
	pool         *pgxpool.Pool
	enc          *encryption.Cipher
	auth         auth.Provider
	hpiClient    *hpi.Client
	primhdClient *primhd.Client
	auditTrail   *audit.Trail
	logger       *slog.Logger
}

// NewServer constructs and wires the addiction server.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
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
	var primhdClient *primhd.Client
	if cfg.PRIMHDBaseURL != "" {
		token := cfg.PRIMHDToken
		primhdClient = primhd.New(cfg.PRIMHDBaseURL, func(_ context.Context) (string, error) {
			return token, nil
		})
	}

	s := &Server{
		cfg:          cfg,
		pool:         pool,
		enc:          enc,
		auth:         authProvider,
		hpiClient:    hpi.NewClient(cfg.RedisURL, cfg.Logger),
		primhdClient: primhdClient,
		auditTrail:   audit.New(pool),
		logger:       cfg.Logger,
	}
	s.mux = s.buildRoutes()
	return s, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) buildRoutes() *http.ServeMux {
	deps := handlerDeps{
		pool:         s.pool,
		enc:          s.enc,
		hpiClient:    s.hpiClient,
		primhdClient: s.primhdClient,
		auditTrail:   s.auditTrail,
		logger:       s.logger,
	}

	// chain applies the standard middleware stack. All addiction routes require the
	// addiction-clinician role in addition to base auth, per extra_sensitive policy.
	chain := func(h http.Handler) http.Handler {
		h = middleware.AuditWrap(s.auditTrail)(h)
		h = auth.RequireRole("addiction-clinician")(h)
		h = auth.RequireAuth(s.auth)(h)
		h = middleware.TenantExtraction()(h)
		h = middleware.CORS([]string{"*"})(h)
		h = middleware.RateLimit(100, 200)(h)
		h = middleware.RecoveryMiddleware()(h)
		return h
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)

	mth := &methadoneHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/methadone/programmes", chain(http.HandlerFunc(mth.ListProgrammes)))
	mux.Handle("POST /api/v1/methadone/programmes", chain(http.HandlerFunc(mth.CreateProgramme)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}", chain(http.HandlerFunc(mth.GetProgramme)))
	mux.Handle("PUT /api/v1/methadone/programmes/{programmeId}", chain(http.HandlerFunc(mth.UpdateProgramme)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}/doses", chain(http.HandlerFunc(mth.ListDoses)))
	mux.Handle("POST /api/v1/methadone/programmes/{programmeId}/doses", chain(http.HandlerFunc(mth.RecordDose)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}/doses/{doseId}", chain(http.HandlerFunc(mth.GetDose)))
	mux.Handle("POST /api/v1/methadone/programmes/{programmeId}/take-home", chain(http.HandlerFunc(mth.ApproveTakeHome)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}/take-home", chain(http.HandlerFunc(mth.ListTakeHome)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}/urine-screens", chain(http.HandlerFunc(mth.ListUrineScreens)))
	mux.Handle("POST /api/v1/methadone/programmes/{programmeId}/urine-screens", chain(http.HandlerFunc(mth.RecordUrineScreen)))
	mux.Handle("GET /api/v1/methadone/programmes/{programmeId}/urine-screens/{screenId}", chain(http.HandlerFunc(mth.GetUrineScreen)))

	csl := &counsellingHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/counselling/sessions", chain(http.HandlerFunc(csl.ListSessions)))
	mux.Handle("POST /api/v1/counselling/sessions", chain(http.HandlerFunc(csl.CreateSession)))
	mux.Handle("GET /api/v1/counselling/sessions/{sessionId}", chain(http.HandlerFunc(csl.GetSession)))
	mux.Handle("PUT /api/v1/counselling/sessions/{sessionId}", chain(http.HandlerFunc(csl.UpdateSession)))
	mux.Handle("GET /api/v1/counselling/group-sessions", chain(http.HandlerFunc(csl.ListGroupSessions)))
	mux.Handle("POST /api/v1/counselling/group-sessions", chain(http.HandlerFunc(csl.CreateGroupSession)))
	mux.Handle("GET /api/v1/counselling/group-sessions/{groupId}", chain(http.HandlerFunc(csl.GetGroupSession)))
	mux.Handle("GET /api/v1/counselling/treatment-plans", chain(http.HandlerFunc(csl.ListTreatmentPlans)))
	mux.Handle("POST /api/v1/counselling/treatment-plans", chain(http.HandlerFunc(csl.CreateTreatmentPlan)))
	mux.Handle("GET /api/v1/counselling/treatment-plans/{planId}", chain(http.HandlerFunc(csl.GetTreatmentPlan)))
	mux.Handle("PUT /api/v1/counselling/treatment-plans/{planId}", chain(http.HandlerFunc(csl.UpdateTreatmentPlan)))
	mux.Handle("POST /api/v1/counselling/treatment-plans/{planId}/goals", chain(http.HandlerFunc(csl.AddGoal)))
	mux.Handle("POST /api/v1/counselling/treatment-plans/{planId}/relapses", chain(http.HandlerFunc(csl.RecordRelapse)))

	prs := &prescribingHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/ost/prescriptions", chain(http.HandlerFunc(prs.ListPrescriptions)))
	mux.Handle("POST /api/v1/ost/prescriptions", chain(http.HandlerFunc(prs.CreatePrescription)))
	mux.Handle("GET /api/v1/ost/prescriptions/{prescriptionId}", chain(http.HandlerFunc(prs.GetPrescription)))
	mux.Handle("PUT /api/v1/ost/prescriptions/{prescriptionId}", chain(http.HandlerFunc(prs.UpdatePrescription)))
	mux.Handle("POST /api/v1/ost/prescriptions/{prescriptionId}/adjust", chain(http.HandlerFunc(prs.AdjustDose)))
	mux.Handle("GET /api/v1/ost/prescriptions/{prescriptionId}/adjustments", chain(http.HandlerFunc(prs.ListAdjustments)))

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
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-addiction"})
}

// RunMigrations runs the module's embedded SQL migrations.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := coredb.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(addictiondb.Migrations, pool)
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

// handlerDeps is the shared dependency bundle injected into all domain handlers.
type handlerDeps struct {
	pool         *pgxpool.Pool
	enc          *encryption.Cipher
	hpiClient    *hpi.Client
	primhdClient *primhd.Client
	auditTrail   *audit.Trail
	logger       *slog.Logger
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
