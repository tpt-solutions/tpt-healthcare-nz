-- Well Child Tamariki Ora checks and growth monitoring.
-- The Well Child schedule is defined by Te Whatu Ora; contacts run from the
-- LMC handover (~4–5 days) through to the B4 School Check (~age 4).
-- Growth points store raw measurements; WHO centile band calculation is client-side.

CREATE TABLE IF NOT EXISTS well_child_checks (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    -- For neonatal and 6wk checks, maternity_episode_id links to the birth episode.
    -- For older children (3mo+), patient_nhi is the primary key.
    maternity_episode_id    UUID        REFERENCES maternity_episodes (id),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    provider_hpi            TEXT        NOT NULL DEFAULT '',
    check_type              TEXT        NOT NULL,
    -- neonatal | 6wk | 3mo | 5mo | 9mo | 12mo | 15mo | 2yr | B4SchoolCheck
    status                  TEXT        NOT NULL DEFAULT 'scheduled',
    -- scheduled | completed | missed | declined
    age_at_check_weeks      SMALLINT,
    weight_kg               NUMERIC(5,2),
    height_cm               NUMERIC(5,1),
    head_circumference_cm   NUMERIC(4,1),
    developmental_concerns  BOOLEAN     NOT NULL DEFAULT false,
    vision_concerns         BOOLEAN     NOT NULL DEFAULT false,
    hearing_concerns        BOOLEAN     NOT NULL DEFAULT false,
    sdq_score               SMALLINT,
    sdq_band                TEXT,
    -- normal | borderline | abnormal (SDQ only applies to 4yr+ B4 School Check)
    immunisations_up_to_date BOOLEAN    NOT NULL DEFAULT true,
    referrals               TEXT,
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    checked_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_well_child_checks_nhi        ON well_child_checks (patient_nhi, checked_at DESC);
CREATE INDEX IF NOT EXISTS idx_well_child_checks_episode    ON well_child_checks (maternity_episode_id);

-- Serial growth measurements specifically for the Well Child programme.
-- Separate from paediatric_growth_records which tracks inpatient measurements.
CREATE TABLE IF NOT EXISTS well_child_growth_points (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    well_child_check_id     UUID        REFERENCES well_child_checks (id),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    weight_kg               NUMERIC(5,2),
    height_cm               NUMERIC(5,1),
    head_circumference_cm   NUMERIC(4,1),
    centile_band            TEXT,
    recorded_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id               UUID        NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_well_child_growth_nhi ON well_child_growth_points (patient_nhi, recorded_at DESC);
