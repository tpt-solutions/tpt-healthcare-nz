// Package api implements the tpt-cardiology HTTP server.
// Covers ECG, echocardiography, ambulatory monitoring, cath lab bookings,
// cardiac rehabilitation, and implantable device management.
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
	carddb "github.com/PhillipC05/tpt-healthcare/modules/tpt-cardiology/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all runtime configuration for the cardiology server.
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

// Server is the tpt-cardiology HTTP server.
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

// NewServer creates and wires the cardiology server.
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

	// Outpatient clinic and follow-up appointments
	op := &outpatientHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/cardiology/appointments", chain(http.HandlerFunc(op.List)))
	mux.Handle("POST /api/v1/cardiology/appointments", chain(http.HandlerFunc(op.Create)))
	mux.Handle("GET /api/v1/cardiology/appointments/{id}", chain(http.HandlerFunc(op.Get)))
	mux.Handle("PUT /api/v1/cardiology/appointments/{id}", chain(http.HandlerFunc(op.Update)))
	mux.Handle("POST /api/v1/cardiology/appointments/{id}/complete", chain(http.HandlerFunc(op.Complete)))
	mux.Handle("POST /api/v1/cardiology/appointments/{id}/did-not-attend", chain(http.HandlerFunc(op.DidNotAttend)))

	// ECG ordering, performance, and reporting
	ecg := &ecgHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/cardiology/ecg", chain(http.HandlerFunc(ecg.List)))
	mux.Handle("POST /api/v1/cardiology/ecg", chain(http.HandlerFunc(ecg.Create)))
	mux.Handle("GET /api/v1/cardiology/ecg/{id}", chain(http.HandlerFunc(ecg.Get)))
	mux.Handle("PUT /api/v1/cardiology/ecg/{id}", chain(http.HandlerFunc(ecg.Update)))

	// Echocardiography requests and reports
	echo := &echoHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/cardiology/echo", chain(http.HandlerFunc(echo.List)))
	mux.Handle("POST /api/v1/cardiology/echo", chain(http.HandlerFunc(echo.Create)))
	mux.Handle("GET /api/v1/cardiology/echo/{id}", chain(http.HandlerFunc(echo.Get)))
	mux.Handle("PUT /api/v1/cardiology/echo/{id}", chain(http.HandlerFunc(echo.Update)))

	// Holter monitoring
	holter := &holterHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/cardiology/holter", chain(http.HandlerFunc(holter.List)))
	mux.Handle("POST /api/v1/cardiology/holter", chain(http.HandlerFunc(holter.Create)))
	mux.Handle("GET /api/v1/cardiology/holter/{id}", chain(http.HandlerFunc(holter.Get)))
	mux.Handle("PUT /api/v1/cardiology/holter/{id}", chain(http.HandlerFunc(holter.Update)))

	// Ambulatory blood pressure monitoring
	abpm := &abpmHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/cardiology/abpm", chain(http.HandlerFunc(abpm.List)))
	mux.Handle("POST /api/v1/cardiology/abpm", chain(http.HandlerFunc(abpm.Create)))
	mux.Handle("GET /api/v1/cardiology/abpm/{id}", chain(http.HandlerFunc(abpm.Get)))
	mux.Handle("PUT /api/v1/cardiology/abpm/{id}", chain(http.HandlerFunc(abpm.Update)))

	// Cath lab: booking, procedure documentation, and post-cath care
	cath := &cathHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/cardiology/cath-lab", chain(http.HandlerFunc(cath.List)))
	mux.Handle("POST /api/v1/cardiology/cath-lab", chain(http.HandlerFunc(cath.Create)))
	mux.Handle("GET /api/v1/cardiology/cath-lab/{id}", chain(http.HandlerFunc(cath.Get)))
	mux.Handle("PUT /api/v1/cardiology/cath-lab/{id}", chain(http.HandlerFunc(cath.Update)))
	mux.Handle("POST /api/v1/cardiology/cath-lab/{id}/start", chain(http.HandlerFunc(cath.Start)))
	mux.Handle("POST /api/v1/cardiology/cath-lab/{id}/complete", chain(http.HandlerFunc(cath.Complete)))
	mux.Handle("GET /api/v1/cardiology/cath-lab/{id}/post-care", chain(http.HandlerFunc(cath.GetPostCare)))
	mux.Handle("POST /api/v1/cardiology/cath-lab/{id}/post-care", chain(http.HandlerFunc(cath.AddPostCare)))

	// Cardiac rehabilitation programmes and sessions
	rehab := &rehabHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/cardiology/rehab", chain(http.HandlerFunc(rehab.ListProgrammes)))
	mux.Handle("POST /api/v1/cardiology/rehab", chain(http.HandlerFunc(rehab.CreateProgramme)))
	mux.Handle("GET /api/v1/cardiology/rehab/{id}", chain(http.HandlerFunc(rehab.GetProgramme)))
	mux.Handle("PUT /api/v1/cardiology/rehab/{id}", chain(http.HandlerFunc(rehab.UpdateProgramme)))
	mux.Handle("GET /api/v1/cardiology/rehab/{id}/sessions", chain(http.HandlerFunc(rehab.ListSessions)))
	mux.Handle("POST /api/v1/cardiology/rehab/{id}/sessions", chain(http.HandlerFunc(rehab.CreateSession)))

	// Implantable device registry and interrogations
	dev := &deviceHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/cardiology/devices", chain(http.HandlerFunc(dev.List)))
	mux.Handle("POST /api/v1/cardiology/devices", chain(http.HandlerFunc(dev.Create)))
	mux.Handle("GET /api/v1/cardiology/devices/{id}", chain(http.HandlerFunc(dev.Get)))
	mux.Handle("PUT /api/v1/cardiology/devices/{id}", chain(http.HandlerFunc(dev.Update)))
	mux.Handle("GET /api/v1/cardiology/devices/{id}/interrogations", chain(http.HandlerFunc(dev.ListInterrogations)))
	mux.Handle("POST /api/v1/cardiology/devices/{id}/interrogations", chain(http.HandlerFunc(dev.CreateInterrogation)))
	mux.Handle("GET /api/v1/cardiology/devices/{id}/interrogations/{interrogationId}", chain(http.HandlerFunc(dev.GetInterrogation)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-cardiology",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-cardiology"})
}

// RunMigrations runs the module's embedded SQL migrations.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := coredb.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(carddb.Migrations, pool)
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
