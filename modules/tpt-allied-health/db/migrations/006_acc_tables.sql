-- Allied health ACC claims table
CREATE TABLE IF NOT EXISTS acc_claims (
    id                      TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi             TEXT NOT NULL,
    clinician_id            TEXT NOT NULL,
    practice_id             TEXT NOT NULL,
    claim_type              TEXT NOT NULL,
    acc_number              TEXT,
    status                  TEXT NOT NULL DEFAULT 'draft',
    diagnosis               TEXT NOT NULL,
    icd10_code              TEXT,
    body_region             TEXT NOT NULL,
    injury_mechanism        TEXT,
    referrer                TEXT,
    approved_sessions       INTEGER NOT NULL DEFAULT 0,
    used_sessions           INTEGER NOT NULL DEFAULT 0,
    start_date              BIGINT NOT NULL,
    expiry_date             BIGINT NOT NULL,
    last_treatment_date     BIGINT,
    next_review_date        BIGINT,
    clinical_notes          TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_acc_claims_patient ON acc_claims(patient_nhi);
CREATE INDEX IF NOT EXISTS idx_acc_claims_clinician ON acc_claims(clinician_id);
CREATE INDEX IF NOT EXISTS idx_acc_claims_status ON acc_claims(status);
CREATE INDEX IF NOT EXISTS idx_acc_claims_number ON acc_claims(acc_number);

-- Allied health ACC treatment sessions
CREATE TABLE IF NOT EXISTS acc_treatment_sessions (
    id                TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    claim_id          TEXT NOT NULL REFERENCES acc_claims(id),
    patient_nhi       TEXT NOT NULL,
    clinician_id      TEXT NOT NULL,
    session_date      TIMESTAMP NOT NULL,
    session_number    INTEGER NOT NULL,
    duration_minutes  INTEGER NOT NULL,
    charge_code       TEXT NOT NULL,
    charge_amount     NUMERIC(10,2) NOT NULL,
    treatment_type    TEXT,
    body_region       TEXT,
    subjective        TEXT,
    objective         TEXT,
    assessment        TEXT,
    plan              TEXT,
    status            TEXT NOT NULL DEFAULT 'planned',
    submitted_at      BIGINT,
    paid_at           BIGINT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_acc_sessions_claim ON acc_treatment_sessions(claim_id);
CREATE INDEX IF NOT EXISTS idx_acc_sessions_date ON acc_treatment_sessions(session_date);

-- Allied health ACC review reports
CREATE TABLE IF NOT EXISTS acc_review_reports (
    id                          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    claim_id                    TEXT NOT NULL REFERENCES acc_claims(id),
    patient_nhi                 TEXT NOT NULL,
    clinician_id                TEXT NOT NULL,
    report_date                 BIGINT NOT NULL,
    report_type                 TEXT NOT NULL,
    sessions_since_last_review  INTEGER,
    progress_summary            TEXT,
    current_status              TEXT,
    goals_achieved              JSONB,
    goals_ongoing               JSONB,
    goals_not_achieved          JSONB,
    outcome_measures            JSONB,
    recommendation              TEXT NOT NULL,
    additional_sessions_requested INTEGER,
    proposed_end_date           BIGINT,
    status                      TEXT NOT NULL DEFAULT 'draft',
    submitted_at                BIGINT,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_acc_reviews_claim ON acc_review_reports(claim_id);
