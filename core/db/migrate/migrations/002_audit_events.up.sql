CREATE TABLE IF NOT EXISTS audit_events (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL,
    principal_id    text        NOT NULL,
    action          text        NOT NULL,
    resource_type   text,
    resource_id     text,
    patient_nhi     text,
    details         jsonb,
    ip_address      inet,
    user_agent      text,
    occurred_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_occurred_at
    ON audit_events (tenant_id, occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_events_patient_nhi
    ON audit_events (patient_nhi)
    WHERE patient_nhi IS NOT NULL;
