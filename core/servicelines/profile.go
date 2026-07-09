package servicelines

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Profile is the set of service lines a tenant has enabled.
type Profile struct {
	TenantID     uuid.UUID `json:"tenant_id"`
	ServiceLines []string  `json:"service_lines"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Store persists which service lines each tenant runs and keeps the
// tenant's active-module list (see modules/tpt-practice's TenantSettings)
// in sync with the modules those service lines imply.
type Store interface {
	// GetProfile returns the tenant's selected service lines. Tenants with
	// no selection yet return an empty, zero-value Profile (not an error).
	GetProfile(ctx context.Context, tenantID uuid.UUID) (*Profile, error)
	// SetProfile replaces the tenant's selected service lines and merges
	// the resolved default modules into the tenant's active-module list.
	SetProfile(ctx context.Context, tenantID uuid.UUID, serviceLineIDs []string) (*Profile, error)
}

// ValidateIDs checks that every ID is a known catalogue entry and that
// there are no duplicates. It returns a descriptive error naming the first
// problem found, or nil if ids is valid.
func ValidateIDs(ids []string) error {
	seen := map[string]bool{}
	for _, id := range ids {
		if !Valid(id) {
			return fmt.Errorf("unknown service line %q", id)
		}
		if seen[id] {
			return fmt.Errorf("duplicate service line %q", id)
		}
		seen[id] = true
	}
	return nil
}

func dedupeSorted(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(a)+len(b))
	for _, v := range a {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, v := range b {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
