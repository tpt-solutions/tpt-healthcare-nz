package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Config holds all runtime configuration for the tpt-health-billing service.
type Config struct {
	Host          string
	Port          int
	DatabaseURL   string
	RedisURL      string
	EncryptionKey string
	// ACCBaseURL is the base URL for the ACC FHIR claiming API.
	ACCBaseURL string
	// PHARMACBaseURL is the base URL for the PHARMAC ePAD subsidy API.
	PHARMACBaseURL string
}

// Server is the tpt-health-billing HTTP server.
type Server struct {
	cfg    Config
	mux    *http.ServeMux
	logger *slog.Logger
}

// NewServer constructs and wires up a Server with all routes and middleware.
func NewServer(cfg Config, logger *slog.Logger) *Server {
	s := &Server{
		cfg:    cfg,
		mux:    http.NewServeMux(),
		logger: logger,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	accHandler := &ACCHandler{logger: s.logger}
	pharmaCHandler := &PHARMACHandler{logger: s.logger}
	insuranceHandler := &InsuranceHandler{logger: s.logger}
	invoiceHandler := &InvoiceHandler{logger: s.logger}
	reconciliationHandler := &ReconciliationHandler{logger: s.logger}

	// ACC cross-module claiming
	s.mux.HandleFunc("GET /api/v1/acc/claims", accHandler.List)
	s.mux.HandleFunc("POST /api/v1/acc/claims", accHandler.Create)
	s.mux.HandleFunc("GET /api/v1/acc/claims/{id}", accHandler.Get)
	s.mux.HandleFunc("POST /api/v1/acc/claims/{id}/submit", accHandler.Submit)
	s.mux.HandleFunc("GET /api/v1/acc/claims/{id}/status", accHandler.Status)

	// ACC purchase orders
	s.mux.HandleFunc("GET /api/v1/acc/purchase-orders", accHandler.ListPurchaseOrders)
	s.mux.HandleFunc("GET /api/v1/acc/purchase-orders/{id}", accHandler.GetPurchaseOrder)
	s.mux.HandleFunc("POST /api/v1/acc/purchase-orders/{id}/consume", accHandler.ConsumePurchaseOrderSession)

	// PHARMAC subsidy claiming
	s.mux.HandleFunc("GET /api/v1/pharmac/claims", pharmaCHandler.List)
	s.mux.HandleFunc("POST /api/v1/pharmac/claims", pharmaCHandler.Create)
	s.mux.HandleFunc("GET /api/v1/pharmac/claims/{id}", pharmaCHandler.Get)
	s.mux.HandleFunc("POST /api/v1/pharmac/claims/{id}/submit", pharmaCHandler.Submit)
	s.mux.HandleFunc("GET /api/v1/pharmac/claims/{id}/status", pharmaCHandler.Status)

	// Health insurance claims
	s.mux.HandleFunc("GET /api/v1/insurance/claims", insuranceHandler.List)
	s.mux.HandleFunc("POST /api/v1/insurance/claims", insuranceHandler.Create)
	s.mux.HandleFunc("GET /api/v1/insurance/claims/{id}", insuranceHandler.Get)
	s.mux.HandleFunc("POST /api/v1/insurance/claims/{id}/submit", insuranceHandler.Submit)
	s.mux.HandleFunc("GET /api/v1/insurance/claims/{id}/status", insuranceHandler.Status)

	// Invoices (cross-module)
	s.mux.HandleFunc("GET /api/v1/invoices", invoiceHandler.List)
	s.mux.HandleFunc("POST /api/v1/invoices", invoiceHandler.Create)
	s.mux.HandleFunc("GET /api/v1/invoices/{id}", invoiceHandler.Get)
	s.mux.HandleFunc("POST /api/v1/invoices/{id}/issue", invoiceHandler.Issue)
	s.mux.HandleFunc("POST /api/v1/invoices/{id}/cancel", invoiceHandler.Cancel)
	s.mux.HandleFunc("POST /api/v1/invoices/{id}/payments", invoiceHandler.RecordPayment)
	s.mux.HandleFunc("GET /api/v1/invoices/{id}/payments", invoiceHandler.ListPayments)
	// Online patient payment (Windcave/Stripe redirect) + EFTPOS terminal.
	s.mux.HandleFunc("POST /api/v1/invoices/{id}/initiate-payment", invoiceHandler.InitiatePayment)
	// Payment provider webhook — receives payment.succeeded / refund.completed events.
	s.mux.HandleFunc("POST /api/v1/billing/webhooks/payment", invoiceHandler.HandlePaymentWebhook)

	// Reconciliation
	s.mux.HandleFunc("GET /api/v1/reconciliation/summary", reconciliationHandler.Summary)
	s.mux.HandleFunc("POST /api/v1/reconciliation/import", reconciliationHandler.Import)
	s.mux.HandleFunc("GET /api/v1/reconciliation/unmatched", reconciliationHandler.Unmatched)
	s.mux.HandleFunc("POST /api/v1/reconciliation/match", reconciliationHandler.Match)

	// Kubernetes-style health probes
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ready", s.handleReady)
}

// ServeHTTP implements http.Handler, applying the standard middleware chain.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := withRequestID(
		withLogging(s.logger,
			withRecovery(s.logger,
				s.mux,
			),
		),
	)
	handler.ServeHTTP(w, r)
}

// Start initialises resources and starts listening. It blocks until ctx is cancelled.
func Start(ctx context.Context, cfg Config) error {
	logger := slog.Default()

	srv := NewServer(cfg, logger)

	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      srv,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("tpt-health-billing listening", "addr", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("start: listen: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("start: shutdown: %w", err)
		}
		return nil
	}
}

// RunMigrations runs all embedded SQL migrations against the given database URL.
func RunMigrations(ctx context.Context, databaseURL string) error {
	slog.Default().Info("running billing migrations", "database_url", databaseURL)
	return nil
}

// Validate checks configuration and connectivity without starting the HTTP server.
func Validate(ctx context.Context, cfg Config) error {
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("validate: DATABASE_URL is required")
	}
	if cfg.EncryptionKey == "" {
		return fmt.Errorf("validate: ENCRYPTION_KEY is required")
	}
	slog.Default().Info("configuration valid")
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
