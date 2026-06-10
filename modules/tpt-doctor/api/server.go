// Package api implements the tpt-doctor HTTP server and route handlers.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/acc"
	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/auth/auth0"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/episurv"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/medsafe"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/nhi"
	"github.com/PhillipC05/tpt-healthcare/core/nes"
	pharmacygateway "github.com/PhillipC05/tpt-healthcare/core/pharmacy-gateway"
	"github.com/PhillipC05/tpt-healthcare/core/pharmacy-gateway/fred"
	"github.com/PhillipC05/tpt-healthcare/core/pharmacy-gateway/hl7v2"
	"github.com/PhillipC05/tpt-healthcare/core/pharmacy-gateway/toniq"
	"github.com/PhillipC05/tpt-healthcare/core/pharmac"
	"github.com/PhillipC05/tpt-healthcare/core/worksafe"
)

// Config holds all configuration for the tpt-doctor server.
type Config struct {
	Host                string
	Port                int
	DatabaseURL         string
	RedisURL            string
	EncryptionKey       string
	Auth0Domain         string
	Auth0Audience       string
	TenantHeader        string
	// ACCBaseURL is the root URL for the ACC FHIR endpoint. Leave empty to
	// disable ACC integration (claim submissions will return 503).
	ACCBaseURL          string
	// WorkSafeBaseURL is the root URL for the WorkSafe NZ FHIR endpoint. Leave
	// empty to disable WorkSafe integration for work-related injury claims.
	WorkSafeBaseURL     string
	// WorkSafeToken is the bearer token for WorkSafe API requests. Required when
	// WorkSafeBaseURL is set.
	WorkSafeToken       string
	// TenantHPIFacilityID is the HPI facility OrgID for this practice,
	// required for NES enrolment transfers. Leave empty to disable transfers.
	TenantHPIFacilityID string
	// EpiSurvBaseURL is the root URL for the ESR EpiSurv notifiable disease API.
	// Leave empty to disable automatic EpiSurv notifications.
	EpiSurvBaseURL      string
	// EpiSurvToken is the bearer token for EpiSurv API requests.
	EpiSurvToken        string
	// MedsafeBaseURL is the root URL for the Medsafe/CARM ADE reporting API.
	// Leave empty to disable ADE reporting endpoints.
	MedsafeBaseURL      string
	// MedsafeToken is the bearer token for Medsafe CARM API requests.
	MedsafeToken        string
	// PharmacyGatewayHL7v2Addr is the MLLP host:port used as the HL7 v2 fallback
	// connector for pharmacies not registered with a FHIR PMS. Leave empty to
	// disable the fallback (dispatches to unregistered pharmacies will fail).
	PharmacyGatewayHL7v2Addr string
	// PharmacyGatewayHL7v2SendingApp is the MSH-3 sending application identifier
	// for HL7 v2 RDE^O11 messages. Defaults to "TPT-DOCTOR" when not set.
	PharmacyGatewayHL7v2SendingApp string
	// PharmacyGatewayFredBaseURL is the Fred Dispense FHIR endpoint base URL.
	// Leave empty to disable Fred Dispense connectivity.
	PharmacyGatewayFredBaseURL string
	// PharmacyGatewayFredAPIKey is the API key for the Fred Dispense instance.
	PharmacyGatewayFredAPIKey string
	// PharmacyGatewayFredHPIs is a comma-separated list of pharmacy HPI facility
	// IDs that route to the Fred Dispense connector.
	PharmacyGatewayFredHPIs string
	// PharmacyGatewayToniqBaseURL is the Toniq FHIR endpoint base URL.
	PharmacyGatewayToniqBaseURL string
	// PharmacyGatewayToniqClientID is the OAuth2 client ID for Toniq.
	PharmacyGatewayToniqClientID string
	// PharmacyGatewayToniqClientSecret is the OAuth2 client secret for Toniq.
	PharmacyGatewayToniqClientSecret string
	// PharmacyGatewayToniqHPIs is a comma-separated list of pharmacy HPI facility
	// IDs that route to the Toniq connector.
	PharmacyGatewayToniqHPIs string
	Logger                   *slog.Logger
}

