-- Osteopathy assessments.
CREATE TABLE IF NOT EXISTS osteopathy_assessments (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi  TEXT NOT NULL,
    clinician_id TEXT NOT NULL,
    chief_complaint TEXT NOT NULL DEFAULT '',
    history      TEXT NOT NULL DEFAULT '',
    examination  TEXT NOT NULL DEFAULT '',
    diagnosis    TEXT NOT NULL DEFAULT '',
    treatment_plan TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    created_at   BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at   BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_osteopathy_assess_patient ON osteopathy_assessments(patient_nhi);

-- Osteopathy treatments.
CREATE TABLE IF NOT EXISTS osteopathy_treatments (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    assessment_id   TEXT NOT NULL DEFAULT '',
    treatment_date  BIGINT NOT NULL DEFAULT 0,
    techniques      TEXT NOT NULL DEFAULT '',
    body_regions    TEXT NOT NULL DEFAULT '',
    duration_min    INT NOT NULL DEFAULT 0,
    outcome         TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_osteopathy_treatment_patient ON osteopathy_treatments(patient_nhi);
