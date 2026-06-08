-- 000001_oncology_patients.up.sql
-- Oncology patient registrations and tumour board (MDT) referrals.

CREATE TABLE IF NOT EXISTS oncology_patients (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID        NOT NULL,
    -- patient_nhi is the deterministic-encrypted NHI for indexing (HIPC Rule 5 / Rule 12).
    patient_nhi          TEXT        NOT NULL DEFAULT '',
    -- fhir_patient is the AES-256-GCM encrypted FHIR Patient JSON.
    fhir_patient         BYTEA,
    cancer_type          TEXT        NOT NULL,
    icd10_code           TEXT        NOT NULL DEFAULT '',
    histology            TEXT        NOT NULL DEFAULT '',
    primary_site         TEXT        NOT NULL DEFAULT '',
    tnm_stage            TEXT        NOT NULL DEFAULT '',
    clinical_t           TEXT        NOT NULL DEFAULT '',
    clinical_n           TEXT        NOT NULL DEFAULT '',
    clinical_m           TEXT        NOT NULL DEFAULT '',
    pathological_t       TEXT        NOT NULL DEFAULT '',
    pathological_n       TEXT        NOT NULL DEFAULT '',
    pathological_m       TEXT        NOT NULL DEFAULT '',
    diagnosis_date       DATE,
    referring_hpi        TEXT        NOT NULL DEFAULT '',
    oncologist_hpi       TEXT        NOT NULL DEFAULT '',
    status               TEXT        NOT NULL DEFAULT 'active',
    notes                TEXT        NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_oncology_patients_tenant_status
    ON oncology_patients (tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_oncology_patients_nhi
    ON oncology_patients (patient_nhi);

CREATE INDEX IF NOT EXISTS idx_oncology_patients_cancer_type
    ON oncology_patients (tenant_id, cancer_type);

CREATE TABLE IF NOT EXISTS tumour_board_referrals (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID        NOT NULL,
    oncology_patient_id  UUID        NOT NULL REFERENCES oncology_patients (id) ON DELETE RESTRICT,
    referred_by_hpi      TEXT        NOT NULL DEFAULT '',
    mdt_type             TEXT        NOT NULL DEFAULT '',   -- e.g. breast, colorectal, haematology
    referral_reason      TEXT        NOT NULL DEFAULT '',
    clinical_summary     BYTEA,                             -- encrypted free text
    imaging_refs         TEXT[]      NOT NULL DEFAULT '{}', -- reference IDs from radiology module
    pathology_refs       TEXT[]      NOT NULL DEFAULT '{}', -- reference IDs from pathology module
    status               TEXT        NOT NULL DEFAULT 'pending',
    scheduled_at         TIMESTAMPTZ,
    reviewed_at          TIMESTAMPTZ,
    outcome_summary      BYTEA,                             -- encrypted outcome text
    recommendation       TEXT        NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tumour_board_referrals_patient
    ON tumour_board_referrals (oncology_patient_id);

CREATE INDEX IF NOT EXISTS idx_tumour_board_referrals_tenant_status
    ON tumour_board_referrals (tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_tumour_board_referrals_scheduled
    ON tumour_board_referrals (tenant_id, scheduled_at)
    WHERE scheduled_at IS NOT NULL;
