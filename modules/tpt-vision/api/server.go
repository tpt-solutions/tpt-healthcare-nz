// Package api implements the tpt-vision HTTP server and route handlers for
// optometry/ophthalmology, prescription management, and optical dispensing.
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
)

// Config holds all configuration for the tpt-vision server.
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

// Server is the tpt-vision HTTP server.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	pool       db.Pool
	enc        *encryption.Cipher
	auth       auth.Provider
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

	trail := audit.NewTrail(pool)

	s := &Server{
		cfg:        cfg,
		pool:       pool,
		enc:        enc,
		auth:       authProvider,
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
	refract := &RefractionHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	ophth := &OphthalmicHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	optical := &OpticalHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	acc := &ACCHandler{
		pool:       s.pool,
		enc:        s.enc,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}

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

	// Health and readiness probes — no auth required.
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// -- Refraction / Prescription routes --
	mux.Handle("GET /api/v1/prescriptions/{patientNhi}", chain(http.HandlerFunc(refract.ListPrescriptions)))
	mux.Handle("POST /api/v1/prescriptions", chain(http.HandlerFunc(refract.CreatePrescription)))
	mux.Handle("GET /api/v1/prescriptions/{patientNhi}/{prescriptionId}", chain(http.HandlerFunc(refract.GetPrescription)))
	mux.Handle("PUT /api/v1/prescriptions/{patientNhi}/{prescriptionId}", chain(http.HandlerFunc(refract.UpdatePrescription)))
	mux.Handle("GET /api/v1/prescriptions/{patientNhi}/current", chain(http.HandlerFunc(refract.CurrentPrescription)))
	mux.Handle("POST /api/v1/prescriptions/convert-snellen", chain(http.HandlerFunc(refract.ConvertSnellen)))
	// FHIR endpoints
	mux.Handle("GET /api/v1/prescriptions/{patientNhi}/{prescriptionId}/fhir", chain(http.HandlerFunc(refract.GetPrescriptionFHIR)))

	// -- Ophthalmic Examination routes --
	mux.Handle("GET /api/v1/exams/{patientNhi}", chain(http.HandlerFunc(ophth.ListExams)))
	mux.Handle("POST /api/v1/exams", chain(http.HandlerFunc(ophth.CreateExam)))
	mux.Handle("GET /api/v1/exams/{patientNhi}/{examId}", chain(http.HandlerFunc(ophth.GetExam)))
	mux.Handle("PUT /api/v1/exams/{patientNhi}/{examId}", chain(http.HandlerFunc(ophth.UpdateExam)))
	mux.Handle("POST /api/v1/exams/{patientNhi}/{examId}/add-iop", chain(http.HandlerFunc(ophth.AddIOPReading)))
	// FHIR endpoints
	mux.Handle("GET /api/v1/exams/{patientNhi}/{examId}/fhir", chain(http.HandlerFunc(ophth.GetExamFHIR)))

	// -- Optical Dispensing routes --
	mux.Handle("GET /api/v1/dispensing/{patientNhi}", chain(http.HandlerFunc(optical.ListOrders)))
	mux.Handle("POST /api/v1/dispensing", chain(http.HandlerFunc(optical.CreateOrder)))
	mux.Handle("GET /api/v1/dispensing/{patientNhi}/{orderId}", chain(http.HandlerFunc(optical.GetOrder)))
	mux.Handle("PUT /api/v1/dispensing/{patientNhi}/{orderId}", chain(http.HandlerFunc(optical.UpdateOrder)))
	mux.Handle("POST /api/v1/dispensing/{patientNhi}/{orderId}/status", chain(http.HandlerFunc(optical.UpdateStatus)))
	// FHIR endpoints
	mux.Handle("GET /api/v1/dispensing/{patientNhi}/{orderId}/fhir", chain(http.HandlerFunc(optical.GetOrderFHIR)))

	// -- ACC Vision Claim routes --
	mux.Handle("GET /api/v1/acc/claims", chain(http.HandlerFunc(acc.ListClaims)))
	mux.Handle("POST /api/v1/acc/claims", chain(http.HandlerFunc(acc.CreateClaim)))
	mux.Handle("GET /api/v1/acc/claims/{claimId}", chain(http.HandlerFunc(acc.GetClaim)))
	mux.Handle("PUT /api/v1/acc/claims/{claimId}", chain(http.HandlerFunc(acc.UpdateClaim)))
	mux.Handle("POST /api/v1/acc/claims/{claimId}/submit", chain(http.HandlerFunc(acc.SubmitClaim)))
	mux.Handle("GET /api/v1/acc/claims/{claimId}/status", chain(http.HandlerFunc(acc.CheckStatus)))
	mux.Handle("POST /api/v1/acc/claims/validate", chain(http.HandlerFunc(acc.ValidateClaim)))
	mux.Handle("GET /api/v1/acc/procedure-codes", chain(http.HandlerFunc(acc.ProcedureCodes)))
	// FHIR endpoints
	mux.Handle("GET /api/v1/acc/claims/{claimId}/fhir", chain(http.HandlerFunc(acc.GetClaimFHIR)))

	return mux
}

// handleHealth responds to liveness probes.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-vision",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// handleReady responds to readiness probes, checking database connectivity.
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
		"service": "tpt-vision",
	})
}

// RunMigrations runs database migrations for the tpt-vision module.
func RunMigrations(ctx context.Context, databaseURL string) error {
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, "modules/tpt-vision/db/migrate"); err != nil {
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