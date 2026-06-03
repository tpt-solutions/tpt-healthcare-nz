-- 003_mental_health_tables.sql
-- Mental Health module clinical tables.
-- All PHI columns are AES-256-GCM encrypted at the application layer (core/encryption).
-- Every row carries extra_sensitive = true (default) to enforce HIPC additional
-- protections before any read or third-party disclosure.

-- ---------------------------------------------------------------------------
-- Mental Health Episodes of Care (inpatient or community)
-- Must be created before mh_assessments so the FK can reference it.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS mh_episodes (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi         TEXT        NOT NULL DEFAULT '',
    tenant_id           UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    responsible_hpi     TEXT        NOT NULL,
    episode_type        TEXT        NOT NULL
                                     CHECK (episode_type IN ('inpatient','community','crisis','day-programme')),
    status              TEXT        NOT NULL DEFAULT 'active'
                                     CHECK (status IN ('active','on-hold','completed','transferred','deceased')),
    admission_reason    BYTEA       NOT NULL DEFAULT '',  -- encrypted clinical notes
    primary_diagnosis   TEXT        NOT NULL DEFAULT '',  -- ICD-10-AM code
    secondary_diagnoses TEXT[]      NOT NULL DEFAULT '{}',
    ward_or_team        TEXT        NOT NULL DEFAULT '',
    bed_number          TEXT        NOT NULL DEFAULT '',
    admitted_at         TIMESTAMPTZ,
    discharged_at       TIMESTAMPTZ,
    discharge_summary   BYTEA,                            -- encrypted FHIR DocumentReference JSON
    extra_sensitive     BOOLEAN     NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS mh_episodes_tenant_idx   ON mh_episodes (tenant_id);
CREATE INDEX IF NOT EXISTS mh_episodes_patient_idx  ON mh_episodes (patient_id);
CREATE INDEX IF NOT EXISTS mh_episodes_status_idx   ON mh_episodes (status);
CREATE INDEX IF NOT EXISTS mh_episodes_type_idx     ON mh_episodes (episode_type);
CREATE INDEX IF NOT EXISTS mh_episodes_admitted_idx ON mh_episodes (admitted_at);

ALTER TABLE mh_episodes ENABLE ROW LEVEL SECURITY;

CREATE POLICY IF NOT EXISTS mh_episodes_tenant_isolation
    ON mh_episodes
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Mental Health Assessments (PHQ-9, GAD-7, AUDIT-C, HoNOS, etc.)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS mh_assessments (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id       UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi      TEXT        NOT NULL DEFAULT '',
    practitioner_hpi TEXT        NOT NULL,
    tenant_id        UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    episode_id       UUID        REFERENCES mh_episodes (id),
    tool             TEXT        NOT NULL
                                 CHECK (tool IN (
                                     'PHQ-9','GAD-7','AUDIT-C','HoNOS','HoNOSCA',
                                     'HoNOS65+','CANSAS','BASIS-32','DASS-21','MINI'
                                 )),
    scores           JSONB       NOT NULL DEFAULT '{}',  -- {"total": 12, "items": [{"q": 1, "score": 2},...]}
    severity         TEXT        NOT NULL DEFAULT ''
                                 CHECK (severity IN ('','minimal','mild','moderate','moderately-severe','severe')),
    clinical_notes   BYTEA       NOT NULL DEFAULT '',    -- encrypted free-text interpretation
    extra_sensitive  BOOLEAN     NOT NULL DEFAULT true,
    assessed_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS mh_assessments_tenant_idx   ON mh_assessments (tenant_id);
CREATE INDEX IF NOT EXISTS mh_assessments_patient_idx  ON mh_assessments (patient_id);
CREATE INDEX IF NOT EXISTS mh_assessments_episode_idx  ON mh_assessments (episode_id);
CREATE INDEX IF NOT EXISTS mh_assessments_tool_idx     ON mh_assessments (tool);
CREATE INDEX IF NOT EXISTS mh_assessments_assessed_idx ON mh_assessments (assessed_at);

ALTER TABLE mh_assessments ENABLE ROW LEVEL SECURITY;

CREATE POLICY IF NOT EXISTS mh_assessments_tenant_isolation
    ON mh_assessments
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Ward Rounds (clinical contacts within an inpatient episode)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS mh_ward_rounds (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    episode_id      UUID        NOT NULL REFERENCES mh_episodes (id) ON DELETE CASCADE,
    patient_id      UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi     TEXT        NOT NULL DEFAULT '',
    tenant_id       UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    clinician_hpi   TEXT        NOT NULL,
    notes           BYTEA       NOT NULL DEFAULT '',  -- encrypted SOAP / MSE narrative
    mental_state    JSONB       NOT NULL DEFAULT '{}',
    risk_level      TEXT        NOT NULL DEFAULT 'low'
                                 CHECK (risk_level IN ('low','medium','high','very-high')),
    plans           BYTEA       NOT NULL DEFAULT '',  -- encrypted treatment plan text
    extra_sensitive BOOLEAN     NOT NULL DEFAULT true,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS mh_ward_rounds_episode_idx  ON mh_ward_rounds (episode_id);
CREATE INDEX IF NOT EXISTS mh_ward_rounds_tenant_idx   ON mh_ward_rounds (tenant_id);
CREATE INDEX IF NOT EXISTS mh_ward_rounds_patient_idx  ON mh_ward_rounds (patient_id);
CREATE INDEX IF NOT EXISTS mh_ward_rounds_occurred_idx ON mh_ward_rounds (occurred_at);

ALTER TABLE mh_ward_rounds ENABLE ROW LEVEL SECURITY;

CREATE POLICY IF NOT EXISTS mh_ward_rounds_tenant_isolation
    ON mh_ward_rounds
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Compulsory Orders (CAO, CTO, SPO) under MHCAA 1992
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS compulsory_orders (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi         TEXT        NOT NULL DEFAULT '',
    tenant_id           UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    episode_id          UUID        REFERENCES mh_episodes (id),
    order_type          TEXT        NOT NULL
                                     CHECK (order_type IN ('CAO','CTO-inpatient','CTO-community','SPO')),
    status              TEXT        NOT NULL DEFAULT 'active'
                                     CHECK (status IN ('active','suspended','expired','revoked','appealed')),
    responsible_hpi     TEXT        NOT NULL,
    second_opinion_hpi  TEXT        NOT NULL DEFAULT '',
    legal_authority     TEXT        NOT NULL DEFAULT '',  -- court / MHRT reference number
    conditions          BYTEA       NOT NULL DEFAULT '',  -- encrypted conditions text
    issued_date         DATE        NOT NULL,
    expiry_date         DATE        NOT NULL,
    first_review_date   DATE        NOT NULL,
    last_review_date    DATE,
    next_review_date    DATE        NOT NULL,
    revocation_reason   BYTEA,                            -- encrypted
    tribunal_reference  TEXT        NOT NULL DEFAULT '',
    extra_sensitive     BOOLEAN     NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS compulsory_orders_tenant_idx        ON compulsory_orders (tenant_id);
CREATE INDEX IF NOT EXISTS compulsory_orders_patient_idx       ON compulsory_orders (patient_id);
CREATE INDEX IF NOT EXISTS compulsory_orders_status_idx        ON compulsory_orders (status);
CREATE INDEX IF NOT EXISTS compulsory_orders_type_idx          ON compulsory_orders (order_type);
CREATE INDEX IF NOT EXISTS compulsory_orders_expiry_idx        ON compulsory_orders (expiry_date);
CREATE INDEX IF NOT EXISTS compulsory_orders_next_review_idx   ON compulsory_orders (next_review_date);

ALTER TABLE compulsory_orders ENABLE ROW LEVEL SECURITY;

CREATE POLICY IF NOT EXISTS compulsory_orders_tenant_isolation
    ON compulsory_orders
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Mental Health Enhanced Consent
-- Extends the standard consent model with MH-specific fields and the
-- elevated disclosure check required by HIPC additional protections.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS mh_consents (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi         TEXT        NOT NULL DEFAULT '',
    tenant_id           UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    consent_type        TEXT        NOT NULL
                                     CHECK (consent_type IN (
                                         'access','disclosure','research',
                                         'family-sharing','cto-related'
                                     )),
    granted             BOOLEAN     NOT NULL DEFAULT false,
    purpose             TEXT        NOT NULL DEFAULT '',
    granted_by          TEXT        NOT NULL DEFAULT '',  -- practitioner HPI or patient NHI
    granted_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at          TIMESTAMPTZ,
    revoked_at          TIMESTAMPTZ,
    third_party_id      TEXT        NOT NULL DEFAULT '',   -- recipient identifier
    third_party_role    TEXT        NOT NULL DEFAULT '',   -- "family", "employer", "insurer", "court"
    conditions          TEXT        NOT NULL DEFAULT '',
    evidence_ref        TEXT        NOT NULL DEFAULT '',   -- DMS / S3 key for signed form
    extra_sensitive     BOOLEAN     NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT mh_consents_active_unique
        UNIQUE NULLS NOT DISTINCT (tenant_id, patient_nhi, consent_type, third_party_id, revoked_at)
);

CREATE INDEX IF NOT EXISTS mh_consents_tenant_idx   ON mh_consents (tenant_id);
CREATE INDEX IF NOT EXISTS mh_consents_patient_idx  ON mh_consents (patient_id);
CREATE INDEX IF NOT EXISTS mh_consents_type_idx     ON mh_consents (consent_type);

ALTER TABLE mh_consents ENABLE ROW LEVEL SECURITY;

CREATE POLICY IF NOT EXISTS mh_consents_tenant_isolation
    ON mh_consents
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));
