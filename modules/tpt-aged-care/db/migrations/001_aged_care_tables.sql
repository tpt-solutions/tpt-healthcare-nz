-- tpt-aged-care: care plans, NASC referrals, service plans, interRAI assessments,
-- funded-hours allocations, and timesheets.

CREATE TABLE IF NOT EXISTS aged_care_plans (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID NOT NULL,
    patient_nhi         VARCHAR(7) NOT NULL,
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    responsible_hpi     VARCHAR(16) NOT NULL,
    plan_type           VARCHAR(30) NOT NULL,
    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    goals               JSONB NOT NULL DEFAULT '[]',
    interventions       JSONB NOT NULL DEFAULT '[]',
    clinical_notes      BYTEA,
    start_date          DATE NOT NULL,
    end_date            DATE,
    next_review_date    DATE,
    facility_name       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ac_plans_tenant ON aged_care_plans (tenant_id);
CREATE INDEX IF NOT EXISTS idx_ac_plans_patient ON aged_care_plans (patient_id);
CREATE INDEX IF NOT EXISTS idx_ac_plans_status ON aged_care_plans (status);

CREATE TABLE IF NOT EXISTS aged_care_nasc_referrals (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID NOT NULL,
    patient_nhi         VARCHAR(7) NOT NULL,
    referrer_hpi        VARCHAR(16) NOT NULL,
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    status              VARCHAR(20) NOT NULL DEFAULT 'pending',
    referral_reason     TEXT NOT NULL,
    urgency_flag        BOOLEAN NOT NULL DEFAULT FALSE,
    nasc_org_code       VARCHAR(20) NOT NULL,
    interrai_ref_id     UUID,
    completed_at        TIMESTAMPTZ,
    decline_reason      TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ac_nasc_ref_tenant ON aged_care_nasc_referrals (tenant_id);
CREATE INDEX IF NOT EXISTS idx_ac_nasc_ref_patient ON aged_care_nasc_referrals (patient_id);
CREATE INDEX IF NOT EXISTS idx_ac_nasc_ref_status ON aged_care_nasc_referrals (status);

CREATE TABLE IF NOT EXISTS aged_care_nasc_service_plans (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID NOT NULL,
    patient_nhi         VARCHAR(7) NOT NULL,
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    referral_id         UUID NOT NULL,
    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    needs_level         VARCHAR(20) NOT NULL,
    services            JSONB NOT NULL DEFAULT '[]',
    goals_notes         BYTEA,
    plan_start_date     DATE NOT NULL,
    plan_end_date       DATE,
    next_review_date    DATE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ac_nasc_sp_tenant ON aged_care_nasc_service_plans (tenant_id);
CREATE INDEX IF NOT EXISTS idx_ac_nasc_sp_patient ON aged_care_nasc_service_plans (patient_id);
CREATE INDEX IF NOT EXISTS idx_ac_nasc_sp_referral ON aged_care_nasc_service_plans (referral_id);
CREATE INDEX IF NOT EXISTS idx_ac_nasc_sp_status ON aged_care_nasc_service_plans (status);

CREATE TABLE IF NOT EXISTS aged_care_interrai_assessments (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID NOT NULL,
    patient_nhi         VARCHAR(7) NOT NULL,
    practitioner_hpi    VARCHAR(16) NOT NULL,
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    instrument          VARCHAR(10) NOT NULL,
    status              VARCHAR(20) NOT NULL DEFAULT 'draft',
    sections            JSONB NOT NULL DEFAULT '{}',
    scales              JSONB NOT NULL DEFAULT '{}',
    caps                JSONB NOT NULL DEFAULT '[]',
    clinical_notes      BYTEA,
    assessed_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ac_interrai_tenant ON aged_care_interrai_assessments (tenant_id);
CREATE INDEX IF NOT EXISTS idx_ac_interrai_patient ON aged_care_interrai_assessments (patient_id);
CREATE INDEX IF NOT EXISTS idx_ac_interrai_instrument ON aged_care_interrai_assessments (instrument);
CREATE INDEX IF NOT EXISTS idx_ac_interrai_status ON aged_care_interrai_assessments (status);

CREATE TABLE IF NOT EXISTS aged_care_funded_hours_allocations (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID NOT NULL,
    patient_nhi         VARCHAR(7) NOT NULL,
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    service_plan_id     UUID,
    funding_type        VARCHAR(30) NOT NULL,
    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    hours_per_week      NUMERIC(6,1) NOT NULL,
    service_type        VARCHAR(30) NOT NULL,
    provider_id         VARCHAR(16),
    provider_name       TEXT,
    start_date          DATE NOT NULL,
    end_date            DATE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ac_alloc_tenant ON aged_care_funded_hours_allocations (tenant_id);
CREATE INDEX IF NOT EXISTS idx_ac_alloc_patient ON aged_care_funded_hours_allocations (patient_id);
CREATE INDEX IF NOT EXISTS idx_ac_alloc_status ON aged_care_funded_hours_allocations (status);

CREATE TABLE IF NOT EXISTS aged_care_funded_hours_timesheets (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    allocation_id       UUID NOT NULL,
    patient_id          UUID NOT NULL,
    patient_nhi         VARCHAR(7) NOT NULL,
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    status              VARCHAR(20) NOT NULL DEFAULT 'pending',
    period_start        DATE NOT NULL,
    period_end          DATE NOT NULL,
    entries             JSONB NOT NULL DEFAULT '[]',
    total_hours         NUMERIC(6,1) NOT NULL DEFAULT 0,
    approved_by_hpi     VARCHAR(16),
    approved_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ac_ts_tenant ON aged_care_funded_hours_timesheets (tenant_id);
CREATE INDEX IF NOT EXISTS idx_ac_ts_patient ON aged_care_funded_hours_timesheets (patient_id);
CREATE INDEX IF NOT EXISTS idx_ac_ts_allocation ON aged_care_funded_hours_timesheets (allocation_id);
CREATE INDEX IF NOT EXISTS idx_ac_ts_status ON aged_care_funded_hours_timesheets (status);
