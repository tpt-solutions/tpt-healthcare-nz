-- NASC (Needs Assessment and Service Coordination) referrals and assessments.

CREATE TABLE IF NOT EXISTS disability_nasc_referrals (
    id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi           TEXT        NOT NULL DEFAULT '',
    referrer_hpi          TEXT        NOT NULL DEFAULT '',
    nasc_organisation     TEXT        NOT NULL DEFAULT '',
    referral_reason       TEXT        NOT NULL DEFAULT '',
    funding_stream        TEXT        NOT NULL DEFAULT 'DSS',
    -- DSS | EGL | ACC | MSD
    urgency               TEXT        NOT NULL DEFAULT 'routine',
    -- urgent | routine
    support_needs_summary TEXT        NOT NULL DEFAULT '',
    eligibility_status    TEXT        NOT NULL DEFAULT 'pending',
    -- pending | eligible | ineligible | deferred
    nasc_reference        TEXT,
    status                TEXT        NOT NULL DEFAULT 'draft',
    -- draft | submitted | acknowledged | assessed | allocated | closed
    notes                 TEXT,
    tenant_id             UUID        NOT NULL,
    submitted_at          TIMESTAMPTZ,
    assessed_at           TIMESTAMPTZ,
    allocated_at          TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_disability_nasc_tenant_status ON disability_nasc_referrals (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_disability_nasc_patient       ON disability_nasc_referrals (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_disability_nasc_created_at    ON disability_nasc_referrals (tenant_id, created_at DESC);
