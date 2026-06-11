-- 000004_adverse_events.up.sql
-- Adverse event recording, SAE classification, and SUSAR reporting.
-- Under ICH E2A and the NZ Medicines Act 1981, fatal/life-threatening SUSARs
-- require a 7-day expedited report to Medsafe; all other SUSARs require 15 days.
-- SAE narratives are AES-256-GCM encrypted to protect participant health information.

CREATE TABLE IF NOT EXISTS ct_adverse_events (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    participant_id      UUID        NOT NULL REFERENCES ct_participants (id) ON DELETE RESTRICT,
    tenant_id           UUID        NOT NULL,
    -- ae_term is the MedDRA preferred term for the adverse event.
    ae_term             TEXT        NOT NULL DEFAULT '',
    -- meddra_code is the MedDRA PT code for structured coding.
    meddra_code         TEXT        NOT NULL DEFAULT '',
    -- ctcae_category is the CTCAE v5.0 system organ class.
    ctcae_category      TEXT        NOT NULL DEFAULT '',
    grade               INT         NOT NULL DEFAULT 1,  -- AEGrade 1–5
    status              TEXT        NOT NULL DEFAULT 'ongoing',  -- AEStatus
    causality           TEXT        NOT NULL DEFAULT 'not-assessable',  -- AECausality
    -- is_serious is true when any ICH E2A SAE criterion is met.
    is_serious          BOOLEAN     NOT NULL DEFAULT false,
    -- is_expected is false when the reaction is not listed in the IB or SmPC.
    is_expected         BOOLEAN     NOT NULL DEFAULT true,
    -- is_related_to_study_drug is the final causality determination.
    is_related_to_study_drug BOOLEAN NOT NULL DEFAULT false,
    onset_date          DATE,
    resolution_date     DATE,
    -- description is the AES-256-GCM encrypted free-text AE description.
    description         BYTEA,
    reported_by_hpi     TEXT        NOT NULL DEFAULT '',
    reported_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- visit_id links the event to the visit at which it was first captured (optional).
    visit_id            UUID        REFERENCES ct_participant_visits (id) ON DELETE RESTRICT,
    -- arm_id records which study arm the participant was on when the AE occurred.
    arm_id              UUID        REFERENCES ct_protocol_arms (id) ON DELETE RESTRICT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_adverse_events_participant
    ON ct_adverse_events (participant_id, onset_date DESC);

CREATE INDEX IF NOT EXISTS idx_ct_adverse_events_tenant_grade
    ON ct_adverse_events (tenant_id, grade, is_serious);

CREATE INDEX IF NOT EXISTS idx_ct_adverse_events_serious
    ON ct_adverse_events (tenant_id, is_serious, status)
    WHERE is_serious = true;

-- ct_saes stores the structured SAE report for events meeting ICH E2A criteria.
-- One SAE report per adverse event; follow-up reports update the same row with
-- version increments. The narrative sections are encrypted.
CREATE TABLE IF NOT EXISTS ct_saes (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    ae_id               UUID        NOT NULL UNIQUE REFERENCES ct_adverse_events (id) ON DELETE RESTRICT,
    tenant_id           UUID        NOT NULL,
    -- version increments with each follow-up report submitted to Medsafe.
    version             INT         NOT NULL DEFAULT 1,
    sae_categories      TEXT[]      NOT NULL DEFAULT '{}',  -- SAECategory values
    -- onset_to_report_days is the number of days from AE onset to initial report.
    onset_to_report_days INT,
    -- initial_report_due_at is the Medsafe reporting deadline (7 or 15 days from onset).
    initial_report_due_at TIMESTAMPTZ,
    initial_report_submitted_at TIMESTAMPTZ,
    -- follow_up_due_at is set when additional information is requested.
    follow_up_due_at    TIMESTAMPTZ,
    follow_up_submitted_at TIMESTAMPTZ,
    -- narrative is the ICH E2A structured case narrative (AES-256-GCM encrypted).
    narrative           BYTEA,
    -- treatment_given is the AES-256-GCM encrypted description of medical intervention.
    treatment_given     BYTEA,
    -- outcome at time of last report.
    outcome_at_report   TEXT        NOT NULL DEFAULT '',
    -- medsafe_report_ref is the acknowledgement number returned by Medsafe.
    medsafe_report_ref  TEXT        NOT NULL DEFAULT '',
    regulatory_status   TEXT        NOT NULL DEFAULT 'pending',  -- RegulatoryReportStatus
    reported_by_hpi     TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_saes_tenant_status
    ON ct_saes (tenant_id, regulatory_status);

CREATE INDEX IF NOT EXISTS idx_ct_saes_due
    ON ct_saes (initial_report_due_at)
    WHERE initial_report_submitted_at IS NULL;

-- ct_susars stores SUSAR (Suspected Unexpected Serious Adverse Reaction) reports.
-- SUSARs are a subset of SAEs where causality is at least possible and the reaction
-- is unexpected (not listed in the Investigator's Brochure). The sponsor must report
-- to Medsafe and notify all participating investigators.
CREATE TABLE IF NOT EXISTS ct_susars (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    sae_id              UUID        NOT NULL REFERENCES ct_saes (id) ON DELETE RESTRICT,
    ae_id               UUID        NOT NULL REFERENCES ct_adverse_events (id) ON DELETE RESTRICT,
    tenant_id           UUID        NOT NULL,
    expectedness        TEXT        NOT NULL DEFAULT 'unexpected',  -- SUSARExpectedness
    -- ib_reference is the section of the Investigator's Brochure checked for expectedness.
    ib_reference        TEXT        NOT NULL DEFAULT '',
    ib_version          TEXT        NOT NULL DEFAULT '',
    -- expedited_report determines the 7-day (fatal/life-threatening) or 15-day clock.
    expedited_report    BOOLEAN     NOT NULL DEFAULT false,
    report_due_at       TIMESTAMPTZ,
    report_submitted_at TIMESTAMPTZ,
    -- cioms_form_ref is the encrypted reference to the CIOMS I form submitted to Medsafe.
    cioms_form_ref      BYTEA,
    medsafe_report_ref  TEXT        NOT NULL DEFAULT '',
    -- investigator_notification_at is when all site investigators were notified.
    investigator_notification_at TIMESTAMPTZ,
    regulatory_status   TEXT        NOT NULL DEFAULT 'pending',  -- RegulatoryReportStatus
    reported_by_hpi     TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_susars_tenant_status
    ON ct_susars (tenant_id, regulatory_status);

CREATE INDEX IF NOT EXISTS idx_ct_susars_due
    ON ct_susars (report_due_at)
    WHERE report_submitted_at IS NULL;
