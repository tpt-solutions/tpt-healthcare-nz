// Package flexitime implements payroll.Provider for the FlexiTime API.
// FlexiTime is a NZ-based workforce management and payroll system popular
// with small-medium healthcare providers.
// API docs: https://developers.flexitime.com
package flexitime

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
	payroll.Register("flexitime", func(ctx context.Context, v *viper.Viper) (payroll.Provider, error) {
		return New(ctx, Config{
			ClientID:     v.GetString("payroll.flexitime.client_id"),
			ClientSecret: v.GetString("payroll.flexitime.client_secret"),
			BaseURL:      v.GetString("payroll.flexitime.base_url"),
		})
	})
}

const defaultBaseURL = "https://api.flexitime.com/v1"

// Config holds FlexiTime OAuth2 credentials.
type Config struct {
	ClientID     string
	ClientSecret string
	BaseURL      string
}

// Provider implements payroll.Provider for FlexiTime.
type Provider struct {
	cfg     Config
	client  *http.Client
	breaker *resilience.Registry
}

// New validates config and constructs a Provider.
func New(_ context.Context, cfg Config) (*Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("flexitime: client_id and client_secret are required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	reg := resilience.NewRegistry()
	reg.Register(resilience.BreakerConfig{Name: "flexitime"})
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}, breaker: reg}, nil
}

func (p *Provider) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("flexitime marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	var result []byte
	err := resilience.Do(ctx, p.breaker, "flexitime", resilience.RetryConfig{MaxAttempts: 3}, func() error {
		req, err := http.NewRequestWithContext(ctx, method, p.cfg.BaseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("flexitime request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		// TODO: attach OAuth2 bearer token.

		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("flexitime http: %w", err)
		}
		defer resp.Body.Close()
		result, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("flexitime read: %w", err)
		}
		if resp.StatusCode >= 400 {
			return &payroll.Error{
				Provider:  "flexitime",
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
		"employment_basis": string(emp.EmploymentType),
		"start_date":     emp.StartDate.Format("2006-01-02"),
		"hourly_rate":    fmt.Sprintf("%.4f", float64(emp.PayRateCents)/100),
		"tax_code":       emp.TaxCode,
	}
	method, path := http.MethodPost, "/employees"
	if emp.ExternalID != "" {
		method, path = http.MethodPut, "/employees/"+emp.ExternalID
	}
	raw, err := p.do(ctx, method, path, payload)
	if err != nil {
		return nil, err
	}
	var resp struct {
		EmployeeID string `json:"employee_id"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("flexitime employee decode: %w", err)
	}
	return &payroll.SyncEmployeeResult{ExternalID: resp.EmployeeID, Created: emp.ExternalID == ""}, nil
}

func (p *Provider) PushTimesheets(ctx context.Context, shifts []payroll.Shift) error {
	entries := make([]map[string]any, len(shifts))
	for i, s := range shifts {
		entries[i] = map[string]any{
			"employee_id": s.ExternalEmployeeID,
			"date":        s.Start.Format("2006-01-02"),
			"start":       s.Start.Format("15:04"),
			"end":         s.End.Format("15:04"),
			"pay_type":    string(s.Type),
		}
	}
	_, err := p.do(ctx, http.MethodPost, "/timesheets", map[string]any{"timesheets": entries})
	return err
}

func (p *Provider) GetPayslips(ctx context.Context, externalEmployeeID, period string) ([]payroll.Payslip, error) {
	raw, err := p.do(ctx, http.MethodGet, fmt.Sprintf("/employees/%s/payslips?period=%s", externalEmployeeID, period), nil)
	if err != nil {
		return nil, err
	}
	var resp []struct {
		ID          string  `json:"id"`
		PeriodStart string  `json:"period_start"`
		PeriodEnd   string  `json:"period_end"`
		GrossPay    float64 `json:"gross_pay"`
		NetPay      float64 `json:"net_pay"`
		Tax         float64 `json:"tax"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("flexitime payslips decode: %w", err)
	}
	slips := make([]payroll.Payslip, len(resp))
	for i, r := range resp {
		start, _ := time.Parse("2006-01-02", r.PeriodStart)
		end, _ := time.Parse("2006-01-02", r.PeriodEnd)
		slips[i] = payroll.Payslip{
			ExternalID:    r.ID,
			PeriodStart:   start,
			PeriodEnd:     end,
			GrossPayCents: int64(r.GrossPay * 100),
			NetPayCents:   int64(r.NetPay * 100),
			TaxCents:      int64(r.Tax * 100),
		}
	}
	return slips, nil
}

func (p *Provider) GetLeaveBalance(ctx context.Context, externalEmployeeID string) (*payroll.LeaveBalance, error) {
	raw, err := p.do(ctx, http.MethodGet, "/employees/"+externalEmployeeID+"/leave_balance", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Annual float64 `json:"annual_leave"`
		Sick   float64 `json:"sick_leave"`
		Lieu   float64 `json:"lieu_leave"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("flexitime leave balance decode: %w", err)
	}
	return &payroll.LeaveBalance{
		ExternalEmployeeID: externalEmployeeID,
		AsAt:               time.Now().UTC(),
		AnnualHours:        int64(resp.Annual),
		SickHours:          int64(resp.Sick),
		LieuHours:          int64(resp.Lieu),
	}, nil
}

func (p *Provider) SubmitLeaveRequest(ctx context.Context, req payroll.LeaveRequest) (string, error) {
	payload := map[string]any{
		"employee_id": req.ExternalEmployeeID,
		"leave_type":  string(req.Type),
		"start_date":  req.Start.Format("2006-01-02"),
		"end_date":    req.End.Format("2006-01-02"),
		"notes":       req.Notes,
	}
	raw, err := p.do(ctx, http.MethodPost, "/leave_requests", payload)
	if err != nil {
		return "", err
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("flexitime leave request decode: %w", err)
	}
	return resp.ID, nil
}

func (p *Provider) HealthCheck(ctx context.Context) (*payroll.HealthStatus, error) {
	start := time.Now()
	_, err := p.do(ctx, http.MethodGet, "/ping", nil)
	latency := time.Since(start)
	if err != nil {
		return &payroll.HealthStatus{OK: false, Provider: "flexitime", Latency: latency, Err: err.Error()}, nil
	}
	return &payroll.HealthStatus{OK: true, Provider: "flexitime", Latency: latency}, nil
}
