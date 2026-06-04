-- Migration 008: Resilience Infrastructure
-- Covers: transactional outbox, River job schema hooks, provider health status,
-- backup runs, retention policy table.

-- ============================================================
-- Transactional Outbox
-- ============================================================

CREATE TABLE IF NOT EXISTS outbox_messages (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL,
    topic            TEXT NOT NULL,
    payload          JSONB NOT NULL DEFAULT '{}',
    status           TEXT NOT NULL DEFAULT 'pending',  -- pending | processing | done | dead
    attempts         INT  NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dead_at          TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at     TIMESTAMPTZ,
    last_error       TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON outbox_messages (topic, next_attempt_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_outbox_dead
    ON outbox_messages (tenant_id, dead_at DESC)
    WHERE status = 'dead';

-- ============================================================
-- River Job Queue Schema
-- River manages its own schema via rivermigration.Migrate().
-- We create the schema here so the migration runner is aware of it;
-- actual table DDL is applied by River at service startup.
-- ============================================================

CREATE SCHEMA IF NOT EXISTS river;

-- ============================================================
-- Provider Health Status Cache
-- ============================================================

CREATE TABLE IF NOT EXISTS provider_health_status (
    provider_type      TEXT NOT NULL,
    provider_name      TEXT NOT NULL,
    ok                 BOOL NOT NULL DEFAULT false,
    last_checked_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    latency_ms         BIGINT NOT NULL DEFAULT 0,
    organisation_name  TEXT,
    error_text         TEXT,
    PRIMARY KEY (provider_type, provider_name)
);

-- ============================================================
-- Backup Runs
-- ============================================================

CREATE TABLE IF NOT EXISTS backup_runs (
    id           TEXT PRIMARY KEY,      -- UUID string; TEXT to allow custom labels
    label        TEXT NOT NULL,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status       TEXT NOT NULL DEFAULT 'running',  -- running | success | failed | verified
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    storage_key  TEXT,
    error_text   TEXT
);

CREATE INDEX IF NOT EXISTS idx_backup_runs_started ON backup_runs (started_at DESC);

-- ============================================================
-- Retention Policy
-- ============================================================

CREATE TABLE IF NOT EXISTS retention_policy (
    table_name       TEXT PRIMARY KEY,
    retention_years  INT NOT NULL,
    strategy         TEXT NOT NULL DEFAULT 'archive',  -- archive | delete
    timestamp_column TEXT NOT NULL DEFAULT 'created_at'
);

-- Seed default retention policies (idempotent via ON CONFLICT DO NOTHING).
INSERT INTO retention_policy (table_name, retention_years, strategy, timestamp_column) VALUES
    ('public.audit_events',    10, 'archive', 'created_at'),
    ('public.fhir_resources',  10, 'archive', 'last_updated'),
    ('public.outbox_messages',  1, 'delete',  'created_at'),
    ('public.patient_invoices', 7, 'archive', 'created_at'),
    ('public.backup_runs',     10, 'delete',  'started_at')
ON CONFLICT (table_name) DO NOTHING;

-- ============================================================
-- pg_cron Extension (enable if available)
-- pg_cron requires superuser; skip silently if not available.
-- Actual schedule registration happens in deploy/pg-cron-setup.sql.
-- ============================================================

DO $$
BEGIN
    -- Only attempt to create the extension if pg_cron is installed.
    IF EXISTS (
        SELECT 1 FROM pg_available_extensions WHERE name = 'pg_cron'
    ) THEN
        CREATE EXTENSION IF NOT EXISTS pg_cron;
    END IF;
END
$$;

-- btree_gist is required for the room_bookings EXCLUDE constraint.
CREATE EXTENSION IF NOT EXISTS btree_gist;
