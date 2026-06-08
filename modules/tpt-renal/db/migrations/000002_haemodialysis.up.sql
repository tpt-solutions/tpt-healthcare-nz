-- 000002_haemodialysis.up.sql
-- Haemodialysis session scheduling and charting (Kt/V, UFR, access).

CREATE TABLE IF NOT EXISTS hd_sessions (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID        NOT NULL,
    renal_patient_id        UUID        NOT NULL REFERENCES renal_patients (id) ON DELETE RESTRICT,
    scheduled_at            TIMESTAMPTZ NOT NULL,
    started_at              TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    -- Vascular access
    access_type             TEXT        NOT NULL DEFAULT '',   -- AVF, AVG, CVC, port
    access_site             TEXT        NOT NULL DEFAULT '',
    -- Dialysis prescription
    dialyser_type           TEXT        NOT NULL DEFAULT '',
    blood_flow_rate_ml_min  NUMERIC(6,1),
    dialysate_flow_rate_ml_min NUMERIC(6,1),
    treatment_time_minutes  INTEGER,
    -- Weight and fluid removal
    pre_weight_kg           NUMERIC(5,2),
    post_weight_kg          NUMERIC(5,2),
    dry_weight_kg           NUMERIC(5,2),
    -- UFR: ultrafiltration rate in ml/hr
    ufr_ml_hr               NUMERIC(6,1),
    -- Dialysis adequacy
    kt_v                    NUMERIC(4,2),
    urea_reduction_ratio    NUMERIC(5,2),
    -- Observations
    pre_bp_systolic         SMALLINT,
    pre_bp_diastolic        SMALLINT,
    post_bp_systolic        SMALLINT,
    post_bp_diastolic       SMALLINT,
    temperature_celsius     NUMERIC(4,1),
    -- Anticoagulation
    anticoagulant           TEXT        NOT NULL DEFAULT '',
    anticoagulant_dose_units TEXT       NOT NULL DEFAULT '',
    -- Complications recorded as JSONB array of {type, severity, action}
    complications           JSONB       NOT NULL DEFAULT '[]',
    -- Staff
    nurse_hpi               TEXT        NOT NULL DEFAULT '',
    technician_hpi          TEXT        NOT NULL DEFAULT '',
    nephrologist_hpi        TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'scheduled',
    notes                   TEXT        NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hd_sessions_patient
    ON hd_sessions (renal_patient_id);

CREATE INDEX IF NOT EXISTS idx_hd_sessions_tenant_status
    ON hd_sessions (tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_hd_sessions_scheduled
    ON hd_sessions (tenant_id, scheduled_at);
