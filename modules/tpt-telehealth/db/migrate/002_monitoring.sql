-- Remote patient monitoring: registered devices and submitted observations.
-- patient_nhi is encrypted at rest in both tables.
CREATE TABLE IF NOT EXISTS monitoring_devices (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi   TEXT        NOT NULL,
    device_type   TEXT        NOT NULL,
    manufacturer  TEXT,
    model         TEXT,
    serial_number TEXT,
    status        TEXT        NOT NULL DEFAULT 'active',
    tenant_id     TEXT        NOT NULL,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS monitoring_devices_tenant
    ON monitoring_devices (tenant_id, status);

CREATE TABLE IF NOT EXISTS monitoring_observations (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi      TEXT        NOT NULL,
    device_id        UUID        REFERENCES monitoring_devices (id) ON DELETE SET NULL,
    loinc_code       TEXT        NOT NULL,
    observation_type TEXT        NOT NULL,
    value_quantity   NUMERIC,
    value_unit       TEXT,
    value_string     TEXT,
    effective_at     TIMESTAMPTZ NOT NULL,
    source           TEXT        NOT NULL DEFAULT 'device',
    tenant_id        TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS monitoring_obs_tenant_patient
    ON monitoring_observations (tenant_id, patient_nhi, effective_at DESC);

CREATE INDEX IF NOT EXISTS monitoring_obs_loinc
    ON monitoring_observations (tenant_id, loinc_code, effective_at DESC);
