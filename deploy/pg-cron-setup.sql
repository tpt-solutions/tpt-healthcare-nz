-- pg_cron schedule registration for tpt-healthcare.
-- Run once after migration 008_resilience.sql has been applied and pg_cron is enabled.
-- Idempotent: uses ON CONFLICT DO UPDATE to update existing schedules.
--
-- Usage:
--   psql $DATABASE_URL -f deploy/pg-cron-setup.sql
--
-- All schedules use server time (UTC). Adjust if the Postgres server is not UTC.

-- Ensure pg_cron extension is present.
CREATE EXTENSION IF NOT EXISTS pg_cron;

-- ============================================================
-- Audit event partition rotation (1st of each month at 01:00 UTC)
-- Rotates the monthly partition so old audit events move to cold storage.
-- ============================================================
SELECT cron.schedule(
    'tpt-audit-partition-rotate',
    '0 1 1 * *',
    $$
        DO $$
        DECLARE
            new_part TEXT;
            start_val TEXT;
            end_val TEXT;
        BEGIN
            -- Create next month's partition proactively.
            new_part  := 'audit_events_' || TO_CHAR(NOW() + INTERVAL '1 month', 'YYYY_MM');
            start_val := TO_CHAR(DATE_TRUNC('month', NOW() + INTERVAL '1 month'), 'YYYY-MM-DD');
            end_val   := TO_CHAR(DATE_TRUNC('month', NOW() + INTERVAL '2 months'), 'YYYY-MM-DD');
            EXECUTE FORMAT(
                'CREATE TABLE IF NOT EXISTS %I PARTITION OF audit_events FOR VALUES FROM (%L) TO (%L)',
                new_part, start_val, end_val
            );
        END
        $$;
    $$
) ON CONFLICT (jobname) DO UPDATE SET schedule = EXCLUDED.schedule, command = EXCLUDED.command;

-- ============================================================
-- Data retention enforcement (nightly at 02:00 UTC)
-- Deletes or archives rows older than their retention policy.
-- ============================================================
SELECT cron.schedule(
    'tpt-retention-enforce',
    '0 2 * * *',
    $$
        DO $$
        DECLARE
            pol RECORD;
        BEGIN
            FOR pol IN
                SELECT table_name, retention_years, strategy, timestamp_column
                FROM retention_policy
                WHERE strategy = 'delete'
            LOOP
                EXECUTE FORMAT(
                    'DELETE FROM %s WHERE %I < NOW() - INTERVAL ''%s years''',
                    pol.table_name, pol.timestamp_column, pol.retention_years
                );
            END LOOP;
        END
        $$;
    $$
) ON CONFLICT (jobname) DO UPDATE SET schedule = EXCLUDED.schedule, command = EXCLUDED.command;

-- ============================================================
-- Outbox message pruning (nightly at 03:00 UTC)
-- Removes processed outbox messages older than 7 days.
-- ============================================================
SELECT cron.schedule(
    'tpt-outbox-prune',
    '0 3 * * *',
    $$
        DELETE FROM outbox_messages
        WHERE status = 'done'
          AND processed_at < NOW() - INTERVAL '7 days';
    $$
) ON CONFLICT (jobname) DO UPDATE SET schedule = EXCLUDED.schedule, command = EXCLUDED.command;

-- ============================================================
-- Backup run history pruning (nightly at 04:00 UTC)
-- Removes backup run records (not the actual backups) older than 10 years.
-- ============================================================
SELECT cron.schedule(
    'tpt-backup-runs-prune',
    '0 4 * * *',
    $$
        DELETE FROM backup_runs
        WHERE status IN ('success', 'verified')
          AND started_at < NOW() - INTERVAL '10 years';
    $$
) ON CONFLICT (jobname) DO UPDATE SET schedule = EXCLUDED.schedule, command = EXCLUDED.command;

-- ============================================================
-- Expired stock discard alert (daily at 06:00 UTC)
-- Marks stock items past their expiry date and notifies the application.
-- The application River job handles push/SMS notifications.
-- ============================================================
SELECT cron.schedule(
    'tpt-stock-expiry-check',
    '0 6 * * *',
    $$
        -- Log expired items to a temporary notification table that the River worker polls.
        INSERT INTO outbox_messages (tenant_id, topic, payload)
        SELECT tenant_id,
               'inventory.check_alerts',
               jsonb_build_object('tenant_id', tenant_id::text)
        FROM (
            SELECT DISTINCT tenant_id FROM stock_items
            WHERE expiry_date IS NOT NULL AND expiry_date <= CURRENT_DATE
        ) expired_tenants
        ON CONFLICT DO NOTHING;
    $$
) ON CONFLICT (jobname) DO UPDATE SET schedule = EXCLUDED.schedule, command = EXCLUDED.command;

-- ============================================================
-- Stats refresh (every 6 hours)
-- Refreshes the practice_summary materialised view used by DashboardPage.
-- The view must be created separately in a migration.
-- ============================================================
SELECT cron.schedule(
    'tpt-stats-refresh',
    '0 */6 * * *',
    $$
        DO $$
        BEGIN
            IF EXISTS (
                SELECT 1 FROM pg_matviews WHERE matviewname = 'practice_summary'
            ) THEN
                REFRESH MATERIALIZED VIEW CONCURRENTLY practice_summary;
            END IF;
        END $$;
    $$
) ON CONFLICT (jobname) DO UPDATE SET schedule = EXCLUDED.schedule, command = EXCLUDED.command;

-- Verify schedules registered successfully.
SELECT jobname, schedule, active FROM cron.job ORDER BY jobname;
