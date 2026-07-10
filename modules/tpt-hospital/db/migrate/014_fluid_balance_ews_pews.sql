-- ICU fluid balance charting
CREATE TABLE IF NOT EXISTS fluid_balance_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    icu_admission_id UUID NOT NULL REFERENCES icu_admissions(id),
    direction       TEXT NOT NULL CHECK (direction IN ('in', 'out')),
    fluid_type      TEXT NOT NULL,
    volume_ml       INTEGER NOT NULL CHECK (volume_ml > 0),
    product_name    TEXT NOT NULL DEFAULT '',
    concentration   TEXT,
    recorded_by     TEXT NOT NULL,
    shift           TEXT NOT NULL DEFAULT 'day',
    comments        TEXT,
    tenant_id       UUID NOT NULL,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_fluid_balance_admission ON fluid_balance_entries (icu_admission_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_fluid_balance_tenant    ON fluid_balance_entries (tenant_id);

-- EWS/PEWS scoring history
CREATE TABLE IF NOT EXISTS early_warning_scores (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    icu_admission_id UUID NOT NULL REFERENCES icu_admissions(id),
    score_type      TEXT NOT NULL CHECK (score_type IN ('ews', 'pews')),
    total_score     INTEGER NOT NULL,
    clinical_risk   TEXT NOT NULL,
    individual_scores JSONB NOT NULL DEFAULT '{}',
    recorded_by     TEXT NOT NULL,
    tenant_id       UUID NOT NULL,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ews_admission ON early_warning_scores (icu_admission_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_ews_tenant    ON early_warning_scores (tenant_id);
