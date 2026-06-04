// Package rbac provides department-scoped role-based access control extending
// the existing auth.Principal. The current Principal already has Roles []string
// and RequireRole() middleware; this package adds department scoping, a formal
// RoleAssignment DB model, and a RequirePermission middleware for fine-grained
// resource/action checks.
package rbac

import (
	"time"

	"github.com/google/uuid"
)

// BuiltInRole is a predefined role name. These strings are stored in
// role_assignments.role and injected into JWT claims at login.
type BuiltInRole string

const (
	RoleNetworkAdmin     BuiltInRole = "network_admin"    // existing — multi-tenant network administration
	RolePracticeAdmin    BuiltInRole = "practice_admin"   // full operational access; no clinical record content
	RoleClinician        BuiltInRole = "clinician"        // clinical data + own departments
	RoleReceptionist     BuiltInRole = "receptionist"     // scheduling, billing, demographics; no clinical notes
	RoleNurse            BuiltInRole = "nurse"            // clinical notes read, vitals write; department-scoped
	RolePharmacist       BuiltInRole = "pharmacist"       // pharmacy module only
	RoleBillingManager   BuiltInRole = "billing_manager"  // billing + finance; no clinical content
	RoleInventoryManager BuiltInRole = "inventory_manager"
	RoleRosterManager    BuiltInRole = "roster_manager"
)

// Department defines an access boundary within a tenant (e.g. GP, Pharmacy, Lab, Admin).
// Departments can form a simple hierarchy via ParentID.
type Department struct {
	ID       uuid.UUID  `db:"id"        json:"id"`
	TenantID uuid.UUID  `db:"tenant_id" json:"tenant_id"`
	Name     string     `db:"name"      json:"name"`
	// Code is a short slug used for display and config (e.g. "gp", "pharmacy", "lab").
	Code       string     `db:"code"       json:"code"`
	ParentID   *uuid.UUID `db:"parent_id"  json:"parent_id,omitempty"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
}

// RoleAssignment links a principal (user) to a role within a tenant,
// optionally scoped to a specific department.
// DepartmentID = nil means the assignment is tenant-wide.
type RoleAssignment struct {
	ID           uuid.UUID  `db:"id"            json:"id"`
	TenantID     uuid.UUID  `db:"tenant_id"     json:"tenant_id"`
	PrincipalID  string     `db:"principal_id"  json:"principal_id"` // auth.Principal.ID (JWT sub)
	Role         BuiltInRole `db:"role"          json:"role"`
	DepartmentID *uuid.UUID `db:"department_id" json:"department_id,omitempty"`
	GrantedBy    string     `db:"granted_by"    json:"granted_by"` // principal ID of granting user
	CreatedAt    time.Time  `db:"created_at"    json:"created_at"`
	RevokedAt    *time.Time `db:"revoked_at"    json:"revoked_at,omitempty"`
}

// IsActive returns true if the assignment has not been revoked.
func (r *RoleAssignment) IsActive() bool {
	return r.RevokedAt == nil
}
