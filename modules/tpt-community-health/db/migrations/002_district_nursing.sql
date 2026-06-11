-- District nursing care plans and visit records.

CREATE TABLE IF NOT EXISTS community_care_plans (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi    TEXT        NOT NULL DEFAULT '',
    -- AES-256-GCM encrypted
    clinician_hpi  TEXT        NOT NULL DEFAULT '',
    plan_name      TEXT        NOT NULL DEFAULT '',
    plan_type      TEXT        NOT NULL DEFAULT '',
    -- wound-care | palliative | diabetes | heart-failure | copd |
    -- post-surgical | post-acute | medication-management
    status         TEXT        NOT NULL DEFAULT 'draft',
    -- draft | active | under-review | completed | suspended
    risk_level     TEXT        NOT NULL DEFAULT 'low',
    -- low | moderate | high | very-high
    primary_need   TEXT        NOT NULL DEFAULT '',
    goals          TEXT        NOT NULL DEFAULT '',
    dhb_funded     BOOLEAN     NOT NULL DEFAULT FALSE,
    funding_code   TEXT,
    consent_given  BOOLEAN     NOT NULL DEFAULT FALSE,
    consent_at     TIMESTAMPTZ,
    notes          TEXT,
    tenant_id      UUID        NOT NULL,
    started_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    review_at      TIMESTAMPTZ,
    completed_at   TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_community_care_plans_tenant_status   ON community_care_plans (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_community_care_plans_tenant_type     ON community_care_plans (tenant_id, plan_type);
CREATE INDEX IF NOT EXISTS idx_community_care_plans_patient         ON community_care_plans (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_community_care_plans_clinician       ON community_care_plans (clinician_hpi);
CREATE INDEX IF NOT EXISTS idx_community_care_plans_started         ON community_care_plans (tenant_id, started_at DESC);

CREATE TABLE IF NOT EXISTS community_nursing_visits (
    id                       UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    care_plan_id             UUID        NOT NULL REFERENCES community_care_plans (id),
    patient_nhi              TEXT        NOT NULL DEFAULT '',
    -- AES-256-GCM encrypted
    clinician_hpi            TEXT        NOT NULL DEFAULT '',
    visit_type               TEXT        NOT NULL DEFAULT 'scheduled',
    -- scheduled | unscheduled | urgent
    status                   TEXT        NOT NULL DEFAULT 'scheduled',
    -- scheduled | in-progress | completed | cancelled
    vital_signs              JSONB,
    -- {temperature, bp_systolic, bp_diastolic, heart_rate, spo2, pain_score, weight_kg, respiratory_rate, blood_glucose}
    wound_assessments        JSONB,
    -- array of wound assessment objects
    medications_administered JSONB,
    -- array of medication administration records
    observations             TEXT,
    patient_education        TEXT,
    concerns                 TEXT,
    escalations              TEXT,
    follow_up_required       BOOLEAN     NOT NULL DEFAULT FALSE,
    notes                    TEXT,
    tenant_id                UUID        NOT NULL,
    scheduled_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at             TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_community_nursing_visits_plan        ON community_nursing_visits (care_plan_id);
CREATE INDEX IF NOT EXISTS idx_community_nursing_visits_patient     ON community_nursing_visits (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_community_nursing_visits_clinician   ON community_nursing_visits (clinician_hpi);
CREATE INDEX IF NOT EXISTS idx_community_nursing_visits_tenant      ON community_nursing_visits (tenant_id, scheduled_at DESC);
CREATE INDEX IF NOT EXISTS idx_community_nursing_visits_status      ON community_nursing_visits (tenant_id, status);
