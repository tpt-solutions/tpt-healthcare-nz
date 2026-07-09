-- Acupuncture needle sessions.
CREATE TABLE IF NOT EXISTS acupuncture_needle_sessions (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    session_date    BIGINT NOT NULL DEFAULT 0,
    body_regions    TEXT NOT NULL DEFAULT '',
    needle_count    INT NOT NULL DEFAULT 0,
    techniques      TEXT NOT NULL DEFAULT '',
    duration_min    INT NOT NULL DEFAULT 0,
    outcome         TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_acupuncture_session_patient ON acupuncture_needle_sessions(patient_nhi);

-- Acupuncture treatment records.
CREATE TABLE IF NOT EXISTS acupuncture_treatments (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    session_id      TEXT NOT NULL DEFAULT '',
    treatment_date  BIGINT NOT NULL DEFAULT 0,
    diagnosis       TEXT NOT NULL DEFAULT '',
    protocol        TEXT NOT NULL DEFAULT '',
    outcome         TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_acupuncture_treatment_patient ON acupuncture_treatments(patient_nhi);

-- Acupuncture ACC claims.
CREATE TABLE IF NOT EXISTS acupuncture_acc_claims (
    id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi      TEXT NOT NULL,
    provider_hpi     TEXT NOT NULL,
    accident_date    TIMESTAMPTZ NOT NULL,
    accident_desc    TEXT NOT NULL DEFAULT '',
    diagnosis        TEXT NOT NULL DEFAULT '',
    injury_region    TEXT NOT NULL DEFAULT '',
    visit_count      INT NOT NULL DEFAULT 0,
    total_fee        INT NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'draft',
    acc_claim_number TEXT NOT NULL DEFAULT '',
    notes            TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_acupuncture_acc_patient ON acupuncture_acc_claims(patient_nhi);
CREATE INDEX IF NOT EXISTS idx_acupuncture_acc_status ON acupuncture_acc_claims(status);
