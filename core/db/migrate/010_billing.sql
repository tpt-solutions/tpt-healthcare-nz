-- 010_billing.sql
-- Cross-module billing tables: invoices, ACC claims, PHARMAC subsidy claims,
-- health insurance claims, payments, and reconciliation.
--
-- Privacy Act 2020 / HIPC Rule 5 compliance note:
-- patient_nhi columns store AES-256-GCM ciphertext (encrypted by the application
-- via core/encryption before INSERT). Never store NHIs in plaintext in these tables.
-- billing_payments.payer may contain patient names — treat as PHI.

-- ---------------------------------------------------------------------------
-- Invoices
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS billing_invoices (
    id                   UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    tenant_id            UUID        NOT NULL,
    source_module        TEXT        NOT NULL,
    source_ref_id        UUID,
    patient_nhi          TEXT        NOT NULL,  -- AES-256-GCM ciphertext
    funding_type         TEXT        NOT NULL
        CHECK (funding_type IN ('ACC', 'PHO', 'PRIVATE', 'DHB', 'VAC')),
    status               TEXT        NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'issued', 'overdue', 'paid', 'cancelled', 'written_off')),
    total_amount_cents   BIGINT      NOT NULL DEFAULT 0,
    subsidy_amount_cents BIGINT      NOT NULL DEFAULT 0,
    patient_amount_cents BIGINT      NOT NULL DEFAULT 0,
    currency             TEXT        NOT NULL DEFAULT 'NZD',
    issued_at            TIMESTAMPTZ,
    due_at               TIMESTAMPTZ,
    paid_at              TIMESTAMPTZ,
    notes                TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS billing_invoices_tenant_id_idx      ON billing_invoices (tenant_id);
CREATE INDEX IF NOT EXISTS billing_invoices_patient_nhi_idx    ON billing_invoices (patient_nhi);
CREATE INDEX IF NOT EXISTS billing_invoices_status_idx         ON billing_invoices (status);
CREATE INDEX IF NOT EXISTS billing_invoices_source_module_idx  ON billing_invoices (source_module);
CREATE INDEX IF NOT EXISTS billing_invoices_due_at_idx         ON billing_invoices (due_at)
    WHERE due_at IS NOT NULL AND status IN ('issued', 'overdue');

-- ---------------------------------------------------------------------------
-- Invoice line items
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS billing_invoice_lines (
    id                   UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    invoice_id           UUID        NOT NULL REFERENCES billing_invoices (id),
    service_code         TEXT        NOT NULL,
    description          TEXT        NOT NULL,
    funding_type         TEXT        NOT NULL,
    quantity             INT         NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_fee_cents       BIGINT      NOT NULL DEFAULT 0,
    subsidy_amount_cents BIGINT      NOT NULL DEFAULT 0,
    provider_hpi         TEXT        NOT NULL,
    service_date         DATE        NOT NULL,
    diagnosis_code       TEXT,
    notes                TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS billing_invoice_lines_invoice_id_idx ON billing_invoice_lines (invoice_id);

-- ---------------------------------------------------------------------------
-- ACC claims (cross-module, single source of truth)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS billing_acc_claims (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    tenant_id           UUID        NOT NULL,
    invoice_id          UUID        REFERENCES billing_invoices (id),
    source_module       TEXT        NOT NULL,
    acc_claim_number    TEXT,
    purchase_order_no   TEXT,
    patient_nhi         TEXT        NOT NULL,  -- AES-256-GCM ciphertext
    provider_hpi        TEXT        NOT NULL,
    discipline          TEXT        NOT NULL,
    date_of_accident    DATE        NOT NULL,
    injury_description  TEXT        NOT NULL,
    diagnosis_codes     TEXT[]      NOT NULL DEFAULT '{}',
    status              TEXT        NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'active', 'declined', 'complete', 'disputed')),
    lodged_at           TIMESTAMPTZ,
    acc_response        JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS billing_acc_claims_tenant_id_idx       ON billing_acc_claims (tenant_id);
CREATE INDEX IF NOT EXISTS billing_acc_claims_patient_nhi_idx     ON billing_acc_claims (patient_nhi);
CREATE INDEX IF NOT EXISTS billing_acc_claims_status_idx          ON billing_acc_claims (status);
CREATE INDEX IF NOT EXISTS billing_acc_claims_claim_number_idx    ON billing_acc_claims (acc_claim_number)
    WHERE acc_claim_number IS NOT NULL;

-- ---------------------------------------------------------------------------
-- ACC purchase orders
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS billing_acc_purchase_orders (
    id                  UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    tenant_id           UUID        NOT NULL,
    claim_id            UUID        NOT NULL REFERENCES billing_acc_claims (id),
    po_number           TEXT        NOT NULL,
    discipline          TEXT        NOT NULL,
    max_sessions        INT         NOT NULL DEFAULT 0 CHECK (max_sessions >= 0),
    used_sessions       INT         NOT NULL DEFAULT 0 CHECK (used_sessions >= 0),
    fee_per_session_cents BIGINT    NOT NULL DEFAULT 0,
    status              TEXT        NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'exhausted', 'cancelled')),
    expiry_date         DATE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT billing_acc_po_sessions_check CHECK (used_sessions <= max_sessions)
);

