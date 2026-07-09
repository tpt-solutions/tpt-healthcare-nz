package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
)

// inlineRepo is a minimal in-memory rbac.Repository for tests.
type inlineRepo struct {
	assignments map[string][]RoleAssignment
}

func newInlineRepo() *inlineRepo {
	return &inlineRepo{assignments: make(map[string][]RoleAssignment)}
}

func (r *inlineRepo) CreateDepartment(_ context.Context, dept Department) (*Department, error) {
	dept.ID = uuid.New()
	return &dept, nil
}
func (r *inlineRepo) ListDepartments(_ context.Context, _ uuid.UUID) ([]Department, error) {
	return nil, nil
}
func (r *inlineRepo) GetDepartment(_ context.Context, _ uuid.UUID) (*Department, error) {
	return nil, nil
}
func (r *inlineRepo) UpdateDepartment(_ context.Context, dept Department) (*Department, error) {
	return &dept, nil
}
func (r *inlineRepo) DeleteDepartment(_ context.Context, _ uuid.UUID) error { return nil }
func (r *inlineRepo) GrantRole(_ context.Context, a RoleAssignment) (*RoleAssignment, error) {
	a.ID = uuid.New()
	r.assignments[a.PrincipalID] = append(r.assignments[a.PrincipalID], a)
	return &a, nil
}
func (r *inlineRepo) RevokeRole(_ context.Context, _ uuid.UUID) error            { return nil }
func (r *inlineRepo) ListAssignments(_ context.Context, _ uuid.UUID, pid string) ([]RoleAssignment, error) {
	return r.assignments[pid], nil
}
func (r *inlineRepo) ListAssignmentsByTenant(_ context.Context, _ uuid.UUID) ([]RoleAssignment, error) {
	return nil, nil
}
func (r *inlineRepo) AssignmentsByPrincipalAndDept(_ context.Context, _ uuid.UUID, pid string, _ []uuid.UUID) ([]RoleAssignment, error) {
	return r.assignments[pid], nil
}

func TestRoleAssignment_IsActive(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		revoked  *time.Time
		expected bool
	}{
		{"active when nil", nil, true},
		{"inactive when revoked", &now, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := RoleAssignment{RevokedAt: tt.revoked}
			assert.Equal(t, tt.expected, a.IsActive())
		})
	}
}

func TestChecker_HasRole(t *testing.T) {
	checker := NewChecker(newInlineRepo())
	tests := []struct {
		name     string
		roles    []string
		role     BuiltInRole
		expected bool
	}{
		{"has role", []string{"clinician"}, RoleClinician, true},
		{"missing role", []string{"clinician"}, RoleNurse, false},
		{"empty roles", []string{}, RoleClinician, false},
		{"multiple roles", []string{"clinician", "nurse"}, RoleNurse, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &auth.Principal{Roles: tt.roles}
			assert.Equal(t, tt.expected, checker.HasRole(context.Background(), p, tt.role))
		})
	}
}

func TestChecker_HasDeptAccess(t *testing.T) {
	checker := NewChecker(newInlineRepo())
	deptID := uuid.New()
	tests := []struct {
		name     string
		roles    []string
		depts    []uuid.UUID
		target   uuid.UUID
		expected bool
	}{
		{"network_admin bypasses", []string{"network_admin"}, nil, deptID, true},
		{"practice_admin bypasses", []string{"practice_admin"}, nil, deptID, true},
		{"matching dept", []string{"clinician"}, []uuid.UUID{deptID}, deptID, true},
		{"no matching dept", []string{"clinician"}, []uuid.UUID{uuid.New()}, deptID, false},
		{"empty depts", []string{"clinician"}, []uuid.UUID{}, deptID, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &auth.Principal{Roles: tt.roles, DepartmentIDs: tt.depts}
			assert.Equal(t, tt.expected, checker.HasDeptAccess(context.Background(), p, tt.target))
		})
	}
}

