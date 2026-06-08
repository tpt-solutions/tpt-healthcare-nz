-- 000004_transplant_waitlist.up.sql
-- Renal transplant waitlist management.

CREATE TABLE IF NOT EXISTS transplant_waitlist_entries (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id               UUID        NOT NULL,
    renal_patient_id        UUID        NOT NULL REFERENCES renal_patients (id) ON DELETE RESTRICT,
    listing_date            DATE        NOT NULL,
    listing_centre          TEXT        NOT NULL DEFAULT '',
    blood_group             TEXT        NOT NULL DEFAULT '',
    -- HLA type is encrypted (PHI — HIPC Rule 5)
    hla_type                BYTEA,
    -- PRA: panel reactive antibodies percentage (0-100)
    pra_percent             SMALLINT,
    mismatch_policy         TEXT        NOT NULL DEFAULT '',
    -- living or deceased-donor
    transplant_type         TEXT        NOT NULL DEFAULT '',
    donor_relationship      TEXT        NOT NULL DEFAULT '',
    -- Status: listed, on-hold, transplanted, removed
    status                  TEXT        NOT NULL DEFAULT 'listed',
    hold_reason             TEXT        NOT NULL DEFAULT '',
    removal_reason          TEXT        NOT NULL DEFAULT '',
    transplant_date         DATE,
    transplant_centre       TEXT        NOT NULL DEFAULT '',
    -- Graft function: immediate, delayed, primary-non-function
    graft_function          TEXT        NOT NULL DEFAULT '',
    rejection_episodes      SMALLINT    NOT NULL DEFAULT 0,
    rejection_treatment     TEXT        NOT NULL DEFAULT '',
    creatinine_at_3m_umol_l NUMERIC(8,2),
    creatinine_at_12m_umol_l NUMERIC(8,2),
    last_review_date        DATE,
    nephrologist_hpi        TEXT        NOT NULL DEFAULT '',
    notes                   TEXT        NOT NULL DEFAULT '',
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_transplant_waitlist_patient
    ON transplant_waitlist_entries (renal_patient_id);

CREATE INDEX IF NOT EXISTS idx_transplant_waitlist_tenant_status
    ON transplant_waitlist_entries (tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_transplant_waitlist_listing_date
    ON transplant_waitlist_entries (tenant_id, listing_date)
    WHERE status = 'listed';
