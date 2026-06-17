package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *FundedHoursHandler) getAllocationByID(ctx context.Context, id string, tenantID uuid.UUID) (allocationRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, service_plan_id,
		        funding_type, status, hours_per_week, service_type,
		        provider_id, provider_name, start_date, end_date, created_at, updated_at
		 FROM aged_care_funded_hours_allocations
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanAllocation(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return allocationRecord{}, errNotFound
		}
		return allocationRecord{}, fmt.Errorf("get allocation: %w", err)
	}
	return rec, nil
}

func (h *FundedHoursHandler) getTimesheetByID(ctx context.Context, id string, tenantID uuid.UUID) (timesheetRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, allocation_id, patient_id, patient_nhi, tenant_id,
		        status, period_start, period_end, entries, total_hours,
		        approved_by_hpi, approved_at, created_at, updated_at
		 FROM aged_care_funded_hours_timesheets
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanTimesheet(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return timesheetRecord{}, errNotFound
		}
		return timesheetRecord{}, fmt.Errorf("get timesheet: %w", err)
	}
	return rec, nil
}

func scanAllocation(s rowScanner) (allocationRecord, error) {
	var rec allocationRecord
	var fundingType, status string
	if err := s.Scan(
		&rec.ID, &rec.PatientID, &rec.PatientNHI, &rec.TenantID, &rec.ServicePlanID,
		&fundingType, &status, &rec.HoursPerWeek, &rec.ServiceType,
		&rec.ProviderID, &rec.ProviderName, &rec.StartDate, &rec.EndDate,
		&rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return allocationRecord{}, err
	}
	rec.FundingType = FundingType(fundingType)
	rec.Status = AllocationStatus(status)
	return rec, nil
}

func scanTimesheet(s rowScanner) (timesheetRecord, error) {
	var rec timesheetRecord
	var status string
	if err := s.Scan(
		&rec.ID, &rec.AllocationID, &rec.PatientID, &rec.PatientNHI, &rec.TenantID,
		&status, &rec.PeriodStart, &rec.PeriodEnd, &rec.Entries, &rec.TotalHours,
		&rec.ApprovedByHPI, &rec.ApprovedAt, &rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return timesheetRecord{}, err
	}
	rec.Status = TimesheetStatus(status)
	return rec, nil
}

func allocationToResponse(rec allocationRecord) FundedHoursAllocation {
	return FundedHoursAllocation{
		ID:            rec.ID,
		PatientID:     rec.PatientID,
		PatientNHI:    rec.PatientNHI,
		TenantID:      rec.TenantID,
		ServicePlanID: rec.ServicePlanID,
		FundingType:   rec.FundingType,
		Status:        rec.Status,
		HoursPerWeek:  rec.HoursPerWeek,
		ServiceType:   rec.ServiceType,
		ProviderID:    rec.ProviderID,
		ProviderName:  rec.ProviderName,
		StartDate:     rec.StartDate,
		EndDate:       rec.EndDate,
		CreatedAt:     rec.CreatedAt,
		UpdatedAt:     rec.UpdatedAt,
	}
}

func timesheetToResponse(rec timesheetRecord) FundedHoursTimesheet {
	return FundedHoursTimesheet{
		ID:            rec.ID,
		AllocationID:  rec.AllocationID,
		PatientID:     rec.PatientID,
		PatientNHI:    rec.PatientNHI,
		TenantID:      rec.TenantID,
		Status:        rec.Status,
		PeriodStart:   rec.PeriodStart,
		PeriodEnd:     rec.PeriodEnd,
		Entries:       rec.Entries,
		TotalHours:    rec.TotalHours,
		ApprovedByHPI: rec.ApprovedByHPI,
		ApprovedAt:    rec.ApprovedAt,
		CreatedAt:     rec.CreatedAt,
		UpdatedAt:     rec.UpdatedAt,
	}
}

// fundedHoursMax returns the larger of a and b.
func fundedHoursMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
