-- 003_ost_prescribing.sql
-- Opioid Substitution Therapy prescribing tables (methadone, buprenorphine, Suboxone).
-- Controlled drug: every write triggers audit trail via application layer.

CREATE TABLE IF NOT EXISTS ost_prescriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    patient_nhi     VARCHAR(12) NOT NULL,
    programme_id    UUID REFERENCES addiction_programmes(id) ON DELETE SET NULL,
    prescriber_id   VARCHAR(64) NOT NULL,
    practice_id     UUID NOT NULL,
    drug            VARCHAR(64) NOT NULL
        CHECK (drug IN ('methadone','buprenorphine','buprenorphine_naloxone')),
    dose_mg         NUMERIC(6,2) NOT NULL,
    formulation     VARCHAR(32) NOT NULL CHECK (formulation IN ('liquid','tablet','sublingual')),
    frequency       VARCHAR(32) NOT NULL DEFAULT 'daily'
        CHECK (frequency IN ('daily','alternate_day','three_times_weekly')),
    supervised      BOOLEAN NOT NULL DEFAULT TRUE,
    take_home_days  INT NOT NULL DEFAULT 0 CHECK (take_home_days BETWEEN 0 AND 7),
    start_date      TIMESTAMPTZ NOT NULL,
    end_date        TIMESTAMPTZ,
    status          VARCHAR(32) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active','paused','discontinued','completed')),
    indication      VARCHAR(64) NOT NULL DEFAULT 'opioid_dependence'
        CHECK (indication IN ('opioid_dependence','pain','palliative')),
    clinical_notes  TEXT,
    extra_sensitive BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ost_prescriptions_patient ON ost_prescriptions(tenant_id, patient_nhi, status);
CREATE INDEX idx_ost_prescriptions_programme ON ost_prescriptions(tenant_id, programme_id);

ALTER TABLE ost_prescriptions ENABLE ROW LEVEL SECURITY;
CREATE POLICY ost_prescriptions_tenant_only ON ost_prescriptions
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

-- Dose adjustments (prescriber-witnessed changes).
CREATE TABLE IF NOT EXISTS ost_dose_adjustments (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    prescription_id   UUID NOT NULL REFERENCES ost_prescriptions(id) ON DELETE CASCADE,
    adjusted_by       VARCHAR(64) NOT NULL,
    adjusted_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    previous_dose_mg  NUMERIC(6,2) NOT NULL,
    new_dose_mg       NUMERIC(6,2) NOT NULL,
    reason            VARCHAR(64) NOT NULL
        CHECK (reason IN ('induction','reduction','clinical_response','adverse_event','other')),
    clinical_notes    TEXT,
    witnessed_by      VARCHAR(128),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ost_adjustments_prescription ON ost_dose_adjustments(tenant_id, prescription_id, adjusted_at DESC);

ALTER TABLE ost_dose_adjustments ENABLE ROW LEVEL SECURITY;
CREATE POLICY ost_adjustments_tenant_only ON ost_dose_adjustments
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);
