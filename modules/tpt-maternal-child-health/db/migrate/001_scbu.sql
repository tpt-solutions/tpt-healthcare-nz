-- SCBU (Special Care Baby Unit) admissions and chart entries.
-- SCBU covers neonates requiring intermediate care: prematurity ~32-36 weeks,
-- jaundice, feeding difficulties, temperature regulation. Sits between the
-- postnatal ward and NICU in acuity.

CREATE TABLE IF NOT EXISTS scbu_admissions (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    episode_id           UUID        NOT NULL,
    patient_nhi          TEXT        NOT NULL DEFAULT '',
    neonatologist_hpi    TEXT        NOT NULL DEFAULT '',
    status               TEXT        NOT NULL DEFAULT 'admitted',
    bed_label            TEXT        NOT NULL DEFAULT '',
    admission_reason     TEXT        NOT NULL DEFAULT '',
    gestation_weeks      SMALLINT,
    birth_weight_grams   INT,
    apgar_1min           SMALLINT,
    apgar_5min           SMALLINT,
    phototherapy_active  BOOLEAN     NOT NULL DEFAULT false,
    feeding_method       TEXT        NOT NULL DEFAULT 'breast',
    tenant_id            UUID        NOT NULL,
    admitted_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    discharged_at        TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scbu_admissions_tenant_status ON scbu_admissions (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_scbu_admissions_episode       ON scbu_admissions (episode_id);

-- Periodic observations: weight, bilirubin (jaundice), vitals, feeds.
CREATE TABLE IF NOT EXISTS scbu_chart_entries (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    scbu_admission_id UUID        NOT NULL REFERENCES scbu_admissions (id),
    nurse_hpi         TEXT        NOT NULL DEFAULT '',
    weight_grams      INT,
    bilirubin_umol    NUMERIC(6,1),
    vitals            JSONB       NOT NULL DEFAULT '{}',
    feed_volume_ml    NUMERIC(5,1),
    notes             TEXT,
    tenant_id         UUID        NOT NULL,
    recorded_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_scbu_chart_entries_admission ON scbu_chart_entries (scbu_admission_id, recorded_at DESC);
