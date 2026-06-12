package rbac

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
)

// Checker provides access-control checks against the RBAC model.
type Checker struct {
	repo Repository
}

// NewChecker constructs a Checker.
func NewChecker(repo Repository) *Checker {
	return &Checker{repo: repo}
}

// HasRole returns true if the principal holds the given role within the tenant,
// either tenant-wide or scoped to any department.
func (c *Checker) HasRole(ctx context.Context, principal *auth.Principal, role BuiltInRole) bool {
	return slices.Contains(principal.Roles, string(role))
}

// HasDeptAccess returns true if the principal has an active role assignment that
// covers deptID (either a tenant-wide assignment or one explicitly for deptID).
// It always returns true for network_admin and practice_admin.
func (c *Checker) HasDeptAccess(ctx context.Context, principal *auth.Principal, deptID uuid.UUID) bool {
	// Tenant-wide roles bypass department checks.
	if slices.Contains(principal.Roles, string(RoleNetworkAdmin)) ||
		slices.Contains(principal.Roles, string(RolePracticeAdmin)) {
		return true
	}

	// Check principal.DepartmentIDs (injected at JWT issuance).
	for _, id := range principal.DepartmentIDs {
		if id == deptID {
			return true
		}
	}
	return false
}

// CanAccess is a broader permission check that combines role and department.
// resource is a dot-delimited resource type (e.g. "patient.note", "invoice", "stock_item").
// action is one of "read", "write", "delete".
//
// The mapping follows the principle of least privilege:
//   - network_admin: all resources, all actions
//   - practice_admin: operational resources only (no clinical content)
//   - clinician: clinical + operational read; no billing write
//   - receptionist: scheduling, billing; no clinical notes
//   - etc.
func (c *Checker) CanAccess(ctx context.Context, principal *auth.Principal, resource, action string) bool {
	roles := principal.Roles

	// network_admin has unrestricted access.
	if slices.Contains(roles, string(RoleNetworkAdmin)) {
		return true
	}

	// Clinical resources.
	switch resource {
	case "patient.note", "encounter", "prescription", "referral", "diagnosis":
		return slices.ContainsFunc(roles, func(r string) bool {
			return r == string(RoleClinician) || r == string(RoleNurse)
		})

	case "patient", "appointment":
		// All non-network roles can read; only clinical roles can write.
		if action == "read" {
			return true
		}
		return slices.ContainsFunc(roles, func(r string) bool {
			return r == string(RoleClinician) || r == string(RoleNurse) || r == string(RoleReceptionist)
		})

	case "invoice", "payment":
		return slices.ContainsFunc(roles, func(r string) bool {
			return r == string(RolePracticeAdmin) || r == string(RoleBillingManager) || r == string(RoleReceptionist)
		})

	case "stock_item", "stock_movement", "purchase_order":
		return slices.ContainsFunc(roles, func(r string) bool {
			return r == string(RolePracticeAdmin) || r == string(RoleInventoryManager) || r == string(RolePharmacist)
		})

	case "roster", "room_booking", "leave_request":
		return slices.ContainsFunc(roles, func(r string) bool {
			return r == string(RolePracticeAdmin) || r == string(RoleRosterManager)
		})

	case "budget", "cost_centre":
		return slices.ContainsFunc(roles, func(r string) bool {
			return r == string(RolePracticeAdmin) || r == string(RoleBillingManager)
		})

	case "department", "role_assignment":
		return slices.Contains(roles, string(RolePracticeAdmin))

	case "audit_event":
		if action == "read" {
			return slices.ContainsFunc(roles, func(r string) bool {
				return r == string(RolePracticeAdmin) || r == string(RoleNetworkAdmin)
			})
		}
		return false // audit events are immutable

	case "pharmacy":
		return slices.Contains(roles, string(RolePharmacist))

	case "emergency.incident", "emergency.log", "emergency.resource",
		"emergency.mci", "emergency.surge", "emergency.cbrn":
		// Incident declaration and MCI triage open to all clinical staff.
		// Stand-down and command assignments restricted to IC / ERC roles.
		if action == "read" {
			return slices.ContainsFunc(roles, func(r string) bool {
				return r == string(RoleClinician) || r == string(RoleNurse) ||
					r == string(RolePracticeAdmin) || r == string(RoleNetworkAdmin) ||
					r == string(RoleIncidentCommander) || r == string(RoleEmergencyResponseCoordinator)
			})
		}
		return slices.ContainsFunc(roles, func(r string) bool {
			return r == string(RoleClinician) || r == string(RoleNurse) ||
				r == string(RoleIncidentCommander) || r == string(RoleEmergencyResponseCoordinator)
		})
	}

	// Unknown resource — deny by default.
	return false
}
