-- Chiropractic ACC claims.
CREATE TABLE IF NOT EXISTS chiropractic_acc_claims (
    id               TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi      TEXT NOT NULL,
    provider_hpi     TEXT NOT NULL,
    practice_id      TEXT NOT NULL DEFAULT '',
    accident_date    TIMESTAMPTZ NOT NULL,
    accident_desc    TEXT NOT NULL DEFAULT '',
    diagnosis        TEXT NOT NULL DEFAULT '',
    region           TEXT NOT NULL DEFAULT '',
    visit_count      INT NOT NULL DEFAULT 0,
    total_fee        INT NOT NULL DEFAULT 0,
    status           TEXT NOT NULL DEFAULT 'draft',
    acc_claim_number TEXT NOT NULL DEFAULT '',
    notes            TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chiropractic_acc_patient ON chiropractic_acc_claims(patient_nhi);
CREATE INDEX IF NOT EXISTS idx_chiropractic_acc_status ON chiropractic_acc_claims(status);
