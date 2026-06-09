-- NASC (Needs Assessment and Service Coordination) referrals for discharge planning.

CREATE TABLE IF NOT EXISTS rehab_nasc_referrals (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    referrer_hpi            TEXT        NOT NULL DEFAULT '',
    nasc_region             TEXT        NOT NULL DEFAULT '',
    referral_reason         TEXT        NOT NULL DEFAULT 'long-term-support',
    -- long-term-support | disability-support | home-modification | equipment | residential-care | day-programme
    discharge_admission_id  UUID        REFERENCES rehab_admissions (id),
    urgency                 TEXT        NOT NULL DEFAULT 'routine',
    -- routine | urgent | emergency
    support_needs_summary   TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'draft',
    -- draft | submitted | acknowledged | assessment-scheduled | assessed | approved | declined | withdrawn
    nasc_reference          TEXT,
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    submitted_at            TIMESTAMPTZ,
    assessed_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rehab_nasc_tenant_status  ON rehab_nasc_referrals (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_rehab_nasc_patient        ON rehab_nasc_referrals (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_rehab_nasc_discharge      ON rehab_nasc_referrals (discharge_admission_id) WHERE discharge_admission_id IS NOT NULL;