func TestChecker_CanAccess(t *testing.T) {
	checker := NewChecker(newInlineRepo())
	anyUUID := uuid.New()
	tests := []struct {
		name     string
		roles    []string
		resource string
		action   string
		expected bool
	}{
		{"network_admin all", []string{"network_admin"}, "patient", "read", true},
		{"network_admin write", []string{"network_admin"}, "patient", "write", true},
		{"clinician patient.note", []string{"clinician"}, "patient.note", "read", true},
		{"nurse encounter", []string{"nurse"}, "encounter", "read", true},
		{"receptionist patient.note denied", []string{"receptionist"}, "patient.note", "read", false},
		{"anyone patient read", []string{"receptionist"}, "patient", "read", true},
		{"clinician patient write", []string{"clinician"}, "patient", "write", true},
		{"receptionist appointment write", []string{"receptionist"}, "appointment", "write", true},
		{"pharmacist patient write denied", []string{"pharmacist"}, "patient", "write", false},
		{"billing_manager invoice", []string{"billing_manager"}, "invoice", "read", true},
		{"practice_admin invoice", []string{"practice_admin"}, "invoice", "write", true},
		{"receptionist payment", []string{"receptionist"}, "payment", "read", true},
		{"clinician invoice denied", []string{"clinician"}, "invoice", "read", false},
		{"pharmacist stock_item", []string{"pharmacist"}, "stock_item", "read", true},
		{"clinician stock denied", []string{"clinician"}, "stock_item", "read", false},
		{"roster_manager roster", []string{"roster_manager"}, "roster", "read", true},
		{"practice_admin room_booking", []string{"practice_admin"}, "room_booking", "write", true},
		{"clinician roster denied", []string{"clinician"}, "roster", "read", false},
		{"billing_manager budget", []string{"billing_manager"}, "budget", "read", true},
		{"practice_admin cost_centre", []string{"practice_admin"}, "cost_centre", "write", true},
		{"receptionist budget denied", []string{"receptionist"}, "budget", "read", false},
		{"practice_admin department", []string{"practice_admin"}, "department", "read", true},
		{"clinician department denied", []string{"clinician"}, "department", "read", false},
		{"practice_admin audit read", []string{"practice_admin"}, "audit_event", "read", true},
		{"audit_event write denied", []string{"practice_admin"}, "audit_event", "write", false},
		{"pharmacist pharmacy", []string{"pharmacist"}, "pharmacy", "read", true},
		{"clinician pharmacy denied", []string{"clinician"}, "pharmacy", "read", false},
		{"clinician emergency read", []string{"clinician"}, "emergency.incident", "read", true},
		{"clinician emergency write", []string{"clinician"}, "emergency.incident", "write", true},
		{"ic emergency write", []string{"incident_commander"}, "emergency.incident", "write", true},
		{"erc emergency write", []string{"emergency_response_coordinator"}, "emergency.resource", "write", true},
		{"pharmacist emergency denied", []string{"pharmacist"}, "emergency.incident", "read", false},
		{"unknown resource", []string{"clinician"}, "unknown_resource", "read", false},
		{"empty roles patient read", []string{}, "patient", "read", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &auth.Principal{ID: "user-1", TenantID: anyUUID, Roles: tt.roles, DepartmentIDs: []uuid.UUID{anyUUID}}
			assert.Equal(t, tt.expected, checker.CanAccess(context.Background(), p, tt.resource, tt.action))
		})
	}
}

func TestRequirePermission_Middleware(t *testing.T) {
	checker := NewChecker(newInlineRepo())
	anyUUID := uuid.New()

	t.Run("unauthorized when no principal", func(t *testing.T) {
		handler := RequirePermission(checker, "patient", "read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("forbidden when no permission", func(t *testing.T) {
		handler := RequirePermission(checker, "invoice", "read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		p := &auth.Principal{ID: "user-1", TenantID: anyUUID, Roles: []string{"clinician"}}
		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(auth.PrincipalToContext(req.Context(), p))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("allowed when has permission", func(t *testing.T) {
		handler := RequirePermission(checker, "patient", "read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		p := &auth.Principal{ID: "user-1", TenantID: anyUUID, Roles: []string{"clinician"}}
		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(auth.PrincipalToContext(req.Context(), p))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// Ensure require is used
var _ = require.NotNil
