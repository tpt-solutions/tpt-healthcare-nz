-- Community rehabilitation episodes (post-discharge follow-up).

CREATE TABLE IF NOT EXISTS rehab_community_episodes (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    referral_source         TEXT        NOT NULL DEFAULT '',
    discharge_admission_id  UUID        REFERENCES rehab_admissions (id),
    episode_type            TEXT        NOT NULL DEFAULT 'post-discharge',
    -- post-discharge | ACC-rehabilitation | community-rehab | outpatient
    primary_diagnosis       TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'referred',
    -- referred | active | completed | withdrawn | declined
    disciplines             TEXT        NOT NULL DEFAULT '',
    visit_frequency         TEXT        NOT NULL DEFAULT 'weekly',
    -- daily | weekly | fortnightly | monthly
    visits_planned          SMALLINT,
    visits_completed        SMALLINT    NOT NULL DEFAULT 0,
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    referred_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at              TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rehab_community_tenant_status ON rehab_community_episodes (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_rehab_community_patient       ON rehab_community_episodes (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_rehab_community_discharge     ON rehab_community_episodes (discharge_admission_id) WHERE discharge_admission_id IS NOT NULL;
