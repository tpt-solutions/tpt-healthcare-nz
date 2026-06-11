-- Aggregate public health surveillance reports submitted to ESR or the Ministry of Health.
-- report_type: weekly_surveillance | monthly_communicable | outbreak_summary
-- report_period: ISO week "YYYY-Www", month "YYYY-MM", or outbreak UUID for outbreak_summary
-- submitted_to: esr | moh
-- status: draft | submitted | acknowledged
CREATE TABLE IF NOT EXISTS public_health_reports (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    report_type     TEXT        NOT NULL,
    report_period   TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'draft',
    data_summary    JSONB       NOT NULL DEFAULT '{}',
    submitted_to    TEXT,
    reference       TEXT,
    tenant_id       UUID        NOT NULL,
    submitted_at    TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ph_reports_tenant_type       ON public_health_reports (tenant_id, report_type);
CREATE INDEX IF NOT EXISTS idx_ph_reports_tenant_status     ON public_health_reports (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_ph_reports_tenant_period     ON public_health_reports (tenant_id, report_period DESC);
-- Prevent duplicate reports for the same period and type within a tenant.
CREATE UNIQUE INDEX IF NOT EXISTS uidx_ph_reports_tenant_type_period
    ON public_health_reports (tenant_id, report_type, report_period);
