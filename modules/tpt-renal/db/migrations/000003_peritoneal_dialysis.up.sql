-- 000003_peritoneal_dialysis.up.sql
-- Peritoneal dialysis (APD/CAPD) episode management.

CREATE TABLE IF NOT EXISTS pd_episodes (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID        NOT NULL,
    renal_patient_id        UUID        NOT NULL REFERENCES renal_patients (id) ON DELETE RESTRICT,
    -- APD = automated peritoneal dialysis, CAPD = continuous ambulatory PD
    modality                TEXT        NOT NULL DEFAULT '',
    start_date              DATE        NOT NULL,
    end_date                DATE,
    -- Prescription
    exchange_volume_ml      INTEGER,
    number_of_exchanges     SMALLINT,
    dwell_time_hours        NUMERIC(4,1),
    pd_solution_type        TEXT        NOT NULL DEFAULT '',
    glucose_concentration   TEXT        NOT NULL DEFAULT '',  -- e.g. 1.36%, 2.27%, 3.86%
    fill_time_minutes       SMALLINT,
    drain_time_minutes      SMALLINT,
    -- APD machine
    machine_type            TEXT        NOT NULL DEFAULT '',
    -- Adequacy (measured monthly)
    kt_v_weekly             NUMERIC(4,2),
    pna_g_kg_day            NUMERIC(4,2),   -- protein nitrogen appearance
    adequacy_met            BOOLEAN     NOT NULL DEFAULT false,
    -- Encrypted prescription notes (HIPC Rule 5)
    prescription_notes      BYTEA,
    nurse_hpi               TEXT        NOT NULL DEFAULT '',
    nephrologist_hpi        TEXT        NOT NULL DEFAULT '',
    -- active, completed, changed (modality switch), ceased
    status                  TEXT        NOT NULL DEFAULT 'active',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pd_episodes_patient
    ON pd_episodes (renal_patient_id);

CREATE INDEX IF NOT EXISTS idx_pd_episodes_tenant_status
    ON pd_episodes (tenant_id, status);

-- Daily exchange records for CAPD or APD session logs.
CREATE TABLE IF NOT EXISTS pd_exchange_records (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID        NOT NULL,
    pd_episode_id           UUID        NOT NULL REFERENCES pd_episodes (id) ON DELETE RESTRICT,
    recorded_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    fill_volume_ml          INTEGER,
    drain_volume_ml         INTEGER,
    -- Negative = net fluid removal
    ultrafiltrate_ml        INTEGER,
    effluent_appearance     TEXT        NOT NULL DEFAULT '',  -- clear, cloudy, bloody
    pain_score              SMALLINT,
    complications           TEXT        NOT NULL DEFAULT '',
    nurse_hpi               TEXT        NOT NULL DEFAULT '',
    notes                   TEXT        NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pd_exchange_records_episode
    ON pd_exchange_records (pd_episode_id);

CREATE INDEX IF NOT EXISTS idx_pd_exchange_records_recorded
    ON pd_exchange_records (tenant_id, recorded_at);
