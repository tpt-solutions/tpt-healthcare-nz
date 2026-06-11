-- Telehealth video consultation sessions.
-- host_url and patient_url are AES-256-GCM encrypted (contain provider JWTs).
-- patient_nhi is encrypted at rest.
CREATE TABLE IF NOT EXISTS telehealth_sessions (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id   TEXT,
    patient_nhi      TEXT        NOT NULL,
    practitioner_hpi TEXT        NOT NULL,
    video_provider   TEXT        NOT NULL,
    external_room_id TEXT        NOT NULL,
    host_url         TEXT        NOT NULL,
    patient_url      TEXT        NOT NULL,
    scheduled_at     TIMESTAMPTZ NOT NULL,
    duration_mins    INT         NOT NULL DEFAULT 30,
    status           TEXT        NOT NULL DEFAULT 'scheduled',
    recording_url    TEXT,
    ended_at         TIMESTAMPTZ,
    tenant_id        TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS telehealth_sessions_tenant_status
    ON telehealth_sessions (tenant_id, status);

CREATE INDEX IF NOT EXISTS telehealth_sessions_practitioner
    ON telehealth_sessions (tenant_id, practitioner_hpi);

CREATE INDEX IF NOT EXISTS telehealth_sessions_scheduled
    ON telehealth_sessions (tenant_id, scheduled_at DESC);
