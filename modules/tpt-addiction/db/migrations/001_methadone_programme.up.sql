-- Opioid Substitution Therapy programme tables for NZ methadone / buprenorphine services.
-- extra_sensitive = true enforces elevated consent checks at application layer.

CREATE TABLE IF NOT EXISTS addiction_programmes (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID        NOT NULL,
    patient_nhi       VARCHAR(12) NOT NULL,
    clinician_id      VARCHAR(64) NOT NULL,
    practice_id       UUID        NOT NULL,
    start_date        TIMESTAMPTZ NOT NULL,
    end_date          TIMESTAMPTZ,
    phase             VARCHAR(32) NOT NULL DEFAULT 'induction'
        CHECK (phase IN ('induction','stabilisation','maintenance','tapering','discharged')),
    substance_primary VARCHAR(128) NOT NULL,
    substance_other   VARCHAR(256),
    initial_dose_mg   NUMERIC(6,2) NOT NULL,
    current_dose_mg   NUMERIC(6,2) NOT NULL,
    target_dose_mg    NUMERIC(6,2),
    take_home_level   INT          NOT NULL DEFAULT 1 CHECK (take_home_level BETWEEN 1 AND 5),
    take_home_max_days INT         NOT NULL DEFAULT 0,
    pregnancy         BOOLEAN      NOT NULL DEFAULT FALSE,
    comorbidities     TEXT[],
    last_urine_date   TIMESTAMPTZ,
    next_review_date  TIMESTAMPTZ  NOT NULL,
    extra_sensitive   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_addiction_programmes_patient   ON addiction_programmes (tenant_id, patient_nhi);
CREATE INDEX IF NOT EXISTS idx_addiction_programmes_clinician ON addiction_programmes (tenant_id, clinician_id);
CREATE INDEX IF NOT EXISTS idx_addiction_programmes_phase     ON addiction_programmes (tenant_id, phase) WHERE phase <> 'discharged';

ALTER TABLE addiction_programmes ENABLE ROW LEVEL SECURITY;
CREATE POLICY addiction_programmes_tenant_only ON addiction_programmes
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

-- Every supervised or take-home dose administered.
CREATE TABLE IF NOT EXISTS methadone_doses (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL,
    programme_id     UUID        NOT NULL REFERENCES addiction_programmes (id) ON DELETE CASCADE,
    administered_at  TIMESTAMPTZ NOT NULL,
    dose_mg          NUMERIC(6,2) NOT NULL,
    formulation      VARCHAR(32) NOT NULL CHECK (formulation IN ('liquid','tablet','sublingual')),
    witnessed_by     VARCHAR(128) NOT NULL,
    dispensed_by     VARCHAR(128) NOT NULL,
    pharmacist_check BOOLEAN      NOT NULL DEFAULT FALSE,
    status           VARCHAR(32) NOT NULL CHECK (status IN ('administered','refused','missed','vomited')),
    notes            TEXT,
    take_home        BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_methadone_doses_programme ON methadone_doses (tenant_id, programme_id, administered_at DESC);

ALTER TABLE methadone_doses ENABLE ROW LEVEL SECURITY;
CREATE POLICY methadone_doses_tenant_only ON methadone_doses
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

-- Take-home approval history (NZ MSSA levels 1-5).
CREATE TABLE IF NOT EXISTS methadone_take_home_approvals (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID        NOT NULL,
    programme_id   UUID        NOT NULL REFERENCES addiction_programmes (id) ON DELETE CASCADE,
    approved_by    VARCHAR(128) NOT NULL,
    approved_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    level          INT          NOT NULL CHECK (level BETWEEN 1 AND 5),
    max_days       INT          NOT NULL DEFAULT 0,
    expires_at     TIMESTAMPTZ,
    revoked_at     TIMESTAMPTZ,
    revoked_by     VARCHAR(128),
    revoked_reason TEXT,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_take_home_programme ON methadone_take_home_approvals (tenant_id, programme_id, approved_at DESC);

ALTER TABLE methadone_take_home_approvals ENABLE ROW LEVEL SECURITY;
CREATE POLICY take_home_tenant_only ON methadone_take_home_approvals
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

-- Urine drug screens (MSSA-compliant reporting).
CREATE TABLE IF NOT EXISTS urine_screens (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID        NOT NULL,
    programme_id   UUID        NOT NULL REFERENCES addiction_programmes (id) ON DELETE CASCADE,
    collected_at   TIMESTAMPTZ NOT NULL,
    collected_by   VARCHAR(128) NOT NULL,
    lab_name       VARCHAR(128),
    lab_reference  VARCHAR(64),
    results        JSONB        NOT NULL DEFAULT '[]',
    mssa_result    VARCHAR(32) NOT NULL CHECK (mssa_result IN ('conforming','non_conforming','borderline')),
    clinical_notes TEXT,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_urine_screens_programme ON urine_screens (tenant_id, programme_id, collected_at DESC);

ALTER TABLE urine_screens ENABLE ROW LEVEL SECURITY;
CREATE POLICY urine_screens_tenant_only ON urine_screens
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);
