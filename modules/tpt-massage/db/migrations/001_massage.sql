-- Massage SOAP notes.
CREATE TABLE IF NOT EXISTS massage_soap_notes (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    subjective      TEXT NOT NULL DEFAULT '',
    objective       TEXT NOT NULL DEFAULT '',
    assessment      TEXT NOT NULL DEFAULT '',
    plan            TEXT NOT NULL DEFAULT '',
    body_regions    TEXT NOT NULL DEFAULT '',
    techniques      TEXT NOT NULL DEFAULT '',
    pressure        TEXT NOT NULL DEFAULT '',
    duration_min    INT NOT NULL DEFAULT 0,
    outcome         TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_massage_soap_patient ON massage_soap_notes(patient_nhi);

-- Massage contraindication screenings.
CREATE TABLE IF NOT EXISTS massage_screenings (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    conditions      TEXT NOT NULL DEFAULT '',
    medications     TEXT NOT NULL DEFAULT '',
    contraindications TEXT NOT NULL DEFAULT '',
    cleared         BOOLEAN NOT NULL DEFAULT TRUE,
    notes           TEXT NOT NULL DEFAULT '',
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_massage_screen_patient ON massage_screenings(patient_nhi);

-- Massage ACC claims.
CREATE TABLE IF NOT EXISTS massage_acc_claims (
    id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi      TEXT NOT NULL,
    provider_hpi     TEXT NOT NULL,
    date_of_accident TIMESTAMPTZ NOT NULL,
    injury_desc      TEXT NOT NULL DEFAULT '',
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

CREATE INDEX IF NOT EXISTS idx_massage_acc_patient ON massage_acc_claims(patient_nhi);
CREATE INDEX IF NOT EXISTS idx_massage_acc_status ON massage_acc_claims(status);
