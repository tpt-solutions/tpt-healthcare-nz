package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-counselling/internal/eap"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-counselling/internal/private"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-counselling/internal/session"
)

var errNotFound = errors.New("record not found")

// ---- EAP Claims ----

const eapSelectCols = `id, client_nhi, counsellor_id, eap_provider, session_count,
	session_fee, total_fee, status, reference, invoice_number, created_at, updated_at`

func (s *Server) listEAPClaims(ctx context.Context) ([]eap.EAPClaim, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+eapSelectCols+`
		 FROM counselling_eap_claims
		 ORDER BY created_at DESC
		 LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("query eap claims: %w", err)
	}
	defer rows.Close()

	var results []eap.EAPClaim
	for rows.Next() {
		var c eap.EAPClaim
		if err := rows.Scan(
			&c.ID, &c.ClientNHI, &c.ProviderHPI, &c.EAPProvider,
			&c.SessionCount, &c.SessionFee, &c.TotalFee,
			&c.Status, &c.Reference, &c.InvoiceNumber,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan eap claim: %w", err)
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

func (s *Server) getEAPClaim(ctx context.Context, id string) (eap.EAPClaim, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+eapSelectCols+`
		 FROM counselling_eap_claims
		 WHERE id = @id`,
		db.NamedArgs{"id": id},
	)
	var c eap.EAPClaim
	if err := row.Scan(
		&c.ID, &c.ClientNHI, &c.ProviderHPI, &c.EAPProvider,
		&c.SessionCount, &c.SessionFee, &c.TotalFee,
		&c.Status, &c.Reference, &c.InvoiceNumber,
		&c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return eap.EAPClaim{}, errNotFound
		}
		return eap.EAPClaim{}, fmt.Errorf("get eap claim: %w", err)
	}
	return c, nil
}

func (s *Server) createEAPClaim(ctx context.Context, c eap.EAPClaim) (eap.EAPClaim, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO counselling_eap_claims
		   (id, client_nhi, counsellor_id, eap_provider, session_count,
		    session_fee, total_fee, status, reference, invoice_number,
		    created_at, updated_at)
		 VALUES
		   (@id, @client_nhi, @counsellor_id, @eap_provider, @session_count,
		    @session_fee, @total_fee, @status, @reference, @invoice_number,
		    @created_at, @updated_at)
		 RETURNING `+eapSelectCols,
		db.NamedArgs{
			"id":             c.ID,
			"client_nhi":     c.ClientNHI,
			"counsellor_id":  c.ProviderHPI,
			"eap_provider":   c.EAPProvider,
			"session_count":  c.SessionCount,
			"session_fee":    c.SessionFee,
			"total_fee":      c.TotalFee,
			"status":         c.Status,
			"reference":      c.Reference,
			"invoice_number": c.InvoiceNumber,
			"created_at":     c.CreatedAt,
			"updated_at":     c.UpdatedAt,
		},
	)
	var result eap.EAPClaim
	if err := row.Scan(
		&result.ID, &result.ClientNHI, &result.ProviderHPI, &result.EAPProvider,
		&result.SessionCount, &result.SessionFee, &result.TotalFee,
		&result.Status, &result.Reference, &result.InvoiceNumber,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		return eap.EAPClaim{}, fmt.Errorf("insert eap claim: %w", err)
	}
	return result, nil
}

