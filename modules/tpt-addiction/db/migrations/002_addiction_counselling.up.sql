-- Addiction-specific counselling: group sessions, individual sessions, treatment plans, goals, relapses.
-- Group sessions table is created first so the FK in counselling_sessions resolves correctly.

CREATE TABLE IF NOT EXISTS addiction_group_sessions (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID         NOT NULL,
    name          VARCHAR(256) NOT NULL,
    clinician_id  VARCHAR(64)  NOT NULL,
    practice_id   UUID         NOT NULL,
    scheduled_at  TIMESTAMPTZ  NOT NULL,
    duration_min  INT          NOT NULL CHECK (duration_min > 0),
    topic         VARCHAR(64)  NOT NULL
        CHECK (topic IN ('relapse_prevention','grief','harm_reduction','life_skills','other')),
    max_attendees INT          NOT NULL DEFAULT 12,
    attendees     VARCHAR(12)[] NOT NULL DEFAULT '{}',
    notes         TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_addiction_group_scheduled ON addiction_group_sessions (tenant_id, scheduled_at DESC);

ALTER TABLE addiction_group_sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY addiction_group_sessions_tenant_only ON addiction_group_sessions
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE TABLE IF NOT EXISTS addiction_counselling_sessions (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL,
    patient_nhi      VARCHAR(12) NOT NULL,
    clinician_id     VARCHAR(64) NOT NULL,
    practice_id      UUID        NOT NULL,
    session_type     VARCHAR(32) NOT NULL CHECK (session_type IN ('individual','group','family')),
    group_session_id UUID        REFERENCES addiction_group_sessions (id) ON DELETE SET NULL,
    session_date     TIMESTAMPTZ NOT NULL,
    duration_min     INT         NOT NULL CHECK (duration_min > 0),
    modality         VARCHAR(64) NOT NULL
        CHECK (modality IN ('motivational_interviewing','cbt','act','relapse_prevention','harm_reduction','other')),
    presenting_issue TEXT        NOT NULL,
    clinical_notes   TEXT,
    risk_assessment  TEXT,
    readiness_score  INT         CHECK (readiness_score BETWEEN 1 AND 10),
    homework_given   TEXT,
    next_session_date TIMESTAMPTZ,
    billing_type     VARCHAR(32) NOT NULL DEFAULT 'dhb_funded'
        CHECK (billing_type IN ('dhb_funded','acc','private','pro_bono')),
    fee_in_cents     INT         NOT NULL DEFAULT 0,
    extra_sensitive  BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_addiction_sessions_patient   ON addiction_counselling_sessions (tenant_id, patient_nhi, session_date DESC);
CREATE INDEX IF NOT EXISTS idx_addiction_sessions_clinician ON addiction_counselling_sessions (tenant_id, clinician_id, session_date DESC);

ALTER TABLE addiction_counselling_sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY addiction_sessions_tenant_only ON addiction_counselling_sessions
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE TABLE IF NOT EXISTS addiction_treatment_plans (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL,
    patient_nhi  VARCHAR(12) NOT NULL,
    programme_id UUID        REFERENCES addiction_programmes (id) ON DELETE SET NULL,
    clinician_id VARCHAR(64) NOT NULL,
    practice_id  UUID        NOT NULL,
    start_date   TIMESTAMPTZ NOT NULL,
    review_date  TIMESTAMPTZ NOT NULL,
    status       VARCHAR(32) NOT NULL DEFAULT 'active'
        CHECK (status IN ('active','completed','discontinued')),
    extra_sensitive BOOLEAN  NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_addiction_plans_patient ON addiction_treatment_plans (tenant_id, patient_nhi);

ALTER TABLE addiction_treatment_plans ENABLE ROW LEVEL SECURITY;
CREATE POLICY addiction_plans_tenant_only ON addiction_treatment_plans
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE TABLE IF NOT EXISTS addiction_goals (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    plan_id     UUID        NOT NULL REFERENCES addiction_treatment_plans (id) ON DELETE CASCADE,
    description TEXT        NOT NULL,
    target_date TIMESTAMPTZ,
    status      VARCHAR(32) NOT NULL DEFAULT 'not_started'
        CHECK (status IN ('not_started','in_progress','achieved','revised')),
    evidence    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_addiction_goals_plan ON addiction_goals (tenant_id, plan_id);

ALTER TABLE addiction_goals ENABLE ROW LEVEL SECURITY;
CREATE POLICY addiction_goals_tenant_only ON addiction_goals
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE TABLE IF NOT EXISTS addiction_relapses (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID        NOT NULL,
    plan_id        UUID        NOT NULL REFERENCES addiction_treatment_plans (id) ON DELETE CASCADE,
    occurred_at    TIMESTAMPTZ NOT NULL,
    substance_used VARCHAR(128) NOT NULL,
    trigger_notes  TEXT,
    severity       VARCHAR(32) NOT NULL CHECK (severity IN ('mild','moderate','severe')),
    intervention   TEXT,
    plan_modified  BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_addiction_relapses_plan ON addiction_relapses (tenant_id, plan_id, occurred_at DESC);

ALTER TABLE addiction_relapses ENABLE ROW LEVEL SECURITY;
CREATE POLICY addiction_relapses_tenant_only ON addiction_relapses
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);
