-- Migration 012: New Platform Features
-- Covers: secure messaging, patient comms preferences, recall management,
-- intake forms, appointment self-service, GP2GP transfers, analytics views.

-- ============================================================
-- Secure Messaging
-- ============================================================

CREATE TABLE IF NOT EXISTS messaging_threads (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    patient_id  TEXT        NOT NULL,
    subject     TEXT        NOT NULL DEFAULT 'Message from your care team',
    status      TEXT        NOT NULL DEFAULT 'open', -- open | archived
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_msg_threads_tenant_patient
    ON messaging_threads (tenant_id, patient_id)
    WHERE status = 'open';

CREATE TABLE IF NOT EXISTS messaging_messages (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id        UUID        NOT NULL REFERENCES messaging_threads(id) ON DELETE CASCADE,
    tenant_id        UUID        NOT NULL,
    sender_id        TEXT        NOT NULL,
    sender_role      TEXT        NOT NULL, -- patient | practitioner | system
    body_encrypted   TEXT        NOT NULL, -- AES-256-GCM encrypted plaintext
    read_at          TIMESTAMPTZ,
    sent_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_msg_messages_thread
    ON messaging_messages (thread_id, sent_at ASC);

CREATE INDEX IF NOT EXISTS idx_msg_messages_unread
    ON messaging_messages (tenant_id, sender_id, read_at)
    WHERE read_at IS NULL;

-- ============================================================
-- Patient Communication Preferences
-- ============================================================

CREATE TABLE IF NOT EXISTS patient_comms_prefs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL,
    patient_id  TEXT        NOT NULL,
    channel     TEXT        NOT NULL, -- push | sms | email
    purpose     TEXT        NOT NULL, -- appointment_reminder | queue_called | test_result | recall | secure_message | billing | marketing
    opted_in    BOOL        NOT NULL DEFAULT true,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_comms_pref UNIQUE (tenant_id, patient_id, channel, purpose)
);

CREATE INDEX IF NOT EXISTS idx_comms_prefs_patient
    ON patient_comms_prefs (tenant_id, patient_id);

-- ============================================================
-- Recall / Care Gap Management
-- ============================================================

CREATE TABLE IF NOT EXISTS recall_items (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID        NOT NULL,
    patient_id          TEXT        NOT NULL,
    due_date            DATE        NOT NULL,
    recall_type         TEXT        NOT NULL,
    description         TEXT        NOT NULL DEFAULT '',
    encounter_id        TEXT,
    created_by_id       TEXT,
    status              TEXT        NOT NULL DEFAULT 'pending', -- pending | sent | booked | completed | declined
    notifications_sent  INT         NOT NULL DEFAULT 0,
    last_notified_at    TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recall_due
    ON recall_items (due_date)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_recall_patient
    ON recall_items (tenant_id, patient_id);

-- ============================================================
-- Intake Forms
-- ============================================================

CREATE TABLE IF NOT EXISTS form_templates (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID        NOT NULL,
    title             TEXT        NOT NULL,
    description       TEXT        NOT NULL DEFAULT '',
    appointment_types JSONB       NOT NULL DEFAULT '[]',
    questions         JSONB       NOT NULL DEFAULT '[]',
    active            BOOL        NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS form_instances (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL,
    template_id     UUID        NOT NULL REFERENCES form_templates(id),
    patient_id      TEXT        NOT NULL,
    appointment_id  TEXT,
    status          TEXT        NOT NULL DEFAULT 'pending', -- pending | sent | completed | expired
    token           TEXT        NOT NULL UNIQUE,
    expires_at      TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_form_instances_patient
    ON form_instances (tenant_id, patient_id);

CREATE TABLE IF NOT EXISTS form_responses (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id  UUID        NOT NULL REFERENCES form_instances(id),
    tenant_id    UUID        NOT NULL,
    patient_id   TEXT        NOT NULL,
    answers      JSONB       NOT NULL DEFAULT '{}',
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- Appointment Self-Service: cancellation waitlist
-- ============================================================

ALTER TABLE appointments
    ADD COLUMN IF NOT EXISTS cancelled_at         TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS cancellation_reason  TEXT,
    ADD COLUMN IF NOT EXISTS sms_confirmed_at     TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS appointment_waitlist (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL,
    patient_id       TEXT        NOT NULL,
    practitioner_hpi TEXT,
    appointment_type TEXT        NOT NULL,
    earliest_date    DATE,
    latest_date      DATE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_waitlist_patient_type UNIQUE (tenant_id, patient_id, appointment_type)
);

CREATE INDEX IF NOT EXISTS idx_waitlist_tenant_type
    ON appointment_waitlist (tenant_id, appointment_type, created_at ASC);

-- ============================================================
-- GP2GP Record Transfers
-- ============================================================

CREATE TABLE IF NOT EXISTS gp2gp_transfers (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    source_tenant_id        UUID        NOT NULL,
    destination_tenant_id   UUID,
    destination_endpoint    TEXT        NOT NULL DEFAULT '',
    patient_id              TEXT        NOT NULL,
    patient_nhi             TEXT        NOT NULL,
    requested_by_id         TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'pending',
    bundle_id               TEXT,
    error                   TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_gp2gp_patient
    ON gp2gp_transfers (source_tenant_id, patient_id);

-- ============================================================
-- Wait Time P50 — Rolling improvement
-- ============================================================

-- A materialized view that computes the P50 (median) actual wait time per
-- queue per hour-of-day over the trailing 7 days. Used by the queue service
-- to give better wait time estimates than the simple average.
CREATE MATERIALIZED VIEW IF NOT EXISTS queue_wait_p50 AS
SELECT
    queue_id,
    EXTRACT(DOW  FROM checked_in_at)::INT AS day_of_week,  -- 0=Sun … 6=Sat
    EXTRACT(HOUR FROM checked_in_at)::INT AS hour_of_day,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY
        EXTRACT(EPOCH FROM (called_at - checked_in_at)) / 60
    ) AS p50_wait_minutes
FROM queue_entries
WHERE status IN ('called','done')
  AND called_at IS NOT NULL
  AND checked_in_at >= NOW() - INTERVAL '7 days'
GROUP BY queue_id, day_of_week, hour_of_day;

CREATE UNIQUE INDEX IF NOT EXISTS idx_queue_wait_p50
    ON queue_wait_p50 (queue_id, day_of_week, hour_of_day);

-- Refresh every 15 minutes via pg_cron.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
    PERFORM cron.schedule(
      'refresh-queue-wait-p50',
      '*/15 * * * *',
      'REFRESH MATERIALIZED VIEW CONCURRENTLY queue_wait_p50'
    );
  END IF;
END $$;

-- ============================================================
-- Population Health Analytics Views
-- ============================================================

-- Aggregate condition counts by tenant and month (de-identified).
-- Used by the population health dashboard.
CREATE OR REPLACE VIEW population_health_summary AS
SELECT
    r.tenant_id,
    DATE_TRUNC('month', r.created_at)   AS month,
    r.resource_type,
    -- SNOMED code extracted from JSONB
    r.resource_data -> 'code' -> 'coding' -> 0 ->> 'code'    AS snomed_code,
    r.resource_data -> 'code' -> 'coding' -> 0 ->> 'display' AS condition_display,
    -- Ethnicity from Patient resource (via sub-query join would be needed in production)
    COUNT(*)                              AS case_count
FROM fhir_resources r
WHERE r.resource_type = 'Condition'
  AND r.deleted_at IS NULL
GROUP BY r.tenant_id, month, r.resource_type, snomed_code, condition_display;

-- Active medication counts by tenant and therapeutic group.
CREATE OR REPLACE VIEW medication_active_summary AS
SELECT
    r.tenant_id,
    DATE_TRUNC('month', r.created_at) AS month,
    r.resource_data -> 'medicationCodeableConcept' -> 'coding' -> 0 ->> 'display' AS medication_name,
    COUNT(*) AS prescription_count
FROM fhir_resources r
WHERE r.resource_type = 'MedicationRequest'
  AND r.resource_data ->> 'status' = 'active'
  AND r.deleted_at IS NULL
GROUP BY r.tenant_id, month, medication_name;

-- ============================================================
-- Financial Analytics
-- ============================================================

CREATE OR REPLACE VIEW financial_summary AS
SELECT
    tenant_id,
    DATE_TRUNC('month', created_at)     AS month,
    funding_stream,                      -- 'acc' | 'pharmac' | 'private' | 'pho'
    SUM(amount_cents)                    AS total_cents,
    COUNT(*)                             AS invoice_count,
    SUM(CASE WHEN status='paid' THEN amount_cents ELSE 0 END) AS paid_cents,
    SUM(CASE WHEN status='overdue' THEN amount_cents ELSE 0 END) AS overdue_cents
FROM invoices
GROUP BY tenant_id, month, funding_stream;

-- ============================================================
-- Health Equity Analytics (Ethnicity + Deprivation)
-- ============================================================

-- Patients with ethnicity and NZ Deprivation Index stored in the FHIR resource.
-- This view is used by the equity dashboard — all data is aggregate, not individual.
CREATE OR REPLACE VIEW equity_condition_summary AS
SELECT
    r.tenant_id,
    DATE_TRUNC('month', r.created_at) AS month,
    r.resource_data -> 'code' -> 'coding' -> 0 ->> 'display' AS condition_display,
    -- Ethnicity extracted from Patient resource (joined via subject reference).
    -- In production this would be a join; simplified here as a placeholder column.
    'unknown'::TEXT                AS ethnicity,
    COUNT(*)                       AS case_count
FROM fhir_resources r
WHERE r.resource_type = 'Condition'
  AND r.deleted_at IS NULL
GROUP BY r.tenant_id, month, condition_display, ethnicity;
