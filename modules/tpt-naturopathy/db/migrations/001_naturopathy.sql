-- Naturopathy supplement catalog.
CREATE TABLE IF NOT EXISTS naturopathy_supplements (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name         TEXT NOT NULL,
    brand        TEXT NOT NULL DEFAULT '',
    category     TEXT NOT NULL DEFAULT '',
    dosage       TEXT NOT NULL DEFAULT '',
    form         TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    created_at   BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at   BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_naturo_supplement_name ON naturopathy_supplements(name);

-- Naturopathy remedies.
CREATE TABLE IF NOT EXISTS naturopathy_remedies (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    prescribed_by   TEXT NOT NULL,
    remedy_type     TEXT NOT NULL DEFAULT '',
    name            TEXT NOT NULL DEFAULT '',
    dosage          TEXT NOT NULL DEFAULT '',
    frequency       TEXT NOT NULL DEFAULT '',
    duration_days   INT NOT NULL DEFAULT 0,
    instructions    TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_naturo_remedy_patient ON naturopathy_remedies(patient_nhi);

-- Naturopathy consultations.
CREATE TABLE IF NOT EXISTS naturopathy_consultations (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    consult_date    BIGINT NOT NULL DEFAULT 0,
    chief_complaint TEXT NOT NULL DEFAULT '',
    history         TEXT NOT NULL DEFAULT '',
    assessment      TEXT NOT NULL DEFAULT '',
    plan            TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_naturo_consult_patient ON naturopathy_consultations(patient_nhi);
