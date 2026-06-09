-- 003_pain_protocols.up.sql
-- WHO analgesic ladder pain assessment and protocol tables.

CREATE TABLE IF NOT EXISTS pain_assessments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL,
    patient_nhi         VARCHAR(12) NOT NULL,
    assessment_date     TIMESTAMPTZ NOT NULL,
    assessor_id         VARCHAR(64) NOT NULL,
    pain_score          INT NOT NULL CHECK (pain_score BETWEEN 0 AND 10),
    severity            VARCHAR(16) NOT NULL CHECK (severity IN ('mild','moderate','severe')),
    pain_type           VARCHAR(32) NOT NULL CHECK (pain_type IN ('nociceptive','neuropathic','visceral','breakthrough','mixed')),
    location            VARCHAR(256),
    quality             VARCHAR(128),
    exacerbating        TEXT,
    relieving           TEXT,
    impact_sleep        INT NOT NULL DEFAULT 0 CHECK (impact_sleep BETWEEN 0 AND 10),
    impact_mobility     INT NOT NULL DEFAULT 0 CHECK (impact_mobility BETWEEN 0 AND 10),
    impact_mood         INT NOT NULL DEFAULT 0 CHECK (impact_mood BETWEEN 0 AND 10),
    breakthrough_episodes INT NOT NULL DEFAULT 0,
    notes               TEXT,
    extra_sensitive     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pain_assessments_patient ON pain_assessments(tenant_id, patient_nhi, assessment_date DESC);
CREATE INDEX IF NOT EXISTS idx_pain_assessments_severity ON pain_assessments(tenant_id, severity) WHERE severity IN ('moderate','severe');

ALTER TABLE pain_assessments ENABLE ROW LEVEL SECURITY;
CREATE POLICY pain_assessments_tenant_only ON pain_assessments
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE TABLE IF NOT EXISTS pain_protocols (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID NOT NULL,
    patient_nhi          VARCHAR(12) NOT NULL,
    step                 VARCHAR(32) NOT NULL
        CHECK (step IN ('step_1_non_opioid','step_2_weak_opioid','step_3_strong_opioid','step_4_interventional')),
    start_date           TIMESTAMPTZ NOT NULL,
    end_date             TIMESTAMPTZ,
    current_regimen      JSONB NOT NULL DEFAULT '[]',
    adjuvants            JSONB NOT NULL DEFAULT '[]',
    breakthrough_plan    JSONB,
    review_frequency_days INT NOT NULL DEFAULT 7,
    next_review_date     TIMESTAMPTZ NOT NULL,
    prescribed_by        VARCHAR(64) NOT NULL,
    goals                TEXT[],
    outcome_score        INT CHECK (outcome_score BETWEEN 0 AND 10),
    outcome_date         TIMESTAMPTZ,
    extra_sensitive      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pain_protocols_patient ON pain_protocols(tenant_id, patient_nhi, start_date DESC);
CREATE INDEX IF NOT EXISTS idx_pain_protocols_active ON pain_protocols(tenant_id, end_date) WHERE end_date IS NULL;

ALTER TABLE pain_protocols ENABLE ROW LEVEL SECURITY;
CREATE POLICY pain_protocols_tenant_only ON pain_protocols
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);
