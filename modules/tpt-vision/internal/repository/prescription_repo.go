package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-vision/internal/refraction"
	"github.com/google/uuid"
)

// PrescriptionRepository handles persistence of refraction prescriptions
type PrescriptionRepository struct {
	pool db.Pool
}

// NewPrescriptionRepository creates a new prescription repository
func NewPrescriptionRepository(pool db.Pool) *PrescriptionRepository {
	return &PrescriptionRepository{pool: pool}
}

// Create inserts a new prescription
func (r *PrescriptionRepository) Create(ctx context.Context, p *refraction.Prescription) error {
	fhirJSON, err := json.Marshal(p.ToFHIRObservation())
	if err != nil {
		return fmt.Errorf("marshal FHIR: %w", err)
	}

	query := `
		INSERT INTO vision_prescriptions (
			id, tenant_id, patient_nhi, clinician_id, practice_id,
			type, distance,
			right_sphere, right_cylinder, right_axis, right_prism, right_prism_dir,
			right_add, right_visual_acuity, right_method, right_notes,
			left_sphere, left_cylinder, left_axis, left_prism, left_prism_dir,
			left_add, left_visual_acuity, left_method, left_notes,
			issued_date, expiry_date, is_current,
			fhir_resource, fhir_version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32
		)
	`

	_, err = r.pool.Exec(ctx, query,
		p.ID, p.TenantID, p.PatientNHI, p.ClinicianID, p.PracticeID,
		p.Type, p.Distance,
		p.RightEye.Sphere, p.RightEye.Cylinder, p.RightEye.Axis, p.RightEye.Prism, p.RightEye.PrismDir,
		p.RightEye.ADD, p.RightEye.VisualAcuity, p.RightEye.Method, p.RightEye.Notes,
		p.LeftEye.Sphere, p.LeftEye.Cylinder, p.LeftEye.Axis, p.LeftEye.Prism, p.LeftEye.PrismDir,
		p.LeftEye.ADD, p.LeftEye.VisualAcuity, p.LeftEye.Method, p.LeftEye.Notes,
		p.IssuedDate, p.ExpiryDate, p.IsCurrent,
		fhirJSON, 1, time.Now(), time.Now(),
	)
	return err
}

