-- FIM (Functional Independence Measure) assessments.
-- 18 items scored 1–7: motor (13) + cognitive (5), total 18–126.

CREATE TABLE IF NOT EXISTS rehab_fim_scores (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id         UUID        NOT NULL REFERENCES rehab_admissions (id),
    assessed_by_hpi      TEXT        NOT NULL DEFAULT '',
    assessment_type      TEXT        NOT NULL DEFAULT 'admission',
    -- admission | midpoint | discharge | follow-up

    -- Self-care (6 items)
    eating               SMALLINT    NOT NULL DEFAULT 1,
    grooming             SMALLINT    NOT NULL DEFAULT 1,
    bathing              SMALLINT    NOT NULL DEFAULT 1,
    dressing_upper       SMALLINT    NOT NULL DEFAULT 1,
    dressing_lower       SMALLINT    NOT NULL DEFAULT 1,
    toileting            SMALLINT    NOT NULL DEFAULT 1,

    -- Sphincter control (2 items)
    bladder_management   SMALLINT    NOT NULL DEFAULT 1,
    bowel_management     SMALLINT    NOT NULL DEFAULT 1,

    -- Transfers (3 items)
    transfer_bed_chair   SMALLINT    NOT NULL DEFAULT 1,
    transfer_toilet      SMALLINT    NOT NULL DEFAULT 1,
    transfer_bath        SMALLINT    NOT NULL DEFAULT 1,

    -- Locomotion (2 items)
    walk_wheelchair      SMALLINT    NOT NULL DEFAULT 1,
    stairs               SMALLINT    NOT NULL DEFAULT 1,

    -- Communication (2 items)
    comprehension        SMALLINT    NOT NULL DEFAULT 1,
    expression           SMALLINT    NOT NULL DEFAULT 1,

    -- Social cognition (3 items)
    social_interaction   SMALLINT    NOT NULL DEFAULT 1,
    problem_solving      SMALLINT    NOT NULL DEFAULT 1,
    memory               SMALLINT    NOT NULL DEFAULT 1,

    -- Computed totals (stored for reporting)
    motor_fim_total      SMALLINT    NOT NULL DEFAULT 13,
    cognitive_fim_total  SMALLINT    NOT NULL DEFAULT 5,
    total_fim_score      SMALLINT    NOT NULL DEFAULT 18,

    notes                TEXT,
    tenant_id            UUID        NOT NULL,
    assessed_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rehab_fim_admission ON rehab_fim_scores (admission_id, assessed_at ASC);
CREATE INDEX IF NOT EXISTS idx_rehab_fim_tenant    ON rehab_fim_scores (tenant_id, assessment_type);
