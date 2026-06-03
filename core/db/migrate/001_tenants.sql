-- 001_tenants.sql
-- Clinic tenant registry and self-registration applications.

CREATE TABLE IF NOT EXISTS tenants (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT        NOT NULL,
    hpi_facility_id TEXT        NOT NULL UNIQUE,
    status          TEXT        NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'suspended')),
    contact_email   TEXT        NOT NULL,
    contact_name    TEXT        NOT NULL,
    address         JSONB       NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS tenants_status_idx ON tenants (status);

-- Self-registration applications submitted by clinics wanting to join the network.
CREATE TABLE IF NOT EXISTS tenant_applications (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_name   TEXT        NOT NULL,
    hpi_facility_id TEXT        NOT NULL,
    contact_name    TEXT        NOT NULL,
    contact_email   TEXT        NOT NULL,
    contact_hpi_cpn TEXT        NOT NULL DEFAULT '',
    address         JSONB       NOT NULL DEFAULT '{}',
    status          TEXT        NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewer_notes  TEXT        NOT NULL DEFAULT '',
    tenant_id       UUID        REFERENCES tenants (id),
    submitted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at     TIMESTAMPTZ,
    reviewed_by     TEXT        NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS tenant_applications_status_idx
    ON tenant_applications (status);
