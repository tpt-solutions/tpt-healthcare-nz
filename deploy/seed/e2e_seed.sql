-- e2e_seed.sql
-- Seed data for running tpt-doctor against a real Postgres instance in local
-- dev / e2e testing. Not part of the numbered migration sequence in
-- core/db/migrate/ — this is fixture data, not schema.
--
-- Applied automatically by modules/tpt-doctor/docker-entrypoint.sh after
-- migrations run, or manually via:
--   psql "$DATABASE_URL" -f deploy/seed/e2e_seed.sql

INSERT INTO tenants (id, name, hpi_facility_id, status, contact_email, contact_name)
VALUES (
    'e2e00000-0000-0000-0000-000000000001',
    'E2E Test Medical Centre',
    'E2E0001',
    'active',
    'e2e.admin@tpt.test',
    'E2E Test Admin'
)
ON CONFLICT (id) DO NOTHING;
