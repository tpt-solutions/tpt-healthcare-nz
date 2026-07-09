-- Postnatal care: mother and baby checks, community midwife home visits.

CREATE TABLE IF NOT EXISTS postnatal_checks (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    maternity_episode_id        UUID        NOT NULL REFERENCES maternity_episodes (id),
    clinician_hpi               TEXT        NOT NULL DEFAULT '',
    check_type                  TEXT        NOT NULL DEFAULT 'routine',
    -- immediate | 2hr | 6hr | day1 | day2 | day3 | day5 | day7 | day10 | day14 | 6wk
    check_subject               TEXT        NOT NULL DEFAULT 'both',
    -- mother | baby | both
    maternal_bp_systolic        SMALLINT,
    maternal_bp_diastolic       SMALLINT,
    maternal_pulse_bpm          SMALLINT,
    maternal_temperature_c      NUMERIC(4,1),
    maternal_fundal_height_cm   NUMERIC(4,1),
    maternal_lochia             TEXT,
    -- rubra | serosa | alba | none
    maternal_perineum           TEXT,
    -- intact | bruising | healing | breakdown
    maternal_mood               TEXT        NOT NULL DEFAULT 'normal',
    -- normal | low | anxious | other
    baby_weight_grams           INT,
    baby_temperature_c          NUMERIC(4,1),
    baby_bilirubin_umol         NUMERIC(6,1),
    baby_jaundice               BOOLEAN     NOT NULL DEFAULT false,
    baby_feeding_method         TEXT,
    baby_feeding_issues         TEXT,
    baby_urine_output           TEXT,
    baby_stool                  TEXT,
    notes                       TEXT,
    checked_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id                   UUID        NOT NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_postnatal_checks_episode ON postnatal_checks (maternity_episode_id, checked_at DESC);

-- Community midwife home or clinic visits after hospital discharge.
CREATE TABLE IF NOT EXISTS community_midwife_visits (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    maternity_episode_id    UUID        NOT NULL REFERENCES maternity_episodes (id),
    midwife_hpi             TEXT        NOT NULL DEFAULT '',
    visit_number            SMALLINT,
    visit_type              TEXT        NOT NULL DEFAULT 'home',
    -- home | clinic | phone
    days_postnatal          SMALLINT,
    mother_wellbeing        TEXT,
    baby_wellbeing          TEXT,
    breastfeeding_support   BOOLEAN     NOT NULL DEFAULT false,
    issues_identified       TEXT,
    referrals               TEXT,
    visited_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id               UUID        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_community_midwife_visits_episode ON community_midwife_visits (maternity_episode_id, visited_at DESC);
