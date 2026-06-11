-- Funded support hours — allocation and usage tracking per service type and period.

CREATE TABLE IF NOT EXISTS disability_funded_hours (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi     TEXT         NOT NULL DEFAULT '',
    support_plan_id UUID         REFERENCES disability_support_plans (id),
    service_type    TEXT         NOT NULL DEFAULT 'community',
    -- community | residential | respite | day-service | personal-care | supported-living
    provider_name   TEXT         NOT NULL DEFAULT '',
    provider_hpi    TEXT         NOT NULL DEFAULT '',
    funding_stream  TEXT         NOT NULL DEFAULT 'DSS',
    allocated_hours NUMERIC(8,2) NOT NULL DEFAULT 0,
    used_hours      NUMERIC(8,2) NOT NULL DEFAULT 0,
    period_type     TEXT         NOT NULL DEFAULT 'weekly',
    -- weekly | fortnightly | monthly | annual
    period_start    DATE         NOT NULL,
    period_end      DATE,
    status          TEXT         NOT NULL DEFAULT 'active',
    -- active | suspended | expired | closed
    notes           TEXT,
    tenant_id       UUID         NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_disability_funded_hours_tenant_status ON disability_funded_hours (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_disability_funded_hours_patient       ON disability_funded_hours (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_disability_funded_hours_plan          ON disability_funded_hours (support_plan_id);
CREATE INDEX IF NOT EXISTS idx_disability_funded_hours_period        ON disability_funded_hours (tenant_id, period_start DESC);
