-- Outbreak investigation records for public health unit (PHU) case cluster management.
-- An outbreak is a cluster of notifiable disease cases sharing a suspected common source.
-- status: suspected | confirmed | controlled | closed
CREATE TABLE IF NOT EXISTS outbreak_investigations (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    outbreak_name        TEXT        NOT NULL,
    disease_code         TEXT        NOT NULL,
    disease_name         TEXT        NOT NULL DEFAULT '',
    status               TEXT        NOT NULL DEFAULT 'suspected',
    start_date           DATE,
    end_date             DATE,
    location_description TEXT,
    suspected_source     TEXT,
    case_count           INT         NOT NULL DEFAULT 0,
    notes                TEXT,
    tenant_id            UUID        NOT NULL,
    confirmed_at         TIMESTAMPTZ,
    controlled_at        TIMESTAMPTZ,
    closed_at            TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_outbreaks_tenant_status      ON outbreak_investigations (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_outbreaks_tenant_disease     ON outbreak_investigations (tenant_id, disease_code);
CREATE INDEX IF NOT EXISTS idx_outbreaks_tenant_active      ON outbreak_investigations (tenant_id, created_at DESC)
    WHERE status NOT IN ('closed');
