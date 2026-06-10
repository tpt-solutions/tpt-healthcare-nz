-- 000001_protocols.up.sql
-- Study protocol library, arms, eligibility criteria, visit schedule, and amendments.
-- HDEC approval number and ANZCTR registration number are required before a protocol
-- can be activated (status -> active). Amendments are version-tracked and append-only.

CREATE TABLE IF NOT EXISTS ct_protocols (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID        NOT NULL,
    title               TEXT        NOT NULL DEFAULT '',
    short_title         TEXT        NOT NULL DEFAULT '',
    -- anzctr_number is the Australia and New Zealand Clinical Trials Registry ID.
    anzctr_number       TEXT        NOT NULL DEFAULT '',
    -- hdec_number is the Health and Disability Ethics Committee approval number.
    hdec_number         TEXT        NOT NULL DEFAULT '',
    -- sponsor is the organisation funding / responsible for the trial.
    sponsor             TEXT        NOT NULL DEFAULT '',
    principal_investigator_hpi TEXT NOT NULL DEFAULT '',
    phase               TEXT        NOT NULL DEFAULT '',   -- TrialPhase
    trial_type          TEXT        NOT NULL DEFAULT 'interventional',
    intervention_type   TEXT        NOT NULL DEFAULT '',
    blinding            TEXT        NOT NULL DEFAULT 'open-label',
    allocation          TEXT        NOT NULL DEFAULT 'randomised',
    -- condition is the primary ICD-10-AM code for the disease under study.
    icd10_code          TEXT        NOT NULL DEFAULT '',
    condition_name      TEXT        NOT NULL DEFAULT '',
    target_enrolment    INT         NOT NULL DEFAULT 0,
    status              TEXT        NOT NULL DEFAULT 'draft',
    approved_at         TIMESTAMPTZ,
    opened_at           TIMESTAMPTZ,
    closed_at           TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    -- protocol_document is the AES-256-GCM encrypted full protocol PDF reference or text.
    protocol_document   BYTEA,
    notes               TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_protocols_tenant_status
    ON ct_protocols (tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_ct_protocols_anzctr
    ON ct_protocols (anzctr_number)
    WHERE anzctr_number <> '';

-- ct_protocol_arms defines the parallel groups within a study (control, treatment, dose levels).
CREATE TABLE IF NOT EXISTS ct_protocol_arms (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    protocol_id     UUID        NOT NULL REFERENCES ct_protocols (id) ON DELETE RESTRICT,
    tenant_id       UUID        NOT NULL,
    name            TEXT        NOT NULL DEFAULT '',
    arm_type        TEXT        NOT NULL DEFAULT '',   -- experimental, active-comparator, placebo, sham
    description     TEXT        NOT NULL DEFAULT '',
    intervention    TEXT        NOT NULL DEFAULT '',
    dose            TEXT        NOT NULL DEFAULT '',
    route           TEXT        NOT NULL DEFAULT '',
    frequency       TEXT        NOT NULL DEFAULT '',
    duration        TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_protocol_arms_protocol
    ON ct_protocol_arms (protocol_id);

-- ct_eligibility_criteria stores the inclusion and exclusion criteria for a protocol.
CREATE TABLE IF NOT EXISTS ct_eligibility_criteria (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    protocol_id     UUID        NOT NULL REFERENCES ct_protocols (id) ON DELETE RESTRICT,
    tenant_id       UUID        NOT NULL,
    criterion_type  TEXT        NOT NULL DEFAULT 'inclusion', -- inclusion | exclusion
    sequence_no     INT         NOT NULL DEFAULT 0,
    text            TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_eligibility_criteria_protocol
    ON ct_eligibility_criteria (protocol_id, criterion_type);

-- ct_scheduled_visits defines the visit schedule template for a protocol.
-- Actual participant visits are tracked in ct_participant_visits.
CREATE TABLE IF NOT EXISTS ct_scheduled_visits (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    protocol_id     UUID        NOT NULL REFERENCES ct_protocols (id) ON DELETE RESTRICT,
    tenant_id       UUID        NOT NULL,
    name            TEXT        NOT NULL DEFAULT '',
    visit_type      TEXT        NOT NULL DEFAULT '',  -- VisitType
    sequence_no     INT         NOT NULL DEFAULT 0,
    -- day_from_baseline is the target study day (negative values for screening visits).
    day_from_baseline INT       NOT NULL DEFAULT 0,
    window_before   INT         NOT NULL DEFAULT 0,   -- allowed days early
    window_after    INT         NOT NULL DEFAULT 0,   -- allowed days late
    -- assessments_required is a JSONB array of assessment descriptors for CRF template.
    assessments_required JSONB  NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_scheduled_visits_protocol
    ON ct_scheduled_visits (protocol_id, sequence_no);

-- ct_protocol_amendments records HDEC-approved changes to the protocol after initial approval.
CREATE TABLE IF NOT EXISTS ct_protocol_amendments (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    protocol_id         UUID        NOT NULL REFERENCES ct_protocols (id) ON DELETE RESTRICT,
    tenant_id           UUID        NOT NULL,
    amendment_number    INT         NOT NULL DEFAULT 1,
    hdec_amendment_ref  TEXT        NOT NULL DEFAULT '',
    summary             TEXT        NOT NULL DEFAULT '',
    -- full_text is the AES-256-GCM encrypted amendment document text.
    full_text           BYTEA,
    effective_date      DATE,
    approved_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_protocol_amendments_protocol
    ON ct_protocol_amendments (protocol_id, amendment_number);
