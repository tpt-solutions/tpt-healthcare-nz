-- tpt-pathology: test catalog, reference ranges, specimens, and diagnostic reports.

CREATE TABLE IF NOT EXISTS pathology_test_catalog (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    loinc_code          VARCHAR(20) NOT NULL UNIQUE,
    display_name        TEXT NOT NULL,
    short_name          TEXT,
    category            TEXT NOT NULL DEFAULT 'laboratory',
    specimen_type       TEXT,
    turnaround_hours    INT NOT NULL DEFAULT 24,
    is_panel            BOOLEAN NOT NULL DEFAULT FALSE,
    components          JSONB NOT NULL DEFAULT '[]',
    active              BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pt_test_cat_loinc ON pathology_test_catalog (loinc_code);
CREATE INDEX IF NOT EXISTS idx_pt_test_cat_category ON pathology_test_catalog (category);

CREATE TABLE IF NOT EXISTS pathology_reference_ranges (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    loinc_code          VARCHAR(20) NOT NULL,
    lab_id              TEXT,
    sex                 TEXT NOT NULL DEFAULT 'any',
    age_from            INT NOT NULL DEFAULT 0,
    age_to              INT NOT NULL DEFAULT 999,
    condition           TEXT,
    low                 NUMERIC(12,4),
    high                NUMERIC(12,4),
    unit                TEXT,
    text_range          TEXT
);

CREATE INDEX IF NOT EXISTS idx_pt_ref_range_loinc ON pathology_reference_ranges (loinc_code);

CREATE TABLE IF NOT EXISTS pathology_specimens (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    patient_nhi         VARCHAR(7),
    accession_number    VARCHAR(30) NOT NULL,
    collection_site     TEXT,
    collected_at        TIMESTAMPTZ,
    received_at         TIMESTAMPTZ,
    status              VARCHAR(20) NOT NULL DEFAULT 'collected',
    specimen_type       TEXT,
    container_type      TEXT,
    collected_by        TEXT,
    ordering_hpi        VARCHAR(16),
    nzl_lab_order       TEXT,
    nzl_funding_code    TEXT,
    nzl_urgency         TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pt_spec_tenant ON pathology_specimens (tenant_id);
CREATE INDEX IF NOT EXISTS idx_pt_spec_patient ON pathology_specimens (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_pt_spec_accession ON pathology_specimens (accession_number);
CREATE INDEX IF NOT EXISTS idx_pt_spec_status ON pathology_specimens (status);

CREATE TABLE IF NOT EXISTS diagnostic_reports (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    patient_nhi         VARCHAR(7),
    specimen_id         UUID,
    accession_number    VARCHAR(30) NOT NULL,
    ordering_hpi        VARCHAR(16) NOT NULL,
    performing_lab      TEXT NOT NULL,
    status              VARCHAR(20) NOT NULL DEFAULT 'registered',
    category            TEXT NOT NULL DEFAULT 'laboratory',
    loinc_code          VARCHAR(20) NOT NULL,
    loinc_display       TEXT NOT NULL,
    fhir_report         BYTEA,
    issued_at           TIMESTAMPTZ,
    effective_at        TIMESTAMPTZ,
    notification_sent   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pt_report_tenant ON diagnostic_reports (tenant_id);
CREATE INDEX IF NOT EXISTS idx_pt_report_patient ON diagnostic_reports (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_pt_report_accession ON diagnostic_reports (accession_number);
CREATE INDEX IF NOT EXISTS idx_pt_report_status ON diagnostic_reports (status);
CREATE INDEX IF NOT EXISTS idx_pt_report_loinc ON diagnostic_reports (loinc_code);
