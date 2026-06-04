package rbac

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines persistence for RBAC data.
type Repository interface {
	// CreateDepartment inserts a new department.
	CreateDepartment(ctx context.Context, dept Department) (*Department, error)

	// ListDepartments returns all active departments for a tenant.
	ListDepartments(ctx context.Context, tenantID uuid.UUID) ([]Department, error)

	// GetDepartment returns a single department by ID.
	GetDepartment(ctx context.Context, id uuid.UUID) (*Department, error)

	// UpdateDepartment updates a department's name, code, or parent.
	UpdateDepartment(ctx context.Context, dept Department) (*Department, error)

	// DeleteDepartment soft-deletes a department.
	DeleteDepartment(ctx context.Context, id uuid.UUID) error

	// GrantRole creates a role assignment. If an identical active assignment
	// already exists it is returned without creating a duplicate.
	GrantRole(ctx context.Context, assignment RoleAssignment) (*RoleAssignment, error)

	// RevokeRole sets revoked_at on the assignment.
	RevokeRole(ctx context.Context, id uuid.UUID) error

	// ListAssignments returns all active role assignments for a principal.
	ListAssignments(ctx context.Context, tenantID uuid.UUID, principalID string) ([]RoleAssignment, error)

	// ListAssignmentsByTenant returns all active role assignments in a tenant
	// (used by the tpt-admin role management UI).
	ListAssignmentsByTenant(ctx context.Context, tenantID uuid.UUID) ([]RoleAssignment, error)

	// AssignmentsByPrincipalAndDept returns assignments for a principal that
	// are either tenant-wide or scoped to one of the given department IDs.
	AssignmentsByPrincipalAndDept(ctx context.Context, tenantID uuid.UUID, principalID string, deptIDs []uuid.UUID) ([]RoleAssignment, error)
}
