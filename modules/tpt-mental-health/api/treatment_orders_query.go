package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (h *TreatmentOrdersHandler) listOrders(
	ctx context.Context,
	tenantID uuid.UUID,
	patientFilter, statusFilter, typeFilter string,
) ([]orderRecord, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, episode_id,
		        order_type, status, responsible_hpi, second_opinion_hpi,
		        legal_authority, conditions,
		        issued_date::text, expiry_date::text, first_review_date::text,
		        last_review_date::text, next_review_date::text,
		        revocation_reason, tribunal_reference, extra_sensitive,
		        created_at, updated_at
		 FROM compulsory_orders
		 WHERE tenant_id = $1
		   AND ($2 = '' OR patient_id::text = $2)
		   AND ($3 = '' OR status = $3)
		   AND ($4 = '' OR order_type = $4)
		 ORDER BY issued_date DESC
		 LIMIT 200`,
		tenantID, patientFilter, statusFilter, typeFilter,
	)
	if err != nil {
		return nil, fmt.Errorf("query orders: %w", err)
	}
	defer rows.Close()

	var results []orderRecord
	for rows.Next() {
		rec, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, rows.Err()
}

func (h *TreatmentOrdersHandler) getOrderByID(ctx context.Context, id string, tenantID uuid.UUID) (orderRecord, error) {
	row := h.pool.QueryRow(ctx,
		`SELECT id, patient_id, patient_nhi, tenant_id, episode_id,
		        order_type, status, responsible_hpi, second_opinion_hpi,
		        legal_authority, conditions,
		        issued_date::text, expiry_date::text, first_review_date::text,
		        last_review_date::text, next_review_date::text,
		        revocation_reason, tribunal_reference, extra_sensitive,
		        created_at, updated_at
		 FROM compulsory_orders
		 WHERE id = $1 AND tenant_id = $2`,
		id, tenantID,
	)
	rec, err := scanOrderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderRecord{}, errNotFound
		}
		return orderRecord{}, fmt.Errorf("get order by id: %w", err)
	}
	return rec, nil
}

func (h *TreatmentOrdersHandler) insertOrder(ctx context.Context, req orderCreateRequest, tenantID uuid.UUID) (orderRecord, error) {
	var condEnc []byte
	if req.Conditions != "" {
		var err error
		condEnc, err = h.enc.Encrypt([]byte(req.Conditions))
		if err != nil {
			return orderRecord{}, fmt.Errorf("encrypt conditions: %w", err)
		}
	}

	var episodeID *string
	if req.EpisodeID != "" {
		episodeID = &req.EpisodeID
	}

	row := h.pool.QueryRow(ctx,
		`INSERT INTO compulsory_orders
		   (patient_id, patient_nhi, tenant_id, episode_id,
		    order_type, status, responsible_hpi, second_opinion_hpi,
		    legal_authority, conditions,
		    issued_date, expiry_date, first_review_date, next_review_date, extra_sensitive)
		 VALUES
		   ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, TRUE)
		 RETURNING id, patient_id, patient_nhi, tenant_id, episode_id,
		           order_type, status, responsible_hpi, second_opinion_hpi,
		           legal_authority, conditions,
		           issued_date::text, expiry_date::text, first_review_date::text,
		           last_review_date::text, next_review_date::text,
		           revocation_reason, tribunal_reference, extra_sensitive,
		           created_at, updated_at`,
		req.PatientID, req.PatientNHI, tenantID, episodeID,
		string(req.OrderType), string(OrderActive),
		req.ResponsibleHPI, req.SecondOpinionHPI,
		req.LegalAuthority, condEnc,
		req.IssuedDate, req.ExpiryDate, req.FirstReviewDate, req.NextReviewDate,
	)
	return scanOrderRow(row)
}

func (h *TreatmentOrdersHandler) updateOrder(ctx context.Context, rec orderRecord, condEnc []byte, tenantID uuid.UUID) (orderRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE compulsory_orders
		 SET responsible_hpi    = $1,
		     second_opinion_hpi = $2,
		     legal_authority    = $3,
		     conditions         = $4,
		     expiry_date        = $5::date,
		     next_review_date   = $6::date,
		     tribunal_reference = $7,
		     status             = $8,
		     updated_at         = now()
		 WHERE id = $9 AND tenant_id = $10
		 RETURNING id, patient_id, patient_nhi, tenant_id, episode_id,
		           order_type, status, responsible_hpi, second_opinion_hpi,
		           legal_authority, conditions,
		           issued_date::text, expiry_date::text, first_review_date::text,
		           last_review_date::text, next_review_date::text,
		           revocation_reason, tribunal_reference, extra_sensitive,
		           created_at, updated_at`,
		rec.ResponsibleHPI, rec.SecondOpinionHPI, rec.LegalAuthority,
		condEnc, rec.ExpiryDate, rec.NextReviewDate, rec.TribunalReference, rec.Status,
		rec.ID, tenantID,
	)
	updated, err := scanOrderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderRecord{}, errNotFound
		}
		return orderRecord{}, fmt.Errorf("update order: %w", err)
	}
	return updated, nil
}

