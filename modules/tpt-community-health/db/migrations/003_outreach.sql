-- Community outreach programmes, events, and patient encounters.

CREATE TABLE IF NOT EXISTS community_outreach_programmes (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    programme_name    TEXT        NOT NULL DEFAULT '',
    programme_type    TEXT        NOT NULL DEFAULT '',
    -- mobile-clinic | health-promotion | screening | vaccination |
    -- wound-clinic | chronic-disease-support
    description       TEXT        NOT NULL DEFAULT '',
    target_population TEXT        NOT NULL DEFAULT '',
    status            TEXT        NOT NULL DEFAULT 'active',
    -- active | paused | completed | discontinued
    coordinator_hpi   TEXT        NOT NULL DEFAULT '',
    funding_source    TEXT,
    notes             TEXT,
    tenant_id         UUID        NOT NULL,
    start_date        DATE        NOT NULL DEFAULT CURRENT_DATE,
    end_date          DATE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_community_outreach_progs_tenant_status  ON community_outreach_programmes (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_community_outreach_progs_tenant_type    ON community_outreach_programmes (tenant_id, programme_type);
CREATE INDEX IF NOT EXISTS idx_community_outreach_progs_coordinator    ON community_outreach_programmes (coordinator_hpi);

CREATE TABLE IF NOT EXISTS community_outreach_events (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    programme_id         UUID        NOT NULL REFERENCES community_outreach_programmes (id),
    event_name           TEXT        NOT NULL DEFAULT '',
    event_type           TEXT        NOT NULL DEFAULT '',
    -- clinic | screening | education | vaccination | health-promotion
    location             TEXT        NOT NULL DEFAULT '',
    clinician_hpis       TEXT        NOT NULL DEFAULT '',
    -- space-separated list of HPI CPN numbers
    target_attendees     INTEGER,
    actual_attendees     INTEGER     NOT NULL DEFAULT 0,
    status               TEXT        NOT NULL DEFAULT 'planned',
    -- planned | confirmed | in-progress | completed | cancelled
    cancellation_reason  TEXT,
    notes                TEXT,
    tenant_id            UUID        NOT NULL,
    scheduled_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at           TIMESTAMPTZ,
    completed_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_community_outreach_events_programme  ON community_outreach_events (programme_id);
CREATE INDEX IF NOT EXISTS idx_community_outreach_events_tenant     ON community_outreach_events (tenant_id, scheduled_at DESC);
CREATE INDEX IF NOT EXISTS idx_community_outreach_events_status     ON community_outreach_events (tenant_id, status);

CREATE TABLE IF NOT EXISTS community_outreach_encounters (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id           UUID        NOT NULL REFERENCES community_outreach_events (id),
    patient_nhi        TEXT        NOT NULL DEFAULT '',
    -- AES-256-GCM encrypted; empty for non-patient (community-member) attendees
    clinician_hpi      TEXT        NOT NULL DEFAULT '',
    attendee_type      TEXT        NOT NULL DEFAULT 'patient',
    -- patient | carer | community-member
    services_provided  TEXT        NOT NULL DEFAULT '',
    screening_type     TEXT,
    -- blood-pressure | diabetes | cervical | bowel | hearing | vision
    screening_result   TEXT,
    referral_type      TEXT,
    -- gp | specialist | mental-health | social-services | housing | other
    referral_reason    TEXT,
    follow_up_required BOOLEAN     NOT NULL DEFAULT FALSE,
    follow_up_details  TEXT,
    consent_given      BOOLEAN     NOT NULL DEFAULT FALSE,
    notes              TEXT,
    tenant_id          UUID        NOT NULL,
    encountered_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_community_outreach_enc_event     ON community_outreach_encounters (event_id);
CREATE INDEX IF NOT EXISTS idx_community_outreach_enc_patient   ON community_outreach_encounters (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_community_outreach_enc_tenant    ON community_outreach_encounters (tenant_id, encountered_at DESC);
CREATE INDEX IF NOT EXISTS idx_community_outreach_enc_follow_up ON community_outreach_encounters (tenant_id, follow_up_required) WHERE follow_up_required;