CREATE UNIQUE INDEX IF NOT EXISTS billing_acc_purchase_orders_po_number_idx ON billing_acc_purchase_orders (po_number);
CREATE INDEX IF NOT EXISTS billing_acc_purchase_orders_claim_id_idx        ON billing_acc_purchase_orders (claim_id);
CREATE INDEX IF NOT EXISTS billing_acc_purchase_orders_tenant_id_idx       ON billing_acc_purchase_orders (tenant_id);
CREATE INDEX IF NOT EXISTS billing_acc_purchase_orders_status_idx          ON billing_acc_purchase_orders (status);

-- ---------------------------------------------------------------------------
-- PHARMAC subsidy claims
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS billing_pharmac_claims (
    id                    UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    tenant_id             UUID        NOT NULL,
    pharmacy_hsp_no       TEXT        NOT NULL,
    status                TEXT        NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'accepted', 'rejected', 'paid')),
    claim_period_start    DATE        NOT NULL,
    claim_period_end      DATE        NOT NULL,
    total_subsidy_cents   BIGINT      NOT NULL DEFAULT 0,
    pharmac_reference_no  TEXT,
    submitted_at          TIMESTAMPTZ,
    paid_at               TIMESTAMPTZ,
    source_dispense_ids   TEXT[]      NOT NULL DEFAULT '{}',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT billing_pharmac_claims_period_check CHECK (claim_period_end >= claim_period_start)
);

CREATE INDEX IF NOT EXISTS billing_pharmac_claims_tenant_id_idx       ON billing_pharmac_claims (tenant_id);
CREATE INDEX IF NOT EXISTS billing_pharmac_claims_status_idx          ON billing_pharmac_claims (status);
CREATE INDEX IF NOT EXISTS billing_pharmac_claims_pharmacy_hsp_no_idx ON billing_pharmac_claims (pharmacy_hsp_no);

-- ---------------------------------------------------------------------------
-- Health insurance claims
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS billing_insurance_claims (
    id                    UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    tenant_id             UUID        NOT NULL,
    invoice_id            UUID        REFERENCES billing_invoices (id),
    patient_nhi           TEXT        NOT NULL,  -- AES-256-GCM ciphertext
    insurer               TEXT        NOT NULL
        CHECK (insurer IN ('SOUTHERN_CROSS', 'NIB', 'AIA', 'PARTNERS_LIFE', 'ACCURO', 'OTHER')),
    policy_number         TEXT        NOT NULL,
    member_id             TEXT        NOT NULL,
    status                TEXT        NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'approved', 'declined', 'paid', 'appealed')),
    claimed_amount_cents  BIGINT      NOT NULL DEFAULT 0,
    approved_amount_cents BIGINT,
    insurer_reference     TEXT,
    submitted_at          TIMESTAMPTZ,
    decision_at           TIMESTAMPTZ,
    paid_at               TIMESTAMPTZ,
    decline_reason        TEXT,
    insurer_response      JSONB,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS billing_insurance_claims_tenant_id_idx   ON billing_insurance_claims (tenant_id);
CREATE INDEX IF NOT EXISTS billing_insurance_claims_patient_nhi_idx ON billing_insurance_claims (patient_nhi);
CREATE INDEX IF NOT EXISTS billing_insurance_claims_status_idx      ON billing_insurance_claims (status);
CREATE INDEX IF NOT EXISTS billing_insurance_claims_insurer_idx     ON billing_insurance_claims (insurer);

-- ---------------------------------------------------------------------------
-- Payments
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS billing_payments (
    id             UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    tenant_id      UUID        NOT NULL,
    invoice_id     UUID        REFERENCES billing_invoices (id),
    payment_method TEXT        NOT NULL
        CHECK (payment_method IN (
            'EFTPOS', 'CASH', 'INTERNET_BANKING', 'DIRECT_DEBIT',
            'INSURANCE', 'ACC', 'PHO', 'DHB', 'WRITEOFF'
        )),
    amount_cents   BIGINT      NOT NULL CHECK (amount_cents > 0),
    reference      TEXT,
    payer          TEXT,       -- may contain PHI (patient/insurer name) — treat as sensitive
    payment_date   DATE        NOT NULL,
    reconciled     BOOLEAN     NOT NULL DEFAULT false,
    reconciled_at  TIMESTAMPTZ,
    notes          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS billing_payments_tenant_id_idx    ON billing_payments (tenant_id);
CREATE INDEX IF NOT EXISTS billing_payments_invoice_id_idx   ON billing_payments (invoice_id);
CREATE INDEX IF NOT EXISTS billing_payments_reconciled_idx   ON billing_payments (reconciled) WHERE reconciled = false;
CREATE INDEX IF NOT EXISTS billing_payments_payment_date_idx ON billing_payments (payment_date);
