-- Holter monitoring (24h, 48h, 7-day, event) and ambulatory blood pressure monitoring (ABPM).

CREATE TABLE IF NOT EXISTS holter_monitors (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    ordering_clinician_hpi  TEXT        NOT NULL DEFAULT '',
    reporting_clinician_hpi TEXT        NOT NULL DEFAULT '',
    monitor_type            TEXT        NOT NULL DEFAULT '24h-holter',
    -- 24h-holter | 48h-holter | 7d-holter | event-monitor
    status                  TEXT        NOT NULL DEFAULT 'ordered',
    -- ordered | fitted | completed | reported | cancelled
    indication              TEXT        NOT NULL DEFAULT '',
    duration_hours          SMALLINT,
    total_beats             INT,
    min_hr_bpm              SMALLINT,
    max_hr_bpm              SMALLINT,
    mean_hr_bpm             SMALLINT,
    af_burden_percent       NUMERIC(5,2),
    pause_count             INT,
    longest_pause_seconds   NUMERIC(4,1),
    svt_episodes            INT,
    vt_episodes             INT,
    vf_episodes             INT,
    pvc_burden_percent      NUMERIC(5,2),
    interpretation          TEXT        NOT NULL DEFAULT '',
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    ordered_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    monitor_on_at           TIMESTAMPTZ,
    monitor_off_at          TIMESTAMPTZ,
    reported_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_holter_monitors_tenant_status ON holter_monitors (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_holter_monitors_patient       ON holter_monitors (patient_nhi, ordered_at DESC);

-- Ambulatory blood pressure monitoring
CREATE TABLE IF NOT EXISTS abpm_studies (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    ordering_clinician_hpi  TEXT        NOT NULL DEFAULT '',
    reporting_clinician_hpi TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'ordered',
    -- ordered | fitted | completed | reported | cancelled
    indication              TEXT        NOT NULL DEFAULT '',
    duration_hours          SMALLINT,
    readings_count          SMALLINT,
    awake_systolic_mean     SMALLINT,
    awake_diastolic_mean    SMALLINT,
    awake_hr_mean           SMALLINT,
    sleep_systolic_mean     SMALLINT,
    sleep_diastolic_mean    SMALLINT,
    sleep_hr_mean           SMALLINT,
    overall_systolic_mean   SMALLINT,
    overall_diastolic_mean  SMALLINT,
    dipping_status          TEXT        NOT NULL DEFAULT '',
    -- dipper | non-dipper | reverse-dipper | extreme-dipper
    wch_pattern             BOOLEAN     NOT NULL DEFAULT false,   -- white-coat hypertension
    masked_hypertension     BOOLEAN     NOT NULL DEFAULT false,
    interpretation          TEXT        NOT NULL DEFAULT '',
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    ordered_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    monitor_on_at           TIMESTAMPTZ,
    monitor_off_at          TIMESTAMPTZ,
    reported_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_abpm_studies_tenant_status ON abpm_studies (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_abpm_studies_patient       ON abpm_studies (patient_nhi, ordered_at DESC);
