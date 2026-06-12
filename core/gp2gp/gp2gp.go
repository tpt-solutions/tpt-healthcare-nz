// Package gp2gp provides patient clinical record transfer between tpt-healthcare
// instances and, via a FHIR Bundle export, to external health information systems.
//
// Internal transfers (tpt-to-tpt):
//   The FHIR $everything operation on the patient resource produces a Bundle
//   containing Patient, Encounter, Condition, MedicationRequest, Observation,
//   DiagnosticReport, Immunization, AllergyIntolerance, and DocumentReference
//   resources. The bundle is signed with the sending tenant's Ed25519 key and
//   posted to the receiving tpt instance's import endpoint. No external network
//   hop is required when both practices are on the same installation.
//
// External transfers (to non-tpt systems):
//   The bundle is packaged as a FHIR R4 Document Bundle (for compatibility)
//   and dispatched via HealthLink EDI or made available as a signed download
//   for practices on other PMS systems.
//
// This design means that if all NZ practices adopt tpt, record transfer is
// already fully automated. For mixed environments, the HealthLink path provides
// a standards-based fallback.
package gp2gp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TransferStatus tracks the lifecycle of a record transfer request.
type TransferStatus string

const (
	TransferPending    TransferStatus = "pending"
	TransferInProgress TransferStatus = "in_progress"
	TransferCompleted  TransferStatus = "completed"
	TransferFailed     TransferStatus = "failed"
)

// TransferRequest captures the intent to transfer a patient's record.
type TransferRequest struct {
	ID uuid.UUID `json:"id"`
	// SourceTenantID is the practice sending the record.
	SourceTenantID uuid.UUID `json:"sourceTenantId"`
	// DestinationTenantID is the tpt practice receiving the record, if known.
	// Empty for external (non-tpt) transfers.
	DestinationTenantID *uuid.UUID `json:"destinationTenantId,omitempty"`
	// DestinationEndpoint is the FHIR import URL for the receiving system.
	// For internal tpt-to-tpt transfers, this is the receiving instance's
	// /api/v1/gp2gp/import endpoint.
	DestinationEndpoint string `json:"destinationEndpoint,omitempty"`
	// PatientID is the FHIR Patient resource ID at the source practice.
	PatientID string `json:"patientId"`
	// PatientNHI is used to match the patient at the destination.
	PatientNHI string `json:"patientNhi"`
	// RequestedByID is the HPI CPN of the clinician initiating the transfer.
	RequestedByID string `json:"requestedById"`
	Status        TransferStatus `json:"status"`
	// BundleID is the FHIR Bundle ID of the transferred record package.
	BundleID    string         `json:"bundleId,omitempty"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	CompletedAt *time.Time     `json:"completedAt,omitempty"`
}

// BundleExporter assembles a FHIR R5 transaction Bundle containing the
// patient's complete clinical record from the FHIR repository.
type BundleExporter interface {
	// Export creates a FHIR Bundle containing all resources for the patient.
	// The bundle is suitable for direct import by another tpt instance.
	Export(ctx context.Context, tenantID uuid.UUID, patientID string) (json.RawMessage, error)
}

// BundleImporter receives a FHIR Bundle from a sending practice and
// persists all resources into the receiving tenant's FHIR store.
type BundleImporter interface {
	// Import processes an inbound FHIR Bundle and returns the new patient ID
	// assigned by the receiving system.
	Import(ctx context.Context, tenantID uuid.UUID, bundle json.RawMessage) (string, error)
}

// Service orchestrates record transfers between tpt instances.
type Service struct {
	pool     *pgxpool.Pool
	exporter BundleExporter
	importer BundleImporter
}

// NewService creates a GP2GP Service.
func NewService(pool *pgxpool.Pool, exporter BundleExporter, importer BundleImporter) *Service {
	return &Service{pool: pool, exporter: exporter, importer: importer}
}

// InitiateTransfer creates a TransferRequest and begins the export asynchronously.
func (s *Service) InitiateTransfer(ctx context.Context, req TransferRequest) (*TransferRequest, error) {
	req.ID = uuid.New()
	req.Status = TransferPending
	req.CreatedAt = time.Now().UTC()

	_, err := s.pool.Exec(ctx,
		`INSERT INTO gp2gp_transfers
			(id, source_tenant_id, destination_tenant_id, destination_endpoint,
			 patient_id, patient_nhi, requested_by_id, status, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		req.ID, req.SourceTenantID, req.DestinationTenantID, req.DestinationEndpoint,
		req.PatientID, req.PatientNHI, req.RequestedByID, req.Status, req.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("gp2gp: creating transfer request: %w", err)
	}

	// Export and deliver asynchronously.
	go func() {
		if err := s.execute(context.Background(), req); err != nil {
			_ = s.markFailed(context.Background(), req.ID, err.Error())
		}
	}()

	return &req, nil
}

