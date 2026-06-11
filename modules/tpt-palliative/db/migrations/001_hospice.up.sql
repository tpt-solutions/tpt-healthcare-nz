-- 001_hospice.up.sql
-- Palliative care patient enrolment, visit tracking, and goals-of-care tables.
-- All palliative data is marked extra_sensitive for enhanced HIPC protection.

CREATE TABLE IF NOT EXISTS palliative_patients (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID NOT NULL,
    patient_nhi          VARCHAR(12) NOT NULL,
    primary_diagnosis    VARCHAR(512) NOT NULL,
    secondary_diagnoses  TEXT[],
    performance_status   VARCHAR(4) NOT NULL CHECK (performance_status IN ('100','90','80','70','60','50','40','30','20','10','0')),
    care_setting         VARCHAR(32) NOT NULL CHECK (care_setting IN ('home','inpatient','residential','hospital')),
    location_id          UUID,
    responsible_clinician_id VARCHAR(64) NOT NULL,
    nurse_coordinator_id VARCHAR(64),
    admission_date       TIMESTAMPTZ NOT NULL,
    expected_discharge_date TIMESTAMPTZ,
    discharge_date       TIMESTAMPTZ,
    discharge_reason     VARCHAR(32) CHECK (discharge_reason IN ('death','transfer','recovered','changed_mind')),
    advance_care_plan_id UUID,
    spiritual_needs      TEXT,
    cultural_needs       TEXT,
    preferred_place_of_death VARCHAR(128),
    dnacpr_in_place      BOOLEAN NOT NULL DEFAULT FALSE,
    extra_sensitive      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_palliative_patients_patient ON palliative_patients(tenant_id, patient_nhi);
CREATE INDEX IF NOT EXISTS idx_palliative_patients_clinician ON palliative_patients(tenant_id, responsible_clinician_id);
CREATE INDEX IF NOT EXISTS idx_palliative_patients_discharged ON palliative_patients(tenant_id, discharge_date) WHERE discharge_date IS NULL;

ALTER TABLE palliative_patients ENABLE ROW LEVEL SECURITY;
CREATE POLICY palliative_patients_tenant_only ON palliative_patients
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE TABLE IF NOT EXISTS goals_of_care (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL,
    patient_id   UUID NOT NULL REFERENCES palliative_patients(id) ON DELETE CASCADE,
    goal         TEXT NOT NULL,
    category     VARCHAR(32) NOT NULL CHECK (category IN ('comfort','symptom_control','dignity','family_support','spiritual')),
    priority     INT NOT NULL DEFAULT 1,
    achieved     BOOLEAN NOT NULL DEFAULT FALSE,
    achieved_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_goals_patient ON goals_of_care(tenant_id, patient_id, achieved);

ALTER TABLE goals_of_care ENABLE ROW LEVEL SECURITY;
CREATE POLICY goals_of_care_tenant_only ON goals_of_care
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE TABLE IF NOT EXISTS palliative_family_contacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    patient_id      UUID NOT NULL REFERENCES palliative_patients(id) ON DELETE CASCADE,
    name            VARCHAR(128) NOT NULL,
    relationship    VARCHAR(64) NOT NULL,
    phone           VARCHAR(32),
    email           VARCHAR(128),
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,
    is_emergency_contact BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_family_contacts_patient ON palliative_family_contacts(tenant_id, patient_id);

ALTER TABLE palliative_family_contacts ENABLE ROW LEVEL SECURITY;
CREATE POLICY family_contacts_tenant_only ON palliative_family_contacts
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);

CREATE TABLE IF NOT EXISTS palliative_visits (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    patient_id      UUID NOT NULL REFERENCES palliative_patients(id) ON DELETE CASCADE,
    visit_type      VARCHAR(32) NOT NULL CHECK (visit_type IN ('scheduled','urgent','virtual','bereavement')),
    visit_date      TIMESTAMPTZ NOT NULL,
    clinician_id    VARCHAR(64) NOT NULL,
    disciplines     TEXT[] NOT NULL DEFAULT '{}',
    symptoms        JSONB NOT NULL DEFAULT '[]',
    notes           TEXT,
    next_review_date TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_palliative_visits_patient ON palliative_visits(tenant_id, patient_id, visit_date DESC);
CREATE INDEX IF NOT EXISTS idx_palliative_visits_date ON palliative_visits(tenant_id, visit_date DESC);

ALTER TABLE palliative_visits ENABLE ROW LEVEL SECURITY;
CREATE POLICY palliative_visits_tenant_only ON palliative_visits
    USING (tenant_id = current_setting('app.current_tenant_id')::UUID);
