-- TCM herb catalog.
CREATE TABLE IF NOT EXISTS tcm_herbs (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name         TEXT NOT NULL,
    pinyin       TEXT NOT NULL DEFAULT '',
    category     TEXT NOT NULL DEFAULT '',
    properties   TEXT NOT NULL DEFAULT '',
    dosage       TEXT NOT NULL DEFAULT '',
    contraindications TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    created_at   BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at   BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_tcm_herb_name ON tcm_herbs(name);

-- TCM herbal prescriptions.
CREATE TABLE IF NOT EXISTS tcm_prescriptions (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    diagnosis       TEXT NOT NULL DEFAULT '',
    herbs           JSONB NOT NULL DEFAULT '[]',
    dosage          TEXT NOT NULL DEFAULT '',
    duration_days   INT NOT NULL DEFAULT 0,
    instructions    TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_tcm_rx_patient ON tcm_prescriptions(patient_nhi);

-- TCM diagnoses.
CREATE TABLE IF NOT EXISTS tcm_diagnoses (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    tongue          TEXT NOT NULL DEFAULT '',
    pulse           TEXT NOT NULL DEFAULT '',
    syndrome        TEXT NOT NULL DEFAULT '',
    pattern         TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_tcm_diag_patient ON tcm_diagnoses(patient_nhi);
