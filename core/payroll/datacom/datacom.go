// Package datacom implements payroll.Provider for Datacom Preceda.
// Datacom is an enterprise-scale payroll system widely used by NZ hospitals
// and large DHB-aligned health organisations.
// API: Datacom Preceda REST API (OAuth2 client credentials).
package datacom

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
	payroll.Register("datacom", func(ctx context.Context, v *viper.Viper) (payroll.Provider, error) {
		return New(ctx, Config{
			ClientID:     v.GetString("payroll.datacom.client_id"),
			ClientSecret: v.GetString("payroll.datacom.client_secret"),
			TenantCode:   v.GetString("payroll.datacom.tenant_code"),
			BaseURL:      v.GetString("payroll.datacom.base_url"),
		})
	})
}

// Config holds Datacom Preceda credentials.
type Config struct {
	ClientID     string
	ClientSecret string
	TenantCode   string // Datacom organisation code
	BaseURL      string
}

// Provider implements payroll.Provider for Datacom Preceda.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("datacom: client_id and client_secret are required")
	}
	if cfg.TenantCode == "" {
		return nil, fmt.Errorf("datacom: tenant_code (Datacom organisation code) is required")
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("datacom: base_url is required (provided by Datacom for your organisation)")
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "datacom"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("datacom marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	var result []byte
	err := resilience.Do(ctx, p.breaker, "datacom", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, method, p.cfg.BaseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("datacom request: %w", err)
		}
		req.Header.Set("X-Tenant-Code", p.cfg.TenantCode)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		// TODO: attach OAuth2 client credentials bearer token.

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("datacom http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("datacom read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &payroll.Error{
				Provider:  "datacom",
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
		"FullName":       emp.Name,
		"Email":          emp.Email,
		"EmploymentBasis": string(emp.EmploymentType),
		"StartDate":      emp.StartDate.Format("2006-01-02"),
		"HourlyRate":     fmt.Sprintf("%.4f", float64(emp.PayRateCents)/100),
		"TaxCode":        emp.TaxCode,
		"KiwiSaverRate":  fmt.Sprintf("%.2f", emp.KiwiSaverRate),
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
		EmployeeCode string `json:"EmployeeCode"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("datacom employee decode: %w", err)
	}
	return &payroll.SyncEmployeeResult{ExternalID: resp.EmployeeCode, Created: emp.ExternalID == ""}, nil
}

func (p *Provider) PushTimesheets(ctx context.Context, shifts []payroll.Shift) error {
	entries := make([]map[string]any, len(shifts))
	for i, s := range shifts {
		entries[i] = map[string]any{
			"EmployeeCode": s.ExternalEmployeeID,
			"WorkDate":     s.Start.Format("2006-01-02"),
			"StartTime":    s.Start.Format("15:04"),
			"EndTime":      s.End.Format("15:04"),
			"PayType":      string(s.Type),
		}
	}
	_, err := p.do(ctx, http.MethodPost, "/timesheets/import", map[string]any{"Entries": entries})
	return err
}

func (p *Provider) GetPayslips(ctx context.Context, externalEmployeeID, period string) ([]payroll.Payslip, error) {
	raw, err := p.do(ctx, http.MethodGet, fmt.Sprintf("/employees/%s/payslips?period=%s", externalEmployeeID, period), nil)
	if err != nil {
		return nil, err
	}
	var resp []struct {
		PayslipID   string  `json:"PayslipID"`
		PeriodStart string  `json:"PeriodStart"`
		PeriodEnd   string  `json:"PeriodEnd"`
		GrossPay    float64 `json:"GrossPay"`
		NetPay      float64 `json:"NetPay"`
		PAYE        float64 `json:"PAYE"`
		KiwiSaver   float64 `json:"KiwiSaver"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("datacom payslips decode: %w", err)
	}
	slips := make([]payroll.Payslip, len(resp))
	for i, r := range resp {
		start, _ := time.Parse("2006-01-02", r.PeriodStart)
		end, _ := time.Parse("2006-01-02", r.PeriodEnd)
		slips[i] = payroll.Payslip{
			ExternalID:     r.PayslipID,
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
	raw, err := p.do(ctx, http.MethodGet, "/employees/"+externalEmployeeID+"/leave-balance", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		AnnualLeaveHours float64 `json:"AnnualLeaveHours"`
		SickLeaveHours   float64 `json:"SickLeaveHours"`
		LieuLeaveHours   float64 `json:"LieuLeaveHours"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("datacom leave balance decode: %w", err)
	}
	return &payroll.LeaveBalance{
		ExternalEmployeeID: externalEmployeeID,
		AsAt:               time.Now().UTC(),
		AnnualHours:        int64(resp.AnnualLeaveHours),
		SickHours:          int64(resp.SickLeaveHours),
		LieuHours:          int64(resp.LieuLeaveHours),
	}, nil
}

func (p *Provider) SubmitLeaveRequest(ctx context.Context, req payroll.LeaveRequest) (string, error) {
	payload := map[string]any{
		"EmployeeCode": req.ExternalEmployeeID,
		"LeaveType":    string(req.Type),
		"StartDate":    req.Start.Format("2006-01-02"),
		"EndDate":      req.End.Format("2006-01-02"),
		"Notes":        req.Notes,
	}
	raw, err := p.do(ctx, http.MethodPost, "/leave-requests", payload)
	if err != nil {
		return "", err
	}
	var resp struct {
		LeaveRequestID string `json:"LeaveRequestID"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("datacom leave request decode: %w", err)
	}
	return resp.LeaveRequestID, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*payroll.HealthStatus, error) {
	start := time.Now()
	_, err := p.do(ctx, http.MethodGet, "/health", nil)
	latency := time.Since(start)
	if err != nil {
		return &payroll.HealthStatus{OK: false, Provider: "datacom", Latency: latency, Err: err.Error()}, nil
	}
	return &payroll.HealthStatus{OK: true, Provider: "datacom", Latency: latency}, nil
}
