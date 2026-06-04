package rbac

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements Repository against a PostgreSQL pool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository constructs a PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateDepartment(ctx context.Context, dept Department) (*Department, error) {
	const q = `
		INSERT INTO departments (id, tenant_id, name, code, parent_id)
		VALUES (gen_random_uuid(), @tenant_id, @name, @code, @parent_id)
		RETURNING id, tenant_id, name, code, parent_id, created_at`
	row := r.pool.QueryRow(ctx, q, map[string]any{
		"tenant_id": dept.TenantID,
		"name":      dept.Name,
		"code":      dept.Code,
		"parent_id": dept.ParentID,
	})
	return scanDepartment(row)
}

func (r *PostgresRepository) ListDepartments(ctx context.Context, tenantID uuid.UUID) ([]Department, error) {
	const q = `
		SELECT id, tenant_id, name, code, parent_id, created_at
		FROM departments
		WHERE tenant_id = @tenant_id AND deleted_at IS NULL
		ORDER BY name`
	rows, err := r.pool.Query(ctx, q, map[string]any{"tenant_id": tenantID})
	if err != nil {
		return nil, fmt.Errorf("rbac list departments: %w", err)
	}
	defer rows.Close()
	var depts []Department
	for rows.Next() {
		d, err := scanDepartment(rows)
		if err != nil {
			return nil, err
		}
		depts = append(depts, *d)
	}
	return depts, rows.Err()
}

func (r *PostgresRepository) GetDepartment(ctx context.Context, id uuid.UUID) (*Department, error) {
	const q = `
		SELECT id, tenant_id, name, code, parent_id, created_at
		FROM departments WHERE id = @id AND deleted_at IS NULL`
	return scanDepartment(r.pool.QueryRow(ctx, q, map[string]any{"id": id}))
}

func (r *PostgresRepository) UpdateDepartment(ctx context.Context, dept Department) (*Department, error) {
	const q = `
		UPDATE departments SET name = @name, code = @code, parent_id = @parent_id
		WHERE id = @id AND deleted_at IS NULL
		RETURNING id, tenant_id, name, code, parent_id, created_at`
	return scanDepartment(r.pool.QueryRow(ctx, q, map[string]any{
		"id": dept.ID, "name": dept.Name, "code": dept.Code, "parent_id": dept.ParentID,
	}))
}

func (r *PostgresRepository) DeleteDepartment(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE departments SET deleted_at = NOW() WHERE id = @id`,
		map[string]any{"id": id})
	return err
}

func (r *PostgresRepository) GrantRole(ctx context.Context, a RoleAssignment) (*RoleAssignment, error) {
	const q = `
		INSERT INTO role_assignments (id, tenant_id, principal_id, role, department_id, granted_by)
		VALUES (gen_random_uuid(), @tenant_id, @principal_id, @role, @department_id, @granted_by)
		ON CONFLICT (tenant_id, principal_id, role, COALESCE(department_id, '00000000-0000-0000-0000-000000000000'))
		  WHERE revoked_at IS NULL
		DO UPDATE SET granted_by = EXCLUDED.granted_by
		RETURNING id, tenant_id, principal_id, role, department_id, granted_by, created_at, revoked_at`
	return scanAssignment(r.pool.QueryRow(ctx, q, map[string]any{
		"tenant_id":     a.TenantID,
		"principal_id":  a.PrincipalID,
		"role":          a.Role,
		"department_id": a.DepartmentID,
		"granted_by":    a.GrantedBy,
	}))
}

func (r *PostgresRepository) RevokeRole(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE role_assignments SET revoked_at = NOW() WHERE id = @id`,
		map[string]any{"id": id})
	return err
}

func (r *PostgresRepository) ListAssignments(ctx context.Context, tenantID uuid.UUID, principalID string) ([]RoleAssignment, error) {
	return r.queryAssignments(ctx, `
		WHERE tenant_id = @tenant_id AND principal_id = @principal_id AND revoked_at IS NULL`,
		map[string]any{"tenant_id": tenantID, "principal_id": principalID})
}

func (r *PostgresRepository) ListAssignmentsByTenant(ctx context.Context, tenantID uuid.UUID) ([]RoleAssignment, error) {
	return r.queryAssignments(ctx,
		`WHERE tenant_id = @tenant_id AND revoked_at IS NULL`,
		map[string]any{"tenant_id": tenantID})
}

func (r *PostgresRepository) AssignmentsByPrincipalAndDept(ctx context.Context, tenantID uuid.UUID, principalID string, deptIDs []uuid.UUID) ([]RoleAssignment, error) {
	return r.queryAssignments(ctx,
		`WHERE tenant_id = @tenant_id AND principal_id = @principal_id AND revoked_at IS NULL
		   AND (department_id IS NULL OR department_id = ANY(@dept_ids))`,
		map[string]any{"tenant_id": tenantID, "principal_id": principalID, "dept_ids": deptIDs})
}

func (r *PostgresRepository) queryAssignments(ctx context.Context, where string, args map[string]any) ([]RoleAssignment, error) {
	q := `SELECT id, tenant_id, principal_id, role, department_id, granted_by, created_at, revoked_at
	      FROM role_assignments ` + where
	rows, err := r.pool.Query(ctx, q, args)
	if err != nil {
		return nil, fmt.Errorf("rbac query assignments: %w", err)
	}
	defer rows.Close()
	var result []RoleAssignment
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *a)
	}
	return result, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanDepartment(s scanner) (*Department, error) {
	var d Department
	if err := s.Scan(&d.ID, &d.TenantID, &d.Name, &d.Code, &d.ParentID, &d.CreatedAt); err != nil {
		return nil, fmt.Errorf("rbac scan department: %w", err)
	}
	return &d, nil
}

func scanAssignment(s scanner) (*RoleAssignment, error) {
	var a RoleAssignment
	if err := s.Scan(&a.ID, &a.TenantID, &a.PrincipalID, &a.Role, &a.DepartmentID, &a.GrantedBy, &a.CreatedAt, &a.RevokedAt); err != nil {
		return nil, fmt.Errorf("rbac scan assignment: %w", err)
	}
	return &a, nil
}
