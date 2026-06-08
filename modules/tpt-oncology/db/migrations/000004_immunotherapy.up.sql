-- 000004_immunotherapy.up.sql
-- Immunotherapy and targeted therapy treatment episodes.

CREATE TABLE IF NOT EXISTS immunotherapy_episodes (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID        NOT NULL,
    oncology_patient_id  UUID        NOT NULL REFERENCES oncology_patients (id) ON DELETE RESTRICT,
    agent_name           TEXT        NOT NULL,           -- e.g. "pembrolizumab", "osimertinib"
    brand_name           TEXT        NOT NULL DEFAULT '',
    nzmt_id              TEXT        NOT NULL DEFAULT '',
    agent_class          TEXT        NOT NULL DEFAULT 'other',
    intent               TEXT        NOT NULL DEFAULT 'curative',
    -- biomarker_result captures the companion diagnostic result that justifies use,
    -- e.g. PD-L1 TPS, TMB, MSI, BRAF V600E, EGFR mutation, ALK fusion.
    biomarker_type       TEXT        NOT NULL DEFAULT '',
    biomarker_result     TEXT        NOT NULL DEFAULT '',
    prescribed_dose      TEXT        NOT NULL DEFAULT '', -- free text; dosing varies by agent
    route                TEXT        NOT NULL DEFAULT 'iv',
    frequency            TEXT        NOT NULL DEFAULT '', -- e.g. "Q3W", "daily"
    start_date           DATE,
    end_date             DATE,
    status               TEXT        NOT NULL DEFAULT 'active',
    hold_reason          TEXT        NOT NULL DEFAULT '',
    hold_date            DATE,
    resume_date          DATE,
    discontinuation_reason TEXT      NOT NULL DEFAULT '',
    prescriber_hpi       TEXT        NOT NULL DEFAULT '',
    funding_source       TEXT        NOT NULL DEFAULT '', -- PHARMAC special authority, hospital fund
    special_authority_no TEXT        NOT NULL DEFAULT '',
    notes                BYTEA,       -- encrypted
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_immunotherapy_episodes_patient
    ON immunotherapy_episodes (oncology_patient_id, status);

CREATE INDEX IF NOT EXISTS idx_immunotherapy_episodes_tenant_status
    ON immunotherapy_episodes (tenant_id, status);

CREATE INDEX IF NOT EXISTS idx_immunotherapy_episodes_agent
    ON immunotherapy_episodes (tenant_id, agent_name);

CREATE TABLE IF NOT EXISTS immunotherapy_administrations (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL,
    episode_id       UUID        NOT NULL REFERENCES immunotherapy_episodes (id) ON DELETE RESTRICT,
    cycle_number     INT         NOT NULL DEFAULT 1,
    dose_given       TEXT        NOT NULL DEFAULT '',
    route            TEXT        NOT NULL DEFAULT 'iv',
    administered_at  TIMESTAMPTZ NOT NULL,
    administered_by_hpi TEXT     NOT NULL DEFAULT '',
    infusion_duration_minutes INT,
    reaction_observed BOOLEAN    NOT NULL DEFAULT false,
    reaction_grade   SMALLINT,  -- CTCAE grade 1-5 if reaction occurred
    notes            BYTEA,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_immunotherapy_administrations_episode
    ON immunotherapy_administrations (episode_id, administered_at);
