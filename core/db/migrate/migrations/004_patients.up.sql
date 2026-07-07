CREATE TABLE IF NOT EXISTS patients (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    nhi_encrypted   BYTEA       NOT NULL,
    nhi_index       TEXT        NOT NULL DEFAULT '',
    tenant_id       UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    fhir_resource   BYTEA       NOT NULL,
    name_search     TEXT        NOT NULL DEFAULT '',
    dob_index       TEXT        NOT NULL DEFAULT '',
    gender          TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS patients_tenant_idx ON patients (tenant_id);
CREATE INDEX IF NOT EXISTS patients_nhi_idx    ON patients (nhi_index);
CREATE INDEX IF NOT EXISTS patients_name_idx   ON patients USING GIN (to_tsvector('simple', name_search));