func (s *Server) updateEAPClaim(ctx context.Context, c eap.EAPClaim) (eap.EAPClaim, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE counselling_eap_claims
		 SET client_nhi     = @client_nhi,
		     counsellor_id  = @counsellor_id,
		     eap_provider   = @eap_provider,
		     session_count  = @session_count,
		     session_fee    = @session_fee,
		     total_fee      = @total_fee,
		     status         = @status,
		     reference      = @reference,
		     invoice_number = @invoice_number,
		     updated_at     = @updated_at
		 WHERE id = @id
		 RETURNING `+eapSelectCols,
		db.NamedArgs{
			"id":             c.ID,
			"client_nhi":     c.ClientNHI,
			"counsellor_id":  c.ProviderHPI,
			"eap_provider":   c.EAPProvider,
			"session_count":  c.SessionCount,
			"session_fee":    c.SessionFee,
			"total_fee":      c.TotalFee,
			"status":         c.Status,
			"reference":      c.Reference,
			"invoice_number": c.InvoiceNumber,
			"updated_at":     c.UpdatedAt,
		},
	)
	var result eap.EAPClaim
	if err := row.Scan(
		&result.ID, &result.ClientNHI, &result.ProviderHPI, &result.EAPProvider,
		&result.SessionCount, &result.SessionFee, &result.TotalFee,
		&result.Status, &result.Reference, &result.InvoiceNumber,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return eap.EAPClaim{}, errNotFound
		}
		return eap.EAPClaim{}, fmt.Errorf("update eap claim: %w", err)
	}
	return result, nil
}

// ---- Sessions ----

const sessionSelectCols = `id, client_nhi, clinician_id, practice_id, session_date,
	session_number, modality, mode, duration_min, presenting_issue, clinical_notes,
	risk_assessment, intervention, outcome, homework, next_session_date,
	billing_type, fee_in_cents, created_at, updated_at`

func (s *Server) listSessions(ctx context.Context, clientNHI string) ([]session.Session, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+sessionSelectCols+`
		 FROM counselling_sessions
		 WHERE client_nhi = @nhi
		 ORDER BY session_date DESC
		 LIMIT 200`,
		db.NamedArgs{"nhi": clientNHI},
	)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var results []session.Session
	for rows.Next() {
		var sess session.Session
		if err := rows.Scan(
			&sess.ID, &sess.ClientNHI, &sess.ClinicianID, &sess.PracticeID,
			&sess.SessionDate, &sess.SessionNumber, &sess.Modality, &sess.Mode,
			&sess.DurationMin, &sess.PresentingIssue, &sess.ClinicalNotes,
			&sess.RiskAssessment, &sess.Intervention, &sess.Outcome,
			&sess.Homework, &sess.NextSessionDate,
			&sess.BillingType, &sess.FeeInCents,
			&sess.CreatedAt, &sess.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		results = append(results, sess)
	}
	return results, rows.Err()
}

func (s *Server) getSession(ctx context.Context, id, clientNHI string) (session.Session, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+sessionSelectCols+`
		 FROM counselling_sessions
		 WHERE id = @id AND client_nhi = @nhi`,
		db.NamedArgs{"id": id, "nhi": clientNHI},
	)
	var sess session.Session
	if err := row.Scan(
		&sess.ID, &sess.ClientNHI, &sess.ClinicianID, &sess.PracticeID,
		&sess.SessionDate, &sess.SessionNumber, &sess.Modality, &sess.Mode,
		&sess.DurationMin, &sess.PresentingIssue, &sess.ClinicalNotes,
		&sess.RiskAssessment, &sess.Intervention, &sess.Outcome,
		&sess.Homework, &sess.NextSessionDate,
		&sess.BillingType, &sess.FeeInCents,
		&sess.CreatedAt, &sess.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return session.Session{}, errNotFound
		}
		return session.Session{}, fmt.Errorf("get session: %w", err)
	}
	return sess, nil
}

func (s *Server) createSession(ctx context.Context, sess session.Session) (session.Session, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO counselling_sessions
		   (id, client_nhi, clinician_id, practice_id, session_date,
		    session_number, modality, mode, duration_min, presenting_issue,
		    clinical_notes, risk_assessment, intervention, outcome, homework,
		    next_session_date, billing_type, fee_in_cents, created_at, updated_at)
		 VALUES
		   (@id, @client_nhi, @clinician_id, @practice_id, @session_date,
		    @session_number, @modality, @mode, @duration_min, @presenting_issue,
		    @clinical_notes, @risk_assessment, @intervention, @outcome, @homework,
		    @next_session_date, @billing_type, @fee_in_cents, @created_at, @updated_at)
		 RETURNING `+sessionSelectCols,
		db.NamedArgs{
			"id":               sess.ID,
			"client_nhi":       sess.ClientNHI,
			"clinician_id":     sess.ClinicianID,
			"practice_id":      sess.PracticeID,
			"session_date":     sess.SessionDate,
			"session_number":   sess.SessionNumber,
			"modality":         sess.Modality,
			"mode":             sess.Mode,
			"duration_min":     sess.DurationMin,
			"presenting_issue": sess.PresentingIssue,
			"clinical_notes":   sess.ClinicalNotes,
			"risk_assessment":  sess.RiskAssessment,
			"intervention":     sess.Intervention,
			"outcome":          sess.Outcome,
			"homework":         sess.Homework,
			"next_session_date": sess.NextSessionDate,
			"billing_type":     sess.BillingType,
			"fee_in_cents":     sess.FeeInCents,
			"created_at":       sess.CreatedAt,
			"updated_at":       sess.UpdatedAt,
		},
	)
	var result session.Session
	if err := row.Scan(
		&result.ID, &result.ClientNHI, &result.ClinicianID, &result.PracticeID,
		&result.SessionDate, &result.SessionNumber, &result.Modality, &result.Mode,
		&result.DurationMin, &result.PresentingIssue, &result.ClinicalNotes,
		&result.RiskAssessment, &result.Intervention, &result.Outcome,
		&result.Homework, &result.NextSessionDate,
		&result.BillingType, &result.FeeInCents,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		return session.Session{}, fmt.Errorf("insert session: %w", err)
	}
	return result, nil
}

