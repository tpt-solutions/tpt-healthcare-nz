// Package api implements the tpt-hospital HTTP server and route handlers.
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
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/events"
	"github.com/PhillipC05/tpt-healthcare/core/hl7"
	"github.com/PhillipC05/tpt-healthcare/core/hpi"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// Config holds all configuration for the tpt-hospital server.
type Config struct {
	Host          string
	Port          int
	DatabaseURL   string
	RedisURL      string
	EncryptionKey string
	Auth0Domain   string
	Auth0Audience string
	TenantHeader  string
	// HL7MLLPAddr is the address (host:port) of the downstream HL7 v2 MLLP
	// listener (lab/radiology/PAS feed) that receives ORM and ADT messages.
	// Leave empty to disable HL7 dispatch.
	HL7MLLPAddr string
	Logger      *slog.Logger
}

// Server is the tpt-hospital HTTP server.
type Server struct {
	cfg        Config
	mux        *http.ServeMux
	pool       db.Pool
	enc        *encryption.Cipher
	auth       auth.Provider
	hpiClient  *hpi.Client
	auditTrail *audit.Trail
	eventBus   *events.Bus
	hl7Client  *hl7.MLLPClient
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

	hpiClient := hpi.NewClient(cfg.RedisURL, cfg.Logger)
	trail := audit.NewTrail(pool)

	var hl7Client *hl7.MLLPClient
	if cfg.HL7MLLPAddr != "" {
		hl7Client, err = hl7.NewMLLPClient(cfg.HL7MLLPAddr, 10*time.Second)
		if err != nil {
			return nil, fmt.Errorf("init HL7 MLLP client: %w", err)
		}
	}

	s := &Server{
		cfg:        cfg,
		pool:       pool,
		enc:        enc,
		auth:       authProvider,
		hpiClient:  hpiClient,
		auditTrail: trail,
		eventBus:   events.New(),
		hl7Client:  hl7Client,
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
//
// Hospital-only departments served by this module:
//   - Core inpatient: admissions, wards/beds, discharge summaries
//   - Emergency department: ED triage, queue management
//   - Critical care: ICU, ventilation charting
//   - Surgical: theatre scheduling, pre-admission assessment
//   - Clinical coding + billing: ICD-10-AM/ACHI, casemix/DRG
//   - Inpatient pharmacy: medication charts, IV pharmacy, reconciliation
//   - Infection control: HAI surveillance, isolation precautions
//   - Outpatient: hospital-based specialist clinics and waitlists
//   - Hospital in the Home (HITH): virtual ward episodes and nursing visits
//   - Emergency & disaster management: CIMS incident command, MCI triage,
//     surge capacity management, CBRN decontamination (HERP / CDEM Act 2002)
//
// Specialist departments are separate modules:
//
//	tpt-oncology, tpt-renal, tpt-maternity, tpt-cardiology,
//	tpt-rehabilitation, tpt-paediatrics, tpt-blood-bank
func (s *Server) buildRoutes() *http.ServeMux {
	admissions := &AdmissionsHandler{pool: s.pool, enc: s.enc, hpiClient: s.hpiClient, auditTrail: s.auditTrail, hl7Client: s.hl7Client, logger: s.logger}
	wards := &WardsHandler{pool: s.pool, auditTrail: s.auditTrail, logger: s.logger}
	ed := &EDHandler{pool: s.pool, enc: s.enc, hpiClient: s.hpiClient, auditTrail: s.auditTrail, logger: s.logger}
	icu := &ICUHandler{pool: s.pool, enc: s.enc, hpiClient: s.hpiClient, auditTrail: s.auditTrail, logger: s.logger}
	theatre := &TheatreHandler{pool: s.pool, enc: s.enc, hpiClient: s.hpiClient, auditTrail: s.auditTrail, logger: s.logger}
	preadmission := &PreAdmissionHandler{pool: s.pool, enc: s.enc, hpiClient: s.hpiClient, auditTrail: s.auditTrail, logger: s.logger}
	coding := &CodingHandler{pool: s.pool, auditTrail: s.auditTrail, logger: s.logger}
	billing := &BillingHandler{pool: s.pool, auditTrail: s.auditTrail, logger: s.logger}
	pharmacy := &PharmacyHandler{pool: s.pool, enc: s.enc, hpiClient: s.hpiClient, auditTrail: s.auditTrail, logger: s.logger}
	cpoe := &CPOEHandler{pool: s.pool, hpiClient: s.hpiClient, auditTrail: s.auditTrail, hl7Client: s.hl7Client, logger: s.logger}
	infectionControl := &InfectionControlHandler{pool: s.pool, enc: s.enc, auditTrail: s.auditTrail, logger: s.logger}
	outpatient := &OutpatientHandler{pool: s.pool, enc: s.enc, hpiClient: s.hpiClient, auditTrail: s.auditTrail, logger: s.logger}
	hith := &HITHHandler{pool: s.pool, enc: s.enc, hpiClient: s.hpiClient, auditTrail: s.auditTrail, logger: s.logger}

	// Emergency & disaster management handlers (CIMS / MCI / surge / CBRN).
	emergency := &EmergencyHandler{pool: s.pool, auditTrail: s.auditTrail, eventBus: s.eventBus, logger: s.logger}
	mci := &MCIHandler{pool: s.pool, enc: s.enc, auditTrail: s.auditTrail, logger: s.logger}
	surge := &SurgeHandler{pool: s.pool, auditTrail: s.auditTrail, eventBus: s.eventBus, logger: s.logger}
	cbrn := &CBRNHandler{pool: s.pool, auditTrail: s.auditTrail, logger: s.logger}

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

	// ── Core inpatient ────────────────────────────────────────────────────────
	mux.Handle("GET /api/v1/admissions", chain(http.HandlerFunc(admissions.List)))
	mux.Handle("POST /api/v1/admissions", chain(http.HandlerFunc(admissions.Create)))
	mux.Handle("GET /api/v1/admissions/{id}", chain(http.HandlerFunc(admissions.Get)))
	mux.Handle("PUT /api/v1/admissions/{id}", chain(http.HandlerFunc(admissions.Update)))
	mux.Handle("POST /api/v1/admissions/{id}/discharge", chain(http.HandlerFunc(admissions.Discharge)))
	mux.Handle("POST /api/v1/admissions/{id}/transfer", chain(http.HandlerFunc(admissions.Transfer)))
	mux.Handle("GET /api/v1/admissions/{admissionId}/discharge-summary", chain(http.HandlerFunc(admissions.GetDischargeSummary)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/discharge-summary", chain(http.HandlerFunc(admissions.CreateDischargeSummary)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/discharge-summary/auto-populate", chain(http.HandlerFunc(admissions.AutoPopulateDischargeSummaryHandler)))

	// ── Ward and bed management ───────────────────────────────────────────────
	mux.Handle("GET /api/v1/wards", chain(http.HandlerFunc(wards.ListWards)))
	mux.Handle("GET /api/v1/wards/{wardId}", chain(http.HandlerFunc(wards.GetWard)))
	mux.Handle("GET /api/v1/wards/{wardId}/beds", chain(http.HandlerFunc(wards.ListBeds)))
	mux.Handle("PUT /api/v1/wards/{wardId}/beds/{bedId}", chain(http.HandlerFunc(wards.UpdateBed)))
	mux.Handle("GET /api/v1/wards/capacity", chain(http.HandlerFunc(wards.HospitalCapacity)))
	mux.Handle("GET /api/v1/wards/flow-forecast", chain(http.HandlerFunc(wards.PatientFlowForecast)))

	// ── Emergency department ──────────────────────────────────────────────────
	mux.Handle("GET /api/v1/ed/triage", chain(http.HandlerFunc(ed.List)))
	mux.Handle("POST /api/v1/ed/triage", chain(http.HandlerFunc(ed.Create)))
	mux.Handle("GET /api/v1/ed/triage/{id}", chain(http.HandlerFunc(ed.Get)))
	mux.Handle("PUT /api/v1/ed/triage/{id}", chain(http.HandlerFunc(ed.Update)))
	mux.Handle("POST /api/v1/ed/triage/{id}/assign", chain(http.HandlerFunc(ed.Assign)))
	mux.Handle("POST /api/v1/ed/triage/{id}/dispose", chain(http.HandlerFunc(ed.Dispose)))
	mux.Handle("GET /api/v1/ed/queue", chain(http.HandlerFunc(ed.Queue)))
	mux.Handle("GET /api/v1/ed/stats", chain(http.HandlerFunc(ed.Stats)))

	// ── ICU ───────────────────────────────────────────────────────────────────
	mux.Handle("GET /api/v1/icu/admissions", chain(http.HandlerFunc(icu.List)))
	mux.Handle("POST /api/v1/icu/admissions", chain(http.HandlerFunc(icu.Create)))
	mux.Handle("GET /api/v1/icu/admissions/{id}", chain(http.HandlerFunc(icu.Get)))
	mux.Handle("POST /api/v1/icu/admissions/{id}/chart", chain(http.HandlerFunc(icu.AddChart)))
	mux.Handle("GET /api/v1/icu/admissions/{id}/chart", chain(http.HandlerFunc(icu.ListChart)))
	mux.Handle("POST /api/v1/icu/admissions/{id}/discharge", chain(http.HandlerFunc(icu.Discharge)))

	// ── ICU fluid balance ─────────────────────────────────────────────────────
	mux.Handle("POST /api/v1/icu/admissions/{id}/fluid-balance", chain(http.HandlerFunc(icu.AddFluidBalance)))
	mux.Handle("GET /api/v1/icu/admissions/{id}/fluid-balance", chain(http.HandlerFunc(icu.ListFluidBalance)))

	// ── ICU EWS/PEWS early-warning scoring ───────────────────────────────────
	mux.Handle("POST /api/v1/icu/admissions/{id}/ews", chain(http.HandlerFunc(icu.CalculateEWS)))
	mux.Handle("POST /api/v1/icu/admissions/{id}/pews", chain(http.HandlerFunc(icu.CalculatePEWS)))
	mux.Handle("GET /api/v1/icu/admissions/{id}/ews", chain(http.HandlerFunc(icu.ListEWS)))

	// ── Paediatric dosing calculator ──────────────────────────────────────────
	mux.Handle("POST /api/v1/icu/paediatric-dose", chain(http.HandlerFunc(icu.CalculatePaediatricDose)))

	// ── Surgical theatre ──────────────────────────────────────────────────────
	mux.Handle("GET /api/v1/theatre/bookings", chain(http.HandlerFunc(theatre.List)))
	mux.Handle("POST /api/v1/theatre/bookings", chain(http.HandlerFunc(theatre.Create)))
	mux.Handle("GET /api/v1/theatre/bookings/{id}", chain(http.HandlerFunc(theatre.Get)))
	mux.Handle("PUT /api/v1/theatre/bookings/{id}", chain(http.HandlerFunc(theatre.Update)))
	mux.Handle("POST /api/v1/theatre/bookings/{id}/confirm", chain(http.HandlerFunc(theatre.Confirm)))
	mux.Handle("POST /api/v1/theatre/bookings/{id}/cancel", chain(http.HandlerFunc(theatre.Cancel)))
	mux.Handle("POST /api/v1/theatre/bookings/{id}/start", chain(http.HandlerFunc(theatre.Start)))
	mux.Handle("POST /api/v1/theatre/bookings/{id}/complete", chain(http.HandlerFunc(theatre.Complete)))
	mux.Handle("GET /api/v1/theatre/schedule", chain(http.HandlerFunc(theatre.DaySchedule)))

	// ── Pre-admission assessment ──────────────────────────────────────────────
	mux.Handle("GET /api/v1/preadmission/assessments", chain(http.HandlerFunc(preadmission.List)))
	mux.Handle("POST /api/v1/preadmission/assessments", chain(http.HandlerFunc(preadmission.Create)))
	mux.Handle("GET /api/v1/preadmission/assessments/{id}", chain(http.HandlerFunc(preadmission.Get)))
	mux.Handle("PUT /api/v1/preadmission/assessments/{id}", chain(http.HandlerFunc(preadmission.Update)))
	mux.Handle("POST /api/v1/preadmission/assessments/{id}/complete", chain(http.HandlerFunc(preadmission.Complete)))
	mux.Handle("POST /api/v1/preadmission/assessments/{id}/approve", chain(http.HandlerFunc(preadmission.Approve)))

	// ── Clinical coding (ICD-10-AM + ACHI) ───────────────────────────────────
	mux.Handle("GET /api/v1/admissions/{admissionId}/coding", chain(http.HandlerFunc(coding.List)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/coding", chain(http.HandlerFunc(coding.Add)))
	mux.Handle("DELETE /api/v1/admissions/{admissionId}/coding/{codeId}", chain(http.HandlerFunc(coding.Remove)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/coding/validate", chain(http.HandlerFunc(coding.Validate)))

	// ── Hospital billing (casemix / DRG) ──────────────────────────────────────
	mux.Handle("GET /api/v1/admissions/{admissionId}/drg", chain(http.HandlerFunc(billing.GetDRG)))
	mux.Handle("GET /api/v1/admissions/{admissionId}/invoice", chain(http.HandlerFunc(billing.GetInvoice)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/invoice", chain(http.HandlerFunc(billing.CreateInvoice)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/invoice/submit", chain(http.HandlerFunc(billing.SubmitInvoice)))

	// ── Inpatient pharmacy (medication charts, IV pharmacy, reconciliation) ───
	// NOTE: tpt-pharmacy handles community dispensing. This covers inpatient
	// prescribing, administration recording, and medication reconciliation only.
	mux.Handle("GET /api/v1/admissions/{admissionId}/medications", chain(http.HandlerFunc(pharmacy.List)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/medications", chain(http.HandlerFunc(pharmacy.Prescribe)))
	mux.Handle("GET /api/v1/admissions/{admissionId}/medications/{medId}", chain(http.HandlerFunc(pharmacy.Get)))
	mux.Handle("PUT /api/v1/admissions/{admissionId}/medications/{medId}", chain(http.HandlerFunc(pharmacy.Update)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/medications/{medId}/administer", chain(http.HandlerFunc(pharmacy.Administer)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/medications/{medId}/cease", chain(http.HandlerFunc(pharmacy.Cease)))
	mux.Handle("GET /api/v1/admissions/{admissionId}/medications/reconciliation", chain(http.HandlerFunc(pharmacy.GetReconciliation)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/medications/reconciliation", chain(http.HandlerFunc(pharmacy.ReconcileMedications)))

	// ── S8 controlled drug register + bedside verification ───────────────────
	mux.Handle("GET /api/v1/admissions/{admissionId}/controlled-drug-register", chain(http.HandlerFunc(pharmacy.ListControlledDrugRegister)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/controlled-drug-register", chain(http.HandlerFunc(pharmacy.AddControlledDrugEntry)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/medications/{medId}/verify", chain(http.HandlerFunc(pharmacy.VerifyBedside)))

	// ── IV Pump / Smart Infusion Integration ───────────────────────────────────
	mux.Handle("POST /api/v1/admissions/{admissionId}/medications/{medId}/iv-pump/link", chain(http.HandlerFunc(pharmacy.LinkIVPump)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/medications/{medId}/iv-pump/status", chain(http.HandlerFunc(pharmacy.UpdateIVPumpStatus)))
	mux.Handle("GET /api/v1/admissions/{admissionId}/medications/{medId}/iv-pump", chain(http.HandlerFunc(pharmacy.ListIVInfusions)))

	// ── Discharge Summary GP Transmission ─────────────────────────────────────
	mux.Handle("POST /api/v1/admissions/{admissionId}/discharge-summary/notify-gp", chain(http.HandlerFunc(admissions.NotifyGP)))

	// ── CPOE — Computerised Provider Order Entry (lab/imaging/consult orders) ─
	mux.Handle("GET /api/v1/admissions/{admissionId}/orders", chain(http.HandlerFunc(cpoe.List)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/orders", chain(http.HandlerFunc(cpoe.Create)))
	mux.Handle("GET /api/v1/admissions/{admissionId}/orders/{orderId}", chain(http.HandlerFunc(cpoe.Get)))
	mux.Handle("PUT /api/v1/admissions/{admissionId}/orders/{orderId}", chain(http.HandlerFunc(cpoe.Update)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/orders/{orderId}/cancel", chain(http.HandlerFunc(cpoe.Cancel)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/orders/{orderId}/complete", chain(http.HandlerFunc(cpoe.Complete)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/orders/{orderId}/dispatch", chain(http.HandlerFunc(cpoe.Dispatch)))
	mux.Handle("POST /api/v1/orders/result-callback", chain(http.HandlerFunc(cpoe.ResultCallback)))

	// ── Infection control (HAI surveillance, isolation precautions) ───────────
	mux.Handle("GET /api/v1/infection-control/alerts", chain(http.HandlerFunc(infectionControl.ListAlerts)))
	mux.Handle("POST /api/v1/infection-control/alerts", chain(http.HandlerFunc(infectionControl.CreateAlert)))
	mux.Handle("GET /api/v1/infection-control/alerts/{id}", chain(http.HandlerFunc(infectionControl.GetAlert)))
	mux.Handle("PUT /api/v1/infection-control/alerts/{id}", chain(http.HandlerFunc(infectionControl.UpdateAlert)))
	mux.Handle("GET /api/v1/admissions/{admissionId}/isolation", chain(http.HandlerFunc(infectionControl.ListIsolation)))
	mux.Handle("POST /api/v1/admissions/{admissionId}/isolation", chain(http.HandlerFunc(infectionControl.ApplyIsolation)))
	mux.Handle("DELETE /api/v1/admissions/{admissionId}/isolation/{isolationId}", chain(http.HandlerFunc(infectionControl.RemoveIsolation)))

	// ── Outpatient specialist clinics ─────────────────────────────────────────
	mux.Handle("GET /api/v1/outpatient/clinics", chain(http.HandlerFunc(outpatient.ListClinics)))
	mux.Handle("GET /api/v1/outpatient/clinics/{id}", chain(http.HandlerFunc(outpatient.GetClinic)))
	mux.Handle("GET /api/v1/outpatient/clinics/{id}/appointments", chain(http.HandlerFunc(outpatient.ListAppointments)))
	mux.Handle("POST /api/v1/outpatient/clinics/{id}/appointments", chain(http.HandlerFunc(outpatient.BookAppointment)))
	mux.Handle("PUT /api/v1/outpatient/clinics/{id}/appointments/{apptId}", chain(http.HandlerFunc(outpatient.UpdateAppointment)))
	mux.Handle("POST /api/v1/outpatient/clinics/{id}/appointments/{apptId}/attend", chain(http.HandlerFunc(outpatient.Attend)))
	mux.Handle("GET /api/v1/outpatient/waitlist", chain(http.HandlerFunc(outpatient.ListWaitlist)))
	mux.Handle("POST /api/v1/outpatient/waitlist", chain(http.HandlerFunc(outpatient.AddToWaitlist)))
	mux.Handle("PUT /api/v1/outpatient/waitlist/{id}", chain(http.HandlerFunc(outpatient.UpdateWaitlistEntry)))
	mux.Handle("DELETE /api/v1/outpatient/waitlist/{id}", chain(http.HandlerFunc(outpatient.RemoveFromWaitlist)))

	// ── Hospital in the Home (HITH) ───────────────────────────────────────────
	mux.Handle("GET /api/v1/hith/episodes", chain(http.HandlerFunc(hith.ListEpisodes)))
	mux.Handle("POST /api/v1/hith/episodes", chain(http.HandlerFunc(hith.CreateEpisode)))
	mux.Handle("GET /api/v1/hith/episodes/{id}", chain(http.HandlerFunc(hith.GetEpisode)))
	mux.Handle("PUT /api/v1/hith/episodes/{id}", chain(http.HandlerFunc(hith.UpdateEpisode)))
	mux.Handle("POST /api/v1/hith/episodes/{id}/visits", chain(http.HandlerFunc(hith.AddVisit)))
	mux.Handle("GET /api/v1/hith/episodes/{id}/visits", chain(http.HandlerFunc(hith.ListVisits)))
	mux.Handle("PUT /api/v1/hith/episodes/{id}/visits/{visitId}", chain(http.HandlerFunc(hith.UpdateVisit)))
	mux.Handle("POST /api/v1/hith/episodes/{id}/discharge", chain(http.HandlerFunc(hith.Discharge)))

	// ── Emergency & disaster management (CIMS) ────────────────────────────────
	mux.Handle("POST /api/v1/emergency/incidents", chain(http.HandlerFunc(emergency.DeclareIncident)))
	mux.Handle("GET /api/v1/emergency/incidents", chain(http.HandlerFunc(emergency.ListIncidents)))
	mux.Handle("GET /api/v1/emergency/incidents/{id}", chain(http.HandlerFunc(emergency.GetIncident)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/activate", chain(http.HandlerFunc(emergency.ActivateIncident)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/escalate", chain(http.HandlerFunc(emergency.EscalateIncident)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/stand-down", chain(http.HandlerFunc(emergency.StandDown)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/close", chain(http.HandlerFunc(emergency.CloseIncident)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/assign-role", chain(http.HandlerFunc(emergency.AssignRole)))
	mux.Handle("GET /api/v1/emergency/incidents/{id}/log", chain(http.HandlerFunc(emergency.ListLog)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/log", chain(http.HandlerFunc(emergency.AddLog)))
	mux.Handle("GET /api/v1/emergency/incidents/{id}/resources", chain(http.HandlerFunc(emergency.ListResources)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/resources", chain(http.HandlerFunc(emergency.RequestResource)))
	mux.Handle("PATCH /api/v1/emergency/incidents/{id}/resources/{rid}", chain(http.HandlerFunc(emergency.UpdateResource)))

	// ── MCI triage ────────────────────────────────────────────────────────────
	mux.Handle("POST /api/v1/emergency/incidents/{id}/mci/patients", chain(http.HandlerFunc(mci.TagPatient)))
	mux.Handle("GET /api/v1/emergency/incidents/{id}/mci/patients", chain(http.HandlerFunc(mci.ListPatients)))
	mux.Handle("PUT /api/v1/emergency/incidents/{id}/mci/patients/{pid}", chain(http.HandlerFunc(mci.UpdatePatient)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/mci/patients/{pid}/identify", chain(http.HandlerFunc(mci.IdentifyPatient)))
	mux.Handle("GET /api/v1/emergency/incidents/{id}/mci/summary", chain(http.HandlerFunc(mci.Summary)))

	// ── Surge capacity ────────────────────────────────────────────────────────
	mux.Handle("GET /api/v1/emergency/incidents/{id}/surge", chain(http.HandlerFunc(surge.GetSurgeStatus)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/surge/snapshot", chain(http.HandlerFunc(surge.RecordSnapshot)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/surge/escalate", chain(http.HandlerFunc(surge.EscalateSurge)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/surge/de-escalate", chain(http.HandlerFunc(surge.DeEscalateSurge)))
	mux.Handle("GET /api/v1/emergency/incidents/{id}/surge/history", chain(http.HandlerFunc(surge.ListSnapshots)))

	// ── CBRN decontamination (only active for type='cbrn' incidents) ──────────
	mux.Handle("GET /api/v1/emergency/incidents/{id}/cbrn/zones", chain(http.HandlerFunc(cbrn.ListZones)))
	mux.Handle("GET /api/v1/emergency/incidents/{id}/cbrn/patients", chain(http.HandlerFunc(cbrn.ListPatients)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/cbrn/patients/{pid}/decon-start", chain(http.HandlerFunc(cbrn.StartDecon)))
	mux.Handle("POST /api/v1/emergency/incidents/{id}/cbrn/patients/{pid}/decon-complete", chain(http.HandlerFunc(cbrn.CompleteDecon)))

	return mux
}

// handleHealth responds to liveness probes.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "tpt-hospital",
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
		"service": "tpt-hospital",
	})
}

// RunMigrations runs database migrations for the tpt-hospital module.
func RunMigrations(ctx context.Context, databaseURL string) error {
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect for migrations: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, "modules/tpt-hospital/db/migrate"); err != nil {
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

// apiError is the standard error response envelope.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// errNotFound is a sentinel for missing records.
var errNotFound = errors.New("record not found")

// writeJSON serialises v as JSON and writes it to w with the given status code.
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
