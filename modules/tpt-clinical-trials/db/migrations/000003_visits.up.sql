-- 000003_visits.up.sql
-- Participant study visits, CRF data capture, and protocol deviation recording.
-- CRF data is stored as AES-256-GCM encrypted JSONB to protect health information
-- collected during visits (HIPC Rule 5). Protocol deviations are reported to the
-- sponsor and HDEC as required under ICH E6(R3) GCP.

CREATE TABLE IF NOT EXISTS ct_participant_visits (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    participant_id      UUID        NOT NULL REFERENCES ct_participants (id) ON DELETE RESTRICT,
    -- scheduled_visit_id links back to the protocol visit template (NULL for unscheduled visits).
    scheduled_visit_id  UUID        REFERENCES ct_scheduled_visits (id) ON DELETE RESTRICT,
    tenant_id           UUID        NOT NULL,
    visit_name          TEXT        NOT NULL DEFAULT '',
    visit_type          TEXT        NOT NULL DEFAULT '',  -- VisitType
    sequence_no         INT         NOT NULL DEFAULT 0,
    status              TEXT        NOT NULL DEFAULT 'scheduled',  -- VisitStatus
    -- planned_date is the target date per the protocol schedule.
    planned_date        DATE,
    -- actual_date is when the visit occurred; used to detect window violations.
    actual_date         DATE,
    completed_by_hpi    TEXT        NOT NULL DEFAULT '',
    -- within_window is false if actual_date falls outside the scheduled window.
    within_window       BOOLEAN     NOT NULL DEFAULT true,
    days_from_baseline  INT,
    window_deviation_days INT       NOT NULL DEFAULT 0,
    notes               TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_participant_visits_participant
    ON ct_participant_visits (participant_id, sequence_no);

CREATE INDEX IF NOT EXISTS idx_ct_participant_visits_status
    ON ct_participant_visits (tenant_id, status, planned_date);

-- ct_crf_entries stores the completed case report form data for a visit.
-- Each row represents one field's captured value. The field_key and field_type
-- are validated against the protocol visit template's assessments_required.
-- The entire entry set for a visit is submitted atomically.
CREATE TABLE IF NOT EXISTS ct_crf_entries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    visit_id        UUID        NOT NULL REFERENCES ct_participant_visits (id) ON DELETE RESTRICT,
    participant_id  UUID        NOT NULL REFERENCES ct_participants (id) ON DELETE RESTRICT,
    tenant_id       UUID        NOT NULL,
    field_key       TEXT        NOT NULL DEFAULT '',
    field_type      TEXT        NOT NULL DEFAULT '',  -- CRFFieldType
    -- value_encrypted is the AES-256-GCM encrypted field value.
    value_encrypted BYTEA,
    unit            TEXT        NOT NULL DEFAULT '',
    normal_range    TEXT        NOT NULL DEFAULT '',
    abnormal        BOOLEAN     NOT NULL DEFAULT false,
    clinically_significant BOOLEAN NOT NULL DEFAULT false,
    entered_by_hpi  TEXT        NOT NULL DEFAULT '',
    entered_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- query_flag is set when a data manager raises a query against this entry.
    query_flag      BOOLEAN     NOT NULL DEFAULT false,
    query_text      TEXT        NOT NULL DEFAULT '',
    query_resolved  BOOLEAN     NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_crf_entries_visit
    ON ct_crf_entries (visit_id);

CREATE INDEX IF NOT EXISTS idx_ct_crf_entries_participant_field
    ON ct_crf_entries (participant_id, field_key);

CREATE INDEX IF NOT EXISTS idx_ct_crf_entries_query
    ON ct_crf_entries (tenant_id, query_flag, query_resolved)
    WHERE query_flag = true;

-- ct_protocol_deviations records all departures from the approved protocol.
-- Major and critical deviations must be reported to the HDEC within the
-- timeframes specified in the ethics approval conditions.
CREATE TABLE IF NOT EXISTS ct_protocol_deviations (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    participant_id  UUID        NOT NULL REFERENCES ct_participants (id) ON DELETE RESTRICT,
    visit_id        UUID        REFERENCES ct_participant_visits (id) ON DELETE RESTRICT,
    tenant_id       UUID        NOT NULL,
    category        TEXT        NOT NULL DEFAULT '',   -- DeviationCategory
    severity        TEXT        NOT NULL DEFAULT 'minor',  -- DeviationSeverity
    description     TEXT        NOT NULL DEFAULT '',
    -- impact is the AES-256-GCM encrypted clinical impact assessment.
    impact          BYTEA,
    corrective_action TEXT      NOT NULL DEFAULT '',
    reported_by_hpi TEXT        NOT NULL DEFAULT '',
    occurred_at     TIMESTAMPTZ,
    reported_at     TIMESTAMPTZ,
    -- hdec_reported_at is set when the deviation is reported to the ethics committee.
    hdec_reported_at TIMESTAMPTZ,
    sponsor_notified_at TIMESTAMPTZ,
    resolved        BOOLEAN     NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_protocol_deviations_participant
    ON ct_protocol_deviations (participant_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_ct_protocol_deviations_severity
    ON ct_protocol_deviations (tenant_id, severity, resolved);