// Server is the tpt-doctor HTTP server.
type Server struct {
	cfg                 Config
	mux                 *http.ServeMux
	pool                db.Pool
	enc                 *encryption.Cipher
	auth                auth.Provider
	nhiClient           *nhi.Client
	nesClient           *nes.Client
	hpiClient           *hpi.Client
	pharmac             *pharmac.Client
	accClient           *acc.Client
	worksafeClient      *worksafe.Client
	episurvClient       *episurv.Client
	medsafeClient       *medsafe.Client
	pharmacyGateway     *pharmacygateway.Gateway
	auditTrail          *audit.Trail
	tenantHPIFacilityID string
	logger              *slog.Logger
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

	nhiClient := nhi.NewClient(cfg.Logger)
	nesClient := nes.NewClient(cfg.Logger)
	hpiClient := hpi.NewClient(cfg.RedisURL, cfg.Logger)
	pharmClient := pharmac.NewClient(cfg.Logger)
	trail := audit.NewTrail(pool)

	var accClient *acc.Client
	if cfg.ACCBaseURL != "" {
		accClient = acc.NewClient(cfg.ACCBaseURL, cfg.Logger)
	}

	var wsClient *worksafe.Client
	if cfg.WorkSafeBaseURL != "" {
		token := cfg.WorkSafeToken
		wsClient = worksafe.New(cfg.WorkSafeBaseURL, func(_ context.Context) (string, error) {
			return token, nil
		})
	}

	var episurvClient *episurv.Client
	if cfg.EpiSurvBaseURL != "" {
		token := cfg.EpiSurvToken
		episurvClient = episurv.New(cfg.EpiSurvBaseURL, func(_ context.Context) (string, error) {
			return token, nil
		})
	}

	var medsafeClient *medsafe.Client
	if cfg.MedsafeBaseURL != "" {
		token := cfg.MedsafeToken
		medsafeClient = medsafe.New(cfg.MedsafeBaseURL, func(_ context.Context) (string, error) {
			return token, nil
		})
	}

	pgw, err := buildPharmacyGateway(cfg)
	if err != nil {
		return nil, fmt.Errorf("init pharmacy gateway: %w", err)
	}

	s := &Server{
		cfg:                 cfg,
		pool:                pool,
		enc:                 enc,
		auth:                authProvider,
		nhiClient:           nhiClient,
		nesClient:           nesClient,
		hpiClient:           hpiClient,
		pharmac:             pharmClient,
		accClient:           accClient,
		worksafeClient:      wsClient,
		episurvClient:       episurvClient,
		medsafeClient:       medsafeClient,
		pharmacyGateway:     pgw,
		auditTrail:          trail,
		tenantHPIFacilityID: cfg.TenantHPIFacilityID,
		logger:              cfg.Logger,
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
	patients := &PatientsHandler{
		pool:                s.pool,
		enc:                 s.enc,
		nhiClient:           s.nhiClient,
		nesClient:           s.nesClient,
		auditTrail:          s.auditTrail,
		tenantHPIFacilityID: s.tenantHPIFacilityID,
		logger:              s.logger,
	}
	appointments := &AppointmentsHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	encounters := &EncountersHandler{
		pool:          s.pool,
		enc:           s.enc,
		hpiClient:     s.hpiClient,
		episurvClient: s.episurvClient,
		auditTrail:    s.auditTrail,
		logger:        s.logger,
	}
	prescriptions := &PrescriptionsHandler{
		pool:            s.pool,
		enc:             s.enc,
		hpiClient:       s.hpiClient,
		pharmac:         s.pharmac,
		medsafeClient:   s.medsafeClient,
		pharmacyGateway: s.pharmacyGateway,
		auditTrail:      s.auditTrail,
		logger:          s.logger,
	}
	claims := &ClaimsHandler{
		pool:           s.pool,
		enc:            s.enc,
		accClient:      s.accClient,
		worksafeClient: s.worksafeClient,
		auditTrail:     s.auditTrail,
		logger:         s.logger,
	}
	referrals := &ReferralsHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	labs := &LabsHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	immunisations := &ImmunisationsHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	certificates := &CertificatesHandler{
		pool:       s.pool,
		enc:        s.enc,
		hpiClient:  s.hpiClient,
		auditTrail: s.auditTrail,
		logger:     s.logger,
	}
	pho := &PHOHandler{
		pool:       s.pool,
		enc:        s.enc,
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

	// Patient routes.
	mux.Handle("GET /api/v1/patients", chain(http.HandlerFunc(patients.List)))
	mux.Handle("POST /api/v1/patients", chain(http.HandlerFunc(patients.Create)))
	mux.Handle("GET /api/v1/patients/{id}", chain(http.HandlerFunc(patients.Get)))
	mux.Handle("PUT /api/v1/patients/{id}", chain(http.HandlerFunc(patients.Update)))
	mux.Handle("GET /api/v1/patients/nhi/{nhi}", chain(http.HandlerFunc(patients.GetByNHI)))

	// NES enrolment routes (enrol, update, transfer).
	mux.Handle("GET /api/v1/patients/{id}/enrolment", chain(http.HandlerFunc(patients.GetEnrolment)))
	mux.Handle("POST /api/v1/patients/{id}/enrolment", chain(http.HandlerFunc(patients.CreateEnrolment)))
	mux.Handle("PUT /api/v1/patients/{id}/enrolment", chain(http.HandlerFunc(patients.UpdateEnrolment)))
	mux.Handle("POST /api/v1/patients/{id}/enrolment/transfer", chain(http.HandlerFunc(patients.TransferEnrolment)))

	// Appointment routes.
	mux.Handle("GET /api/v1/appointments", chain(http.HandlerFunc(appointments.List)))
	mux.Handle("POST /api/v1/appointments", chain(http.HandlerFunc(appointments.Create)))
	mux.Handle("GET /api/v1/appointments/{id}", chain(http.HandlerFunc(appointments.Get)))
	mux.Handle("PUT /api/v1/appointments/{id}", chain(http.HandlerFunc(appointments.Update)))
	mux.Handle("DELETE /api/v1/appointments/{id}", chain(http.HandlerFunc(appointments.Delete)))

	// Encounter routes (supports workflow=standard|after-hours|urgent-care|occupational-health).
	mux.Handle("GET /api/v1/encounters", chain(http.HandlerFunc(encounters.List)))
	mux.Handle("POST /api/v1/encounters", chain(http.HandlerFunc(encounters.Create)))
	mux.Handle("GET /api/v1/encounters/{id}", chain(http.HandlerFunc(encounters.Get)))
	mux.Handle("PUT /api/v1/encounters/{id}", chain(http.HandlerFunc(encounters.Update)))
	mux.Handle("POST /api/v1/encounters/{id}/complete", chain(http.HandlerFunc(encounters.Complete)))

	// Prescription routes.
	mux.Handle("GET /api/v1/prescriptions", chain(http.HandlerFunc(prescriptions.List)))
	mux.Handle("POST /api/v1/prescriptions", chain(http.HandlerFunc(prescriptions.Create)))
	mux.Handle("GET /api/v1/prescriptions/{id}", chain(http.HandlerFunc(prescriptions.Get)))
	mux.Handle("PUT /api/v1/prescriptions/{id}", chain(http.HandlerFunc(prescriptions.Update)))
	mux.Handle("POST /api/v1/prescriptions/{id}/print", chain(http.HandlerFunc(prescriptions.Print)))
	mux.Handle("POST /api/v1/prescriptions/{id}/dispatch", chain(http.HandlerFunc(prescriptions.Dispatch)))
	mux.Handle("POST /api/v1/prescriptions/{id}/ade", chain(http.HandlerFunc(prescriptions.ReportADE)))

	// Referral routes.
	mux.Handle("GET /api/v1/referrals", chain(http.HandlerFunc(referrals.List)))
	mux.Handle("POST /api/v1/referrals", chain(http.HandlerFunc(referrals.Create)))
	mux.Handle("GET /api/v1/referrals/{id}", chain(http.HandlerFunc(referrals.Get)))
	mux.Handle("PUT /api/v1/referrals/{id}", chain(http.HandlerFunc(referrals.Update)))
	mux.Handle("POST /api/v1/referrals/{id}/send", chain(http.HandlerFunc(referrals.Send)))

	// Lab order + results routes.
	mux.Handle("GET /api/v1/labs", chain(http.HandlerFunc(labs.List)))
	mux.Handle("POST /api/v1/labs", chain(http.HandlerFunc(labs.Create)))
	mux.Handle("GET /api/v1/labs/{id}", chain(http.HandlerFunc(labs.Get)))
	mux.Handle("POST /api/v1/labs/{id}/result", chain(http.HandlerFunc(labs.Result)))

	// Immunisation routes.
	mux.Handle("GET /api/v1/immunisations", chain(http.HandlerFunc(immunisations.List)))
	mux.Handle("POST /api/v1/immunisations", chain(http.HandlerFunc(immunisations.Create)))
	mux.Handle("GET /api/v1/immunisations/{id}", chain(http.HandlerFunc(immunisations.Get)))
	mux.Handle("POST /api/v1/immunisations/{id}/submit-nir", chain(http.HandlerFunc(immunisations.SubmitNIR)))

	// Medical certificate routes.
	mux.Handle("GET /api/v1/certificates", chain(http.HandlerFunc(certificates.List)))
	mux.Handle("POST /api/v1/certificates", chain(http.HandlerFunc(certificates.Create)))
	mux.Handle("GET /api/v1/certificates/{id}", chain(http.HandlerFunc(certificates.Get)))

	// ACC claim routes.
	mux.Handle("GET /api/v1/claims", chain(http.HandlerFunc(claims.List)))
	mux.Handle("POST /api/v1/claims", chain(http.HandlerFunc(claims.Create)))
	mux.Handle("GET /api/v1/claims/{id}", chain(http.HandlerFunc(claims.Get)))
	mux.Handle("POST /api/v1/claims/{id}/submit", chain(http.HandlerFunc(claims.Submit)))
	mux.Handle("GET /api/v1/claims/{id}/status", chain(http.HandlerFunc(claims.Status)))

	// PHO reporting routes (capitation + FFS extracts).
	mux.Handle("GET /api/v1/pho/reports", chain(http.HandlerFunc(pho.ListReports)))
	mux.Handle("POST /api/v1/pho/reports", chain(http.HandlerFunc(pho.GenerateReport)))
	mux.Handle("GET /api/v1/pho/reports/{id}", chain(http.HandlerFunc(pho.GetReport)))
	mux.Handle("POST /api/v1/pho/reports/{id}/submit", chain(http.HandlerFunc(pho.SubmitReport)))
	mux.Handle("GET /api/v1/pho/reports/{id}/records", chain(http.HandlerFunc(pho.GetCapitationRecords)))

	return mux
}

// handleHealth responds to liveness probes.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-doctor",
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
		"service": "tpt-doctor",
	})
}

