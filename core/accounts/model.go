// Package accounts provides lightweight internal GL primitives for operational
// budget visibility. This is NOT a full accounting system — GST, P&L, and
// balance sheets live in the external accounting provider (Xero/QBO/FreshBooks).
// accounts is for practice managers to track cost centres, set budgets, and
// view actual vs budget variance using data synced from the accounting provider.
package accounts

import (
	"time"

	"github.com/google/uuid"
)

// CostCentre maps to a practice department or functional area.
// It mirrors the rbac.Department concept but is specifically for financial reporting.
type CostCentre struct {
	ID           uuid.UUID  `db:"id"           json:"id"`
	TenantID     uuid.UUID  `db:"tenant_id"    json:"tenant_id"`
	DepartmentID *uuid.UUID `db:"department_id" json:"department_id,omitempty"` // links to rbac.Department
	Name         string     `db:"name"         json:"name"`
	Code         string     `db:"code"         json:"code"` // e.g. "GP-DEPT", "ADMIN"
	CreatedAt    time.Time  `db:"created_at"   json:"created_at"`
}

// Budget is an annual budget for a cost centre.
type Budget struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	TenantID     uuid.UUID `db:"tenant_id"     json:"tenant_id"`
	CostCentreID uuid.UUID `db:"cost_centre_id" json:"cost_centre_id"`
	FinancialYear int      `db:"financial_year" json:"financial_year"` // e.g. 2026
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	Lines        []BudgetLine `db:"-"           json:"lines,omitempty"`
}

// BudgetLine holds the monthly plan and actual for a single budget category.
// ActualCents is populated by syncing from the external accounting provider.
type BudgetLine struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	BudgetID     uuid.UUID `db:"budget_id"     json:"budget_id"`
	Month        int       `db:"month"         json:"month"`        // 1–12
	Category     string    `db:"category"      json:"category"`     // e.g. "Staff", "Supplies", "Rent"
	PlannedCents int64     `db:"planned_cents" json:"planned_cents"`
	ActualCents  int64     `db:"actual_cents"  json:"actual_cents"` // synced from accounting provider
	// ExternalRef is the accounting provider's account code used to pull actuals.
	ExternalRef  string    `db:"external_ref"  json:"external_ref,omitempty"`
}

// VarianceReport is computed from a Budget's lines for a given period.
type VarianceReport struct {
	CostCentreID   uuid.UUID         `json:"cost_centre_id"`
	CostCentreName string            `json:"cost_centre_name"`
	FinancialYear  int               `json:"financial_year"`
	AsAtMonth      int               `json:"as_at_month"`
	Lines          []VarianceLine    `json:"lines"`
	TotalPlanned   int64             `json:"total_planned_cents"`
	TotalActual    int64             `json:"total_actual_cents"`
	TotalVariance  int64             `json:"total_variance_cents"` // actual - planned
}

// VarianceLine is one row of the variance report.
type VarianceLine struct {
	Month          int    `json:"month"`
	Category       string `json:"category"`
	PlannedCents   int64  `json:"planned_cents"`
	ActualCents    int64  `json:"actual_cents"`
	VarianceCents  int64  `json:"variance_cents"` // actual - planned; negative = underspend
	VariancePct    float64 `json:"variance_pct"`  // VarianceCents / PlannedCents * 100
}
