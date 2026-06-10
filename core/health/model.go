// Package health provides aggregated health status for all configured external
// providers (accounting, payroll, SMS, email, storage, payment, fax, video).
// A River job polls each provider's HealthCheck() every 5 minutes and stores
// the result in provider_health_status. The HTTP /health endpoint reads from
// that cache so it never blocks on outbound calls.
package health

import (
	"time"
)

// ProviderType classifies the integration domain.
type ProviderType string

const (
	// Business integration provider types
	ProviderTypeAccounting ProviderType = "accounting"
	ProviderTypePayroll    ProviderType = "payroll"
	ProviderTypeSMS        ProviderType = "sms"
	ProviderTypeEmail      ProviderType = "email"
	ProviderTypeStorage    ProviderType = "storage"
	ProviderTypePayment    ProviderType = "payment"
	ProviderTypeFax        ProviderType = "fax"
	ProviderTypeVideo      ProviderType = "video"

	// NZ health system provider types
	ProviderTypeNHI       ProviderType = "nhi"
	ProviderTypeHPI       ProviderType = "hpi"
	ProviderTypeNES       ProviderType = "nes"
	ProviderTypeACC       ProviderType = "acc"
	ProviderTypePHARMAC   ProviderType = "pharmac"
	ProviderTypePRIMHD    ProviderType = "primhd"
	ProviderTypeWorkSafe  ProviderType = "worksafe"
	ProviderTypeMedsafe   ProviderType = "medsafe"
	ProviderTypeEpiSurv   ProviderType = "episurv"
	ProviderTypeERMS      ProviderType = "erms"
)

// Status is a cached health-check result for a single provider.
type Status struct {
	ProviderType    ProviderType  `db:"provider_type"    json:"provider_type"`
	ProviderName    string        `db:"provider_name"    json:"provider_name"`
	OK              bool          `db:"ok"               json:"ok"`
	LastCheckedAt   time.Time     `db:"last_checked_at"  json:"last_checked_at"`
	LatencyMs       int64         `db:"latency_ms"       json:"latency_ms"`
	OrganisationName string       `db:"organisation_name" json:"organisation_name,omitempty"`
	ErrorText       string        `db:"error_text"       json:"error_text,omitempty"`
}

// Report is the aggregate returned by the /health endpoint.
type Report struct {
	AllOK     bool      `json:"all_ok"`
	Providers []Status  `json:"providers"`
	GeneratedAt time.Time `json:"generated_at"`
}
