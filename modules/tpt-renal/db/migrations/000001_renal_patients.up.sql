-- 000001_renal_patients.up.sql
-- Renal patient registrations with CKD staging.

CREATE TABLE IF NOT EXISTS renal_patients (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID        NOT NULL,
    -- patient_nhi is the deterministic-encrypted NHI for indexing (HIPC Rule 5 / Rule 12).
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    -- fhir_patient is the AES-256-GCM encrypted FHIR Patient JSON.
    fhir_patient            BYTEA,
    -- CKD staging per KDIGO 2012 guidelines (1-5, 5D for dialysis)
    ckd_stage               TEXT        NOT NULL DEFAULT '',
    -- GFR category: G1-G5 per KDIGO
    gfr_category            TEXT        NOT NULL DEFAULT '',
    -- Albuminuria category: A1 (<30 mg/g), A2 (30-300), A3 (>300)
    albuminuria_category    TEXT        NOT NULL DEFAULT '',
    primary_cause           TEXT        NOT NULL DEFAULT '',
    icd10_code              TEXT        NOT NULL DEFAULT '',
    egfr_ml_min             NUMERIC(6,2),
    creatinine_umol_l       NUMERIC(8,2),
    -- Current dialysis modality: none, haemodialysis, peritoneal, transplant
    dialysis_modality       TEXT        NOT NULL DEFAULT 'none',
    dialysis_start_date     DATE,
    nephrologist_hpi        TEXT        NOT NULL DEFAULT '',
    primary_nurse_hpi       TEXT        NOT NULL DEFAULT '',
    referring_hpi           TEXT        NOT NULL DEFAULT '',
    referral_date           DATE,
    status                  TEXT        NOT NULL DEFAULT 'active',
    notes                   TEXT        NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_renal_patients_tenant_status
    ON renal_patients (tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_renal_patients_nhi
    ON renal_patients (patient_nhi);

CREATE INDEX IF NOT EXISTS idx_renal_patients_ckd_stage
    ON renal_patients (tenant_id, ckd_stage);

CREATE INDEX IF NOT EXISTS idx_renal_patients_modality
    ON renal_patients (tenant_id, dialysis_modality);
