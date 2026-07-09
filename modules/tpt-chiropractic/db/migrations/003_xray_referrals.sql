-- Chiropractic X-ray referrals.
CREATE TABLE IF NOT EXISTS chiropractic_xray_referrals (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi  TEXT NOT NULL,
    clinician_id TEXT NOT NULL,
    practice_id  TEXT NOT NULL DEFAULT '',
    region       TEXT NOT NULL DEFAULT '',
    views        TEXT NOT NULL DEFAULT '',
    urgency      TEXT NOT NULL DEFAULT 'routine',
    indication   TEXT NOT NULL DEFAULT '',
    findings     TEXT NOT NULL DEFAULT '',
    radiologist  TEXT NOT NULL DEFAULT '',
    report_url   TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'ordered',
    created_at   BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at   BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_chiropractic_xray_patient ON chiropractic_xray_referrals(patient_nhi);
