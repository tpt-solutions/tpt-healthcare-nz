-- ACC rehabilitation plans (injury-related funded rehabilitation).

CREATE TABLE IF NOT EXISTS rehab_acc_plans (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    acc_claim_number        TEXT        NOT NULL DEFAULT '',
    acc_contract_type       TEXT        NOT NULL DEFAULT 'social-rehabilitation',
    -- social-rehabilitation | vocational-rehabilitation | home-modifications | equipment
    injury_description      TEXT        NOT NULL DEFAULT '',
    rehabilitation_goals    TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'draft',
    -- draft | submitted | approved | active | review | completed | declined
    funding_approved_nzd    NUMERIC(10,2),
    funding_spent_nzd       NUMERIC(10,2) NOT NULL DEFAULT 0,
    review_date             DATE,
    plan_date               DATE        NOT NULL DEFAULT now()::date,
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    approved_at             TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rehab_acc_plans_tenant_status ON rehab_acc_plans (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_rehab_acc_plans_patient       ON rehab_acc_plans (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_rehab_acc_plans_claim         ON rehab_acc_plans (acc_claim_number) WHERE acc_claim_number != '';
