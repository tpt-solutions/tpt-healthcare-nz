-- 000007_palliative.up.sql
-- Palliative oncology care plans, goals of care, and symptom burden records.

CREATE TABLE IF NOT EXISTS palliative_care_plans (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID        NOT NULL,
    oncology_patient_id   UUID        NOT NULL REFERENCES oncology_patients (id) ON DELETE RESTRICT,
    lead_clinician_hpi    TEXT        NOT NULL DEFAULT '',
    palliative_team_hpi   TEXT[]      NOT NULL DEFAULT '{}',
    -- prognosis_weeks is the estimated prognosis in weeks; null = not disclosed/unknown.
    prognosis_weeks       INT,
    preferred_place_of_care TEXT      NOT NULL DEFAULT '',   -- home|hospice|hospital
    preferred_place_of_death TEXT     NOT NULL DEFAULT '',
    -- advance_care_plan_id references a document in the consent/ACP module.
    advance_care_plan_id  UUID,
    dnar_status           TEXT        NOT NULL DEFAULT 'not-documented', -- not-documented|for-cpr|dnar
    dnar_date             DATE,
    te_ara_whakapiri_active BOOLEAN   NOT NULL DEFAULT false,
    status                TEXT        NOT NULL DEFAULT 'active',
    commenced_date        DATE,
    completed_date        DATE,
    notes                 BYTEA,       -- encrypted
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_palliative_care_plans_patient
    ON palliative_care_plans (oncology_patient_id, status);

CREATE INDEX IF NOT EXISTS idx_palliative_care_plans_tenant_status
    ON palliative_care_plans (tenant_id, status);

CREATE TABLE IF NOT EXISTS palliative_goals (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    care_plan_id     UUID        NOT NULL REFERENCES palliative_care_plans (id) ON DELETE CASCADE,
    category         TEXT        NOT NULL DEFAULT 'symptom-control',
    description      BYTEA       NOT NULL,  -- encrypted goal text
    target_date      DATE,
    status           TEXT        NOT NULL DEFAULT 'active',
    achievement_note BYTEA,       -- encrypted note when achieved/cancelled
    reviewed_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_palliative_goals_plan
    ON palliative_goals (care_plan_id, status);

CREATE TABLE IF NOT EXISTS palliative_symptom_records (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL,
    care_plan_id     UUID        NOT NULL REFERENCES palliative_care_plans (id) ON DELETE CASCADE,
    recorded_by_hpi  TEXT        NOT NULL DEFAULT '',
    assessment_date  TIMESTAMPTZ NOT NULL DEFAULT now(),
    symptom_name     TEXT        NOT NULL,
    -- severity_score uses a 0-10 NRS (Numeric Rating Scale); ESAS-r compatible.
    severity_score   SMALLINT    NOT NULL DEFAULT 0 CHECK (severity_score BETWEEN 0 AND 10),
    -- intervention_given documents the management action taken at this assessment.
    intervention_given TEXT      NOT NULL DEFAULT '',
    response_at_review TEXT      NOT NULL DEFAULT '',
    notes            BYTEA,       -- encrypted
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_palliative_symptom_records_plan
    ON palliative_symptom_records (care_plan_id, assessment_date DESC);

CREATE INDEX IF NOT EXISTS idx_palliative_symptom_records_symptom
    ON palliative_symptom_records (care_plan_id, symptom_name, assessment_date DESC);
