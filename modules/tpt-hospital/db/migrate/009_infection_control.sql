-- Infection control: HAI alerts and isolation orders
CREATE TABLE IF NOT EXISTS ic_alerts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_type  TEXT NOT NULL,
    severity    TEXT NOT NULL DEFAULT 'warning',
    ward_id     UUID,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    actions     TEXT[] NOT NULL DEFAULT '{}',
    active      BOOLEAN NOT NULL DEFAULT true,
    tenant_id   UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ic_alerts_tenant_active   ON ic_alerts (tenant_id, active);
CREATE INDEX IF NOT EXISTS idx_ic_alerts_ward            ON ic_alerts (ward_id) WHERE ward_id IS NOT NULL;

-- Isolation precaution orders per admission
CREATE TABLE IF NOT EXISTS isolation_orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id    UUID NOT NULL REFERENCES hospital_admissions(id),
    patient_id      UUID NOT NULL,
    isolation_type  TEXT NOT NULL,
    reason          TEXT NOT NULL,
    organism        TEXT,
    ppe_required    TEXT[] NOT NULL DEFAULT '{}',
    special_notes   TEXT,
    ordered_by_hpi  TEXT NOT NULL,
    tenant_id       UUID NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_isolation_orders_admission ON isolation_orders (admission_id);
CREATE INDEX IF NOT EXISTS idx_isolation_orders_active    ON isolation_orders (tenant_id) WHERE ended_at IS NULL;
