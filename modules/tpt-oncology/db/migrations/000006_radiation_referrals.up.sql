-- 000006_radiation_referrals.up.sql
-- Radiation oncology referrals and fraction delivery records.

CREATE TABLE IF NOT EXISTS radiation_referrals (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID        NOT NULL,
    oncology_patient_id   UUID        NOT NULL REFERENCES oncology_patients (id) ON DELETE RESTRICT,
    referred_by_hpi       TEXT        NOT NULL DEFAULT '',
    radiation_oncologist_hpi TEXT     NOT NULL DEFAULT '',
    intent                TEXT        NOT NULL DEFAULT 'curative',
    modality              TEXT        NOT NULL DEFAULT 'ebrt',
    treatment_site        TEXT        NOT NULL DEFAULT '',     -- anatomical site
    icd10_code            TEXT        NOT NULL DEFAULT '',
    clinical_indication   BYTEA,       -- encrypted clinical rationale
    -- Planning parameters filled once the radiation team accepts and plans.
    prescribed_dose_gy    NUMERIC(6, 2),
    fractions_planned     INT,
    fraction_dose_gy      NUMERIC(6, 3),
    technique             TEXT        NOT NULL DEFAULT '',     -- e.g. "3D-CRT", "SBRT 5F"
    simulation_date       DATE,
    planned_start_date    DATE,
    actual_start_date     DATE,
    actual_end_date       DATE,
    treatment_unit        TEXT        NOT NULL DEFAULT '',     -- linear accelerator ID
    status                TEXT        NOT NULL DEFAULT 'referred',
    decline_reason        TEXT        NOT NULL DEFAULT '',
    outcome_summary       BYTEA,       -- encrypted post-treatment summary
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_radiation_referrals_patient
    ON radiation_referrals (oncology_patient_id, status);

CREATE INDEX IF NOT EXISTS idx_radiation_referrals_tenant_status
    ON radiation_referrals (tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_radiation_referrals_planned_start
    ON radiation_referrals (tenant_id, planned_start_date)
    WHERE planned_start_date IS NOT NULL;

CREATE TABLE IF NOT EXISTS radiation_fractions (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL,
    referral_id      UUID        NOT NULL REFERENCES radiation_referrals (id) ON DELETE RESTRICT,
    fraction_number  INT         NOT NULL,
    planned_date     DATE        NOT NULL,
    actual_date      DATE,
    delivered_dose_gy NUMERIC(6, 3),
    status           TEXT        NOT NULL DEFAULT 'planned',
    missed_reason    TEXT        NOT NULL DEFAULT '',
    delivered_by_hpi TEXT        NOT NULL DEFAULT '',
    treatment_unit   TEXT        NOT NULL DEFAULT '',
    notes            TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (referral_id, fraction_number)
);

CREATE INDEX IF NOT EXISTS idx_radiation_fractions_referral
    ON radiation_fractions (referral_id, fraction_number);

CREATE INDEX IF NOT EXISTS idx_radiation_fractions_planned
    ON radiation_fractions (tenant_id, planned_date, status);
