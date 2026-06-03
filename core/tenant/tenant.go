// Package tenant manages clinic tenants and their onboarding applications.
package tenant

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Tenant status constants.
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
)

// Application status constants.
const (
	AppStatusPending  = "pending"
	AppStatusApproved = "approved"
	AppStatusRejected = "rejected"
)

// Tenant is an approved clinic active on the network.
type Tenant struct {
	ID            uuid.UUID      `json:"id"`
	Name          string         `json:"name"`
	HPIFacilityID string         `json:"hpi_facility_id"`
	Status        string         `json:"status"`
	ContactEmail  string         `json:"contact_email"`
	ContactName   string         `json:"contact_name"`
	Address       map[string]any `json:"address"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// Application is a clinic's self-registration request awaiting admin review.
type Application struct {
	ID            uuid.UUID      `json:"id"`
	PracticeName  string         `json:"practice_name"`
	HPIFacilityID string         `json:"hpi_facility_id"`
	ContactName   string         `json:"contact_name"`
	ContactEmail  string         `json:"contact_email"`
	ContactHPICPN string         `json:"contact_hpi_cpn,omitempty"`
	Address       map[string]any `json:"address"`
	Status        string         `json:"status"`
	ReviewerNotes string         `json:"reviewer_notes,omitempty"`
	TenantID      *uuid.UUID     `json:"tenant_id,omitempty"`
	SubmittedAt   time.Time      `json:"submitted_at"`
	ReviewedAt    *time.Time     `json:"reviewed_at,omitempty"`
	ReviewedBy    string         `json:"reviewed_by,omitempty"`
}

// SubmitRequest is the payload for a new clinic application.
type SubmitRequest struct {
	PracticeName  string         `json:"practice_name"`
	HPIFacilityID string         `json:"hpi_facility_id"`
	ContactName   string         `json:"contact_name"`
	ContactEmail  string         `json:"contact_email"`
	ContactHPICPN string         `json:"contact_hpi_cpn"`
	Address       map[string]any `json:"address"`
}

// ReviewRequest is the admin payload when approving or rejecting an application.
type ReviewRequest struct {
	Notes string `json:"notes"`
}

// Store defines persistence operations for tenant management.
type Store interface {
	// Submit creates a new pending application and returns it.
	Submit(ctx context.Context, req SubmitRequest) (*Application, error)
	// GetApplication retrieves an application by ID.
	GetApplication(ctx context.Context, id uuid.UUID) (*Application, error)
	// ListApplications returns applications filtered by status; pass "" for all.
	ListApplications(ctx context.Context, status string) ([]*Application, error)
	// Approve transitions an application to approved, creates the tenant record,
	// and returns the newly created Tenant.
	Approve(ctx context.Context, applicationID uuid.UUID, reviewerID, notes string) (*Tenant, error)
	// Reject transitions an application to rejected.
	Reject(ctx context.Context, applicationID uuid.UUID, reviewerID, notes string) error
	// ListTenants returns all tenant records.
	ListTenants(ctx context.Context) ([]*Tenant, error)
	// GetTenant retrieves a tenant by UUID.
	GetTenant(ctx context.Context, id uuid.UUID) (*Tenant, error)
}
