// Package api implements the tpt-maternal-child-health HTTP server.
// Covers the full maternal and child health continuum: LMC and maternity care,
// intrapartum (birthing), postnatal, NICU/SCBU, paediatric inpatient care (PICU),
// Well Child Tamariki Ora, birth notification (NBRS), and MMPO claiming.
// Can be deployed to serve hospital maternity/paediatric units, birth centres,
// or independent LMC midwife practices.
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
	mchdb "github.com/PhillipC05/tpt-healthcare/modules/tpt-maternal-child-health/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds all runtime configuration for the maternal-child-health server.
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

// Server is the tpt-maternal-child-health HTTP server.
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

// NewServer creates and wires the maternal-child-health server including DB, auth, and encryption.
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

	// Maternity episodes of care
	ep := &episodeHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/maternity/episodes", chain(http.HandlerFunc(ep.List)))
	mux.Handle("POST /api/v1/maternity/episodes", chain(http.HandlerFunc(ep.Create)))
	mux.Handle("GET /api/v1/maternity/episodes/{id}", chain(http.HandlerFunc(ep.Get)))
	mux.Handle("PUT /api/v1/maternity/episodes/{id}", chain(http.HandlerFunc(ep.Update)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/close", chain(http.HandlerFunc(ep.Close)))

	// LMC registration and case-loading
	lmc := &lmcHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/maternity/lmc", chain(http.HandlerFunc(lmc.List)))
	mux.Handle("POST /api/v1/maternity/lmc", chain(http.HandlerFunc(lmc.Register)))
	mux.Handle("GET /api/v1/maternity/lmc/{id}", chain(http.HandlerFunc(lmc.Get)))
	mux.Handle("PUT /api/v1/maternity/lmc/{id}", chain(http.HandlerFunc(lmc.Update)))
	mux.Handle("POST /api/v1/maternity/lmc/{id}/handover", chain(http.HandlerFunc(lmc.Handover)))

	// Antenatal care: visits, scans, screening
	ant := &antenatalHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/maternity/episodes/{id}/antenatal/visits", chain(http.HandlerFunc(ant.ListVisits)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/antenatal/visits", chain(http.HandlerFunc(ant.CreateVisit)))
	mux.Handle("GET /api/v1/maternity/episodes/{id}/antenatal/visits/{visitId}", chain(http.HandlerFunc(ant.GetVisit)))
	mux.Handle("GET /api/v1/maternity/episodes/{id}/antenatal/scans", chain(http.HandlerFunc(ant.ListScans)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/antenatal/scans", chain(http.HandlerFunc(ant.CreateScan)))
	mux.Handle("GET /api/v1/maternity/episodes/{id}/antenatal/screening", chain(http.HandlerFunc(ant.ListScreening)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/antenatal/screening", chain(http.HandlerFunc(ant.CreateScreening)))

	// Intrapartum (birth): episode, partogram, CTG
	ip := &intrapartumHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/maternity/episodes/{id}/intrapartum", chain(http.HandlerFunc(ip.List)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/intrapartum", chain(http.HandlerFunc(ip.Start)))
	mux.Handle("GET /api/v1/maternity/episodes/{id}/intrapartum/{birthId}", chain(http.HandlerFunc(ip.Get)))
	mux.Handle("PUT /api/v1/maternity/episodes/{id}/intrapartum/{birthId}", chain(http.HandlerFunc(ip.Update)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/intrapartum/{birthId}/complete", chain(http.HandlerFunc(ip.Complete)))
	mux.Handle("GET /api/v1/maternity/episodes/{id}/intrapartum/{birthId}/partogram", chain(http.HandlerFunc(ip.ListPartogram)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/intrapartum/{birthId}/partogram", chain(http.HandlerFunc(ip.AddPartogramEntry)))
	mux.Handle("GET /api/v1/maternity/episodes/{id}/intrapartum/{birthId}/ctg", chain(http.HandlerFunc(ip.ListCTG)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/intrapartum/{birthId}/ctg", chain(http.HandlerFunc(ip.AddCTG)))

	// Postnatal care: checks and community midwife visits
	pn := &postnatalHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/maternity/episodes/{id}/postnatal/checks", chain(http.HandlerFunc(pn.ListChecks)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/postnatal/checks", chain(http.HandlerFunc(pn.CreateCheck)))
	mux.Handle("GET /api/v1/maternity/episodes/{id}/postnatal/community-visits", chain(http.HandlerFunc(pn.ListCommunityVisits)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/postnatal/community-visits", chain(http.HandlerFunc(pn.CreateCommunityVisit)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/postnatal/discharge", chain(http.HandlerFunc(pn.Discharge)))

	// NBRS birth notification
	nbrs := &nbrsHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/maternity/episodes/{id}/nbrs", chain(http.HandlerFunc(nbrs.Get)))
	mux.Handle("POST /api/v1/maternity/episodes/{id}/nbrs", chain(http.HandlerFunc(nbrs.Submit)))

	// Neonatal ICU (NICU): neonates born at this facility, typically <32 weeks or acutely unwell.
	// PICU (children >28 days) is under /api/v1/paediatrics/picu below.
	nicu := &nicuHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/maternity/nicu", chain(http.HandlerFunc(nicu.List)))
	mux.Handle("POST /api/v1/maternity/nicu", chain(http.HandlerFunc(nicu.Admit)))
	mux.Handle("GET /api/v1/maternity/nicu/{id}", chain(http.HandlerFunc(nicu.Get)))
	mux.Handle("PUT /api/v1/maternity/nicu/{id}", chain(http.HandlerFunc(nicu.Update)))
	mux.Handle("POST /api/v1/maternity/nicu/{id}/discharge", chain(http.HandlerFunc(nicu.Discharge)))
	mux.Handle("GET /api/v1/maternity/nicu/{id}/ventilation", chain(http.HandlerFunc(nicu.ListVentilation)))
	mux.Handle("POST /api/v1/maternity/nicu/{id}/ventilation", chain(http.HandlerFunc(nicu.RecordVentilation)))
	mux.Handle("GET /api/v1/maternity/nicu/{id}/discharge-plan", chain(http.HandlerFunc(nicu.GetDischargePlan)))
	mux.Handle("PUT /api/v1/maternity/nicu/{id}/discharge-plan", chain(http.HandlerFunc(nicu.UpdateDischargePlan)))

	// SCBU (Special Care Baby Unit): neonates ~32–36 weeks, intermediate care
	scbu := &scbuHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/maternity/scbu", chain(http.HandlerFunc(scbu.List)))
	mux.Handle("POST /api/v1/maternity/scbu", chain(http.HandlerFunc(scbu.Create)))
	mux.Handle("GET /api/v1/maternity/scbu/{id}", chain(http.HandlerFunc(scbu.Get)))
	mux.Handle("PUT /api/v1/maternity/scbu/{id}", chain(http.HandlerFunc(scbu.Update)))
	mux.Handle("POST /api/v1/maternity/scbu/{id}/discharge", chain(http.HandlerFunc(scbu.Discharge)))
	mux.Handle("POST /api/v1/maternity/scbu/{id}/transfer-nicu", chain(http.HandlerFunc(scbu.TransferNICU)))
	mux.Handle("GET /api/v1/maternity/scbu/{id}/chart", chain(http.HandlerFunc(scbu.ListChart)))
	mux.Handle("POST /api/v1/maternity/scbu/{id}/chart", chain(http.HandlerFunc(scbu.AddChartEntry)))

	// MMPO claiming (LMC Schedule of Payments)
	mmpo := &mmpoHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/maternity/mmpo/claims", chain(http.HandlerFunc(mmpo.List)))
	mux.Handle("POST /api/v1/maternity/mmpo/claims", chain(http.HandlerFunc(mmpo.Create)))
	mux.Handle("GET /api/v1/maternity/mmpo/claims/{id}", chain(http.HandlerFunc(mmpo.Get)))
	mux.Handle("PUT /api/v1/maternity/mmpo/claims/{id}", chain(http.HandlerFunc(mmpo.Update)))
	mux.Handle("POST /api/v1/maternity/mmpo/claims/{id}/submit", chain(http.HandlerFunc(mmpo.Submit)))
	mux.Handle("POST /api/v1/maternity/mmpo/claims/{id}/withdraw", chain(http.HandlerFunc(mmpo.Withdraw)))

	// Paediatric inpatient admissions, PICU, growth, milestones, child protection
	paeds := &paediatricHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/paediatrics/admissions", chain(http.HandlerFunc(paeds.List)))
	mux.Handle("POST /api/v1/paediatrics/admissions", chain(http.HandlerFunc(paeds.Admit)))
	mux.Handle("GET /api/v1/paediatrics/admissions/{id}", chain(http.HandlerFunc(paeds.Get)))
	mux.Handle("PUT /api/v1/paediatrics/admissions/{id}", chain(http.HandlerFunc(paeds.Update)))
	mux.Handle("POST /api/v1/paediatrics/admissions/{id}/discharge", chain(http.HandlerFunc(paeds.Discharge)))
	mux.Handle("GET /api/v1/paediatrics/admissions/{id}/growth", chain(http.HandlerFunc(paeds.ListGrowth)))
	mux.Handle("POST /api/v1/paediatrics/admissions/{id}/growth", chain(http.HandlerFunc(paeds.RecordGrowth)))
	mux.Handle("GET /api/v1/paediatrics/admissions/{id}/milestones", chain(http.HandlerFunc(paeds.ListMilestones)))
	mux.Handle("POST /api/v1/paediatrics/admissions/{id}/milestones", chain(http.HandlerFunc(paeds.RecordMilestone)))
	mux.Handle("GET /api/v1/paediatrics/admissions/{id}/child-protection", chain(http.HandlerFunc(paeds.GetChildProtection)))
	mux.Handle("PUT /api/v1/paediatrics/admissions/{id}/child-protection", chain(http.HandlerFunc(paeds.UpdateChildProtection)))
	mux.Handle("GET /api/v1/paediatrics/picu", chain(http.HandlerFunc(paeds.ListPICU)))
	mux.Handle("POST /api/v1/paediatrics/picu", chain(http.HandlerFunc(paeds.AdmitPICU)))
	mux.Handle("GET /api/v1/paediatrics/picu/{id}", chain(http.HandlerFunc(paeds.GetPICU)))
	mux.Handle("PUT /api/v1/paediatrics/picu/{id}", chain(http.HandlerFunc(paeds.UpdatePICU)))
	mux.Handle("POST /api/v1/paediatrics/picu/{id}/discharge", chain(http.HandlerFunc(paeds.DischargePICU)))

	// Well Child Tamariki Ora (Plunket schedule checks, growth, B4 School Check)
	wc := &wellChildHandler{handlerDeps: deps}
	mux.Handle("GET /api/v1/well-child", chain(http.HandlerFunc(wc.List)))
	mux.Handle("POST /api/v1/well-child", chain(http.HandlerFunc(wc.Create)))
	mux.Handle("GET /api/v1/well-child/{id}", chain(http.HandlerFunc(wc.Get)))
	mux.Handle("PUT /api/v1/well-child/{id}", chain(http.HandlerFunc(wc.Update)))
	mux.Handle("GET /api/v1/well-child/{id}/growth", chain(http.HandlerFunc(wc.ListGrowthPoints)))
	mux.Handle("POST /api/v1/well-child/{id}/growth", chain(http.HandlerFunc(wc.RecordGrowthPoint)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-maternal-child-health",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "tpt-maternal-child-health"})
}

// RunMigrations runs the module's embedded SQL migrations.
func RunMigrations(ctx context.Context, databaseURL string, logger *slog.Logger) error {
	pool, err := coredb.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()
	r := migrate.New(mchdb.Migrations, pool)
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
