-- Paediatric inpatient admissions, PICU, growth records, developmental milestones,
-- and child protection flags.
-- PICU covers children >28 days old requiring intensive care. Neonates are managed
-- in the neonatal NICU (see 007_nicu.sql) linked to a maternity episode.

CREATE TABLE IF NOT EXISTS paediatric_admissions (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    proxy_guardian_nhi      TEXT        NOT NULL DEFAULT '',
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'admitted',
    -- admitted | stable | discharge-planning | discharged | transferred
    admission_type          TEXT        NOT NULL DEFAULT 'acute',
    -- elective | acute | transfer
    admission_reason        TEXT        NOT NULL DEFAULT '',
    ward                    TEXT        NOT NULL DEFAULT '',
    bed_label               TEXT        NOT NULL DEFAULT '',
    age_years               SMALLINT,
    age_months              SMALLINT,
    weight_kg               NUMERIC(5,2),
    height_cm               NUMERIC(5,1),
    tenant_id               UUID        NOT NULL,
    admitted_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    discharged_at           TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_paed_admissions_tenant_status ON paediatric_admissions (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_paed_admissions_nhi           ON paediatric_admissions (patient_nhi);

-- PICU (Paediatric Intensive Care Unit): children >28 days requiring intensive support.
CREATE TABLE IF NOT EXISTS picu_admissions (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    paediatric_admission_id UUID        REFERENCES paediatric_admissions (id),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'admitted',
    -- admitted | stable | critical | discharged
    admission_reason        TEXT        NOT NULL DEFAULT '',
    admission_type          TEXT        NOT NULL DEFAULT 'acute',
    -- acute | elective | transfer
    respiratory_support     TEXT        NOT NULL DEFAULT 'none',
    -- none | HFNC | CPAP | conventional-vent | HFOV
    tpn_active              BOOLEAN     NOT NULL DEFAULT false,
    inotropes_active        BOOLEAN     NOT NULL DEFAULT false,
    bed_label               TEXT        NOT NULL DEFAULT '',
    tenant_id               UUID        NOT NULL,
    admitted_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    discharged_at           TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_picu_admissions_tenant_status ON picu_admissions (tenant_id, status);

-- Serial growth measurements: weight, height, head circumference.
-- centile_band is stored as a text label (e.g. "50th") but calculation uses
-- WHO growth standards and is performed client-side.
CREATE TABLE IF NOT EXISTS paediatric_growth_records (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    paediatric_admission_id UUID        REFERENCES paediatric_admissions (id),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    weight_kg               NUMERIC(5,2),
    height_cm               NUMERIC(5,1),
    head_circumference_cm   NUMERIC(4,1),
    bmi                     NUMERIC(4,1),
    centile_band            TEXT,
    recorded_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id               UUID        NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_paed_growth_records_nhi ON paediatric_growth_records (patient_nhi, recorded_at DESC);

-- Developmental milestones: flagged as achieved or not-yet by domain.
CREATE TABLE IF NOT EXISTS developmental_milestones (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    paediatric_admission_id UUID        REFERENCES paediatric_admissions (id),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    domain                  TEXT        NOT NULL,
    -- gross-motor | fine-motor | speech-language | social-emotional | cognitive
    milestone_description   TEXT        NOT NULL DEFAULT '',
    expected_age_months     SMALLINT,
    achieved                BOOLEAN     NOT NULL DEFAULT false,
    achieved_at             DATE,
    concern_noted           BOOLEAN     NOT NULL DEFAULT false,
    notes                   TEXT,
    assessed_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    tenant_id               UUID        NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_developmental_milestones_nhi ON developmental_milestones (patient_nhi);

-- Child protection flags: concern raised, notified to Oranga Tamariki, etc.
-- Complies with Children's Act 2014 (NZ) mandatory reporting obligations.
CREATE TABLE IF NOT EXISTS child_protection_flags (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    paediatric_admission_id UUID        NOT NULL REFERENCES paediatric_admissions (id),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    raised_by_hpi           TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'concern-raised',
    -- none | concern-raised | notified | under-investigation
    concern_description     TEXT        NOT NULL DEFAULT '',
    notified_at             TIMESTAMPTZ,
    notified_body           TEXT,
    -- Oranga Tamariki | Police | other
    case_reference          TEXT,
    resolved_at             TIMESTAMPTZ,
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_child_protection_flags_admission ON child_protection_flags (paediatric_admission_id);
CREATE INDEX IF NOT EXISTS idx_child_protection_flags_status    ON child_protection_flags (status);
