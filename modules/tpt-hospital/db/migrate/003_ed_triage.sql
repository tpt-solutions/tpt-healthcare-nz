-- Emergency Department presentations
CREATE TABLE IF NOT EXISTS ed_presentations (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id             UUID,
    patient_nhi            TEXT NOT NULL DEFAULT '',
    triage_category        SMALLINT NOT NULL CHECK (triage_category BETWEEN 1 AND 5),
    status                 TEXT NOT NULL DEFAULT 'triaged',
    chief_complaint        TEXT NOT NULL,
    triage_notes           TEXT,
    triage_nurse_hpi       TEXT,
    assigned_bed_id        UUID,
    assigned_clinician_hpi TEXT,
    disposition            TEXT,
    disposition_notes      TEXT,
    admission_id           UUID,
    arrival_mode           TEXT,
    tenant_id              UUID NOT NULL,
    arrived_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    triaged_at             TIMESTAMPTZ,
    disposed_at            TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ed_presentations_tenant_status   ON ed_presentations (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_ed_presentations_triage_category ON ed_presentations (tenant_id, triage_category) WHERE status != 'disposed';
CREATE INDEX IF NOT EXISTS idx_ed_presentations_arrived_at      ON ed_presentations (arrived_at DESC);
