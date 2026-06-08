-- NBRS (National Baby Record System) birth notifications.
-- Every birth in NZ must be notified to NBRS within 5 working days.
-- Fields map to the NBRS electronic notification form requirements.

CREATE TABLE IF NOT EXISTS nbrs_notifications (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    maternity_episode_id    UUID        NOT NULL REFERENCES maternity_episodes (id),
    intrapartum_episode_id  UUID        REFERENCES intrapartum_episodes (id),
    nhi_mother              TEXT        NOT NULL DEFAULT '',
    nhi_baby                TEXT        NOT NULL DEFAULT '',
    baby_first_name         TEXT        NOT NULL DEFAULT '',
    baby_family_name        TEXT        NOT NULL DEFAULT '',
    birth_date              DATE,
    birth_time              TIME,
    sex                     TEXT        NOT NULL DEFAULT 'unknown',
    -- male | female | indeterminate | unknown
    birth_weight_grams      INT,
    gestation_weeks         SMALLINT,
    birth_order             SMALLINT    NOT NULL DEFAULT 1,
    plurality               TEXT        NOT NULL DEFAULT 'single',
    -- single | twin | triplet | higher
    father_name             TEXT,
    ethnicities             JSONB       NOT NULL DEFAULT '[]',
    birth_facility_hpi      TEXT        NOT NULL DEFAULT '',
    attending_clinician_hpi TEXT        NOT NULL DEFAULT '',
    notification_status     TEXT        NOT NULL DEFAULT 'pending',
    -- pending | submitted | accepted | rejected | error
    submitted_at            TIMESTAMPTZ,
    response_code           TEXT,
    response_message        TEXT,
    tenant_id               UUID        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_nbrs_notifications_episode ON nbrs_notifications (maternity_episode_id);
CREATE INDEX IF NOT EXISTS idx_nbrs_notifications_status  ON nbrs_notifications (notification_status);