func (s *Server) updateSession(ctx context.Context, sess session.Session) (session.Session, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE counselling_sessions
		 SET clinician_id      = @clinician_id,
		     practice_id       = @practice_id,
		     session_date      = @session_date,
		     session_number    = @session_number,
		     modality          = @modality,
		     mode              = @mode,
		     duration_min      = @duration_min,
		     presenting_issue  = @presenting_issue,
		     clinical_notes    = @clinical_notes,
		     risk_assessment   = @risk_assessment,
		     intervention      = @intervention,
		     outcome           = @outcome,
		     homework          = @homework,
		     next_session_date = @next_session_date,
		     billing_type      = @billing_type,
		     fee_in_cents      = @fee_in_cents,
		     updated_at        = @updated_at
		 WHERE id = @id
		 RETURNING `+sessionSelectCols,
		db.NamedArgs{
			"id":                sess.ID,
			"clinician_id":      sess.ClinicianID,
			"practice_id":       sess.PracticeID,
			"session_date":      sess.SessionDate,
			"session_number":    sess.SessionNumber,
			"modality":          sess.Modality,
			"mode":              sess.Mode,
			"duration_min":      sess.DurationMin,
			"presenting_issue":  sess.PresentingIssue,
			"clinical_notes":    sess.ClinicalNotes,
			"risk_assessment":   sess.RiskAssessment,
			"intervention":      sess.Intervention,
			"outcome":           sess.Outcome,
			"homework":          sess.Homework,
			"next_session_date": sess.NextSessionDate,
			"billing_type":      sess.BillingType,
			"fee_in_cents":      sess.FeeInCents,
			"updated_at":        sess.UpdatedAt,
		},
	)
	var result session.Session
	if err := row.Scan(
		&result.ID, &result.ClientNHI, &result.ClinicianID, &result.PracticeID,
		&result.SessionDate, &result.SessionNumber, &result.Modality, &result.Mode,
		&result.DurationMin, &result.PresentingIssue, &result.ClinicalNotes,
		&result.RiskAssessment, &result.Intervention, &result.Outcome,
		&result.Homework, &result.NextSessionDate,
		&result.BillingType, &result.FeeInCents,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return session.Session{}, errNotFound
		}
		return session.Session{}, fmt.Errorf("update session: %w", err)
	}
	return result, nil
}

// ---- Private Clients ----

const privateClientSelectCols = `id, name_enc, email_enc, phone_enc, nhi_enc,
	employer, notes, active, created_at, updated_at`

func (s *Server) listPrivateClients(ctx context.Context) ([]private.PrivateClient, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+privateClientSelectCols+`
		 FROM counselling_private_clients
		 WHERE active = TRUE
		 ORDER BY created_at DESC
		 LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("query private clients: %w", err)
	}
	defer rows.Close()

	var results []private.PrivateClient
	for rows.Next() {
		var c private.PrivateClient
		if err := rows.Scan(
			&c.ID, &c.NameEnc, &c.EmailEnc, &c.PhoneEnc, &c.NHIEnc,
			&c.Employer, &c.Notes, &c.Active,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan private client: %w", err)
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

func (s *Server) createPrivateClient(ctx context.Context, c private.PrivateClient) (private.PrivateClient, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO counselling_private_clients
		   (id, name_enc, email_enc, phone_enc, nhi_enc,
		    employer, notes, active, created_at, updated_at)
		 VALUES
		   (@id, @name_enc, @email_enc, @phone_enc, @nhi_enc,
		    @employer, @notes, @active, @created_at, @updated_at)
		 RETURNING `+privateClientSelectCols,
		db.NamedArgs{
			"id":         c.ID,
			"name_enc":   c.NameEnc,
			"email_enc":  c.EmailEnc,
			"phone_enc":  c.PhoneEnc,
			"nhi_enc":    c.NHIEnc,
			"employer":   c.Employer,
			"notes":      c.Notes,
			"active":     c.Active,
			"created_at": c.CreatedAt,
			"updated_at": c.UpdatedAt,
		},
	)
	var result private.PrivateClient
	if err := row.Scan(
		&result.ID, &result.NameEnc, &result.EmailEnc, &result.PhoneEnc, &result.NHIEnc,
		&result.Employer, &result.Notes, &result.Active,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		return private.PrivateClient{}, fmt.Errorf("insert private client: %w", err)
	}
	return result, nil
}

// ---- Private Invoices ----

const invoiceSelectCols = `id, client_nhi, invoice_number, sessions, session_fee,
	total_amount, tax_amount, status, due_date, paid_date, created_at, updated_at`

func (s *Server) createPrivateInvoice(ctx context.Context, inv private.Invoice) (private.Invoice, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO counselling_private_invoices
		   (id, client_nhi, invoice_number, sessions, session_fee,
		    total_amount, tax_amount, status, due_date, paid_date,
		    created_at, updated_at)
		 VALUES
		   (@id, @client_nhi, @invoice_number, @sessions, @session_fee,
		    @total_amount, @tax_amount, @status, @due_date, @paid_date,
		    @created_at, @updated_at)
		 RETURNING `+invoiceSelectCols,
		db.NamedArgs{
			"id":             inv.ID,
			"client_nhi":     inv.ClientNHI,
			"invoice_number": inv.InvoiceNumber,
			"sessions":       inv.Sessions,
			"session_fee":    inv.SessionFee,
			"total_amount":   inv.TotalAmount,
			"tax_amount":     inv.TaxAmount,
			"status":         inv.Status,
			"due_date":       inv.DueDate,
			"paid_date":      inv.PaidDate,
			"created_at":     inv.CreatedAt,
			"updated_at":     inv.UpdatedAt,
		},
	)
	var result private.Invoice
	if err := row.Scan(
		&result.ID, &result.ClientNHI, &result.InvoiceNumber,
		&result.Sessions, &result.SessionFee, &result.TotalAmount,
		&result.TaxAmount, &result.Status, &result.DueDate, &result.PaidDate,
		&result.CreatedAt, &result.UpdatedAt,
	); err != nil {
		return private.Invoice{}, fmt.Errorf("insert private invoice: %w", err)
	}
	return result, nil
}
