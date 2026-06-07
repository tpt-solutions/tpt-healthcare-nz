-- Intensive Care Unit admissions and hourly chart
CREATE TABLE IF NOT EXISTS icu_admissions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id       UUID NOT NULL,
    patient_nhi      TEXT NOT NULL DEFAULT '',
    admission_id     UUID NOT NULL REFERENCES hospital_admissions(id),
    intensivist_hpi  TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active',
    bed_id           UUID,
    admission_reason TEXT NOT NULL DEFAULT '',
    diagnosis        TEXT,
    ventilation_mode TEXT NOT NULL DEFAULT 'none',
    apache_score     SMALLINT,
    sedation_level   SMALLINT,
    tenant_id        UUID NOT NULL,
    admitted_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    discharged_at    TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_icu_admissions_tenant_status ON icu_admissions (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_icu_admissions_admission     ON icu_admissions (admission_id);

-- Hourly nursing documentation (ICU flow sheet)
CREATE TABLE IF NOT EXISTS icu_chart_entries (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    icu_admission_id UUID NOT NULL REFERENCES icu_admissions(id),
    nurse_hpi        TEXT NOT NULL,
    vitals           JSONB NOT NULL DEFAULT '{}',
    sedation_level   SMALLINT,
    ventilation_mode TEXT NOT NULL DEFAULT 'none',
    notes            TEXT,
    tenant_id        UUID NOT NULL,
    recorded_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_icu_chart_entries_admission ON icu_chart_entries (icu_admission_id, recorded_at DESC);
