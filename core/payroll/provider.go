// Package payroll defines the PayrollProvider interface for outsourced payroll
// integrations. The platform pushes timesheets and leave requests; the payroll
// system is the source of truth for pay calculations, tax, and KiwiSaver.
// Large organisations (hospitals) that run their own payroll teams still use
// one of these systems — this interface accommodates all scales.
package payroll

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// EmploymentType classifies the employment arrangement.
type EmploymentType string

const (
	EmploymentFull    EmploymentType = "full_time"
	EmploymentPart    EmploymentType = "part_time"
	EmploymentCasual  EmploymentType = "casual"
	EmploymentContract EmploymentType = "contract"
)

// ShiftType classifies the nature of a worked shift for payroll calculation.
type ShiftType string

const (
	ShiftOrdinary ShiftType = "ordinary"
	ShiftOnCall   ShiftType = "on_call"
	ShiftOvertime ShiftType = "overtime"
)

// Employee is the normalised employee record pushed to the payroll system.
type Employee struct {
	ExternalID     string         `json:"external_id,omitempty"` // payroll system ID; empty on first sync
	Name           string         `json:"name"`
	Email          string         `json:"email"`
	HPICPN         string         `json:"hpi_cpn,omitempty"` // Health Provider Index Common Person Number
	EmploymentType EmploymentType `json:"employment_type"`
	StartDate      time.Time      `json:"start_date"`
	DepartmentIDs  []string       `json:"department_ids,omitempty"`
	// PayRateCents is the hourly rate in NZD cents.
	PayRateCents int64  `json:"pay_rate_cents"`
	// TaxCode is the NZ IR330 tax code (e.g. "M", "ME", "SH", "SL").
	TaxCode      string `json:"tax_code"`
	KiwiSaverRate float64 `json:"kiwisaver_rate"` // employee contribution rate, e.g. 0.03
}

// SyncEmployeeResult is returned by SyncEmployee.
type SyncEmployeeResult struct {
	ExternalID string `json:"external_id"`
	Created    bool   `json:"created"`
}

// Shift is a single worked period to be submitted as a timesheet entry.
type Shift struct {
	ExternalEmployeeID string    `json:"external_employee_id"`
	Start              time.Time `json:"start"`
	End                time.Time `json:"end"`
	DepartmentID       string    `json:"department_id,omitempty"`
	Type               ShiftType `json:"type"`
	Notes              string    `json:"notes,omitempty"`
}

// Payslip is a single pay period summary retrieved from the payroll system.
type Payslip struct {
	ExternalID       string    `json:"external_id"`
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
	GrossPayCents    int64     `json:"gross_pay_cents"`
	NetPayCents      int64     `json:"net_pay_cents"`
	TaxCents         int64     `json:"tax_cents"`
	KiwiSaverCents   int64     `json:"kiwisaver_cents"`
	DownloadURL      string    `json:"download_url,omitempty"`
}

// LeaveType classifies the leave category.
type LeaveType string

const (
	LeaveAnnual   LeaveType = "annual"
	LeaveSick     LeaveType = "sick"
	LeaveLieu     LeaveType = "lieu"
	LeaveBereavement LeaveType = "bereavement"
	LeaveParental LeaveType = "parental"
	LeaveOther    LeaveType = "other"
)

// LeaveBalance holds an employee's remaining leave entitlements.
// All values are in hours (int64) to avoid float rounding errors.
type LeaveBalance struct {
	ExternalEmployeeID string    `json:"external_employee_id"`
	AsAt               time.Time `json:"as_at"`
	AnnualHours        int64     `json:"annual_hours"`
	SickHours          int64     `json:"sick_hours"`
	LieuHours          int64     `json:"lieu_hours"`
	OtherHours         int64     `json:"other_hours"`
}

// LeaveRequest is a leave application to be submitted to the payroll system.
type LeaveRequest struct {
	ExternalEmployeeID string    `json:"external_employee_id"`
	Type               LeaveType `json:"type"`
	Start              time.Time `json:"start"`
	End                time.Time `json:"end"`
	Notes              string    `json:"notes,omitempty"`
}

// HealthStatus reports the result of a provider connectivity check.
type HealthStatus struct {
	OK           bool          `json:"ok"`
	Provider     string        `json:"provider"`
	Organisation string        `json:"organisation,omitempty"`
	Latency      time.Duration `json:"latency_ms"`
	Err          string        `json:"error,omitempty"`
}

// Error is a payroll provider application-level error.
type Error struct {
	Provider  string
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("payroll(%s): %s — %s", e.Provider, e.Code, e.Message)
}

// Provider is the interface all payroll backends must implement.
type Provider interface {
	// SyncEmployee upserts an employee in the payroll system.
	// Set ExternalID on the employee to update an existing record.
	SyncEmployee(ctx context.Context, emp Employee) (*SyncEmployeeResult, error)

	// PushTimesheets submits a batch of completed shifts as timesheet entries.
	PushTimesheets(ctx context.Context, shifts []Shift) error

	// GetPayslips returns payslips for the given employee and pay period.
	// period format: "2026-05" (YYYY-MM).
	GetPayslips(ctx context.Context, externalEmployeeID, period string) ([]Payslip, error)

	// GetLeaveBalance returns the employee's current leave entitlements.
	GetLeaveBalance(ctx context.Context, externalEmployeeID string) (*LeaveBalance, error)

	// SubmitLeaveRequest submits a leave application and returns the payroll system's leave ID.
	SubmitLeaveRequest(ctx context.Context, req LeaveRequest) (externalLeaveID string, err error)

	// HealthCheck verifies connectivity and authentication.
	HealthCheck(ctx context.Context) (*HealthStatus, error)
}

// Factory is a constructor function registered by each backend.
type Factory func(ctx context.Context, v *viper.Viper) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Factory)
)

// Register associates name with factory. Called from each backend's init().
func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("payroll: provider %q already registered", name))
	}
	registry[name] = factory
}

// New instantiates the provider named by viper key "payroll.provider".
func New(ctx context.Context, v *viper.Viper) (Provider, error) {
	name := v.GetString("payroll.provider")
	if name == "" {
		return nil, errors.New("payroll: payroll.provider config key is not set")
	}
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, name)
	}
	return factory(ctx, v)
}

// ErrUnknownProvider is returned when no registered backend matches.
var ErrUnknownProvider = errors.New("payroll: unknown provider")
