-- ECG studies: resting, stress, and ambulatory ECG orders, performance, and reporting.
-- Stores full quantitative measurements and structured interpretation.

CREATE TABLE IF NOT EXISTS ecg_studies (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    ordering_clinician_hpi  TEXT        NOT NULL DEFAULT '',
    reporting_clinician_hpi TEXT        NOT NULL DEFAULT '',
    study_type              TEXT        NOT NULL DEFAULT 'resting',
    -- resting | stress | ambulatory-24h
    status                  TEXT        NOT NULL DEFAULT 'ordered',
    -- ordered | performed | reported | cancelled
    indication              TEXT        NOT NULL DEFAULT '',
    heart_rate_bpm          SMALLINT,
    rhythm                  TEXT        NOT NULL DEFAULT '',
    pr_interval_ms          SMALLINT,
    qrs_duration_ms         SMALLINT,
    qt_interval_ms          SMALLINT,
    qtc_ms                  SMALLINT,
    qrs_axis_degrees        SMALLINT,
    p_axis_degrees          SMALLINT,
    st_changes              TEXT        NOT NULL DEFAULT 'none',
    t_wave_changes          TEXT        NOT NULL DEFAULT 'none',
    lbbb                    BOOLEAN     NOT NULL DEFAULT false,
    rbbb                    BOOLEAN     NOT NULL DEFAULT false,
    wolff_parkinson_white   BOOLEAN     NOT NULL DEFAULT false,
    interpretation          TEXT        NOT NULL DEFAULT '',
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    ordered_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    performed_at            TIMESTAMPTZ,
    reported_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ecg_studies_tenant_status ON ecg_studies (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_ecg_studies_patient       ON ecg_studies (patient_nhi, ordered_at DESC);
