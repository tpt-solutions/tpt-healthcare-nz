-- Health billing: invoices.
CREATE TABLE IF NOT EXISTS billing_invoices (
    id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id        TEXT NOT NULL DEFAULT '',
    source_module    TEXT NOT NULL DEFAULT '',
    source_ref_id    TEXT NOT NULL DEFAULT '',
    patient_nhi_enc  BYTEA NOT NULL,
    funding_type     TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'draft',
    total_amount     NUMERIC(12,2) NOT NULL DEFAULT 0,
    subsidy_amount   NUMERIC(12,2) NOT NULL DEFAULT 0,
    patient_amount   NUMERIC(12,2) NOT NULL DEFAULT 0,
    issued_at        TIMESTAMPTZ,
    due_at           TIMESTAMPTZ,
    paid_at          TIMESTAMPTZ,
    notes            TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_inv_tenant ON billing_invoices(tenant_id);
CREATE INDEX IF NOT EXISTS idx_billing_inv_status ON billing_invoices(status);
CREATE INDEX IF NOT EXISTS idx_billing_inv_source ON billing_invoices(source_module);

-- Health billing: invoice line items.
CREATE TABLE IF NOT EXISTS billing_invoice_lines (
    id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    invoice_id       TEXT NOT NULL REFERENCES billing_invoices(id) ON DELETE CASCADE,
    service_code     TEXT NOT NULL DEFAULT '',
    description      TEXT NOT NULL DEFAULT '',
    funding_type     TEXT NOT NULL DEFAULT '',
    quantity         INT NOT NULL DEFAULT 1,
    unit_fee         NUMERIC(12,2) NOT NULL DEFAULT 0,
    subsidy_amount   NUMERIC(12,2) NOT NULL DEFAULT 0,
    provider_hpi     TEXT NOT NULL DEFAULT '',
    service_date     TIMESTAMPTZ NOT NULL DEFAULT now(),
    diagnosis_code   TEXT NOT NULL DEFAULT '',
    notes            TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_billing_line_invoice ON billing_invoice_lines(invoice_id);

-- Health billing: payments.
CREATE TABLE IF NOT EXISTS billing_payments (
    id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id        TEXT NOT NULL DEFAULT '',
    invoice_id       TEXT,
    payment_method   TEXT NOT NULL DEFAULT '',
    amount           NUMERIC(12,2) NOT NULL DEFAULT 0,
    reference        TEXT NOT NULL DEFAULT '',
    payer            TEXT NOT NULL DEFAULT '',
    payment_date     TIMESTAMPTZ NOT NULL DEFAULT now(),
    reconciled       BOOLEAN NOT NULL DEFAULT FALSE,
    reconciled_at    TIMESTAMPTZ,
    notes            TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_pay_tenant ON billing_payments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_billing_pay_invoice ON billing_payments(invoice_id);
CREATE INDEX IF NOT EXISTS idx_billing_pay_reconciled ON billing_payments(reconciled);

-- Health billing: ACC claims.
CREATE TABLE IF NOT EXISTS billing_acc_claims (
    id                   TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id            TEXT NOT NULL DEFAULT '',
    source_module        TEXT NOT NULL DEFAULT '',
    acc_claim_number     TEXT NOT NULL DEFAULT '',
    purchase_order_number TEXT NOT NULL DEFAULT '',
    patient_nhi_enc      BYTEA NOT NULL,
    provider_hpi         TEXT NOT NULL DEFAULT '',
    date_of_accident     TIMESTAMPTZ NOT NULL,
    injury_description   TEXT NOT NULL DEFAULT '',
    diagnosis_codes      JSONB NOT NULL DEFAULT '[]',
    discipline           TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'pending',
    lodged_at            TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_acc_tenant ON billing_acc_claims(tenant_id);
CREATE INDEX IF NOT EXISTS idx_billing_acc_status ON billing_acc_claims(status);

-- Health billing: ACC purchase orders.
CREATE TABLE IF NOT EXISTS billing_acc_purchase_orders (
    id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id        TEXT NOT NULL DEFAULT '',
    claim_id         TEXT NOT NULL REFERENCES billing_acc_claims(id) ON DELETE CASCADE,
    po_number        TEXT NOT NULL DEFAULT '',
    discipline       TEXT NOT NULL DEFAULT '',
    max_sessions     INT NOT NULL DEFAULT 0,
    used_sessions    INT NOT NULL DEFAULT 0,
    fee_per_session  NUMERIC(12,2) NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'active',
    expiry_date      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_po_claim ON billing_acc_purchase_orders(claim_id);
CREATE INDEX IF NOT EXISTS idx_billing_po_status ON billing_acc_purchase_orders(status);

-- Health billing: insurance claims.
CREATE TABLE IF NOT EXISTS billing_insurance_claims (
    id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id        TEXT NOT NULL DEFAULT '',
    invoice_id       TEXT,
    patient_nhi_enc  BYTEA NOT NULL,
    insurer          TEXT NOT NULL DEFAULT '',
    policy_number    TEXT NOT NULL DEFAULT '',
    member_id        TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'draft',
    claimed_amount   NUMERIC(12,2) NOT NULL DEFAULT 0,
    approved_amount  NUMERIC(12,2) NOT NULL DEFAULT 0,
    insurer_reference TEXT NOT NULL DEFAULT '',
    submitted_at     TIMESTAMPTZ,
    decision_at      TIMESTAMPTZ,
    paid_at          TIMESTAMPTZ,
    decline_reason   TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_ins_tenant ON billing_insurance_claims(tenant_id);
CREATE INDEX IF NOT EXISTS idx_billing_ins_status ON billing_insurance_claims(status);

-- Health billing: PHARMAC subsidy claims.
CREATE TABLE IF NOT EXISTS billing_pharmac_claims (
    id                    TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id             TEXT NOT NULL DEFAULT '',
    pharmacy_hsp_no       TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'draft',
    claim_period_start    TIMESTAMPTZ NOT NULL,
    claim_period_end      TIMESTAMPTZ NOT NULL,
    source_dispense_ids   JSONB NOT NULL DEFAULT '[]',
    total_subsidy_amount  NUMERIC(12,2) NOT NULL DEFAULT 0,
    pharmac_reference_no  TEXT NOT NULL DEFAULT '',
    submitted_at          TIMESTAMPTZ,
    paid_at               TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_billing_pharmac_tenant ON billing_pharmac_claims(tenant_id);
CREATE INDEX IF NOT EXISTS idx_billing_pharmac_status ON billing_pharmac_claims(status);

-- Health billing: reconciliation import batches.
CREATE TABLE IF NOT EXISTS billing_import_batches (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id       TEXT NOT NULL DEFAULT '',
    source          TEXT NOT NULL DEFAULT '',
    imported_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    record_count    INT NOT NULL DEFAULT 0,
    records         JSONB NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS idx_billing_import_tenant ON billing_import_batches(tenant_id);