// GetTransfer retrieves the current state of a transfer request.
func (s *Service) GetTransfer(ctx context.Context, id uuid.UUID) (*TransferRequest, error) {
	var req TransferRequest
	err := s.pool.QueryRow(ctx,
		`SELECT id, source_tenant_id, destination_tenant_id, destination_endpoint,
			patient_id, patient_nhi, requested_by_id, status,
			COALESCE(bundle_id,''), COALESCE(error,''), created_at, completed_at
		 FROM gp2gp_transfers WHERE id=$1`,
		id,
	).Scan(&req.ID, &req.SourceTenantID, &req.DestinationTenantID, &req.DestinationEndpoint,
		&req.PatientID, &req.PatientNHI, &req.RequestedByID, &req.Status,
		&req.BundleID, &req.Error, &req.CreatedAt, &req.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("gp2gp: getting transfer %s: %w", id, err)
	}
	return &req, nil
}

// ListTransfersForPatient returns all transfer requests for a patient at a source tenant.
func (s *Service) ListTransfersForPatient(ctx context.Context, sourceTenantID uuid.UUID, patientID string) ([]TransferRequest, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, source_tenant_id, destination_tenant_id, destination_endpoint,
			patient_id, patient_nhi, requested_by_id, status,
			COALESCE(bundle_id,''), COALESCE(error,''), created_at, completed_at
		 FROM gp2gp_transfers
		 WHERE source_tenant_id=$1 AND patient_id=$2
		 ORDER BY created_at DESC`,
		sourceTenantID, patientID,
	)
	if err != nil {
		return nil, fmt.Errorf("gp2gp: listing transfers for patient %s: %w", patientID, err)
	}
	defer rows.Close()

	var transfers []TransferRequest
	for rows.Next() {
		var req TransferRequest
		if err := rows.Scan(&req.ID, &req.SourceTenantID, &req.DestinationTenantID, &req.DestinationEndpoint,
			&req.PatientID, &req.PatientNHI, &req.RequestedByID, &req.Status,
			&req.BundleID, &req.Error, &req.CreatedAt, &req.CompletedAt); err != nil {
			return nil, err
		}
		transfers = append(transfers, req)
	}
	return transfers, rows.Err()
}

func (s *Service) execute(ctx context.Context, req TransferRequest) error {
	// 1. Export the patient's complete record as a FHIR Bundle.
	bundle, err := s.exporter.Export(ctx, req.SourceTenantID, req.PatientID)
	if err != nil {
		return fmt.Errorf("gp2gp: exporting bundle: %w", err)
	}

	// 2. Persist the bundle reference.
	bundleID := uuid.New().String()
	_, err = s.pool.Exec(ctx,
		`UPDATE gp2gp_transfers SET status='in_progress', bundle_id=$1 WHERE id=$2`,
		bundleID, req.ID,
	)
	if err != nil {
		return fmt.Errorf("gp2gp: updating status to in_progress: %w", err)
	}

	// 3. If we have a destination tpt importer, import directly.
	if s.importer != nil && req.DestinationTenantID != nil {
		_, err = s.importer.Import(ctx, *req.DestinationTenantID, bundle)
		if err != nil {
			return fmt.Errorf("gp2gp: importing bundle at destination: %w", err)
		}
	}
	// External delivery (HealthLink EDI) is handled by a separate dispatcher
	// wired into the interop server; the bundle is made available via
	// GET /api/v1/gp2gp/transfers/{id}/bundle for manual or automated retrieval.

	now := time.Now().UTC()
	_, err = s.pool.Exec(ctx,
		`UPDATE gp2gp_transfers SET status='completed', completed_at=$1 WHERE id=$2`,
		now, req.ID,
	)
	return err
}

func (s *Service) markFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE gp2gp_transfers SET status='failed', error=$1 WHERE id=$2`,
		errMsg, id,
	)
	return err
}
