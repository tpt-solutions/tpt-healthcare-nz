package accounts

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines persistence for cost-centre and budget data.
type Repository interface {
	// Cost centres
	CreateCostCentre(ctx context.Context, cc CostCentre) (*CostCentre, error)
	ListCostCentres(ctx context.Context, tenantID uuid.UUID) ([]CostCentre, error)
	GetCostCentre(ctx context.Context, id uuid.UUID) (*CostCentre, error)
	UpdateCostCentre(ctx context.Context, cc CostCentre) (*CostCentre, error)

	// Budgets
	CreateBudget(ctx context.Context, b Budget) (*Budget, error)
	GetBudget(ctx context.Context, costCentreID uuid.UUID, financialYear int) (*Budget, error)
	UpsertBudgetLine(ctx context.Context, line BudgetLine) (*BudgetLine, error)
	// UpdateActuals is called during the accounting sync job to refresh actual spend.
	UpdateActuals(ctx context.Context, budgetID uuid.UUID, month int, category string, actualCents int64) error

	// Reports
	VarianceReport(ctx context.Context, costCentreID uuid.UUID, financialYear, asAtMonth int) (*VarianceReport, error)
}