// GetByID retrieves a prescription by ID
func (r *PrescriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (*refraction.Prescription, error) {
	query := `SELECT * FROM vision_prescriptions WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	return r.scanPrescription(row)
}

// GetByPatientNHI retrieves all prescriptions for a patient
func (r *PrescriptionRepository) GetByPatientNHI(ctx context.Context, patientNHI string) ([]*refraction.Prescription, error) {
	query := `SELECT * FROM vision_prescriptions WHERE patient_nhi = $1 ORDER BY issued_date DESC`
	rows, err := r.pool.Query(ctx, query, patientNHI)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prescriptions []*refraction.Prescription
	for rows.Next() {
		p, err := r.scanPrescription(rows)
		if err != nil {
			return nil, err
		}
		prescriptions = append(prescriptions, p)
	}
	return prescriptions, rows.Err()
}

// GetCurrentPrescription retrieves the current prescription for a patient by type and distance
func (r *PrescriptionRepository) GetCurrentPrescription(ctx context.Context, patientNHI string, prescType refraction.PrescriptionType, distance refraction.DistanceType) (*refraction.Prescription, error) {
	query := `
		SELECT * FROM vision_prescriptions 
		WHERE patient_nhi = $1 AND type = $2 AND distance = $3 AND is_current = TRUE
		LIMIT 1
	`
	row := r.pool.QueryRow(ctx, query, patientNHI, prescType, distance)
	return r.scanPrescription(row)
}

// Update updates an existing prescription
func (r *PrescriptionRepository) Update(ctx context.Context, p *refraction.Prescription) error {
	fhirJSON, err := json.Marshal(p.ToFHIRObservation())
	if err != nil {
		return fmt.Errorf("marshal FHIR: %w", err)
	}

	query := `
		UPDATE vision_prescriptions SET
			type = $2, distance = $3,
			right_sphere = $4, right_cylinder = $5, right_axis = $6, right_prism = $7, right_prism_dir = $8,
			right_add = $9, right_visual_acuity = $10, right_method = $11, right_notes = $12,
			left_sphere = $13, left_cylinder = $14, left_axis = $15, left_prism = $16, left_prism_dir = $17,
			left_add = $18, left_visual_acuity = $19, left_method = $20, left_notes = $21,
			issued_date = $22, expiry_date = $23, is_current = $24,
			fhir_resource = $25, fhir_version = fhir_version + 1, updated_at = NOW()
		WHERE id = $1
	`

	_, err = r.pool.Exec(ctx, query,
		p.ID,
		p.Type, p.Distance,
		p.RightEye.Sphere, p.RightEye.Cylinder, p.RightEye.Axis, p.RightEye.Prism, p.RightEye.PrismDir,
		p.RightEye.ADD, p.RightEye.VisualAcuity, p.RightEye.Method, p.RightEye.Notes,
		p.LeftEye.Sphere, p.LeftEye.Cylinder, p.LeftEye.Axis, p.LeftEye.Prism, p.LeftEye.PrismDir,
		p.LeftEye.ADD, p.LeftEye.VisualAcuity, p.LeftEye.Method, p.LeftEye.Notes,
		p.IssuedDate, p.ExpiryDate, p.IsCurrent,
		fhirJSON,
	)
	return err
}

// Delete soft-deletes a prescription (sets is_current = false)
func (r *PrescriptionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE vision_prescriptions SET is_current = FALSE, updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	return err
}

// scanPrescription scans a row into a Prescription struct
func (r *PrescriptionRepository) scanPrescription(scanner interface{ Scan(...any) error }) (*refraction.Prescription, error) {
	var p refraction.Prescription
	var rightPrismDir, leftPrismDir sql.NullString
	var rightAxis, leftAxis sql.NullInt32
	var rightNotes, leftNotes sql.NullString
	var tenantID, clinicianID, practiceID uuid.UUID
	var issuedDate, expiryDate, createdAt, updatedAt time.Time

	err := scanner.Scan(
		&p.ID, &tenantID, &p.PatientNHI, &clinicianID, &practiceID,
		&p.Type, &p.Distance,
		&p.RightEye.Sphere, &p.RightEye.Cylinder, &rightAxis, &p.RightEye.Prism, &rightPrismDir,
		&p.RightEye.ADD, &p.RightEye.VisualAcuity, &p.RightEye.Method, &rightNotes,
		&p.LeftEye.Sphere, &p.LeftEye.Cylinder, &leftAxis, &p.LeftEye.Prism, &leftPrismDir,
		&p.LeftEye.ADD, &p.LeftEye.VisualAcuity, &p.LeftEye.Method, &leftNotes,
		&issuedDate, &expiryDate, &p.IsCurrent,
		&p.FHIRResource, &p.FHIRVersion, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if rightAxis.Valid {
		p.RightEye.Axis = int(rightAxis.Int32)
	}
	if rightPrismDir.Valid {
		p.RightEye.PrismDir = refraction.PrismDirection(rightPrismDir.String)
	}
	if rightNotes.Valid {
		p.RightEye.Notes = rightNotes.String
	}
	if leftAxis.Valid {
		p.LeftEye.Axis = int(leftAxis.Int32)
	}
	if leftPrismDir.Valid {
		p.LeftEye.PrismDir = refraction.PrismDirection(leftPrismDir.String)
	}
	if leftNotes.Valid {
		p.LeftEye.Notes = leftNotes.String
	}

	p.IssuedDate = issuedDate.UnixMilli()
	p.ExpiryDate = expiryDate.UnixMilli()
	p.CreatedAt = createdAt.UnixMilli()
	p.UpdatedAt = updatedAt.UnixMilli()

	return &p, nil
}