func (h *TreatmentOrdersHandler) recordReview(
	ctx context.Context,
	id, reviewedAt, nextReviewDate, newStatus string,
	tenantID uuid.UUID,
) (orderRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE compulsory_orders
		 SET last_review_date = $1::date,
		     next_review_date = $2::date,
		     status           = $3,
		     updated_at       = now()
		 WHERE id = $4 AND tenant_id = $5
		 RETURNING id, patient_id, patient_nhi, tenant_id, episode_id,
		           order_type, status, responsible_hpi, second_opinion_hpi,
		           legal_authority, conditions,
		           issued_date::text, expiry_date::text, first_review_date::text,
		           last_review_date::text, next_review_date::text,
		           revocation_reason, tribunal_reference, extra_sensitive,
		           created_at, updated_at`,
		reviewedAt, nextReviewDate, newStatus, id, tenantID,
	)
	updated, err := scanOrderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderRecord{}, errNotFound
		}
		return orderRecord{}, fmt.Errorf("record review: %w", err)
	}
	return updated, nil
}

func (h *TreatmentOrdersHandler) revokeOrder(ctx context.Context, id string, reasonEnc []byte, tenantID uuid.UUID) (orderRecord, error) {
	row := h.pool.QueryRow(ctx,
		`UPDATE compulsory_orders
		 SET status            = $1,
		     revocation_reason = $2,
		     updated_at        = now()
		 WHERE id = $3 AND tenant_id = $4
		 RETURNING id, patient_id, patient_nhi, tenant_id, episode_id,
		           order_type, status, responsible_hpi, second_opinion_hpi,
		           legal_authority, conditions,
		           issued_date::text, expiry_date::text, first_review_date::text,
		           last_review_date::text, next_review_date::text,
		           revocation_reason, tribunal_reference, extra_sensitive,
		           created_at, updated_at`,
		string(OrderRevoked), reasonEnc, id, tenantID,
	)
	updated, err := scanOrderRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderRecord{}, errNotFound
		}
		return orderRecord{}, fmt.Errorf("revoke order: %w", err)
	}
	return updated, nil
}

func (h *TreatmentOrdersHandler) decryptOrder(rec orderRecord) (CompulsoryOrder, error) {
	var conditions string
	if len(rec.ConditionsEnc) > 0 {
		plain, err := h.enc.Decrypt(rec.ConditionsEnc)
		if err != nil {
			return CompulsoryOrder{}, fmt.Errorf("decrypt conditions: %w", err)
		}
		conditions = string(plain)
	}
	return CompulsoryOrder{
		ID:                rec.ID,
		PatientID:         rec.PatientID,
		PatientNHI:        rec.PatientNHI,
		TenantID:          rec.TenantID,
		EpisodeID:         rec.EpisodeID,
		OrderType:         OrderType(rec.OrderType),
		Status:            OrderStatus(rec.Status),
		ResponsibleHPI:    rec.ResponsibleHPI,
		SecondOpinionHPI:  rec.SecondOpinionHPI,
		LegalAuthority:    rec.LegalAuthority,
		Conditions:        conditions,
		IssuedDate:        rec.IssuedDate,
		ExpiryDate:        rec.ExpiryDate,
		FirstReviewDate:   rec.FirstReviewDate,
		LastReviewDate:    rec.LastReviewDate,
		NextReviewDate:    rec.NextReviewDate,
		TribunalReference: rec.TribunalReference,
		ExtraSensitive:    rec.ExtraSensitive,
		CreatedAt:         rec.CreatedAt,
		UpdatedAt:         rec.UpdatedAt,
	}, nil
}

func scanOrder(s rowScanner) (orderRecord, error) {
	return scanOrderRow(s)
}

func scanOrderRow(s rowScanner) (orderRecord, error) {
	var rec orderRecord
	if err := s.Scan(
		&rec.ID, &rec.PatientID, &rec.PatientNHI, &rec.TenantID, &rec.EpisodeID,
		&rec.OrderType, &rec.Status, &rec.ResponsibleHPI, &rec.SecondOpinionHPI,
		&rec.LegalAuthority, &rec.ConditionsEnc,
		&rec.IssuedDate, &rec.ExpiryDate, &rec.FirstReviewDate,
		&rec.LastReviewDate, &rec.NextReviewDate,
		&rec.RevocationEnc, &rec.TribunalReference, &rec.ExtraSensitive,
		&rec.CreatedAt, &rec.UpdatedAt,
	); err != nil {
		return orderRecord{}, err
	}
	return rec, nil
}

func validOrderType(t OrderType) bool {
	switch t {
	case OrderCAO, OrderCTOInpatient, OrderCTOCommunity, OrderSPO:
		return true
	}
	return false
}
