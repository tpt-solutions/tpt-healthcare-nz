-- tpt-blood-bank: donors, donations, blood products, and crossmatches
-- Mirrors the columns queried in api/donors_query.go, api/inventory.go and
-- api/crossmatch_query.go.

CREATE TABLE IF NOT EXISTS donors (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    nhi                 VARCHAR(7) NOT NULL,
    blood_group         VARCHAR(10) NOT NULL,
    rhd                 VARCHAR(10) NOT NULL CHECK (rhd IN ('POSITIVE', 'NEGATIVE')),
    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    deferral_reason     TEXT,
    deferral_end_date   TIMESTAMPTZ,
    total_donations     INTEGER NOT NULL DEFAULT 0,
    last_donation_at    TIMESTAMPTZ,
    haemoglobin_gdl     NUMERIC(4,1),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_donors_tenant ON donors (tenant_id);
CREATE INDEX IF NOT EXISTS idx_donors_nhi ON donors (nhi);

CREATE TABLE IF NOT EXISTS blood_products (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    product_type        VARCHAR(20) NOT NULL,
    abo                 VARCHAR(5) NOT NULL,
    rhd                 VARCHAR(10) NOT NULL CHECK (rhd IN ('POSITIVE', 'NEGATIVE')),
    donation_id         UUID,
    donor_id            UUID REFERENCES donors(id),
    status              VARCHAR(20) NOT NULL DEFAULT 'collected',
    volume_ml           INTEGER NOT NULL,
    collection_date     TIMESTAMPTZ NOT NULL,
    expiry_date         TIMESTAMPTZ NOT NULL,
    test_results        JSONB NOT NULL DEFAULT '[]',
    storage_location    TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_blood_products_tenant ON blood_products (tenant_id);
CREATE INDEX IF NOT EXISTS idx_blood_products_status ON blood_products (status);
CREATE INDEX IF NOT EXISTS idx_blood_products_expiry ON blood_products (expiry_date);

CREATE TABLE IF NOT EXISTS donations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    donor_id            UUID NOT NULL REFERENCES donors(id) ON DELETE CASCADE,
    product_unit_id     UUID REFERENCES blood_products(id),
    volume_ml           INTEGER NOT NULL,
    donation_type       VARCHAR(30) NOT NULL,
    collected_at        TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_donations_tenant ON donations (tenant_id);
CREATE INDEX IF NOT EXISTS idx_donations_donor ON donations (donor_id);

CREATE TABLE IF NOT EXISTS crossmatches (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    patient_id          UUID NOT NULL,
    patient_nhi         VARCHAR(7) NOT NULL,
    patient_abo         VARCHAR(5) NOT NULL,
    patient_rhd         VARCHAR(10) NOT NULL CHECK (patient_rhd IN ('POSITIVE', 'NEGATIVE')),
    antibody_screen     VARCHAR(20) NOT NULL,
    product_unit_ids    UUID[] NOT NULL DEFAULT '{}',
    status              VARCHAR(20) NOT NULL DEFAULT 'matched',
    compatibility       VARCHAR(20) NOT NULL,
    requested_by        UUID NOT NULL,
    issued_by           UUID,
    transfused_by       UUID,
    emergency_reason    TEXT,
    notes               TEXT,
    requested_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    issued_at           TIMESTAMPTZ,
    transfused_at       TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_crossmatches_tenant ON crossmatches (tenant_id);
CREATE INDEX IF NOT EXISTS idx_crossmatches_patient ON crossmatches (patient_id);
CREATE INDEX IF NOT EXISTS idx_crossmatches_status ON crossmatches (status);
