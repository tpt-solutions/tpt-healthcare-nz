// Package ipayroll implements payroll.Provider for the iPayroll REST API.
// iPayroll is a NZ-based payroll system with strong uptake in the health sector.
// API docs: https://developer.ipayroll.co.nz
package ipayroll

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"

	"github.com/PhillipC05/tpt-healthcare/core/payroll"
	"github.com/PhillipC05/tpt-healthcare/core/resilience"
)

func init() {
	payroll.Register("ipayroll", func(ctx context.Context, v *viper.Viper) (payroll.Provider, error) {
		return New(ctx, Config{
			Username: v.GetString("payroll.ipayroll.username"),
			APIKey:   v.GetString("payroll.ipayroll.api_key"),
			BaseURL:  v.GetString("payroll.ipayroll.base_url"),
		})
	})
}

const defaultBaseURL = "https://api.ipayroll.co.nz/api/v1"

// Config holds iPayroll API credentials.
type Config struct {
	Username string
	APIKey   string
	BaseURL  string
}

// Provider implements payroll.Provider for iPayroll.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.Username == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("ipayroll: username and api_key are required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "ipayroll"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("ipayroll marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	var result []byte
	err := resilience.Do(ctx, p.breaker, "ipayroll", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, method, p.cfg.BaseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("ipayroll request: %w", err)
		}
		req.SetBasicAuth(p.cfg.Username, p.cfg.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("ipayroll http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("ipayroll read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &payroll.Error{
				Provider:  "ipayroll",
				Code:      fmt.Sprintf("HTTP%d", resp.StatusCode),
				Message:   string(result),
				Retryable: resp.StatusCode >= 500,
			}
		}
		return nil
	})
	return result, err
}

func (p *Provider) SyncEmployee(ctx context.Context, emp payroll.Employee) (*payroll.SyncEmployeeResult, error) {
	payload := map[string]any{
		"name":           emp.Name,
		"email":          emp.Email,
		"employmentType": string(emp.EmploymentType),
		"startDate":      emp.StartDate.Format("2006-01-02"),
		"hourlyRate":     fmt.Sprintf("%.2f", float64(emp.PayRateCents)/100),
		"taxCode":        emp.TaxCode,
		"kiwiSaverRate":  fmt.Sprintf("%.2f", emp.KiwiSaverRate),
	}
	method, path := http.MethodPost, "/employees"
	if emp.ExternalID != "" {
		method, path = http.MethodPatch, "/employees/"+emp.ExternalID
	}
	raw, err := p.do(ctx, method, path, payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		EmployeeID string `json:"employeeId"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("ipayroll employee decode: %w", err)
	}
	return &payroll.SyncEmployeeResult{ExternalID: resp.EmployeeID, Created: emp.ExternalID == ""}, nil
}

func (p *Provider) PushTimesheets(ctx context.Context, shifts []payroll.Shift) error {
	entries := make([]map[string]any, len(shifts))
	for i, s := range shifts {
		entries[i] = map[string]any{
			"employeeId": s.ExternalEmployeeID,
			"date":       s.Start.Format("2006-01-02"),
			"startTime":  s.Start.Format("15:04"),
			"endTime":    s.End.Format("15:04"),
			"type":       string(s.Type),
		}
	}
	_, err := p.do(ctx, http.MethodPost, "/timesheets/bulk", map[string]any{"entries": entries})
	return err
}

func (p *Provider) GetPayslips(ctx context.Context, externalEmployeeID, period string) ([]payroll.Payslip, error) {
	raw, err := p.do(ctx, http.MethodGet, fmt.Sprintf("/employees/%s/payslips?period=%s", externalEmployeeID, period), nil)
	if err != nil {
		return nil, err
	}
	var resp []struct {
		ID          string  `json:"id"`
		PeriodStart string  `json:"periodStart"`
		PeriodEnd   string  `json:"periodEnd"`
		GrossPay    float64 `json:"grossPay"`
		NetPay      float64 `json:"netPay"`
		PAYE        float64 `json:"paye"`
		KiwiSaver   float64 `json:"kiwiSaver"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("ipayroll payslips decode: %w", err)
	}
	slips := make([]payroll.Payslip, len(resp))
	for i, r := range resp {
		start, _ := time.Parse("2006-01-02", r.PeriodStart)
		end, _ := time.Parse("2006-01-02", r.PeriodEnd)
		slips[i] = payroll.Payslip{
			ExternalID:     r.ID,
			PeriodStart:    start,
			PeriodEnd:      end,
			GrossPayCents:  int64(r.GrossPay * 100),
			NetPayCents:    int64(r.NetPay * 100),
			TaxCents:       int64(r.PAYE * 100),
			KiwiSaverCents: int64(r.KiwiSaver * 100),
		}
	}
	return slips, nil
}

func (p *Provider) GetLeaveBalance(ctx context.Context, externalEmployeeID string) (*payroll.LeaveBalance, error) {
	raw, err := p.do(ctx, http.MethodGet, "/employees/"+externalEmployeeID+"/leaveBalances", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		AnnualLeave float64 `json:"annualLeaveHours"`
		SickLeave   float64 `json:"sickLeaveHours"`
		LieuLeave   float64 `json:"lieuLeaveHours"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("ipayroll leave balance decode: %w", err)
	}
	return &payroll.LeaveBalance{
		ExternalEmployeeID: externalEmployeeID,
		AsAt:               time.Now().UTC(),
		AnnualHours:        int64(resp.AnnualLeave),
		SickHours:          int64(resp.SickLeave),
		LieuHours:          int64(resp.LieuLeave),
	}, nil
}

func (p *Provider) SubmitLeaveRequest(ctx context.Context, req payroll.LeaveRequest) (string, error) {
	payload := map[string]any{
		"employeeId": req.ExternalEmployeeID,
		"leaveType":  string(req.Type),
		"startDate":  req.Start.Format("2006-01-02"),
		"endDate":    req.End.Format("2006-01-02"),
		"notes":      req.Notes,
	}
	raw, err := p.do(ctx, http.MethodPost, "/leaveRequests", payload)
	if err != nil {
		return "", err
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("ipayroll leave request decode: %w", err)
	}
	return resp.ID, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*payroll.HealthStatus, error) {
	start := time.Now()
	_, err := p.do(ctx, http.MethodGet, "/health", nil)
	latency := time.Since(start)
	if err != nil {
		return &payroll.HealthStatus{OK: false, Provider: "ipayroll", Latency: latency, Err: err.Error()}, nil
	}
	return &payroll.HealthStatus{OK: true, Provider: "ipayroll", Latency: latency}, nil
}
