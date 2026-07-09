package mock

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/PhillipC05/tpt-healthcare/core/rbac"
)

// Repository is an in-memory fake implementing rbac.Repository for unit tests.
type Repository struct {
	mu          sync.Mutex
	departments map[uuid.UUID]rbac.Department
	assignments map[uuid.UUID]rbac.RoleAssignment
	byPrincipal map[string][]rbac.RoleAssignment
}

// NewRepository creates a new in-memory rbac.Repository.
func NewRepository() *Repository {
	return &Repository{
		departments: make(map[uuid.UUID]rbac.Department),
		assignments: make(map[uuid.UUID]rbac.RoleAssignment),
		byPrincipal: make(map[string][]rbac.RoleAssignment),
	}
}

func (r *Repository) CreateDepartment(_ context.Context, dept rbac.Department) (*rbac.Department, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	dept.ID = uuid.New()
	r.departments[dept.ID] = dept
	return &dept, nil
}

func (r *Repository) ListDepartments(_ context.Context, _ uuid.UUID) ([]rbac.Department, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []rbac.Department
	for _, d := range r.departments {
		out = append(out, d)
	}
	return out, nil
}

func (r *Repository) GetDepartment(_ context.Context, id uuid.UUID) (*rbac.Department, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.departments[id]
	if !ok {
		return nil, nil
	}
	return &d, nil
}

func (r *Repository) UpdateDepartment(_ context.Context, dept rbac.Department) (*rbac.Department, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.departments[dept.ID] = dept
	return &dept, nil
}

func (r *Repository) DeleteDepartment(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.departments, id)
	return nil
}

func (r *Repository) GrantRole(_ context.Context, assignment rbac.RoleAssignment) (*rbac.RoleAssignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	assignment.ID = uuid.New()
	r.assignments[assignment.ID] = assignment
	r.byPrincipal[assignment.PrincipalID] = append(r.byPrincipal[assignment.PrincipalID], assignment)
	return &assignment, nil
}

func (r *Repository) RevokeRole(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.assignments[id]; ok {
		now := time.Now().UTC()
		a.RevokedAt = &now
		r.assignments[id] = a
	}
	return nil
}

func (r *Repository) ListAssignments(_ context.Context, _ uuid.UUID, principalID string) ([]rbac.RoleAssignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byPrincipal[principalID], nil
}

func (r *Repository) ListAssignmentsByTenant(_ context.Context, _ uuid.UUID) ([]rbac.RoleAssignment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []rbac.RoleAssignment
	for _, a := range r.assignments {
		out = append(out, a)
	}
	return out, nil
}

func (r *Repository) AssignmentsByPrincipalAndDept(_ context.Context, _ uuid.UUID, principalID string, _ []uuid.UUID) ([]rbac.RoleAssignment, error) {
	return r.ListAssignments(context.Background(), uuid.UUID{}, principalID)
}
