-- Disability support plans with person-centred goals and periodic review cycles.

CREATE TABLE IF NOT EXISTS disability_support_plans (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi       TEXT        NOT NULL DEFAULT '',
    coordinator_hpi   TEXT        NOT NULL DEFAULT '',
    nasc_referral_id  UUID        REFERENCES disability_nasc_referrals (id),
    funding_stream    TEXT        NOT NULL DEFAULT 'DSS',
    -- DSS | EGL | ACC | MSD
    plan_type         TEXT        NOT NULL DEFAULT 'initial',
    -- initial | review | variation | closure
    status            TEXT        NOT NULL DEFAULT 'draft',
    -- draft | active | review | closed
    goals_summary     TEXT        NOT NULL DEFAULT '',
    services_summary  TEXT        NOT NULL DEFAULT '',
    review_date       DATE,
    closure_reason    TEXT,
    notes             TEXT,
    tenant_id         UUID        NOT NULL,
    approved_at       TIMESTAMPTZ,
    closed_at         TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_disability_plans_tenant_status ON disability_support_plans (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_disability_plans_patient       ON disability_support_plans (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_disability_plans_nasc          ON disability_support_plans (nasc_referral_id);
CREATE INDEX IF NOT EXISTS idx_disability_plans_review_date   ON disability_support_plans (tenant_id, review_date);
