-- Antenatal care: routine visits, ultrasound scans, and maternal screening.
-- All linked to a maternity_episode.

CREATE TABLE IF NOT EXISTS antenatal_visits (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    episode_id              UUID        NOT NULL REFERENCES maternity_episodes (id),
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    visit_type              TEXT        NOT NULL DEFAULT 'routine',
    -- booking | routine | additional | specialist
    gestation_weeks         SMALLINT,
    bp_systolic             SMALLINT,
    bp_diastolic            SMALLINT,
    weight_kg               NUMERIC(5,1),
    fundal_height_cm        NUMERIC(4,1),
    fetal_presentation      TEXT,
    -- cephalic | breech | transverse | oblique | unengaged
    fetal_heart_rate_bpm    SMALLINT,
    urinalysis_protein      TEXT,
    urinalysis_glucose      TEXT,
    oedema                  TEXT        NOT NULL DEFAULT 'none',
    -- none | mild | moderate | severe
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    visited_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_antenatal_visits_episode ON antenatal_visits (episode_id, visited_at DESC);

-- Ultrasound scans: dating, morphology, growth, wellbeing.
CREATE TABLE IF NOT EXISTS antenatal_scans (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    episode_id                  UUID        NOT NULL REFERENCES maternity_episodes (id),
    scan_type                   TEXT        NOT NULL DEFAULT 'growth',
    -- dating | combined-first-trimester | morphology | growth | wellbeing | other
    gestation_weeks             SMALLINT,
    gestation_days              SMALLINT,
    estimated_fetal_weight_g    INT,
    presentation                TEXT,
    liquor                      TEXT,
    -- normal | polyhydramnios | oligohydramnios
    placenta_position           TEXT,
    cervical_length_mm          NUMERIC(4,1),
    findings                    TEXT,
    sonographer_hpi             TEXT        NOT NULL DEFAULT '',
    scanned_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id                   UUID        NOT NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_antenatal_scans_episode ON antenatal_scans (episode_id, scanned_at DESC);

-- Maternal screening results: first-trimester combined, NIPT, GDM, GBS, etc.
CREATE TABLE IF NOT EXISTS antenatal_screening (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    episode_id      UUID        NOT NULL REFERENCES maternity_episodes (id),
    screen_type     TEXT        NOT NULL,
    -- combined-first-trimester | NIPT | GDM-50g | GDM-75g | GBS | group-and-hold
    -- rhesus | rubella | syphilis | HIV | hep-b | hep-c | chlamydia | other
    result          TEXT,
    result_value    NUMERIC(10,4),
    result_unit     TEXT,
    risk_score      TEXT,
    high_risk       BOOLEAN     NOT NULL DEFAULT false,
    collected_at    TIMESTAMPTZ,
    reported_at     TIMESTAMPTZ,
    notes           TEXT,
    tenant_id       UUID        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_antenatal_screening_episode ON antenatal_screening (episode_id);
