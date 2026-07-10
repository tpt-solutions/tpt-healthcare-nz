package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errNotFound = errors.New("record not found")

// queryDB holds the shared database and encryption dependencies for query functions.
type queryDB struct {
	pool *pgxpool.Pool
	enc  *encryption.Encryptor
}

func newQueryDB(pool *pgxpool.Pool, enc *encryption.Encryptor) queryDB {
	return queryDB{pool: pool, enc: enc}
}

// ---------- Invoice queries ----------

func (q queryDB) listInvoices(ctx context.Context, tenantID, status, sourceModule, fundingType string) ([]Invoice, error) {
	sql := `SELECT id, tenant_id, source_module, source_ref_id, patient_nhi_enc,
	               funding_type, status, total_amount, subsidy_amount, patient_amount,
	               issued_at, due_at, paid_at, notes, created_at, updated_at
	        FROM billing_invoices WHERE 1=1`
	args := db.NamedArgs{}

	if tenantID != "" {
		sql += ` AND tenant_id = @tenant_id`
		args["tenant_id"] = tenantID
	}
	if status != "" {
		sql += ` AND status = @status`
		args["status"] = status
	}
	if sourceModule != "" {
		sql += ` AND source_module = @source_module`
		args["source_module"] = sourceModule
	}
	if fundingType != "" {
		sql += ` AND funding_type = @funding_type`
		args["funding_type"] = fundingType
	}
	sql += ` ORDER BY created_at DESC`

	rows, err := q.pool.Query(ctx, sql, args)
	if err != nil {
		return nil, fmt.Errorf("list invoices: %w", err)
	}
	defer rows.Close()

	var invoices []Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows, q.enc)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list invoices rows: %w", err)
	}
	return invoices, nil
}

// dbRow is satisfied by both pgx.Row and pgx.Rows.
type dbRow interface {
	Scan(dest ...any) error
}

func scanInvoice(row dbRow, enc *encryption.Encryptor) (Invoice, error) {
	var inv Invoice
	var issuedAt, dueAt, paidAt *time.Time
	var nhiEnc []byte

	if err := row.Scan(
		&inv.ID, &inv.TenantID, &inv.SourceModule, &inv.SourceRefID, &nhiEnc,
		&inv.FundingType, &inv.Status, &inv.TotalAmountNZD, &inv.SubsidyAmountNZD,
		&inv.PatientAmountNZD, &issuedAt, &dueAt, &paidAt, &inv.Notes,
		&inv.CreatedAt, &inv.UpdatedAt,
	); err != nil {
		return Invoice{}, err
	}
	if issuedAt != nil {
		inv.IssuedAt = issuedAt
	}
	if dueAt != nil {
		inv.DueAt = dueAt
	}
	if paidAt != nil {
		inv.PaidAt = paidAt
	}
	if enc != nil && len(nhiEnc) > 0 {
		decrypted, err := enc.Decrypt(nhiEnc)
		if err == nil {
			inv.PatientNHI = string(decrypted)
		}
	}
	return inv, nil
}

func (q queryDB) getInvoice(ctx context.Context, id string) (Invoice, error) {
	row := q.pool.QueryRow(ctx,
		`SELECT id, tenant_id, source_module, source_ref_id, patient_nhi_enc,
		        funding_type, status, total_amount, subsidy_amount, patient_amount,
		        issued_at, due_at, paid_at, notes, created_at, updated_at
		 FROM billing_invoices WHERE id = @id`,
		db.NamedArgs{"id": id},
	)
	inv, err := scanInvoice(row, q.enc)
	if err != nil {
		if db.IsNoRows(err) {
			return Invoice{}, errNotFound
		}
		return Invoice{}, fmt.Errorf("get invoice: %w", err)
	}
	// Load line items
	lines, err := q.getInvoiceLines(ctx, id)
	if err != nil {
		return Invoice{}, err
	}
	inv.Lines = lines
	return inv, nil
}

