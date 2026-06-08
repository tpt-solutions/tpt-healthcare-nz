-- 002_advance_care_planning.sql
-- Advance Care Plan (ACP) tables: plan document, legal proxies, and care decisions.

CREATE TABLE IF NOT EXISTS acp_plans (
    id                              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                       UUID NOT NULL,
    patient_nhi                     VARCHAR(12) NOT NULL,
    status                          VARCHAR(32) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft','proposed','active','suspended','completed','withdrawn')),
    treatment_intent                VARCHAR(32) NOT NULL
        CHECK (treatment_intent IN ('curative','palliative','symptom_control','comfort_only')),
    dnacpr                          BOOLEAN NOT NULL DEFAULT FALSE,
    dnacpr_documented_at            TIMESTAMPTZ,
    dnacpr_signed_by                VARCHAR(128),
    resuscitation_discussed_patient BOOLEAN NOT NULL DEFAULT FALSE,
    resuscitation_discussed_family  BOOLEAN NOT NULL DEFAULT FALSE,
    preferred_place_of_care         VARCHAR(128),
    preferred_place_of_death        VARCHAR(128),
    organ_donation_wishes           BOOLEAN,
    spiritual_wishes                TEXT,
    cultural_wishes                 TEXT,
    legal_proxy_name                VARCHAR(128),
    legal_proxy_relationship        VARCHAR(64),
    legal_proxy_phone               VARCHAR(32),
    legal_proxy_email               VARCHAR(128),
    legal_proxy_epa_reference       VARCHAR(64),
    legal_proxy_is_active           BOOLEAN NOT NULL DEFAULT FALSE,
    review_date                     TIMESTAMPTZ NOT NULL,
    extra_sensitive                 BOOLEAN NOT NULL DEFAULT TRUE,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_acp_plans_patient ON acp_plans(tenant_id, patient_nhi);
CREATE INDEX idx_acp_plans_status ON acp_plans(tenant_id, status) WHERE status IN ('draft','proposed','active');

ALTER TABLE acp_plans ENABLE ROW LEVEL SECURITY;
CREATE POLICY acp_plans_tenant_only ON acp_plans
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

-- Individual care decisions within a plan
CREATE TABLE IF NOT EXISTS acp_decisions (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID NOT NULL,
    plan_id                  UUID NOT NULL REFERENCES acp_plans(id) ON DELETE CASCADE,
    treatment                VARCHAR(64) NOT NULL
        CHECK (treatment IN ('mechanical_ventilation','cpr','antibiotics','artificial_nutrition','artificial_hydration','dialysis','blood_transfusion','hospital_transfer')),
    decision                 VARCHAR(32) NOT NULL CHECK (decision IN ('yes','no','maybe','time_limited')),
    reason                   TEXT,
    time_limited_until       TIMESTAMPTZ,
    clinical_recommendation  TEXT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_acp_decisions_plan ON acp_decisions(tenant_id, plan_id, treatment);

ALTER TABLE acp_decisions ENABLE ROW LEVEL SECURITY;
CREATE POLICY acp_decisions_tenant_only ON acp_decisions
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);
