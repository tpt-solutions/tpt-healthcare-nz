-- tpt-dental: Dental Charting
-- Stores the full FDI two-digit charted state for each patient.
-- One row per patient per tenant (upserted on every SaveChart call).

CREATE TABLE IF NOT EXISTS dental_charts (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL,
    patient_nhi  VARCHAR(7)  NOT NULL,
    visit_id     TEXT,
    dentition    VARCHAR(20) NOT NULL DEFAULT 'permanent',
    entries      JSONB       NOT NULL DEFAULT '[]',
    clinician_id TEXT        NOT NULL DEFAULT '',
    practice_id  TEXT        NOT NULL DEFAULT '',
    chart_date   BIGINT      NOT NULL,  -- Unix epoch milliseconds
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_dental_chart_patient UNIQUE (tenant_id, patient_nhi)
);

CREATE INDEX IF NOT EXISTS idx_dental_charts_patient
    ON dental_charts (tenant_id, patient_nhi);

CREATE INDEX IF NOT EXISTS idx_dental_charts_clinician
    ON dental_charts (clinician_id);
