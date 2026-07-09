-- Tenant service-line enablement: which clinical service lines a facility
-- runs (emergency department, ICU, NICU/PICU, theatre, oncology, etc.).
-- A tenant may run multiple service lines simultaneously (e.g. one campus
-- with both adult and paediatric wards) rather than being forced into a
-- single fixed hospital template. The catalogue of valid service_line_id
-- values is defined in core/servicelines.
CREATE TABLE IF NOT EXISTS tenant_service_lines (
    tenant_id       UUID        NOT NULL REFERENCES tenants(id),
    service_line_id TEXT        NOT NULL,
    enabled_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, service_line_id)
);

CREATE INDEX IF NOT EXISTS idx_tenant_service_lines_line
    ON tenant_service_lines (service_line_id);
