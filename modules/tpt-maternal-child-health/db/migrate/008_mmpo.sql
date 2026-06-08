-- MMPO (Midwifery and Maternity Provider Organisation) claiming.
-- Independent LMC midwives claim funding through MMPO using standardised
-- service codes from the LMC Schedule of Payments (Te Whatu Ora).

CREATE TABLE IF NOT EXISTS mmpo_claims (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    maternity_episode_id    UUID        NOT NULL REFERENCES maternity_episodes (id),
    lmc_hpi                 TEXT        NOT NULL DEFAULT '',
    mmpo_provider_number    TEXT        NOT NULL DEFAULT '',
    claim_type              TEXT        NOT NULL DEFAULT 'antenatal-visit',
    -- booking | antenatal-visit | intrapartum-primary | intrapartum-secondary
    -- postnatal-visit | on-call | rural-premium | other
    service_date            DATE        NOT NULL,
    service_code            TEXT        NOT NULL DEFAULT '',
    units                   NUMERIC(5,2) NOT NULL DEFAULT 1,
    amount_nzd              NUMERIC(10,2),
    status                  TEXT        NOT NULL DEFAULT 'draft',
    -- draft | submitted | accepted | rejected | paid | withdrawn
    claim_reference         TEXT,
    submitted_at            TIMESTAMPTZ,
    response_code           TEXT,
    response_message        TEXT,
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mmpo_claims_episode ON mmpo_claims (maternity_episode_id);
CREATE INDEX IF NOT EXISTS idx_mmpo_claims_lmc_status ON mmpo_claims (lmc_hpi, status);
