-- NICU (Neonatal Intensive Care Unit) admissions, ventilation charting,
-- and discharge planning. NICU handles neonates <32 weeks or requiring
-- intensive respiratory or haemodynamic support beyond SCBU capability.

CREATE TABLE IF NOT EXISTS nicu_admissions (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    maternity_episode_id    UUID        REFERENCES maternity_episodes (id),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    neonatologist_hpi       TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'admitted',
    -- admitted | stable | critical | discharge-planning | discharged | transferred | deceased
    admission_reason        TEXT        NOT NULL DEFAULT '',
    admission_type          TEXT        NOT NULL DEFAULT 'inborn',
    -- inborn | outborn | transfer
    gestation_at_birth_weeks    SMALLINT,
    birth_weight_grams          INT,
    current_weight_grams        INT,
    corrected_age_weeks         SMALLINT,
    bed_label               TEXT        NOT NULL DEFAULT '',
    apgar_1min              SMALLINT,
    apgar_5min              SMALLINT,
    respiratory_support     TEXT        NOT NULL DEFAULT 'none',
    -- none | HFNC | CPAP | HFOV | conventional-vent
    surfactant_given        BOOLEAN     NOT NULL DEFAULT false,
    tpn_active              BOOLEAN     NOT NULL DEFAULT false,
    phototherapy_active     BOOLEAN     NOT NULL DEFAULT false,
    antibiotics_active      BOOLEAN     NOT NULL DEFAULT false,
    tenant_id               UUID        NOT NULL,
    admitted_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    discharged_at           TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_nicu_admissions_tenant_status ON nicu_admissions (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_nicu_admissions_episode       ON nicu_admissions (maternity_episode_id);

-- Ventilator settings and blood gas results; charted at least 4-hourly.
CREATE TABLE IF NOT EXISTS nicu_ventilation_chart (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    nicu_admission_id       UUID        NOT NULL REFERENCES nicu_admissions (id),
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    mode                    TEXT        NOT NULL DEFAULT 'CPAP',
    -- no-support | HFNC | CPAP | SIMV | AC-PC | HFOV
    fio2                    NUMERIC(4,2),           -- 0.21–1.00
    peep_cmh2o              NUMERIC(4,1),
    pip_cmh2o               NUMERIC(4,1),
    map_cmh2o               NUMERIC(4,1),
    amplitude_cmh2o         NUMERIC(4,1),
    frequency_hz            NUMERIC(4,1),
    tidal_volume_ml         NUMERIC(4,1),
    rate_per_min            SMALLINT,
    spo2_percent            SMALLINT,
    ph                      NUMERIC(4,2),
    pco2_mmhg               NUMERIC(5,1),
    po2_mmhg                NUMERIC(5,1),
    base_excess             NUMERIC(4,1),
    hco3_meql               NUMERIC(4,1),
    lactate                 NUMERIC(4,2),
    blood_gas_type          TEXT        NOT NULL DEFAULT 'none',
    -- ABG | VBG | CBG | none
    notes                   TEXT,
    recorded_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id               UUID        NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_nicu_vent_chart_admission ON nicu_ventilation_chart (nicu_admission_id, recorded_at DESC);

-- Discharge planning records: goals, follow-up, and parent readiness.
CREATE TABLE IF NOT EXISTS nicu_discharge_plans (
    id                              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    nicu_admission_id               UUID        NOT NULL REFERENCES nicu_admissions (id),
    clinician_hpi                   TEXT        NOT NULL DEFAULT '',
    planned_discharge_date          DATE,
    discharge_destination           TEXT,
    -- home | community-hospital | other-ward | other-hospital | deceased
    discharge_weight_target_grams   INT,
    feeding_plan                    TEXT,
    medications                     TEXT,
    follow_up_appointments          TEXT,
    parent_education_completed      JSONB       NOT NULL DEFAULT '{}',
    car_seat_organised              BOOLEAN     NOT NULL DEFAULT false,
    home_oxygen_required            BOOLEAN     NOT NULL DEFAULT false,
    apnoea_monitor_required         BOOLEAN     NOT NULL DEFAULT false,
    community_nurse_referral        BOOLEAN     NOT NULL DEFAULT false,
    notes                           TEXT,
    tenant_id                       UUID        NOT NULL,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_nicu_discharge_plans_admission ON nicu_discharge_plans (nicu_admission_id);
