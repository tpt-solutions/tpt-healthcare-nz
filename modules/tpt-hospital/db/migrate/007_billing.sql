-- Hospital billing: casemix invoices (AR-DRG)
CREATE TABLE IF NOT EXISTS hospital_invoices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id    UUID NOT NULL REFERENCES hospital_admissions(id),
    patient_id      UUID NOT NULL,
    ar_drg          TEXT,
    funding_source  TEXT NOT NULL DEFAULT 'dhb',
    status          TEXT NOT NULL DEFAULT 'draft',
    lines           JSONB NOT NULL DEFAULT '[]',
    subtotal_nzd    NUMERIC(12,2) NOT NULL DEFAULT 0,
    gst_amount_nzd  NUMERIC(12,2) NOT NULL DEFAULT 0,
    total_nzd       NUMERIC(12,2) NOT NULL DEFAULT 0,
    notes           TEXT,
    tenant_id       UUID NOT NULL,
    submitted_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hospital_invoices_admission ON hospital_invoices (admission_id);
CREATE INDEX IF NOT EXISTS idx_hospital_invoices_tenant    ON hospital_invoices (tenant_id, status);
