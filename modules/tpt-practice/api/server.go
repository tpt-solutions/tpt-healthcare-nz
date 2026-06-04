// Package api provides the HTTP server for the tpt-practice module.
// It handles practice management endpoints: roster, rooms, leave, inventory,
// budget, accounting sync, payroll sync, department management, and onboarding.
package api

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PhillipC05/tpt-healthcare/core/accounts"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/inventory"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/rbac"
)

// Config holds dependencies for the API server.
type Config struct {
	Pool     *pgxpool.Pool
	Logger   *slog.Logger
	Auth     auth.Provider
	Checker  *rbac.Checker
}

// Server is the HTTP multiplexer for tpt-practice.
type Server struct {
	mux    *http.ServeMux
	cfg    Config
	rbacRepo     rbac.Repository
	inventoryRepo inventory.Repository
	accountsRepo  accounts.Repository
}

// NewServer constructs and configures the API server.
func NewServer(cfg Config) *Server {
	s := &Server{
		mux:          http.NewServeMux(),
		cfg:          cfg,
		rbacRepo:     rbac.NewPostgresRepository(cfg.Pool),
		inventoryRepo: nil, // inventory postgres repo wired here when implemented
		accountsRepo:  accounts.NewPostgresRepository(cfg.Pool),
	}
	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	chain := func(h http.Handler) http.Handler {
		if s.cfg.Auth != nil {
			h = middleware.AuditWrap(h, nil)
			h = auth.RequireAuth(s.cfg.Auth)(h)
		}
		h = middleware.CORS(h)
		h = middleware.RateLimit(h)
		h = middleware.Recovery(h, s.cfg.Logger)
		return h
	}

	// Onboarding wizard
	s.mux.Handle("GET /api/v1/practice/onboarding", chain(http.HandlerFunc(s.getOnboardingWizard)))
	s.mux.Handle("PUT /api/v1/practice/onboarding/step/{step}", chain(http.HandlerFunc(s.updateWizardStep)))

	// Departments
	s.mux.Handle("GET /api/v1/practice/departments", chain(http.HandlerFunc(s.listDepartments)))
	s.mux.Handle("POST /api/v1/practice/departments", chain(http.HandlerFunc(s.createDepartment)))
	s.mux.Handle("GET /api/v1/practice/departments/{id}", chain(http.HandlerFunc(s.getDepartment)))
	s.mux.Handle("PUT /api/v1/practice/departments/{id}", chain(http.HandlerFunc(s.updateDepartment)))
	s.mux.Handle("DELETE /api/v1/practice/departments/{id}", chain(http.HandlerFunc(s.deleteDepartment)))

	// Role assignments
	s.mux.Handle("GET /api/v1/practice/roles", chain(http.HandlerFunc(s.listRoleAssignments)))
	s.mux.Handle("POST /api/v1/practice/roles", chain(http.HandlerFunc(s.grantRole)))
	s.mux.Handle("DELETE /api/v1/practice/roles/{id}", chain(http.HandlerFunc(s.revokeRole)))

	// Roster
	s.mux.Handle("GET /api/v1/practice/roster", chain(http.HandlerFunc(s.listShifts)))
	s.mux.Handle("POST /api/v1/practice/roster", chain(http.HandlerFunc(s.createShift)))
	s.mux.Handle("DELETE /api/v1/practice/roster/{id}", chain(http.HandlerFunc(s.deleteShift)))

	// Room bookings
	s.mux.Handle("GET /api/v1/practice/rooms", chain(http.HandlerFunc(s.listRooms)))
	s.mux.Handle("POST /api/v1/practice/rooms", chain(http.HandlerFunc(s.createRoom)))
	s.mux.Handle("GET /api/v1/practice/rooms/bookings", chain(http.HandlerFunc(s.listBookings)))
	s.mux.Handle("POST /api/v1/practice/rooms/bookings", chain(http.HandlerFunc(s.createBooking)))
	s.mux.Handle("DELETE /api/v1/practice/rooms/bookings/{id}", chain(http.HandlerFunc(s.deleteBooking)))

	// Leave
	s.mux.Handle("GET /api/v1/practice/leave", chain(http.HandlerFunc(s.listLeaveRequests)))
	s.mux.Handle("POST /api/v1/practice/leave", chain(http.HandlerFunc(s.createLeaveRequest)))
	s.mux.Handle("POST /api/v1/practice/leave/{id}/approve", chain(http.HandlerFunc(s.approveLeave)))
	s.mux.Handle("POST /api/v1/practice/leave/{id}/decline", chain(http.HandlerFunc(s.declineLeave)))

	// Inventory
	s.mux.Handle("GET /api/v1/practice/inventory", chain(http.HandlerFunc(s.listStockItems)))
	s.mux.Handle("POST /api/v1/practice/inventory", chain(http.HandlerFunc(s.createStockItem)))
	s.mux.Handle("GET /api/v1/practice/inventory/{id}", chain(http.HandlerFunc(s.getStockItem)))
	s.mux.Handle("POST /api/v1/practice/inventory/{id}/receive", chain(http.HandlerFunc(s.receiveStock)))
	s.mux.Handle("POST /api/v1/practice/inventory/{id}/consume", chain(http.HandlerFunc(s.consumeStock)))
	s.mux.Handle("GET /api/v1/practice/inventory/purchase-orders", chain(http.HandlerFunc(s.listPurchaseOrders)))
	s.mux.Handle("POST /api/v1/practice/inventory/purchase-orders", chain(http.HandlerFunc(s.createPurchaseOrder)))
	s.mux.Handle("GET /api/v1/practice/inventory/cold-chain/breaches", chain(http.HandlerFunc(s.listColdChainBreaches)))

	// Budget & cost centres
	s.mux.Handle("GET /api/v1/practice/cost-centres", chain(http.HandlerFunc(s.listCostCentres)))
	s.mux.Handle("POST /api/v1/practice/cost-centres", chain(http.HandlerFunc(s.createCostCentre)))
	s.mux.Handle("GET /api/v1/practice/cost-centres/{id}/budget/{year}", chain(http.HandlerFunc(s.getBudget)))
	s.mux.Handle("PUT /api/v1/practice/cost-centres/{id}/budget/{year}/lines", chain(http.HandlerFunc(s.upsertBudgetLine)))
	s.mux.Handle("GET /api/v1/practice/cost-centres/{id}/variance/{year}", chain(http.HandlerFunc(s.getVarianceReport)))

	// Accounting & payroll sync status
	s.mux.Handle("GET /api/v1/practice/accounting/status", chain(http.HandlerFunc(s.accountingStatus)))
	s.mux.Handle("POST /api/v1/practice/accounting/sync", chain(http.HandlerFunc(s.triggerAccountingSync)))
	s.mux.Handle("GET /api/v1/practice/payroll/status", chain(http.HandlerFunc(s.payrollStatus)))
	s.mux.Handle("POST /api/v1/practice/payroll/sync", chain(http.HandlerFunc(s.triggerPayrollSync)))
	s.mux.Handle("GET /api/v1/practice/payroll/payslips", chain(http.HandlerFunc(s.listPayslips)))
	s.mux.Handle("GET /api/v1/practice/payroll/leave-balance", chain(http.HandlerFunc(s.leaveBalance)))
}
