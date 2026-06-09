-- Home visits — scheduling and documentation for NZ community health.

CREATE TABLE IF NOT EXISTS community_home_visits (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi          TEXT        NOT NULL DEFAULT '',
    -- AES-256-GCM encrypted; use HIPC Rule 5 encryption at rest
    clinician_hpi        TEXT        NOT NULL DEFAULT '',
    visit_type           TEXT        NOT NULL DEFAULT '',
    -- wound-care | medication-review | assessment | follow-up | palliative |
    -- post-acute | diabetes-care | respiratory | rehabilitation | postnatal
    priority             TEXT        NOT NULL DEFAULT 'routine',
    -- urgent | high | routine | low
    status               TEXT        NOT NULL DEFAULT 'scheduled',
    -- scheduled | in-transit | arrived | in-progress | completed | cancelled | rescheduled | dna
    address              TEXT        NOT NULL DEFAULT '',
    safety_notes         TEXT,
    access_instructions  TEXT,
    vital_signs          JSONB,
    -- {temperature, bp_systolic, bp_diastolic, heart_rate, spo2, pain_score, weight_kg, respiratory_rate, blood_glucose}
    wound_assessments    JSONB,
    -- [{site, cause, length_cm, width_cm, depth_cm, tissue_type, exudate, dressing_applied, ...}]
    observations         TEXT,
    concerns             TEXT,
    escalations          TEXT,
    cancellation_reason  TEXT,
    follow_up_required   BOOLEAN     NOT NULL DEFAULT FALSE,
    follow_up_details    TEXT,
    tenant_id            UUID        NOT NULL,
    scheduled_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    actual_start_at      TIMESTAMPTZ,
    actual_end_at        TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_community_home_visits_tenant_status  ON community_home_visits (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_community_home_visits_tenant_type    ON community_home_visits (tenant_id, visit_type);
CREATE INDEX IF NOT EXISTS idx_community_home_visits_patient        ON community_home_visits (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_community_home_visits_clinician      ON community_home_visits (clinician_hpi);
CREATE INDEX IF NOT EXISTS idx_community_home_visits_scheduled      ON community_home_visits (tenant_id, scheduled_at DESC);
