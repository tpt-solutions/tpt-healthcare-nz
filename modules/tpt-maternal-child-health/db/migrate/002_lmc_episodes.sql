-- Core maternity episode of care: spans booking through postnatal discharge.
-- One episode per pregnancy; links all domain tables (antenatal, intrapartum, postnatal, NICU).

CREATE TABLE IF NOT EXISTS maternity_episodes (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi                 TEXT        NOT NULL DEFAULT '',
    lmc_hpi                     TEXT        NOT NULL DEFAULT '',
    status                      TEXT        NOT NULL DEFAULT 'booking',
    -- booking | antenatal | intrapartum | postnatal | completed | closed
    edd                         DATE,
    lmp                         DATE,
    gestation_at_booking_weeks  SMALLINT,
    gravida                     SMALLINT,
    parity                      SMALLINT,
    risk_level                  TEXT        NOT NULL DEFAULT 'standard',
    -- standard | enhanced | obstetric
    notes                       TEXT,
    tenant_id                   UUID        NOT NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_maternity_episodes_tenant_status ON maternity_episodes (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_maternity_episodes_nhi          ON maternity_episodes (patient_nhi);

-- LMC case-loading: formal acceptance of lead maternity carer responsibility.
-- A woman may have at most one active LMC registration per episode; handover
-- ends the current registration and starts a new one.

CREATE TABLE IF NOT EXISTS lmc_registrations (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    episode_id          UUID        NOT NULL REFERENCES maternity_episodes (id),
    lmc_hpi             TEXT        NOT NULL DEFAULT '',
    lmc_organisation    TEXT        NOT NULL DEFAULT '',
    registration_type   TEXT        NOT NULL DEFAULT 'primary',
    -- primary | secondary | handover
    accepted_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    handover_at         TIMESTAMPTZ,
    handover_to_hpi     TEXT,
    handover_reason     TEXT,
    tenant_id           UUID        NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_lmc_registrations_episode ON lmc_registrations (episode_id);
CREATE INDEX IF NOT EXISTS idx_lmc_registrations_lmc_hpi ON lmc_registrations (lmc_hpi);
