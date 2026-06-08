-- 000005_fluid_balance.up.sql
-- Fluid balance and dry-weight tracking.

CREATE TABLE IF NOT EXISTS fluid_balance_records (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID        NOT NULL,
    renal_patient_id        UUID        NOT NULL REFERENCES renal_patients (id) ON DELETE RESTRICT,
    recorded_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    weight_kg               NUMERIC(5,2),
    dry_weight_kg           NUMERIC(5,2),
    -- Positive = above dry weight
    weight_above_dry_kg     NUMERIC(5,2),
    -- 24-hour totals
    fluid_intake_ml         INTEGER,
    fluid_output_ml         INTEGER,
    -- Positive = positive balance (fluid retained)
    fluid_balance_ml        INTEGER,
    -- Oedema: none, mild, moderate, severe
    oedema_severity         TEXT        NOT NULL DEFAULT 'none',
    oedema_sites            TEXT[]      NOT NULL DEFAULT '{}',
    bp_systolic             SMALLINT,
    bp_diastolic            SMALLINT,
    heart_rate              SMALLINT,
    nurse_hpi               TEXT        NOT NULL DEFAULT '',
    notes                   TEXT        NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_fluid_balance_patient
    ON fluid_balance_records (renal_patient_id);

CREATE INDEX IF NOT EXISTS idx_fluid_balance_recorded
    ON fluid_balance_records (tenant_id, recorded_at);