func (q queryDB) getInvoiceLines(ctx context.Context, invoiceID string) ([]InvoiceLine, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, service_code, description, funding_type, quantity, unit_fee,
		        subsidy_amount, provider_hpi, service_date, diagnosis_code, notes
		 FROM billing_invoice_lines WHERE invoice_id = @invoice_id ORDER BY service_date`,
		db.NamedArgs{"invoice_id": invoiceID},
	)
	if err != nil {
		return nil, fmt.Errorf("get invoice lines: %w", err)
	}
	defer rows.Close()

	var lines []InvoiceLine
	for rows.Next() {
		var l InvoiceLine
		if err := rows.Scan(
			&l.ID, &l.ServiceCode, &l.Description, &l.FundingType,
			&l.Quantity, &l.UnitFeeNZD, &l.SubsidyAmountNZD,
			&l.ProviderHPI, &l.ServiceDate, &l.DiagnosisCode, &l.Notes,
		); err != nil {
			return nil, fmt.Errorf("scan invoice line: %w", err)
		}
		lines = append(lines, l)
	}
	return lines, rows.Err()
}

func (q queryDB) insertInvoice(ctx context.Context, inv Invoice, lines []InvoiceLine) error {
	nhiEnc, err := q.enc.Encrypt([]byte(inv.PatientNHI))
	if err != nil {
		return fmt.Errorf("encrypt NHI: %w", err)
	}

	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO billing_invoices
		    (id, tenant_id, source_module, source_ref_id, patient_nhi_enc,
		     funding_type, status, total_amount, subsidy_amount, patient_amount, notes)
		 VALUES (@id, @tenant_id, @source_module, @source_ref_id, @patient_nhi_enc,
		         @funding_type, @status, @total_amount, @subsidy_amount, @patient_amount, @notes)`,
		db.NamedArgs{
			"id":               inv.ID,
			"tenant_id":        inv.TenantID,
			"source_module":    inv.SourceModule,
			"source_ref_id":    inv.SourceRefID,
			"patient_nhi_enc":  nhiEnc,
			"funding_type":     inv.FundingType,
			"status":           inv.Status,
			"total_amount":     inv.TotalAmountNZD,
			"subsidy_amount":   inv.SubsidyAmountNZD,
			"patient_amount":   inv.PatientAmountNZD,
			"notes":            inv.Notes,
		},
	)
	if err != nil {
		return fmt.Errorf("insert invoice: %w", err)
	}

	for _, l := range lines {
		_, err = tx.Exec(ctx,
			`INSERT INTO billing_invoice_lines
			    (id, invoice_id, service_code, description, funding_type, quantity,
			     unit_fee, subsidy_amount, provider_hpi, service_date, diagnosis_code, notes)
			 VALUES (@id, @invoice_id, @service_code, @description, @funding_type, @quantity,
			         @unit_fee, @subsidy_amount, @provider_hpi, @service_date, @diagnosis_code, @notes)`,
			db.NamedArgs{
				"id":              l.ID,
				"invoice_id":      inv.ID,
				"service_code":    l.ServiceCode,
				"description":     l.Description,
				"funding_type":    l.FundingType,
				"quantity":        l.Quantity,
				"unit_fee":        l.UnitFeeNZD,
				"subsidy_amount":  l.SubsidyAmountNZD,
				"provider_hpi":    l.ProviderHPI,
				"service_date":    l.ServiceDate,
				"diagnosis_code":  l.DiagnosisCode,
				"notes":           l.Notes,
			},
		)
		if err != nil {
			return fmt.Errorf("insert invoice line: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (q queryDB) updateInvoiceStatus(ctx context.Context, id string, status InvoiceStatus) error {
	tag, err := q.pool.Exec(ctx,
		`UPDATE billing_invoices SET status = @status, updated_at = now() WHERE id = @id`,
		db.NamedArgs{"id": id, "status": string(status)},
	)
	if err != nil {
		return fmt.Errorf("update invoice status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

func (q queryDB) issueInvoice(ctx context.Context, id string, dueAt time.Time) error {
	tag, err := q.pool.Exec(ctx,
		`UPDATE billing_invoices SET status = 'issued', issued_at = now(), due_at = @due_at, updated_at = now()
		 WHERE id = @id AND status = 'draft'`,
		db.NamedArgs{"id": id, "due_at": dueAt},
	)
	if err != nil {
		return fmt.Errorf("issue invoice: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

func (q queryDB) cancelInvoice(ctx context.Context, id string) error {
	tag, err := q.pool.Exec(ctx,
		`UPDATE billing_invoices SET status = 'cancelled', updated_at = now()
		 WHERE id = @id AND status IN ('draft', 'issued')`,
		db.NamedArgs{"id": id},
	)
	if err != nil {
		return fmt.Errorf("cancel invoice: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

func (q queryDB) insertPayment(ctx context.Context, p Payment) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO billing_payments
		    (id, tenant_id, invoice_id, payment_method, amount, reference, payer, payment_date, notes)
		 VALUES (@id, @tenant_id, @invoice_id, @payment_method, @amount, @reference, @payer, @payment_date, @notes)`,
		db.NamedArgs{
			"id":             p.ID,
			"tenant_id":      p.TenantID,
			"invoice_id":     p.InvoiceID,
			"payment_method": string(p.PaymentMethod),
			"amount":         p.AmountNZD,
			"reference":      p.Reference,
			"payer":          p.Payer,
			"payment_date":   p.PaymentDate,
			"notes":          p.Notes,
		},
	)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}

func (q queryDB) sumPayments(ctx context.Context, invoiceID string) (float64, error) {
	var total float64
	err := q.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM billing_payments WHERE invoice_id = @invoice_id`,
		db.NamedArgs{"invoice_id": invoiceID},
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum payments: %w", err)
	}
	return total, nil
}

func (q queryDB) getPatientAmount(ctx context.Context, invoiceID string) (float64, error) {
	var amt float64
	err := q.pool.QueryRow(ctx,
		`SELECT patient_amount FROM billing_invoices WHERE id = @id`,
		db.NamedArgs{"id": invoiceID},
	).Scan(&amt)
	if err != nil {
		return 0, fmt.Errorf("get patient amount: %w", err)
	}
	return amt, nil
}

func (q queryDB) listPayments(ctx context.Context, invoiceID string) ([]Payment, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, tenant_id, invoice_id, payment_method, amount, reference, payer,
		        payment_date, reconciled, reconciled_at, notes, created_at
		 FROM billing_payments WHERE invoice_id = @invoice_id ORDER BY payment_date`,
		db.NamedArgs{"invoice_id": invoiceID},
	)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()

	var payments []Payment
	for rows.Next() {
		var p Payment
		var reconciledAt *time.Time
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.InvoiceID, &p.PaymentMethod, &p.AmountNZD,
			&p.Reference, &p.Payer, &p.PaymentDate, &p.Reconciled, &reconciledAt,
			&p.Notes, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		if reconciledAt != nil {
			p.ReconciledAt = reconciledAt
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

// ---------- ACC Claim queries ----------

func (q queryDB) listACCClaims(ctx context.Context, tenantID, status, sourceModule string) ([]ACCClaim, error) {
	sql := `SELECT id, tenant_id, source_module, acc_claim_number, purchase_order_number,
	               patient_nhi_enc, provider_hpi, date_of_accident, injury_description,
	               diagnosis_codes, discipline, status, lodged_at, created_at, updated_at
	        FROM billing_acc_claims WHERE 1=1`
	args := db.NamedArgs{}

	if tenantID != "" {
		sql += ` AND tenant_id = @tenant_id`
		args["tenant_id"] = tenantID
	}
	if status != "" {
		sql += ` AND status = @status`
		args["status"] = status
	}
	if sourceModule != "" {
		sql += ` AND source_module = @source_module`
		args["source_module"] = sourceModule
	}
	sql += ` ORDER BY created_at DESC`

	rows, err := q.pool.Query(ctx, sql, args)
	if err != nil {
		return nil, fmt.Errorf("list ACC claims: %w", err)
	}
	defer rows.Close()

	var claims []ACCClaim
	for rows.Next() {
		c, err := scanACCClaim(rows, q.enc)
		if err != nil {
			return nil, err
		}
		claims = append(claims, c)
	}
	return claims, rows.Err()
}

func scanACCClaim(row dbRow, enc *encryption.Encryptor) (ACCClaim, error) {
	var c ACCClaim
	var nhiEnc []byte
	var diagCodes []byte // JSONB
	var lodgedAt *time.Time

	if err := row.Scan(
		&c.ID, &c.TenantID, &c.SourceModule, &c.ACCClaimNumber, &c.PurchaseOrderNumber,
		&nhiEnc, &c.ProviderHPI, &c.DateOfAccident, &c.InjuryDescription,
		&diagCodes, &c.Discipline, &c.Status, &lodgedAt, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return ACCClaim{}, err
	}
	if enc != nil && len(nhiEnc) > 0 {
		decrypted, err := enc.Decrypt(nhiEnc)
		if err == nil {
			c.PatientNHI = string(decrypted)
		}
	}
	if len(diagCodes) > 0 {
		_ = json.Unmarshal(diagCodes, &c.DiagnosisCodes)
	}
	if c.DiagnosisCodes == nil {
		c.DiagnosisCodes = []string{}
	}
	if lodgedAt != nil {
		c.LodgedAt = lodgedAt
	}
	return c, nil
}

func (q queryDB) getACCClaim(ctx context.Context, id string) (ACCClaim, error) {
	row := q.pool.QueryRow(ctx,
		`SELECT id, tenant_id, source_module, acc_claim_number, purchase_order_number,
		        patient_nhi_enc, provider_hpi, date_of_accident, injury_description,
		        diagnosis_codes, discipline, status, lodged_at, created_at, updated_at
		 FROM billing_acc_claims WHERE id = @id`,
		db.NamedArgs{"id": id},
	)
	c, err := scanACCClaim(row, q.enc)
	if err != nil {
		if db.IsNoRows(err) {
			return ACCClaim{}, errNotFound
		}
		return ACCClaim{}, fmt.Errorf("get ACC claim: %w", err)
	}
	return c, nil
}

func (q queryDB) insertACCClaim(ctx context.Context, c ACCClaim) error {
	nhiEnc, err := q.enc.Encrypt([]byte(c.PatientNHI))
	if err != nil {
		return fmt.Errorf("encrypt NHI: %w", err)
	}
	diagJSON, err := json.Marshal(c.DiagnosisCodes)
	if err != nil {
		return fmt.Errorf("marshal diagnosis codes: %w", err)
	}

	_, err = q.pool.Exec(ctx,
		`INSERT INTO billing_acc_claims
		    (id, tenant_id, source_module, patient_nhi_enc, provider_hpi,
		     date_of_accident, injury_description, diagnosis_codes, discipline, status)
		 VALUES (@id, @tenant_id, @source_module, @patient_nhi_enc, @provider_hpi,
		         @date_of_accident, @injury_description, @diagnosis_codes, @discipline, @status)`,
		db.NamedArgs{
			"id":                c.ID,
			"tenant_id":         c.TenantID,
			"source_module":     c.SourceModule,
			"patient_nhi_enc":   nhiEnc,
			"provider_hpi":      c.ProviderHPI,
			"date_of_accident":  c.DateOfAccident,
			"injury_description": c.InjuryDescription,
			"diagnosis_codes":   diagJSON,
			"discipline":        string(c.Discipline),
			"status":            string(c.Status),
		},
	)
	if err != nil {
		return fmt.Errorf("insert ACC claim: %w", err)
	}
	return nil
}

func (q queryDB) updateACCClaimStatus(ctx context.Context, id string, status ACCClaimStatus, claimNumber, poNumber string) error {
	tag, err := q.pool.Exec(ctx,
		`UPDATE billing_acc_claims
		 SET status = @status, acc_claim_number = @claim_number, purchase_order_number = @po_number,
		     lodged_at = now(), updated_at = now()
		 WHERE id = @id`,
		db.NamedArgs{
			"id":           id,
			"status":       string(status),
			"claim_number": claimNumber,
			"po_number":    poNumber,
		},
	)
	if err != nil {
		return fmt.Errorf("update ACC claim status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// ---------- ACC Purchase Order queries ----------

func (q queryDB) listACCPurchaseOrders(ctx context.Context, claimID, status, discipline string) ([]ACCPurchaseOrder, error) {
	sql := `SELECT id, tenant_id, claim_id, po_number, discipline, max_sessions, used_sessions,
	               fee_per_session, status, expiry_date, created_at, updated_at
	        FROM billing_acc_purchase_orders WHERE 1=1`
	args := db.NamedArgs{}

	if claimID != "" {
		sql += ` AND claim_id = @claim_id`
		args["claim_id"] = claimID
	}
	if status != "" {
		sql += ` AND status = @status`
		args["status"] = status
	}
	if discipline != "" {
		sql += ` AND discipline = @discipline`
		args["discipline"] = discipline
	}
	sql += ` ORDER BY created_at DESC`

	rows, err := q.pool.Query(ctx, sql, args)
	if err != nil {
		return nil, fmt.Errorf("list ACC purchase orders: %w", err)
	}
	defer rows.Close()

	var pos []ACCPurchaseOrder
	for rows.Next() {
		po, err := scanACCPurchaseOrder(rows)
		if err != nil {
			return nil, err
		}
		pos = append(pos, po)
	}
	return pos, rows.Err()
}

func scanACCPurchaseOrder(row dbRow) (ACCPurchaseOrder, error) {
	var po ACCPurchaseOrder
	var expiryDate *time.Time

	if err := row.Scan(
		&po.ID, &po.TenantID, &po.ClaimID, &po.PONumber, &po.Discipline,
		&po.MaxSessions, &po.UsedSessions, &po.FeePerSessionNZD,
		&po.Status, &expiryDate, &po.CreatedAt, &po.UpdatedAt,
	); err != nil {
		return ACCPurchaseOrder{}, err
	}
	if expiryDate != nil {
		po.ExpiryDate = expiryDate
	}
	po.RemainingSession = po.MaxSessions - po.UsedSessions
	return po, nil
}

func (q queryDB) getACCPurchaseOrder(ctx context.Context, id string) (ACCPurchaseOrder, error) {
	row := q.pool.QueryRow(ctx,
		`SELECT id, tenant_id, claim_id, po_number, discipline, max_sessions, used_sessions,
		        fee_per_session, status, expiry_date, created_at, updated_at
		 FROM billing_acc_purchase_orders WHERE id = @id`,
		db.NamedArgs{"id": id},
	)
	po, err := scanACCPurchaseOrder(row)
	if err != nil {
		if db.IsNoRows(err) {
			return ACCPurchaseOrder{}, errNotFound
		}
		return ACCPurchaseOrder{}, fmt.Errorf("get ACC purchase order: %w", err)
	}
	return po, nil
}

func (q queryDB) insertACCPurchaseOrder(ctx context.Context, po ACCPurchaseOrder) error {
	_, err := q.pool.Exec(ctx,
		`INSERT INTO billing_acc_purchase_orders
		    (id, tenant_id, claim_id, po_number, discipline, max_sessions, used_sessions,
		     fee_per_session, status, expiry_date)
		 VALUES (@id, @tenant_id, @claim_id, @po_number, @discipline, @max_sessions, @used_sessions,
		         @fee_per_session, @status, @expiry_date)`,
		db.NamedArgs{
			"id":             po.ID,
			"tenant_id":      po.TenantID,
			"claim_id":       po.ClaimID,
			"po_number":      po.PONumber,
			"discipline":     string(po.Discipline),
			"max_sessions":   po.MaxSessions,
			"used_sessions":  po.UsedSessions,
			"fee_per_session": po.FeePerSessionNZD,
			"status":         string(po.Status),
			"expiry_date":    po.ExpiryDate,
		},
	)
	if err != nil {
		return fmt.Errorf("insert ACC purchase order: %w", err)
	}
	return nil
}

func (q queryDB) consumeACCPurchaseOrderSession(ctx context.Context, id string) (ACCPurchaseOrder, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return ACCPurchaseOrder{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Load PO and check constraints
	po, err := scanACCPurchaseOrder(tx.QueryRow(ctx,
		`SELECT id, tenant_id, claim_id, po_number, discipline, max_sessions, used_sessions,
		        fee_per_session, status, expiry_date, created_at, updated_at
		 FROM billing_acc_purchase_orders WHERE id = @id FOR UPDATE`,
		db.NamedArgs{"id": id},
	))
	if err != nil {
		if db.IsNoRows(err) {
			return ACCPurchaseOrder{}, errNotFound
		}
		return ACCPurchaseOrder{}, fmt.Errorf("load ACC PO: %w", err)
	}
	if string(po.Status) != string(ACCPOStatusActive) {
		return ACCPurchaseOrder{}, fmt.Errorf("purchase order is not active")
	}
	if po.UsedSessions >= po.MaxSessions {
		return ACCPurchaseOrder{}, fmt.Errorf("purchase order sessions exhausted")
	}
	if po.ExpiryDate != nil && time.Now().UTC().After(*po.ExpiryDate) {
		return ACCPurchaseOrder{}, fmt.Errorf("purchase order expired")
	}

	newUsed := po.UsedSessions + 1
	newStatus := ACCPOStatusActive
	if newUsed >= po.MaxSessions {
		newStatus = ACCPOStatusExhausted
	}

	_, err = tx.Exec(ctx,
		`UPDATE billing_acc_purchase_orders
		 SET used_sessions = @used_sessions, status = @status, updated_at = now()
		 WHERE id = @id`,
		db.NamedArgs{
			"id":            id,
			"used_sessions": newUsed,
			"status":        string(newStatus),
		},
	)
	if err != nil {
		return ACCPurchaseOrder{}, fmt.Errorf("update ACC PO: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ACCPurchaseOrder{}, fmt.Errorf("commit: %w", err)
	}

	po.UsedSessions = newUsed
	po.Status = newStatus
	po.RemainingSession = po.MaxSessions - newUsed
	return po, nil
}

// ---------- Insurance Claim queries ----------

func (q queryDB) listInsuranceClaims(ctx context.Context, tenantID, status, insurer string) ([]InsuranceClaim, error) {
	sql := `SELECT id, tenant_id, invoice_id, patient_nhi_enc, insurer, policy_number, member_id,
	               status, claimed_amount, approved_amount, insurer_reference,
	               submitted_at, decision_at, paid_at, decline_reason, created_at, updated_at
	        FROM billing_insurance_claims WHERE 1=1`
	args := db.NamedArgs{}

	if tenantID != "" {
		sql += ` AND tenant_id = @tenant_id`
		args["tenant_id"] = tenantID
	}
	if status != "" {
		sql += ` AND status = @status`
		args["status"] = status
	}
	if insurer != "" {
		sql += ` AND insurer = @insurer`
		args["insurer"] = insurer
	}
	sql += ` ORDER BY created_at DESC`

	rows, err := q.pool.Query(ctx, sql, args)
	if err != nil {
		return nil, fmt.Errorf("list insurance claims: %w", err)
	}
	defer rows.Close()

	var claims []InsuranceClaim
	for rows.Next() {
		c, err := scanInsuranceClaim(rows, q.enc)
		if err != nil {
			return nil, err
		}
		claims = append(claims, c)
	}
	return claims, rows.Err()
}

func scanInsuranceClaim(row dbRow, enc *encryption.Encryptor) (InsuranceClaim, error) {
	var c InsuranceClaim
	var nhiEnc []byte
	var invoiceID *string
	var submittedAt, decisionAt, paidAt *time.Time

	if err := row.Scan(
		&c.ID, &c.TenantID, &invoiceID, &nhiEnc, &c.Insurer, &c.PolicyNumber, &c.MemberID,
		&c.Status, &c.ClaimedAmountNZD, &c.ApprovedAmountNZD, &c.InsurerReference,
		&submittedAt, &decisionAt, &paidAt, &c.DeclineReason, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return InsuranceClaim{}, err
	}
	if enc != nil && len(nhiEnc) > 0 {
		decrypted, err := enc.Decrypt(nhiEnc)
		if err == nil {
			c.PatientNHI = string(decrypted)
		}
	}
	if invoiceID != nil {
		c.InvoiceID = *invoiceID
	}
	if submittedAt != nil {
		c.SubmittedAt = submittedAt
	}
	if decisionAt != nil {
		c.DecisionAt = decisionAt
	}
	if paidAt != nil {
		c.PaidAt = paidAt
	}
	return c, nil
}

func (q queryDB) getInsuranceClaim(ctx context.Context, id string) (InsuranceClaim, error) {
	row := q.pool.QueryRow(ctx,
		`SELECT id, tenant_id, invoice_id, patient_nhi_enc, insurer, policy_number, member_id,
		        status, claimed_amount, approved_amount, insurer_reference,
		        submitted_at, decision_at, paid_at, decline_reason, created_at, updated_at
		 FROM billing_insurance_claims WHERE id = @id`,
		db.NamedArgs{"id": id},
	)
	c, err := scanInsuranceClaim(row, q.enc)
	if err != nil {
		if db.IsNoRows(err) {
			return InsuranceClaim{}, errNotFound
		}
		return InsuranceClaim{}, fmt.Errorf("get insurance claim: %w", err)
	}
	return c, nil
}

func (q queryDB) insertInsuranceClaim(ctx context.Context, c InsuranceClaim) error {
	nhiEnc, err := q.enc.Encrypt([]byte(c.PatientNHI))
	if err != nil {
		return fmt.Errorf("encrypt NHI: %w", err)
	}

	var invoiceID any
	if c.InvoiceID != "" {
		invoiceID = c.InvoiceID
	}

	_, err = q.pool.Exec(ctx,
		`INSERT INTO billing_insurance_claims
		    (id, tenant_id, invoice_id, patient_nhi_enc, insurer, policy_number, member_id,
		     status, claimed_amount)
		 VALUES (@id, @tenant_id, @invoice_id, @patient_nhi_enc, @insurer, @policy_number, @member_id,
		         @status, @claimed_amount)`,
		db.NamedArgs{
			"id":              c.ID,
			"tenant_id":       c.TenantID,
			"invoice_id":      invoiceID,
			"patient_nhi_enc": nhiEnc,
			"insurer":         string(c.Insurer),
			"policy_number":   c.PolicyNumber,
			"member_id":       c.MemberID,
			"status":          string(c.Status),
			"claimed_amount":  c.ClaimedAmountNZD,
		},
	)
	if err != nil {
		return fmt.Errorf("insert insurance claim: %w", err)
	}
	return nil
}

func (q queryDB) updateInsuranceClaimSubmitted(ctx context.Context, id string, insurerRef string) error {
	tag, err := q.pool.Exec(ctx,
		`UPDATE billing_insurance_claims
		 SET status = 'submitted', insurer_reference = @ref, submitted_at = now(), updated_at = now()
		 WHERE id = @id AND status = 'draft'`,
		db.NamedArgs{"id": id, "ref": insurerRef},
	)
	if err != nil {
		return fmt.Errorf("update insurance claim submitted: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// ---------- PHARMAC Claim queries ----------

func (q queryDB) listPHARMACClaims(ctx context.Context, tenantID, status, pharmacyHSPNo string) ([]PHARMACSubsidyClaim, error) {
	sql := `SELECT id, tenant_id, pharmacy_hsp_no, status, claim_period_start, claim_period_end,
	               source_dispense_ids, total_subsidy_amount, pharmac_reference_no,
	               submitted_at, paid_at, created_at, updated_at
	        FROM billing_pharmac_claims WHERE 1=1`
	args := db.NamedArgs{}

	if tenantID != "" {
		sql += ` AND tenant_id = @tenant_id`
		args["tenant_id"] = tenantID
	}
	if status != "" {
		sql += ` AND status = @status`
		args["status"] = status
	}
	if pharmacyHSPNo != "" {
		sql += ` AND pharmacy_hsp_no = @pharmacy_hsp_no`
		args["pharmacy_hsp_no"] = pharmacyHSPNo
	}
	sql += ` ORDER BY created_at DESC`

	rows, err := q.pool.Query(ctx, sql, args)
	if err != nil {
		return nil, fmt.Errorf("list PHARMAC claims: %w", err)
	}
	defer rows.Close()

	var claims []PHARMACSubsidyClaim
	for rows.Next() {
		c, err := scanPHARMACClaim(rows)
		if err != nil {
			return nil, err
		}
		claims = append(claims, c)
	}
	return claims, rows.Err()
}

func scanPHARMACClaim(row dbRow) (PHARMACSubsidyClaim, error) {
	var c PHARMACSubsidyClaim
	var dispenseIDs []byte // JSONB
	var submittedAt, paidAt *time.Time

	if err := row.Scan(
		&c.ID, &c.TenantID, &c.PharmacyHSPNo, &c.Status, &c.ClaimPeriodStart, &c.ClaimPeriodEnd,
		&dispenseIDs, &c.TotalSubsidyAmountNZD, &c.PHARMACReferenceNo,
		&submittedAt, &paidAt, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return PHARMACSubsidyClaim{}, err
	}
	if len(dispenseIDs) > 0 {
		_ = json.Unmarshal(dispenseIDs, &c.SourceDispenseIDs)
	}
	if c.SourceDispenseIDs == nil {
		c.SourceDispenseIDs = []string{}
	}
	if submittedAt != nil {
		c.SubmittedAt = submittedAt
	}
	if paidAt != nil {
		c.PaidAt = paidAt
	}
	return c, nil
}

func (q queryDB) getPHARMACClaim(ctx context.Context, id string) (PHARMACSubsidyClaim, error) {
	row := q.pool.QueryRow(ctx,
		`SELECT id, tenant_id, pharmacy_hsp_no, status, claim_period_start, claim_period_end,
		        source_dispense_ids, total_subsidy_amount, pharmac_reference_no,
		        submitted_at, paid_at, created_at, updated_at
		 FROM billing_pharmac_claims WHERE id = @id`,
		db.NamedArgs{"id": id},
	)
	c, err := scanPHARMACClaim(row)
	if err != nil {
		if db.IsNoRows(err) {
			return PHARMACSubsidyClaim{}, errNotFound
		}
		return PHARMACSubsidyClaim{}, fmt.Errorf("get PHARMAC claim: %w", err)
	}
	return c, nil
}

func (q queryDB) insertPHARMACClaim(ctx context.Context, c PHARMACSubsidyClaim) error {
	dispenseJSON, err := json.Marshal(c.SourceDispenseIDs)
	if err != nil {
		return fmt.Errorf("marshal dispense IDs: %w", err)
	}

	_, err = q.pool.Exec(ctx,
		`INSERT INTO billing_pharmac_claims
		    (id, tenant_id, pharmacy_hsp_no, status, claim_period_start, claim_period_end,
		     source_dispense_ids, total_subsidy_amount)
		 VALUES (@id, @tenant_id, @pharmacy_hsp_no, @status, @claim_period_start, @claim_period_end,
		         @source_dispense_ids, @total_subsidy_amount)`,
		db.NamedArgs{
			"id":                    c.ID,
			"tenant_id":             c.TenantID,
			"pharmacy_hsp_no":       c.PharmacyHSPNo,
			"status":                string(c.Status),
			"claim_period_start":    c.ClaimPeriodStart,
			"claim_period_end":      c.ClaimPeriodEnd,
			"source_dispense_ids":   dispenseJSON,
			"total_subsidy_amount":  c.TotalSubsidyAmountNZD,
		},
	)
	if err != nil {
		return fmt.Errorf("insert PHARMAC claim: %w", err)
	}
	return nil
}

func (q queryDB) updatePHARMACClaimSubmitted(ctx context.Context, id string, pharmacRef string) error {
	tag, err := q.pool.Exec(ctx,
		`UPDATE billing_pharmac_claims
		 SET status = 'submitted', pharmac_reference_no = @ref, submitted_at = now(), updated_at = now()
		 WHERE id = @id AND status = 'draft'`,
		db.NamedArgs{"id": id, "ref": pharmacRef},
	)
	if err != nil {
		return fmt.Errorf("update PHARMAC claim submitted: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// ---------- Reconciliation queries ----------

func (q queryDB) reconciliationSummary(ctx context.Context, tenantID string) (ReconciliationSummary, error) {
	summary := ReconciliationSummary{
		TenantID: tenantID,
		AsAt:     time.Now().UTC(),
		AgingBuckets: AgingBuckets{},
	}

	// Total outstanding (issued + overdue invoices)
	err := q.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(patient_amount), 0) FROM billing_invoices
		 WHERE tenant_id = @tenant_id AND status IN ('issued', 'overdue')`,
		db.NamedArgs{"tenant_id": tenantID},
	).Scan(&summary.TotalOutstandingNZD)
	if err != nil {
		return summary, fmt.Errorf("sum outstanding: %w", err)
	}

	// Overdue amount
	err = q.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(patient_amount), 0) FROM billing_invoices
		 WHERE tenant_id = @tenant_id AND status = 'overdue'`,
		db.NamedArgs{"tenant_id": tenantID},
	).Scan(&summary.OverdueAmountNZD)
	if err != nil {
		return summary, fmt.Errorf("sum overdue: %w", err)
	}

	// Unreconciled payments
	err = q.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM billing_payments
		 WHERE tenant_id = @tenant_id AND reconciled = false`,
		db.NamedArgs{"tenant_id": tenantID},
	).Scan(&summary.UnreconciledPaymentsNZD)
	if err != nil {
		return summary, fmt.Errorf("sum unreconciled: %w", err)
	}

	// Pending ACC amount
	err = q.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_amount), 0) FROM billing_invoices
		 WHERE tenant_id = @tenant_id AND funding_type = 'ACC' AND status IN ('issued', 'overdue')`,
		db.NamedArgs{"tenant_id": tenantID},
	).Scan(&summary.PendingACCAmountNZD)
	if err != nil {
		return summary, fmt.Errorf("sum pending ACC: %w", err)
	}

	// Pending PHARMAC amount
	err = q.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total_subsidy_amount), 0) FROM billing_pharmac_claims
		 WHERE tenant_id = @tenant_id AND status = 'submitted'`,
		db.NamedArgs{"tenant_id": tenantID},
	).Scan(&summary.PendingPHARMACAmountNZD)
	if err != nil {
		return summary, fmt.Errorf("sum pending PHARMAC: %w", err)
	}

	// Pending insurance amount
	err = q.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(claimed_amount), 0) FROM billing_insurance_claims
		 WHERE tenant_id = @tenant_id AND status = 'submitted'`,
		db.NamedArgs{"tenant_id": tenantID},
	).Scan(&summary.PendingInsuranceAmountNZD)
	if err != nil {
		return summary, fmt.Errorf("sum pending insurance: %w", err)
	}

	// Aging buckets
	now := time.Now().UTC()
	err = q.pool.QueryRow(ctx,
		`SELECT
		    COALESCE(SUM(CASE WHEN now() - issued_at <= interval '30 days' THEN patient_amount ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN now() - issued_at > interval '30 days' AND now() - issued_at <= interval '60 days' THEN patient_amount ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN now() - issued_at > interval '60 days' AND now() - issued_at <= interval '90 days' THEN patient_amount ELSE 0 END), 0),
		    COALESCE(SUM(CASE WHEN now() - issued_at > interval '90 days' THEN patient_amount ELSE 0 END), 0)
		 FROM billing_invoices
		 WHERE tenant_id = @tenant_id AND status IN ('issued', 'overdue') AND issued_at IS NOT NULL`,
		db.NamedArgs{"tenant_id": tenantID},
	).Scan(
		&summary.AgingBuckets.Current,
		&summary.AgingBuckets.Days31to60,
		&summary.AgingBuckets.Days61to90,
		&summary.AgingBuckets.Over90Days,
	)
	if err != nil {
		return summary, fmt.Errorf("compute aging: %w", err)
	}

	_ = now
	return summary, nil
}

func (q queryDB) insertImportBatch(ctx context.Context, batch ImportBatch) error {
	recordsJSON, err := json.Marshal(batch.Records)
	if err != nil {
		return fmt.Errorf("marshal records: %w", err)
	}

	_, err = q.pool.Exec(ctx,
		`INSERT INTO billing_import_batches (id, tenant_id, source, record_count, records)
		 VALUES (@id, @tenant_id, @source, @record_count, @records)`,
		db.NamedArgs{
			"id":           batch.ID,
			"tenant_id":    batch.TenantID,
			"source":       batch.Source,
			"record_count": batch.RecordCount,
			"records":      recordsJSON,
		},
	)
	if err != nil {
		return fmt.Errorf("insert import batch: %w", err)
	}
	return nil
}

func (q queryDB) listUnmatchedPayments(ctx context.Context, tenantID string) ([]Payment, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, tenant_id, invoice_id, payment_method, amount, reference, payer,
		        payment_date, reconciled, reconciled_at, notes, created_at
		 FROM billing_payments
		 WHERE tenant_id = @tenant_id AND reconciled = false AND invoice_id IS NULL
		 ORDER BY payment_date DESC`,
		db.NamedArgs{"tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("list unmatched payments: %w", err)
	}
	defer rows.Close()

	var payments []Payment
	for rows.Next() {
		var p Payment
		var reconciledAt *time.Time
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.InvoiceID, &p.PaymentMethod, &p.AmountNZD,
			&p.Reference, &p.Payer, &p.PaymentDate, &p.Reconciled, &reconciledAt,
			&p.Notes, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan unmatched payment: %w", err)
		}
		if reconciledAt != nil {
			p.ReconciledAt = reconciledAt
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

func (q queryDB) matchPayment(ctx context.Context, paymentID, invoiceID string) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Load payment
	var pAmount float64
	err = tx.QueryRow(ctx,
		`SELECT amount FROM billing_payments WHERE id = @id AND reconciled = false AND invoice_id IS NULL FOR UPDATE`,
		db.NamedArgs{"id": paymentID},
	).Scan(&pAmount)
	if err != nil {
		if db.IsNoRows(err) {
			return errNotFound
		}
		return fmt.Errorf("load payment: %w", err)
	}

	// Load invoice patient_amount
	var patientAmount float64
	err = tx.QueryRow(ctx,
		`SELECT patient_amount FROM billing_invoices WHERE id = @id AND status IN ('issued', 'overdue')`,
		db.NamedArgs{"id": invoiceID},
	).Scan(&patientAmount)
	if err != nil {
		if db.IsNoRows(err) {
			return errNotFound
		}
		return fmt.Errorf("load invoice: %w", err)
	}

	// Check outstanding
	var totalPaid float64
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM billing_payments WHERE invoice_id = @id AND reconciled = true`,
		db.NamedArgs{"id": invoiceID},
	).Scan(&totalPaid)
	if err != nil {
		return fmt.Errorf("sum existing payments: %w", err)
	}
	outstanding := patientAmount - totalPaid
	if pAmount > outstanding+0.01 {
		return fmt.Errorf("payment amount exceeds outstanding balance")
	}

	// Update payment
	_, err = tx.Exec(ctx,
		`UPDATE billing_payments SET invoice_id = @invoice_id, reconciled = true, reconciled_at = now()
		 WHERE id = @id`,
		db.NamedArgs{"id": paymentID, "invoice_id": invoiceID},
	)
	if err != nil {
		return fmt.Errorf("update payment: %w", err)
	}

	// Check if fully paid
	newTotal := totalPaid + pAmount
	if newTotal >= patientAmount-0.01 {
		_, err = tx.Exec(ctx,
			`UPDATE billing_invoices SET status = 'paid', paid_at = now(), updated_at = now()
			 WHERE id = @id`,
			db.NamedArgs{"id": invoiceID},
		)
		if err != nil {
			return fmt.Errorf("mark invoice paid: %w", err)
		}
	}

	return tx.Commit(ctx)
}
