-- Therapy goal setting — STG and LTG with discipline tracking.

CREATE TABLE IF NOT EXISTS rehab_goals (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id    UUID        NOT NULL REFERENCES rehab_admissions (id),
    discipline      TEXT        NOT NULL DEFAULT 'multidisciplinary',
    -- physiotherapy | occupational-therapy | speech-language-therapy | social-work | nursing | multidisciplinary
    goal_type       TEXT        NOT NULL DEFAULT 'STG',
    -- STG | LTG
    goal_text       TEXT        NOT NULL DEFAULT '',
    target_date     DATE,
    status          TEXT        NOT NULL DEFAULT 'active',
    -- active | achieved | not-achieved | modified | discontinued
    progress_notes  TEXT,
    achieved_at     TIMESTAMPTZ,
    tenant_id       UUID        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rehab_goals_admission  ON rehab_goals (admission_id, goal_type);
CREATE INDEX IF NOT EXISTS idx_rehab_goals_tenant     ON rehab_goals (tenant_id, status);
