-- Inpatient medication charts, administration records, and reconciliation
CREATE TABLE IF NOT EXISTS inpatient_medications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id    UUID NOT NULL REFERENCES hospital_admissions(id),
    patient_id      UUID NOT NULL,
    prescriber_hpi  TEXT NOT NULL,
    generic_name    TEXT NOT NULL,
    brand_name      TEXT,
    nzmt_code       TEXT,
    dose            TEXT NOT NULL,
    route           TEXT NOT NULL DEFAULT 'oral',
    frequency       TEXT NOT NULL,
    max_daily_dose  TEXT,
    indication      TEXT,
    start_date      TIMESTAMPTZ NOT NULL,
    end_date        TIMESTAMPTZ,
    status          TEXT NOT NULL DEFAULT 'active',
    is_iv           BOOLEAN NOT NULL DEFAULT false,
    iv_rate         TEXT,
    allergies_checked BOOLEAN NOT NULL DEFAULT false,
    tenant_id       UUID NOT NULL,
    ceased_at       TIMESTAMPTZ,
    ceased_reason   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_inpatient_medications_admission ON inpatient_medications (admission_id, status);
CREATE INDEX IF NOT EXISTS idx_inpatient_medications_tenant    ON inpatient_medications (tenant_id);

-- Administration records (medication administration record — MAR)
CREATE TABLE IF NOT EXISTS med_administration_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    medication_id   UUID NOT NULL REFERENCES inpatient_medications(id),
    admission_id    UUID NOT NULL,
    administered_by TEXT NOT NULL,
    actual_dose     TEXT NOT NULL,
    route           TEXT NOT NULL,
    notes           TEXT,
    withheld        BOOLEAN NOT NULL DEFAULT false,
    withheld_reason TEXT,
    tenant_id       UUID NOT NULL,
    administered_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_med_admin_records_medication ON med_administration_records (medication_id, administered_at DESC);

-- Medication reconciliation (admission / discharge)
CREATE TABLE IF NOT EXISTS med_reconciliations (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id         UUID NOT NULL REFERENCES hospital_admissions(id),
    clinician_hpi        TEXT NOT NULL,
    reconciliation_type  TEXT NOT NULL CHECK (reconciliation_type IN ('admission', 'discharge')),
    home_medications     TEXT[] NOT NULL DEFAULT '{}',
    chart_medications    TEXT[] NOT NULL DEFAULT '{}',
    discrepancies        TEXT[] NOT NULL DEFAULT '{}',
    actions_taken        TEXT[] NOT NULL DEFAULT '{}',
    clinical_notes       TEXT,
    tenant_id            UUID NOT NULL,
    completed_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_med_reconciliations_admission ON med_reconciliations (admission_id, completed_at DESC);
