// Package api implements the HTTP server for tpt-community-health.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/auth/auth0"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-community-health/internal/repository"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds server configuration.
type Config struct {
	Addr           string
	ReadTimeout    int
	WriteTimeout   int
	IdleTimeout    int
	AllowedOrigins []string
	Logger         *slog.Logger
	DatabaseURL    string
	Auth0Domain    string
	Auth0Audience  string
}

// DefaultConfig returns default server configuration.
func DefaultConfig() Config {
	return Config{
		Addr:           ":8092",
		ReadTimeout:    15,
		WriteTimeout:   15,
		IdleTimeout:    60,
		AllowedOrigins: []string{"*"},
		Logger:         slog.Default(),
	}
}

// Server holds the HTTP server and dependencies.
type Server struct {
	router       *mux.Router
	auth         auth.Provider
	config       Config
	pool         *pgxpool.Pool
	auditTrail   *audit.Trail
	logger       *slog.Logger
	visitRepo    *repository.VisitRepository
	planRepo     *repository.CarePlanRepository
	outreachRepo *repository.OutreachRepository
}

// NewServer creates a new HTTP server.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Addr == "" {
		cfg = DefaultConfig()
	}
	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = []string{"*"}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	authProvider, err := auth0.NewProvider(cfg.Auth0Domain, cfg.Auth0Audience)
	if err != nil {
		return nil, fmt.Errorf("init auth0 provider: %w", err)
	}

	trail := audit.NewTrail(pool)

	s := &Server{
		auth:         authProvider,
		config:       cfg,
		pool:         pool,
		auditTrail:   trail,
		logger:       cfg.Logger,
		visitRepo:    repository.NewVisitRepository(pool),
		planRepo:     repository.NewCarePlanRepository(pool),
		outreachRepo: repository.NewOutreachRepository(pool),
	}

	s.buildRoutes()
	return s, nil
}

// Handler returns the root HTTP handler with middleware applied.
func (s *Server) Handler() http.Handler {
	var h http.Handler = s.router
	if s.auditTrail != nil {
		h = middleware.AuditWrap(s.auditTrail)(h)
	}
	h = middleware.CORS(s.config.AllowedOrigins)(h)
	h = middleware.RateLimit(100, 200)(h)
	h = middleware.RecoveryMiddleware()(h)
	return h
}

