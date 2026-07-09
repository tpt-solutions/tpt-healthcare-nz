-- Counselling EAP claims.
CREATE TABLE IF NOT EXISTS counselling_eap_claims (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    client_nhi      TEXT NOT NULL,
    counsellor_id   TEXT NOT NULL,
    eap_provider    TEXT NOT NULL DEFAULT '',
    session_count   INT NOT NULL DEFAULT 0,
    session_fee     INT NOT NULL DEFAULT 0,
    total_fee       INT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'submitted',
    reference       TEXT NOT NULL DEFAULT '',
    invoice_number  TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_counselling_eap_client ON counselling_eap_claims(client_nhi);
CREATE INDEX IF NOT EXISTS idx_counselling_eap_status ON counselling_eap_claims(status);

-- Counselling session notes (mental health — extra-sensitive).
CREATE TABLE IF NOT EXISTS counselling_sessions (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    client_nhi      TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    practice_id     TEXT NOT NULL DEFAULT '',
    session_date    BIGINT NOT NULL DEFAULT 0,
    session_number  INT NOT NULL DEFAULT 0,
    modality        TEXT NOT NULL DEFAULT '',
    mode            TEXT NOT NULL DEFAULT '',
    duration_min    INT NOT NULL DEFAULT 0,
    presenting_issue TEXT NOT NULL DEFAULT '',
    clinical_notes  TEXT NOT NULL DEFAULT '',
    risk_assessment TEXT NOT NULL DEFAULT '',
    intervention    TEXT NOT NULL DEFAULT '',
    outcome         TEXT NOT NULL DEFAULT '',
    homework        TEXT NOT NULL DEFAULT '',
    next_session_date BIGINT NOT NULL DEFAULT 0,
    billing_type    TEXT NOT NULL DEFAULT '',
    fee_in_cents    INT NOT NULL DEFAULT 0,
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_counselling_session_client ON counselling_sessions(client_nhi);

-- Private practice clients (PHI encrypted at rest).
CREATE TABLE IF NOT EXISTS counselling_private_clients (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name_enc     BYTEA NOT NULL,
    email_enc    BYTEA NOT NULL,
    phone_enc    BYTEA NOT NULL,
    nhi_enc      BYTEA NOT NULL,
    employer     TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at   BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at   BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

-- Private practice invoices.
CREATE TABLE IF NOT EXISTS counselling_private_invoices (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    client_nhi      TEXT NOT NULL,
    invoice_number  TEXT NOT NULL DEFAULT '',
    sessions        INT NOT NULL DEFAULT 0,
    session_fee     INT NOT NULL DEFAULT 0,
    total_amount    INT NOT NULL DEFAULT 0,
    tax_amount      INT NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'draft',
    due_date        BIGINT NOT NULL DEFAULT 0,
    paid_date       BIGINT NOT NULL DEFAULT 0,
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_counselling_inv_client ON counselling_private_invoices(client_nhi);
