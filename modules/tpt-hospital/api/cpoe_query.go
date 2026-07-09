package api

import (
	"context"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db"
)

const cpoeSelectCols = `id, admission_id, tenant_id, patient_nhi, order_type, priority, status,
    order_code, order_text, clinical_indication, ordered_by, ordered_at,
    scheduled_for, completed_at, cancelled_at, cancel_reason, comments,
    specimen_type, container_type, fasting_required, volume_required,
    body_site, modality, contrast, pregnancy_status, sedation_required, transport_mode,
    result_id, result_at,
    hl7_placer_order_id, hl7_filler_order_id, hl7_dispatched_at,
    created_at, updated_at`

func (h *CPOEHandler) listOrders(ctx context.Context, admissionID, tenantID, orderTypeFilter, statusFilter string) ([]ClinicalOrder, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT `+cpoeSelectCols+`
		 FROM clinical_orders
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		   AND (@order_type = '' OR order_type = @order_type)
		   AND (@status = '' OR status = @status)
		 ORDER BY created_at ASC`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID, "order_type": orderTypeFilter, "status": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query clinical orders: %w", err)
	}
	defer rows.Close()

	var results []ClinicalOrder
	for rows.Next() {
		o, err := scanOrderRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, o)
	}
	return results, rows.Err()
}

func (h *CPOEHandler) getOrderByID(ctx context.Context, orderID, admissionID, tenantID string) (ClinicalOrder, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT `+cpoeSelectCols+`
		 FROM clinical_orders
		 WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": orderID, "admission_id": admissionID, "tenant_id": tenantID},
	)
	o, err := scanOrderRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ClinicalOrder{}, errNotFound
		}
		return ClinicalOrder{}, fmt.Errorf("get clinical order: %w", err)
	}
	return o, nil
}

func (h *CPOEHandler) insertOrder(ctx context.Context, admissionID, tenantID string, req createOrderRequest, orderedBy string) (ClinicalOrder, error) {
	var patientNHI string
	if err := h.pool.QueryRow(ctx,
		`SELECT patient_nhi FROM hospital_admissions WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": admissionID, "tenant_id": tenantID},
	).Scan(&patientNHI); err != nil {
		if db.IsNoRows(err) {
			return ClinicalOrder{}, errNotFound
		}
		return ClinicalOrder{}, fmt.Errorf("get admission for order: %w", err)
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO clinical_orders
		   (admission_id, tenant_id, patient_nhi, order_type, priority, status,
		    order_code, order_text, clinical_indication, ordered_by,
		    scheduled_for, comments,
		    specimen_type, container_type, fasting_required, volume_required,
		    body_site, modality, contrast, pregnancy_status, sedation_required, transport_mode)
		 VALUES
		   (@admission_id, @tenant_id, @patient_nhi, @order_type, @priority, @status,
		    @order_code, @order_text, @clinical_indication, @ordered_by,
		    @scheduled_for, @comments,
		    @specimen_type, @container_type, @fasting_required, @volume_required,
		    @body_site, @modality, @contrast, @pregnancy_status, @sedation_required, @transport_mode)
		 RETURNING `+cpoeSelectCols,
		db.NamedArgs{
			"admission_id":        admissionID,
			"tenant_id":           tenantID,
			"patient_nhi":         patientNHI,
			"order_type":          req.OrderType,
			"priority":            req.Priority,
			"status":              OrderPending,
			"order_code":          req.OrderCode,
			"order_text":          req.OrderText,
			"clinical_indication": req.ClinicalIndication,
			"ordered_by":          orderedBy,
			"scheduled_for":       req.ScheduledFor,
			"comments":            req.Comments,
			"specimen_type":       req.SpecimenType,
			"container_type":      req.ContainerType,
			"fasting_required":    req.FastingRequired,
			"volume_required":     req.VolumeRequired,
			"body_site":           req.BodySite,
			"modality":            req.Modality,
			"contrast":            req.Contrast,
			"pregnancy_status":    req.PregnancyStatus,
			"sedation_required":   req.SedationRequired,
			"transport_mode":      req.TransportMode,
		},
	)
	return scanOrderRow(row)
}

func (h *CPOEHandler) updateOrder(ctx context.Context, o ClinicalOrder) (ClinicalOrder, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE clinical_orders
		 SET priority = @priority, status = @status, comments = @comments,
		     scheduled_for = @scheduled_for, updated_at = now()
		 WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id
		 RETURNING `+cpoeSelectCols,
		db.NamedArgs{
			"priority":      o.Priority,
			"status":        o.Status,
			"comments":      o.Comments,
			"scheduled_for": o.ScheduledFor,
			"id":            o.ID,
			"admission_id":  o.AdmissionID,
			"tenant_id":     o.TenantID,
		},
	)
	updated, err := scanOrderRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ClinicalOrder{}, errNotFound
		}
		return ClinicalOrder{}, fmt.Errorf("update clinical order: %w", err)
	}
	return updated, nil
}

