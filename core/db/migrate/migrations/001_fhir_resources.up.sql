CREATE TABLE IF NOT EXISTS fhir_resources (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_type   text        NOT NULL,
    resource_id     text        NOT NULL,
    version_id      text        NOT NULL DEFAULT '1',
    data            jsonb       NOT NULL,
    tenant_id       uuid        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,
    UNIQUE (resource_type, resource_id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_fhir_resources_type_tenant
    ON fhir_resources (resource_type, tenant_id);

CREATE INDEX IF NOT EXISTS idx_fhir_resources_data_gin
    ON fhir_resources USING GIN (data);
