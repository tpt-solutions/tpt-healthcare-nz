-- 000003_treatment_cycles.up.sql
-- Treatment cycle scheduling and drug administration records.

CREATE TABLE IF NOT EXISTS treatment_cycles (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID        NOT NULL,
    patient_protocol_id   UUID        NOT NULL REFERENCES patient_protocols (id) ON DELETE RESTRICT,
    cycle_number          INT         NOT NULL,
    scheduled_date        DATE        NOT NULL,
    actual_start_date     DATE,
    actual_end_date       DATE,
    status                TEXT        NOT NULL DEFAULT 'scheduled',
    delay_reason          TEXT        NOT NULL DEFAULT '',
    omission_reason       TEXT        NOT NULL DEFAULT '',
    pre_cycle_weight_kg   NUMERIC(5, 2),
    pre_cycle_bsa_m2      NUMERIC(5, 3),
    pre_cycle_egfr        NUMERIC(6, 2), -- eGFR mL/min for renal-adjusted dosing
    pre_cycle_alb_g_l     NUMERIC(5, 2), -- albumin for dose intensity assessment
    administering_hpi     TEXT        NOT NULL DEFAULT '',
    ward_or_location      TEXT        NOT NULL DEFAULT '',
    notes                 BYTEA,         -- encrypted clinical notes
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (patient_protocol_id, cycle_number)
);

CREATE INDEX IF NOT EXISTS idx_treatment_cycles_protocol
    ON treatment_cycles (patient_protocol_id, cycle_number);

CREATE INDEX IF NOT EXISTS idx_treatment_cycles_tenant_scheduled
    ON treatment_cycles (tenant_id, scheduled_date, status);

CREATE TABLE IF NOT EXISTS cycle_administrations (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID        NOT NULL,
    treatment_cycle_id    UUID        NOT NULL REFERENCES treatment_cycles (id) ON DELETE RESTRICT,
    protocol_drug_id      UUID        NOT NULL REFERENCES protocol_drugs (id) ON DELETE RESTRICT,
    drug_name             TEXT        NOT NULL DEFAULT '',  -- denormalised for audit resilience
    planned_dose_mg       NUMERIC(10, 4),
    actual_dose_mg        NUMERIC(10, 4),
    dose_reduction_pct    NUMERIC(5, 2),                   -- percentage reduction applied
    dose_modification_reason TEXT     NOT NULL DEFAULT '',
    route                 TEXT        NOT NULL DEFAULT 'iv',
    start_time            TIMESTAMPTZ,
    end_time              TIMESTAMPTZ,
    status                TEXT        NOT NULL DEFAULT 'given',
    batch_number          TEXT        NOT NULL DEFAULT '',  -- pharmacy dispensing batch
    administered_by_hpi   TEXT        NOT NULL DEFAULT '',
    reaction_observed     BOOLEAN     NOT NULL DEFAULT false,
    reaction_description  BYTEA,       -- encrypted description if reaction_observed
    notes                 BYTEA,       -- encrypted
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cycle_administrations_cycle
    ON cycle_administrations (treatment_cycle_id);

CREATE INDEX IF NOT EXISTS idx_cycle_administrations_tenant_time
    ON cycle_administrations (tenant_id, start_time)
    WHERE start_time IS NOT NULL;