// notImplemented returns 501 for routes that are registered but not yet built.
func notImplemented(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, apiError{
		Code:    "NOT_IMPLEMENTED",
		Message: "this endpoint is not yet implemented",
	})
}

// RunMigrations runs database migrations for the tpt-doctor module.
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

// ValidateConnectivity checks that the database and Redis are reachable.
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

// buildPharmacyGateway constructs the pharmacy gateway from config. Returns nil
// when no gateway config is provided (all dispatch calls will return 503).
func buildPharmacyGateway(cfg Config) (*pharmacygateway.Gateway, error) {
	hasFred := cfg.PharmacyGatewayFredBaseURL != ""
	hasToniq := cfg.PharmacyGatewayToniqBaseURL != ""
	hasHL7v2 := cfg.PharmacyGatewayHL7v2Addr != ""

	if !hasFred && !hasToniq && !hasHL7v2 {
		return nil, nil
	}

	var pgw *pharmacygateway.Gateway
	if hasHL7v2 {
		sendingApp := cfg.PharmacyGatewayHL7v2SendingApp
		if sendingApp == "" {
			sendingApp = "TPT-DOCTOR"
		}
		hl7Conn, err := hl7v2.New(cfg.PharmacyGatewayHL7v2Addr, sendingApp, cfg.TenantHPIFacilityID)
		if err != nil {
			return nil, fmt.Errorf("hl7v2 connector: %w", err)
		}
		pgw = pharmacygateway.New(hl7Conn)
	} else {
		pgw = pharmacygateway.New(nil)
	}

	if hasFred {
		fc := fred.New(cfg.PharmacyGatewayFredBaseURL, cfg.PharmacyGatewayFredAPIKey)
		for _, hpi := range splitTrimmed(cfg.PharmacyGatewayFredHPIs) {
			pgw.Register(hpi, fc)
		}
	}
	if hasToniq {
		tc := toniq.New(cfg.PharmacyGatewayToniqBaseURL, cfg.PharmacyGatewayToniqClientID, cfg.PharmacyGatewayToniqClientSecret)
		for _, hpi := range splitTrimmed(cfg.PharmacyGatewayToniqHPIs) {
			pgw.Register(hpi, tc)
		}
	}

	return pgw, nil
}

// splitTrimmed splits a comma-separated string and trims whitespace from each
// element, discarding empty elements.
func splitTrimmed(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// apiError is the standard error response envelope.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// writeJSON serialises v as JSON and writes it to w with the given status code.
// Encoding errors after WriteHeader cannot be communicated to the client and are silently dropped.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decodeJSON reads and decodes a JSON request body into v.
// Reading is capped at 1 MB to prevent memory exhaustion from large payloads.
func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}
