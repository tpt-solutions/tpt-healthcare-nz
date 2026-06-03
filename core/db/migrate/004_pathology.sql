-- 004_pathology.sql
-- tpt-pathology module tables.
-- All PHI columns are AES-256-GCM encrypted at the application layer (core/encryption).

-- ---------------------------------------------------------------------------
-- Specimen tracking
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS pathology_specimens (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    patient_id       UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi      TEXT        NOT NULL DEFAULT '',
    accession_number TEXT        NOT NULL DEFAULT '',  -- lab accession / order number
    collection_site  TEXT        NOT NULL DEFAULT '',  -- ZNZL CollectionSite
    collected_at     TIMESTAMPTZ,
    received_at      TIMESTAMPTZ,
    status           TEXT        NOT NULL DEFAULT 'collected'
                                  CHECK (status IN (
                                      'collected','in-transit','received',
                                      'processing','reported','discarded'
                                  )),
    specimen_type    TEXT        NOT NULL DEFAULT '',  -- SNOMED code or descriptive text
    container_type   TEXT        NOT NULL DEFAULT '',
    collected_by     TEXT        NOT NULL DEFAULT '',  -- practitioner HPI
    ordering_hpi     TEXT        NOT NULL DEFAULT '',  -- requesting GP / specialist HPI
    nzl_lab_order    TEXT        NOT NULL DEFAULT '',  -- ZNZL LabOrderNumber
    nzl_funding_code TEXT        NOT NULL DEFAULT '',  -- ZNZL FundingCode (e.g. "PHO", "DHB")
    nzl_urgency      TEXT        NOT NULL DEFAULT '',  -- ZNZL UrgencyIndicator
    notes            BYTEA       NOT NULL DEFAULT '',  -- encrypted free-text
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS pathology_specimens_tenant_idx    ON pathology_specimens (tenant_id);
CREATE INDEX IF NOT EXISTS pathology_specimens_patient_idx   ON pathology_specimens (patient_id);
CREATE INDEX IF NOT EXISTS pathology_specimens_accession_idx ON pathology_specimens (accession_number);
CREATE INDEX IF NOT EXISTS pathology_specimens_status_idx    ON pathology_specimens (status);
CREATE INDEX IF NOT EXISTS pathology_specimens_collected_idx ON pathology_specimens (collected_at);

ALTER TABLE pathology_specimens ENABLE ROW LEVEL SECURITY;

CREATE POLICY IF NOT EXISTS pathology_specimens_tenant_isolation
    ON pathology_specimens
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Diagnostic Reports (lab results)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS diagnostic_reports (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID        NOT NULL REFERENCES tenants (id) ON DELETE RESTRICT,
    patient_id        UUID        REFERENCES patients (id) ON DELETE RESTRICT,
    patient_nhi       TEXT        NOT NULL DEFAULT '',  -- encrypted
    specimen_id       UUID        REFERENCES pathology_specimens (id),
    accession_number  TEXT        NOT NULL DEFAULT '',
    ordering_hpi      TEXT        NOT NULL DEFAULT '',  -- requesting GP HPI (for subscription notification)
    performing_lab    TEXT        NOT NULL DEFAULT '',  -- lab NZBN or HPI facility
    status            TEXT        NOT NULL DEFAULT 'registered'
                                   CHECK (status IN (
                                       'registered','partial','preliminary','final',
                                       'amended','corrected','cancelled','entered-in-error'
                                   )),
    category          TEXT        NOT NULL DEFAULT 'laboratory',
    loinc_code        TEXT        NOT NULL DEFAULT '',
    loinc_display     TEXT        NOT NULL DEFAULT '',
    fhir_report       BYTEA       NOT NULL DEFAULT '',  -- encrypted FHIR DiagnosticReport JSON
    issued_at         TIMESTAMPTZ,
    effective_at      TIMESTAMPTZ,
    notification_sent BOOLEAN     NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS diagnostic_reports_tenant_idx    ON diagnostic_reports (tenant_id);
CREATE INDEX IF NOT EXISTS diagnostic_reports_patient_idx   ON diagnostic_reports (patient_id);
CREATE INDEX IF NOT EXISTS diagnostic_reports_accession_idx ON diagnostic_reports (accession_number);
CREATE INDEX IF NOT EXISTS diagnostic_reports_status_idx    ON diagnostic_reports (status);
CREATE INDEX IF NOT EXISTS diagnostic_reports_hpi_idx       ON diagnostic_reports (ordering_hpi);
CREATE INDEX IF NOT EXISTS diagnostic_reports_issued_idx    ON diagnostic_reports (issued_at);
CREATE INDEX IF NOT EXISTS diagnostic_reports_loinc_idx     ON diagnostic_reports (loinc_code);

ALTER TABLE diagnostic_reports ENABLE ROW LEVEL SECURITY;

CREATE POLICY IF NOT EXISTS diagnostic_reports_tenant_isolation
    ON diagnostic_reports
    USING (tenant_id::text = current_setting('app.current_tenant_id', true));

-- ---------------------------------------------------------------------------
-- Test catalog: LOINC test panels and components
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS pathology_test_catalog (
    id               UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    loinc_code       TEXT    NOT NULL UNIQUE,
    display_name     TEXT    NOT NULL DEFAULT '',
    short_name       TEXT    NOT NULL DEFAULT '',
    category         TEXT    NOT NULL DEFAULT 'laboratory'
                             CHECK (category IN (
                                 'laboratory','microbiology','pathology',
                                 'radiology','point-of-care'
                             )),
    specimen_type    TEXT    NOT NULL DEFAULT '',  -- preferred specimen type
    turnaround_hours INT     NOT NULL DEFAULT 0,   -- typical TAT in hours (0 = unknown)
    is_panel         BOOLEAN NOT NULL DEFAULT false,
    components       TEXT[]  NOT NULL DEFAULT '{}', -- LOINC codes for child components (if panel)
    active           BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS pathology_test_catalog_loinc_idx    ON pathology_test_catalog (loinc_code);
CREATE INDEX IF NOT EXISTS pathology_test_catalog_category_idx ON pathology_test_catalog (category);

-- ---------------------------------------------------------------------------
-- Reference ranges per LOINC test, stratified by demographics
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS pathology_reference_ranges (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    loinc_code  TEXT        NOT NULL REFERENCES pathology_test_catalog (loinc_code) ON DELETE CASCADE,
    lab_id      TEXT        NOT NULL DEFAULT '',      -- '' = universal; non-empty = lab-specific override
    sex         TEXT        NOT NULL DEFAULT 'any'
                             CHECK (sex IN ('any','M','F')),
    age_from    INT         NOT NULL DEFAULT 0,        -- years (inclusive)
    age_to      INT         NOT NULL DEFAULT 999,      -- years (inclusive)
    condition   TEXT        NOT NULL DEFAULT '',       -- e.g. 'pregnant', 'fasting', ''
    low         NUMERIC(10,4),
    high        NUMERIC(10,4),
    unit        TEXT        NOT NULL DEFAULT '',
    text_range  TEXT        NOT NULL DEFAULT '',       -- free-text for non-numeric ranges
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS pathology_reference_ranges_loinc_idx ON pathology_reference_ranges (loinc_code);
CREATE INDEX IF NOT EXISTS pathology_reference_ranges_sex_idx   ON pathology_reference_ranges (sex);
CREATE INDEX IF NOT EXISTS pathology_reference_ranges_age_idx   ON pathology_reference_ranges (age_from, age_to);