// buildRoutes registers all routes.
func (s *Server) buildRoutes() {
	r := mux.NewRouter()

	// Health and readiness probes — no auth required.
	r.HandleFunc("/health", s.healthCheck).Methods("GET")
	r.HandleFunc("/ready", s.readinessCheck).Methods("GET")

	// Protected route builder.
	p := func(h http.HandlerFunc) http.Handler {
		return auth.RequireAuth(s.auth)(middleware.TenantExtraction()(h))
	}

	// Home visit routes
	r.HandleFunc("/api/v1/community/home-visits", p(s.createHomeVisit)).Methods("POST")
	r.HandleFunc("/api/v1/community/home-visits", s.listHomeVisits).Methods("GET")
	r.HandleFunc("/api/v1/community/home-visits/{id}", s.getHomeVisit).Methods("GET")
	r.HandleFunc("/api/v1/community/home-visits/{id}", p(s.updateHomeVisit)).Methods("PUT")
	r.HandleFunc("/api/v1/community/home-visits/{id}", p(s.deleteHomeVisit)).Methods("DELETE")
	r.HandleFunc("/api/v1/community/home-visits/{id}/notes", p(s.createVisitNote)).Methods("POST")
	r.HandleFunc("/api/v1/community/home-visits/{id}/notes", s.listVisitNotes).Methods("GET")
	r.HandleFunc("/api/v1/community/home-visits/{id}/safety-checks", p(s.createSafetyCheck)).Methods("POST")
	r.HandleFunc("/api/v1/community/home-visits/{id}/safety-checks", s.listSafetyChecks).Methods("GET")
	r.HandleFunc("/api/v1/community/home-visits/{id}/equipment-checks", p(s.createEquipmentCheck)).Methods("POST")
	r.HandleFunc("/api/v1/community/home-visits/{id}/equipment-checks", s.listEquipmentChecks).Methods("GET")

	// District nursing routes
	r.HandleFunc("/api/v1/community/district-nursing/care-plans", p(s.createCarePlan)).Methods("POST")
	r.HandleFunc("/api/v1/community/district-nursing/care-plans", s.listCarePlans).Methods("GET")
	r.HandleFunc("/api/v1/community/district-nursing/care-plans/{id}", s.getCarePlan).Methods("GET")
	r.HandleFunc("/api/v1/community/district-nursing/care-plans/{id}", p(s.updateCarePlan)).Methods("PUT")
	r.HandleFunc("/api/v1/community/district-nursing/care-plans/{id}", p(s.deleteCarePlan)).Methods("DELETE")
	r.HandleFunc("/api/v1/community/district-nursing/care-plans/{id}/visits", p(s.createNursingVisit)).Methods("POST")
	r.HandleFunc("/api/v1/community/district-nursing/visits", s.listNursingVisits).Methods("GET")

	// Outreach routes
	r.HandleFunc("/api/v1/community/outreach/programs", p(s.createProgram)).Methods("POST")
	r.HandleFunc("/api/v1/community/outreach/programs", s.listPrograms).Methods("GET")
	r.HandleFunc("/api/v1/community/outreach/programs/{id}", s.getProgram).Methods("GET")
	r.HandleFunc("/api/v1/community/outreach/programs/{id}", p(s.updateProgram)).Methods("PUT")
	r.HandleFunc("/api/v1/community/outreach/programs/{id}", p(s.deleteProgram)).Methods("DELETE")
	r.HandleFunc("/api/v1/community/outreach/programs/{id}/events", s.listEvents).Methods("GET")
	r.HandleFunc("/api/v1/community/outreach/programs/{id}/events", p(s.createEvent)).Methods("POST")
	r.HandleFunc("/api/v1/community/outreach/events/{eventId}", s.getEvent).Methods("GET")
	r.HandleFunc("/api/v1/community/outreach/events/{eventId}", p(s.updateEvent)).Methods("PUT")
	r.HandleFunc("/api/v1/community/outreach/events/{eventId}/attendees", p(s.createAttendee)).Methods("POST")
	r.HandleFunc("/api/v1/community/outreach/events/{eventId}/attendees", s.listAttendees).Methods("GET")
	r.HandleFunc("/api/v1/community/outreach/events/{eventId}/referrals", p(s.createReferral)).Methods("POST")
	r.HandleFunc("/api/v1/community/outreach/events/{eventId}/referrals", s.listReferrals).Methods("GET")
	r.HandleFunc("/api/v1/community/outreach/events/{eventId}/screenings", p(s.createScreening)).Methods("POST")
	r.HandleFunc("/api/v1/community/outreach/events/{eventId}/screenings", s.listScreenings).Methods("GET")

	s.router = r
}

// healthCheck handles GET /health.
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-community-health",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// readinessCheck handles GET /ready.
func (s *Server) readinessCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.pool.Ping(ctx); err != nil {
		s.logger.Error("readiness check failed", slog.Any("error", err))
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
			"reason": "database not reachable",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ready",
		"service": "tpt-community-health",
	})
}

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

// parsePagination reads limit and offset from query params with defaults.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		if l > 200 {
			l = 200
		}
		limit = l
	}
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
		offset = o
	}
	return
}

// RunMigrations runs database migrations for the tpt-community-health module.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, "modules/tpt-community-health/db/migrate"); err != nil {
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

	cfg.Logger.Info("database connectivity OK")
	cfg.Logger.Info("connectivity validation complete")
	return nil
}
