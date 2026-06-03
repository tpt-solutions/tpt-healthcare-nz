-- 002_doctor_tables.sql
-- GP module clinical tables with row-level security enforcement.
-- All PHI columns use AES-256-GCM encryption applied at the application layer
-- (core/encryption); the DB stores ciphertext blobs.

-- ---------------------------------------------------------------------------
-- Patients
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS patients (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    nhi_encrypted   BYTEA       NOT NULL,
    nhi_index       TEXT        NOT NULL DEFAULT '',  -- deterministic cipher for index lookup
    tenant_id       UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    fhir_resource   BYTEA       NOT NULL,             -- encrypted FHIR Patient JSON
    name_search     TEXT        NOT NULL DEFAULT '',  -- plaintext search index
    dob_index       TEXT        NOT NULL DEFAULT '',  -- YYYY-MM-DD, plaintext for age/DOB search
    gender          TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS patients_tenant_idx   ON patients (tenant_id);
CREATE INDEX IF NOT EXISTS patients_nhi_idx      ON patients (nhi_index);
CREATE INDEX IF NOT EXISTS patients_name_idx     ON patients USING GIN (to_tsvector('simple', name_search));

ALTER TABLE patients ENABLE ROW LEVEL SECURITY;

-- Row-level security: application sets app.current_tenant_id before each query.
CREATE POLICY IF NOT EXISTS patients_tenant_isolation
    ON patients
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- NES Enrolments (for PHO capitation extract)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS nes_enrolments (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id       UUID        NOT NULL REFERENCES patients (id) ON DELETE CASCADE,
    patient_nhi      TEXT        NOT NULL DEFAULT '',
    tenant_id        UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    practitioner_hpi TEXT        NOT NULL,
    funding_code     TEXT        NOT NULL DEFAULT '',
    enrolment_start  DATE        NOT NULL,
    enrolment_end    DATE,
    transfer_reason  TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS nes_enrolments_tenant_idx   ON nes_enrolments (tenant_id);
CREATE INDEX IF NOT EXISTS nes_enrolments_patient_idx  ON nes_enrolments (patient_id);
CREATE INDEX IF NOT EXISTS nes_enrolments_period_idx   ON nes_enrolments (enrolment_start, enrolment_end);

ALTER TABLE nes_enrolments ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS nes_enrolments_tenant_isolation
    ON nes_enrolments
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Encounters (clinical consultations)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS encounters (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id       UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi      TEXT        NOT NULL DEFAULT '',
    practitioner_hpi TEXT        NOT NULL,
    appointment_id   UUID,
    tenant_id        UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    status           TEXT        NOT NULL DEFAULT 'planned'
                                 CHECK (status IN ('planned','in-progress','on-hold','completed','cancelled')),
    workflow_variant TEXT        NOT NULL DEFAULT 'standard'
                                 CHECK (workflow_variant IN ('standard','after-hours','urgent-care','occupational-health')),
    soap_subjective  TEXT        NOT NULL DEFAULT '',
    soap_objective   TEXT        NOT NULL DEFAULT '',
    soap_assessment  TEXT        NOT NULL DEFAULT '',
    soap_plan        TEXT        NOT NULL DEFAULT '',
    vitals           JSONB       NOT NULL DEFAULT '{}',
    diagnoses        TEXT[]      NOT NULL DEFAULT '{}',
    procedures       TEXT[]      NOT NULL DEFAULT '{}',
    primary_diagnosis TEXT       NOT NULL DEFAULT '',   -- first ICD-10-AM code (for FFS extract)
    ffs_eligible     BOOLEAN     NOT NULL DEFAULT false,
    ffs_funding_code TEXT        NOT NULL DEFAULT '',
    started_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS encounters_tenant_idx    ON encounters (tenant_id);
CREATE INDEX IF NOT EXISTS encounters_patient_idx   ON encounters (patient_id);
CREATE INDEX IF NOT EXISTS encounters_status_idx    ON encounters (status);
CREATE INDEX IF NOT EXISTS encounters_started_idx   ON encounters (started_at);

ALTER TABLE encounters ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS encounters_tenant_isolation
    ON encounters
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Appointments
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS appointments (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id       UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi      TEXT        NOT NULL DEFAULT '',
    practitioner_hpi TEXT        NOT NULL,
    tenant_id        UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    status           TEXT        NOT NULL DEFAULT 'booked'
                                 CHECK (status IN ('proposed','pending','booked','arrived','fulfilled','cancelled','noshow')),
    start_time       TIMESTAMPTZ NOT NULL,
    end_time         TIMESTAMPTZ NOT NULL,
    reason           TEXT        NOT NULL DEFAULT '',
    reminder_sent    BOOLEAN     NOT NULL DEFAULT false,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS appointments_tenant_idx  ON appointments (tenant_id);
CREATE INDEX IF NOT EXISTS appointments_patient_idx ON appointments (patient_id);
CREATE INDEX IF NOT EXISTS appointments_time_idx    ON appointments (start_time, end_time);

ALTER TABLE appointments ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS appointments_tenant_isolation
    ON appointments
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Prescriptions (MedicationRequest)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS prescriptions (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id       UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi      TEXT        NOT NULL DEFAULT '',
    practitioner_hpi TEXT        NOT NULL,
    encounter_id     UUID        REFERENCES encounters (id),
    tenant_id        UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    status           TEXT        NOT NULL DEFAULT 'draft',
    medication_code  TEXT        NOT NULL,  -- NZMT code
    medication_name  TEXT        NOT NULL,
    dose_quantity    TEXT        NOT NULL DEFAULT '',
    dose_unit        TEXT        NOT NULL DEFAULT '',
    frequency        TEXT        NOT NULL DEFAULT '',
    route            TEXT        NOT NULL DEFAULT '',
    duration_days    INT,
    repeats          INT         NOT NULL DEFAULT 0,
    instructions     TEXT        NOT NULL DEFAULT '',
    pharmac_subsidised BOOLEAN   NOT NULL DEFAULT false,
    printed_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS prescriptions_tenant_idx  ON prescriptions (tenant_id);
CREATE INDEX IF NOT EXISTS prescriptions_patient_idx ON prescriptions (patient_id);

ALTER TABLE prescriptions ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS prescriptions_tenant_isolation
    ON prescriptions
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- ACC Claims
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS acc_claims (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    encounter_id        UUID        REFERENCES encounters (id),
    patient_id          UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi         TEXT        NOT NULL DEFAULT '',
    practitioner_hpi    TEXT        NOT NULL,
    tenant_id           UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    form_type           TEXT        NOT NULL CHECK (form_type IN ('ACC45','ACC6')),
    form_number         TEXT        NOT NULL DEFAULT '',
    diagnosis_codes     TEXT[]      NOT NULL DEFAULT '{}',
    injury_date         DATE        NOT NULL,
    injury_description  TEXT        NOT NULL DEFAULT '',
    status              TEXT        NOT NULL DEFAULT 'draft',
    acc_claim_number    TEXT        NOT NULL DEFAULT '',
    rejection_reason    TEXT        NOT NULL DEFAULT '',
    paid_amount         NUMERIC(10,2),
    submitted_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS acc_claims_tenant_idx  ON acc_claims (tenant_id);
CREATE INDEX IF NOT EXISTS acc_claims_patient_idx ON acc_claims (patient_id);
CREATE INDEX IF NOT EXISTS acc_claims_status_idx  ON acc_claims (status);

ALTER TABLE acc_claims ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS acc_claims_tenant_isolation
    ON acc_claims
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Referrals (ServiceRequest)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS referrals (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id      UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi     TEXT        NOT NULL DEFAULT '',
    referring_hpi   TEXT        NOT NULL,
    tenant_id       UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    encounter_id    UUID        REFERENCES encounters (id),
    specialty_code  TEXT        NOT NULL,
    service_type    TEXT        NOT NULL DEFAULT '',
    priority        TEXT        NOT NULL DEFAULT 'routine'
                                CHECK (priority IN ('routine','urgent','asap','stat')),
    reason          TEXT        NOT NULL,
    clinical_notes  TEXT        NOT NULL DEFAULT '',
    status          TEXT        NOT NULL DEFAULT 'draft'
                                CHECK (status IN ('draft','active','completed','revoked')),
    sent_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS referrals_tenant_idx   ON referrals (tenant_id);
CREATE INDEX IF NOT EXISTS referrals_patient_idx  ON referrals (patient_id);
CREATE INDEX IF NOT EXISTS referrals_status_idx   ON referrals (status);

ALTER TABLE referrals ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS referrals_tenant_isolation
    ON referrals
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Lab Orders (ServiceRequest + DiagnosticReport)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS lab_orders (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id      UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi     TEXT        NOT NULL DEFAULT '',
    ordering_hpi    TEXT        NOT NULL,
    encounter_id    UUID        REFERENCES encounters (id),
    tenant_id       UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    tests           TEXT[]      NOT NULL DEFAULT '{}', -- LOINC codes
    priority        TEXT        NOT NULL DEFAULT 'routine',
    clinical_notes  TEXT        NOT NULL DEFAULT '',
    status          TEXT        NOT NULL DEFAULT 'requested',
    fhir_report     BYTEA,                             -- encrypted FHIR DiagnosticReport JSON
    resulted_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS lab_orders_tenant_idx  ON lab_orders (tenant_id);
CREATE INDEX IF NOT EXISTS lab_orders_patient_idx ON lab_orders (patient_id);
CREATE INDEX IF NOT EXISTS lab_orders_status_idx  ON lab_orders (status);

ALTER TABLE lab_orders ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS lab_orders_tenant_isolation
    ON lab_orders
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Immunisations (FHIR Immunization)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS immunisations (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id       UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi      TEXT        NOT NULL DEFAULT '',
    tenant_id        UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    encounter_id     UUID        REFERENCES encounters (id),
    vaccine_code     TEXT        NOT NULL,  -- NZMT code
    vaccine_name     TEXT        NOT NULL,
    lot_number       TEXT        NOT NULL DEFAULT '',
    expiry_date      TEXT        NOT NULL DEFAULT '',  -- YYYY-MM-DD
    dose_number      INT         NOT NULL DEFAULT 1,
    series           TEXT        NOT NULL DEFAULT '',
    body_site_code   TEXT        NOT NULL DEFAULT '',
    route_code       TEXT        NOT NULL DEFAULT '',
    administered_by  TEXT        NOT NULL,             -- HPI CPN
    occurrence_date  TIMESTAMPTZ NOT NULL,
    notes            TEXT        NOT NULL DEFAULT '',
    nir_submitted    BOOLEAN     NOT NULL DEFAULT false,
    nir_reference    TEXT        NOT NULL DEFAULT '',
    nir_submitted_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS immunisations_tenant_idx  ON immunisations (tenant_id);
CREATE INDEX IF NOT EXISTS immunisations_patient_idx ON immunisations (patient_id);
CREATE INDEX IF NOT EXISTS immunisations_vaccine_idx ON immunisations (vaccine_code);

ALTER TABLE immunisations ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS immunisations_tenant_isolation
    ON immunisations
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Medical Certificates
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS certificates (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    type            TEXT        NOT NULL
                                CHECK (type IN ('sick-leave','fit-to-work','acc','immunisation','pre-employment','death')),
    patient_id      UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi     TEXT        NOT NULL DEFAULT '',
    issuing_hpi     TEXT        NOT NULL,
    encounter_id    UUID        REFERENCES encounters (id),
    tenant_id       UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    from_date       TEXT        NOT NULL DEFAULT '',  -- YYYY-MM-DD
    to_date         TEXT        NOT NULL DEFAULT '',  -- YYYY-MM-DD
    diagnosis       TEXT        NOT NULL DEFAULT '',
    notes           TEXT        NOT NULL DEFAULT '',
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS certificates_tenant_idx  ON certificates (tenant_id);
CREATE INDEX IF NOT EXISTS certificates_patient_idx ON certificates (patient_id);
CREATE INDEX IF NOT EXISTS certificates_type_idx    ON certificates (type);

ALTER TABLE certificates ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS certificates_tenant_isolation
    ON certificates
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- PHO Reports (capitation + FFS)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS pho_reports (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    type            TEXT        NOT NULL CHECK (type IN ('capitation','ffs')),
    period          TEXT        NOT NULL,  -- YYYY-MM
    status          TEXT        NOT NULL DEFAULT 'draft'
                                CHECK (status IN ('draft','submitted','accepted','rejected')),
    record_count    INT         NOT NULL DEFAULT 0,
    pho_reference   TEXT        NOT NULL DEFAULT '',
    rejection_note  TEXT        NOT NULL DEFAULT '',
    tenant_id       UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    submitted_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS pho_reports_tenant_idx ON pho_reports (tenant_id);
CREATE INDEX IF NOT EXISTS pho_reports_period_idx ON pho_reports (period);

ALTER TABLE pho_reports ENABLE ROW LEVEL SECURITY;
CREATE POLICY IF NOT EXISTS pho_reports_tenant_isolation
    ON pho_reports
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Audit Events (append-only, no RLS — admins need full access)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS audit_events (
    id              BIGSERIAL   PRIMARY KEY,
    tenant_id       TEXT        NOT NULL DEFAULT '',
    principal_id    TEXT        NOT NULL DEFAULT '',
    action          TEXT        NOT NULL,
    resource_type   TEXT        NOT NULL,
    resource_id     TEXT        NOT NULL DEFAULT '',
    patient_nhi     TEXT        NOT NULL DEFAULT '',  -- encrypted NHI for PHI linkage
    source_ip       TEXT        NOT NULL DEFAULT '',
    user_agent      TEXT        NOT NULL DEFAULT '',
    correlation_id  TEXT        NOT NULL DEFAULT '',
    details         JSONB       NOT NULL DEFAULT '{}',
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_events_tenant_idx    ON audit_events (tenant_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS audit_events_principal_idx ON audit_events (principal_id);
CREATE INDEX IF NOT EXISTS audit_events_resource_idx  ON audit_events (resource_type, resource_id);
