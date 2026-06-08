-- Intrapartum (birth) episode, partogram entries, and CTG monitoring records.

CREATE TABLE IF NOT EXISTS intrapartum_episodes (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    maternity_episode_id        UUID        NOT NULL REFERENCES maternity_episodes (id),
    clinician_hpi               TEXT        NOT NULL DEFAULT '',
    status                      TEXT        NOT NULL DEFAULT 'latent-phase',
    -- latent-phase | active-labour | second-stage | third-stage | completed | transferred | abandoned
    onset_type                  TEXT        NOT NULL DEFAULT 'spontaneous',
    -- spontaneous | induced | elective-cs | emergency-cs
    birth_setting               TEXT        NOT NULL DEFAULT 'hospital',
    -- hospital | birth-centre | home
    labour_onset_at             TIMESTAMPTZ,
    active_labour_at            TIMESTAMPTZ,
    birth_at                    TIMESTAMPTZ,
    delivery_method             TEXT,
    -- SVD | instrumental-forceps | instrumental-vacuum | LSCS | emergency-cs
    perineal_outcome            TEXT,
    -- intact | graze | 1st | 2nd | 3rd | 4th | episiotomy
    blood_loss_ml               INT,
    neonatal_sex                TEXT,
    birth_weight_grams          INT,
    gestation_at_birth_weeks    SMALLINT,
    apgar_1min                  SMALLINT,
    apgar_5min                  SMALLINT,
    apgar_10min                 SMALLINT,
    cord_ph                     NUMERIC(4,2),
    notes                       TEXT,
    tenant_id                   UUID        NOT NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_intrapartum_episodes_maternity ON intrapartum_episodes (maternity_episode_id);

-- Serial partogram observations: cervical dilation, descent, contractions, FHR.
-- Recorded every 30-60 min during active labour per WHO partogram standards.
CREATE TABLE IF NOT EXISTS partogram_entries (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    intrapartum_episode_id      UUID        NOT NULL REFERENCES intrapartum_episodes (id),
    clinician_hpi               TEXT        NOT NULL DEFAULT '',
    cervical_dilation_cm        NUMERIC(3,1),
    fetal_station               SMALLINT,           -- cm relative to ischial spines: -5 to +5
    contractions_in_10min       SMALLINT,
    contraction_duration_secs   SMALLINT,
    fetal_heart_rate_bpm        SMALLINT,
    maternal_bp_systolic        SMALLINT,
    maternal_bp_diastolic       SMALLINT,
    maternal_pulse_bpm          SMALLINT,
    temperature_celsius         NUMERIC(4,1),
    urine_volume_ml             INT,
    liquor_colour               TEXT,
    -- clear | meconium-thin | meconium-thick | blood-stained | absent
    caput                       SMALLINT,           -- 0–3
    moulding                    SMALLINT,           -- 0–3
    oxytocin_dose_mu_per_min    NUMERIC(5,1),
    analgesic                   TEXT,
    -- none | entonox | pethidine | epidural | combined-spinal-epidural
    notes                       TEXT,
    recorded_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id                   UUID        NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_partogram_entries_episode ON partogram_entries (intrapartum_episode_id, recorded_at DESC);

-- CTG (cardiotocography) monitoring sessions and classification.
CREATE TABLE IF NOT EXISTS ctg_records (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    intrapartum_episode_id  UUID        NOT NULL REFERENCES intrapartum_episodes (id),
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    baseline_bpm            SMALLINT,
    baseline_variability    TEXT,
    -- absent | minimal | moderate | marked
    accelerations           BOOLEAN,
    decelerations           TEXT,
    -- none | early | variable-mild | variable-significant | late
    uterine_activity        TEXT,
    -- none | irregular | regular | hyperstimulated
    ctg_classification      TEXT        NOT NULL DEFAULT 'normal',
    -- normal | suspicious | pathological
    interpretation_notes    TEXT,
    started_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at                TIMESTAMPTZ,
    tenant_id               UUID        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ctg_records_episode ON ctg_records (intrapartum_episode_id, started_at DESC);