func (h *CPOEHandler) cancelOrder(ctx context.Context, orderID, admissionID, tenantID, reason string) (ClinicalOrder, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE clinical_orders
		 SET status = @status, cancel_reason = @reason, cancelled_at = now(), updated_at = now()
		 WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id
		   AND status NOT IN ('completed', 'cancelled')
		 RETURNING `+cpoeSelectCols,
		db.NamedArgs{
			"status":       OrderCancelled,
			"reason":       reason,
			"id":           orderID,
			"admission_id": admissionID,
			"tenant_id":    tenantID,
		},
	)
	o, err := scanOrderRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ClinicalOrder{}, errNotFound
		}
		return ClinicalOrder{}, fmt.Errorf("cancel clinical order: %w", err)
	}
	return o, nil
}

func (h *CPOEHandler) completeOrder(ctx context.Context, orderID, admissionID, tenantID, resultID string) (ClinicalOrder, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE clinical_orders
		 SET status = @status, result_id = @result_id, result_at = now(), completed_at = now(), updated_at = now()
		 WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id
		   AND status NOT IN ('completed', 'cancelled')
		 RETURNING `+cpoeSelectCols,
		db.NamedArgs{
			"status":       OrderCompleted,
			"result_id":    resultID,
			"id":           orderID,
			"admission_id": admissionID,
			"tenant_id":    tenantID,
		},
	)
	o, err := scanOrderRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ClinicalOrder{}, errNotFound
		}
		return ClinicalOrder{}, fmt.Errorf("complete clinical order: %w", err)
	}
	return o, nil
}

func (h *CPOEHandler) dispatchOrder(ctx context.Context, orderID, admissionID, tenantID, placerOrderID string) (ClinicalOrder, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE clinical_orders
		 SET status = @status, hl7_placer_order_id = @placer, hl7_dispatched_at = now(), updated_at = now()
		 WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id
		   AND status IN ('pending', 'in_progress')
		 RETURNING `+cpoeSelectCols,
		db.NamedArgs{
			"status":       OrderInProgress,
			"placer":       placerOrderID,
			"id":           orderID,
			"admission_id": admissionID,
			"tenant_id":    tenantID,
		},
	)
	o, err := scanOrderRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ClinicalOrder{}, errNotFound
		}
		return ClinicalOrder{}, fmt.Errorf("dispatch clinical order: %w", err)
	}
	return o, nil
}

func (h *CPOEHandler) updateOrderHL7Filler(ctx context.Context, placerOrderID, tenantID, fillerOrderID string) error {
	_, err := h.pool.Exec(ctx,
		`UPDATE clinical_orders
		 SET hl7_filler_order_id = @filler, updated_at = now()
		 WHERE hl7_placer_order_id = @placer AND tenant_id = @tenant_id`,
		db.NamedArgs{"filler": fillerOrderID, "placer": placerOrderID, "tenant_id": tenantID},
	)
	return err
}

func (h *CPOEHandler) getOrderByPlacerID(ctx context.Context, placerOrderID, tenantID string) (ClinicalOrder, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT `+cpoeSelectCols+`
		 FROM clinical_orders
		 WHERE hl7_placer_order_id = @placer AND tenant_id = @tenant_id`,
		db.NamedArgs{"placer": placerOrderID, "tenant_id": tenantID},
	)
	o, err := scanOrderRow(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ClinicalOrder{}, errNotFound
		}
		return ClinicalOrder{}, fmt.Errorf("get order by placer ID: %w", err)
	}
	return o, nil
}

func scanOrderRow(row dbRow) (ClinicalOrder, error) {
	var o ClinicalOrder
	if err := row.Scan(
		&o.ID, &o.AdmissionID, &o.TenantID, &o.PatientNHI,
		&o.OrderType, &o.Priority, &o.Status,
		&o.OrderCode, &o.OrderText, &o.ClinicalIndication, &o.OrderedBy, &o.OrderedAt,
		&o.ScheduledFor, &o.CompletedAt, &o.CancelledAt, &o.CancelReason, &o.Comments,
		&o.SpecimenType, &o.ContainerType, &o.FastingRequired, &o.VolumeRequired,
		&o.BodySite, &o.Modality, &o.Contrast, &o.PregnancyStatus, &o.SedationRequired, &o.TransportMode,
		&o.ResultID, &o.ResultAt,
		&o.HL7PlacerOrderID, &o.HL7FillerOrderID, &o.HL7DispatchedAt,
		&o.CreatedAt, &o.UpdatedAt,
	); err != nil {
		return ClinicalOrder{}, err
	}
	return o, nil
}
