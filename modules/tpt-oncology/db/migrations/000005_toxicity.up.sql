-- 000005_toxicity.up.sql
-- CTCAE v5.0 toxicity assessments and individual adverse event grading.

CREATE TABLE IF NOT EXISTS toxicity_assessments (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID        NOT NULL,
    oncology_patient_id   UUID        NOT NULL REFERENCES oncology_patients (id) ON DELETE RESTRICT,
    -- Link to the treatment that prompted the assessment (either chemo cycle or IT episode).
    treatment_cycle_id    UUID        REFERENCES treatment_cycles (id) ON DELETE SET NULL,
    immunotherapy_episode_id UUID     REFERENCES immunotherapy_episodes (id) ON DELETE SET NULL,
    assessed_by_hpi       TEXT        NOT NULL DEFAULT '',
    assessment_date       DATE        NOT NULL,
    ctcae_version         TEXT        NOT NULL DEFAULT '5.0',
    overall_grade         SMALLINT    NOT NULL DEFAULT 0,  -- highest grade across all events
    status                TEXT        NOT NULL DEFAULT 'draft',
    notes                 BYTEA,       -- encrypted clinical summary
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Exactly one treatment source per assessment.
    CONSTRAINT chk_toxicity_one_source CHECK (
        (treatment_cycle_id IS NOT NULL)::int +
        (immunotherapy_episode_id IS NOT NULL)::int <= 1
    )
);

CREATE INDEX IF NOT EXISTS idx_toxicity_assessments_patient
    ON toxicity_assessments (oncology_patient_id, assessment_date DESC);

CREATE INDEX IF NOT EXISTS idx_toxicity_assessments_tenant
    ON toxicity_assessments (tenant_id, assessment_date DESC);

CREATE INDEX IF NOT EXISTS idx_toxicity_assessments_grade3plus
    ON toxicity_assessments (tenant_id, overall_grade)
    WHERE overall_grade >= 3;

CREATE TABLE IF NOT EXISTS toxicity_adverse_events (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id         UUID        NOT NULL REFERENCES toxicity_assessments (id) ON DELETE CASCADE,
    ctcae_system          TEXT        NOT NULL,  -- organ system from CTCAE v5.0
    ctcae_term            TEXT        NOT NULL,  -- specific adverse event term
    ctcae_code            TEXT        NOT NULL DEFAULT '', -- MedDRA/CTCAE term code
    grade                 SMALLINT    NOT NULL,  -- 1-5
    attribution           TEXT        NOT NULL DEFAULT 'probable', -- possible|probable|definite|unrelated
    onset_date            DATE,
    resolution_date       DATE,
    -- action_taken documents the clinical response to the adverse event.
    action_taken          TEXT        NOT NULL DEFAULT '', -- dose-held|dose-reduced|treatment-stopped|supportive-care|none
    intervention_notes    BYTEA,       -- encrypted
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_toxicity_adverse_events_assessment
    ON toxicity_adverse_events (assessment_id);

CREATE INDEX IF NOT EXISTS idx_toxicity_adverse_events_grade
    ON toxicity_adverse_events (assessment_id, grade);
