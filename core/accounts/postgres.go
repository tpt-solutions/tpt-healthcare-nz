package accounts

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements Repository.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository constructs a PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateCostCentre(ctx context.Context, cc CostCentre) (*CostCentre, error) {
	const q = `
		INSERT INTO cost_centres (id, tenant_id, department_id, name, code)
		VALUES (gen_random_uuid(), @tenant_id, @department_id, @name, @code)
		RETURNING id, tenant_id, department_id, name, code, created_at`
	row := r.pool.QueryRow(ctx, q, map[string]any{
		"tenant_id": cc.TenantID, "department_id": cc.DepartmentID,
		"name": cc.Name, "code": cc.Code,
	})
	return scanCostCentre(row)
}

func (r *PostgresRepository) ListCostCentres(ctx context.Context, tenantID uuid.UUID) ([]CostCentre, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, tenant_id, department_id, name, code, created_at FROM cost_centres WHERE tenant_id = @tenant_id ORDER BY name`,
		map[string]any{"tenant_id": tenantID})
	if err != nil {
		return nil, fmt.Errorf("accounts list cost centres: %w", err)
	}
	defer rows.Close()
	var result []CostCentre
	for rows.Next() {
		cc, err := scanCostCentre(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *cc)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) GetCostCentre(ctx context.Context, id uuid.UUID) (*CostCentre, error) {
	return scanCostCentre(r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, department_id, name, code, created_at FROM cost_centres WHERE id = @id`,
		map[string]any{"id": id}))
}

func (r *PostgresRepository) UpdateCostCentre(ctx context.Context, cc CostCentre) (*CostCentre, error) {
	const q = `
		UPDATE cost_centres SET name = @name, code = @code, department_id = @department_id
		WHERE id = @id
		RETURNING id, tenant_id, department_id, name, code, created_at`
	return scanCostCentre(r.pool.QueryRow(ctx, q, map[string]any{
		"id": cc.ID, "name": cc.Name, "code": cc.Code, "department_id": cc.DepartmentID,
	}))
}

func (r *PostgresRepository) CreateBudget(ctx context.Context, b Budget) (*Budget, error) {
	const q = `
		INSERT INTO budgets (id, tenant_id, cost_centre_id, financial_year)
		VALUES (gen_random_uuid(), @tenant_id, @cost_centre_id, @financial_year)
		RETURNING id, tenant_id, cost_centre_id, financial_year, created_at`
	row := r.pool.QueryRow(ctx, q, map[string]any{
		"tenant_id": b.TenantID, "cost_centre_id": b.CostCentreID, "financial_year": b.FinancialYear,
	})
	var out Budget
	if err := row.Scan(&out.ID, &out.TenantID, &out.CostCentreID, &out.FinancialYear, &out.CreatedAt); err != nil {
		return nil, fmt.Errorf("accounts create budget: %w", err)
	}
	return &out, nil
}

func (r *PostgresRepository) GetBudget(ctx context.Context, costCentreID uuid.UUID, financialYear int) (*Budget, error) {
	var b Budget
	err := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, cost_centre_id, financial_year, created_at FROM budgets WHERE cost_centre_id = @cc AND financial_year = @fy`,
		map[string]any{"cc": costCentreID, "fy": financialYear},
	).Scan(&b.ID, &b.TenantID, &b.CostCentreID, &b.FinancialYear, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("accounts get budget: %w", err)
	}
	lines, err := r.listLines(ctx, b.ID)
	if err != nil {
		return nil, err
	}
	b.Lines = lines
	return &b, nil
}

func (r *PostgresRepository) UpsertBudgetLine(ctx context.Context, line BudgetLine) (*BudgetLine, error) {
	const q = `
		INSERT INTO budget_lines (id, budget_id, month, category, planned_cents, actual_cents, external_ref)
		VALUES (gen_random_uuid(), @budget_id, @month, @category, @planned_cents, 0, @external_ref)
		ON CONFLICT (budget_id, month, category) DO UPDATE SET
			planned_cents = EXCLUDED.planned_cents,
			external_ref  = EXCLUDED.external_ref
		RETURNING id, budget_id, month, category, planned_cents, actual_cents, external_ref`
	row := r.pool.QueryRow(ctx, q, map[string]any{
		"budget_id": line.BudgetID, "month": line.Month, "category": line.Category,
		"planned_cents": line.PlannedCents, "external_ref": line.ExternalRef,
	})
	return scanBudgetLine(row)
}

func (r *PostgresRepository) UpdateActuals(ctx context.Context, budgetID uuid.UUID, month int, category string, actualCents int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE budget_lines SET actual_cents = @actual WHERE budget_id = @bid AND month = @month AND category = @category`,
		map[string]any{"actual": actualCents, "bid": budgetID, "month": month, "category": category})
	return err
}

func (r *PostgresRepository) VarianceReport(ctx context.Context, costCentreID uuid.UUID, financialYear, asAtMonth int) (*VarianceReport, error) {
	budget, err := r.GetBudget(ctx, costCentreID, financialYear)
	if err != nil {
		return nil, err
	}
	cc, err := r.GetCostCentre(ctx, costCentreID)
	if err != nil {
		return nil, err
	}
	report := &VarianceReport{
		CostCentreID:   costCentreID,
		CostCentreName: cc.Name,
		FinancialYear:  financialYear,
		AsAtMonth:      asAtMonth,
	}
	for _, l := range budget.Lines {
		if l.Month > asAtMonth {
			continue
		}
		variance := l.ActualCents - l.PlannedCents
		pct := 0.0
		if l.PlannedCents != 0 {
			pct = float64(variance) / float64(l.PlannedCents) * 100
		}
		report.Lines = append(report.Lines, VarianceLine{
			Month:         l.Month,
			Category:      l.Category,
			PlannedCents:  l.PlannedCents,
			ActualCents:   l.ActualCents,
			VarianceCents: variance,
			VariancePct:   pct,
		})
		report.TotalPlanned += l.PlannedCents
		report.TotalActual += l.ActualCents
	}
	report.TotalVariance = report.TotalActual - report.TotalPlanned
	return report, nil
}

func (r *PostgresRepository) listLines(ctx context.Context, budgetID uuid.UUID) ([]BudgetLine, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, budget_id, month, category, planned_cents, actual_cents, external_ref FROM budget_lines WHERE budget_id = @id ORDER BY month, category`,
		map[string]any{"id": budgetID})
	if err != nil {
		return nil, fmt.Errorf("accounts list budget lines: %w", err)
	}
	defer rows.Close()
	var result []BudgetLine
	for rows.Next() {
		l, err := scanBudgetLine(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *l)
	}
	return result, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanCostCentre(s scanner) (*CostCentre, error) {
	var cc CostCentre
	if err := s.Scan(&cc.ID, &cc.TenantID, &cc.DepartmentID, &cc.Name, &cc.Code, &cc.CreatedAt); err != nil {
		return nil, fmt.Errorf("accounts scan cost centre: %w", err)
	}
	return &cc, nil
}

func scanBudgetLine(s scanner) (*BudgetLine, error) {
	var l BudgetLine
	if err := s.Scan(&l.ID, &l.BudgetID, &l.Month, &l.Category, &l.PlannedCents, &l.ActualCents, &l.ExternalRef); err != nil {
		return nil, fmt.Errorf("accounts scan budget line: %w", err)
	}
	return &l, nil
}
