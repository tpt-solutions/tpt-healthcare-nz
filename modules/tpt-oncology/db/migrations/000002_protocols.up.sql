-- 000002_protocols.up.sql
-- Chemotherapy protocol library and patient protocol assignments.

CREATE TABLE IF NOT EXISTS chemotherapy_protocols (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL,
    name             TEXT        NOT NULL,           -- e.g. "R-CHOP", "FOLFOX-6"
    acronym          TEXT        NOT NULL DEFAULT '', -- short code used in reporting
    version          TEXT        NOT NULL DEFAULT '1.0',
    category         TEXT        NOT NULL DEFAULT 'other',
    intent           TEXT        NOT NULL DEFAULT 'curative', -- curative|palliative|adjuvant|neoadjuvant
    cycle_length_days INT        NOT NULL DEFAULT 21,
    total_cycles     INT,                            -- null = determined per patient
    reference_url    TEXT        NOT NULL DEFAULT '',
    notes            TEXT        NOT NULL DEFAULT '',
    status           TEXT        NOT NULL DEFAULT 'active',
    created_by_hpi   TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_chemotherapy_protocols_name_version
    ON chemotherapy_protocols (tenant_id, name, version);

CREATE INDEX IF NOT EXISTS idx_chemotherapy_protocols_category
    ON chemotherapy_protocols (tenant_id, category, status);

CREATE TABLE IF NOT EXISTS protocol_drugs (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    protocol_id      UUID        NOT NULL REFERENCES chemotherapy_protocols (id) ON DELETE CASCADE,
    drug_name        TEXT        NOT NULL,
    generic_name     TEXT        NOT NULL DEFAULT '',
    nzmt_id          TEXT        NOT NULL DEFAULT '', -- NZMT product ID (PHARMAC formulary)
    dose_mg_m2       NUMERIC(10, 4),                 -- dose per m² BSA; null if flat-dosed
    dose_mg_flat     NUMERIC(10, 4),                 -- flat dose (mg); null if weight-based
    dose_auc         NUMERIC(6, 2),                  -- carboplatin AUC dosing
    route            TEXT        NOT NULL DEFAULT 'iv',
    day_of_cycle     INT[]       NOT NULL DEFAULT '{}', -- e.g. {1,8} for D1 and D8
    infusion_duration_minutes INT,
    premedication    TEXT        NOT NULL DEFAULT '',
    sequence_order   INT         NOT NULL DEFAULT 1,
    notes            TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_protocol_drugs_protocol
    ON protocol_drugs (protocol_id, sequence_order);

CREATE TABLE IF NOT EXISTS patient_protocols (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID        NOT NULL,
    oncology_patient_id  UUID        NOT NULL REFERENCES oncology_patients (id) ON DELETE RESTRICT,
    protocol_id          UUID        NOT NULL REFERENCES chemotherapy_protocols (id) ON DELETE RESTRICT,
    assigned_by_hpi      TEXT        NOT NULL DEFAULT '',
    planned_cycles       INT,
    completed_cycles     INT         NOT NULL DEFAULT 0,
    start_date           DATE,
    end_date             DATE,
    status               TEXT        NOT NULL DEFAULT 'planned',
    discontinuation_reason TEXT      NOT NULL DEFAULT '',
    bsa_m2               NUMERIC(5, 3), -- body surface area used for dosing
    notes                BYTEA,         -- encrypted clinical notes
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_patient_protocols_patient
    ON patient_protocols (oncology_patient_id, status);

CREATE INDEX IF NOT EXISTS idx_patient_protocols_tenant
    ON patient_protocols (tenant_id, status);
