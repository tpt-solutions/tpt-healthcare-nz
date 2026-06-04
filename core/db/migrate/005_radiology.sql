-- Radiology module: imaging studies, RIS orders, reports, and image-sharing tokens.

CREATE TABLE IF NOT EXISTS imaging_studies (
    id                TEXT        PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id         TEXT        NOT NULL,
    patient_nhi       TEXT        NOT NULL,
    study_instance_uid TEXT       NOT NULL,
    accession_number  TEXT,
    modality          TEXT        NOT NULL,
    body_part         TEXT,
    study_date        DATE,
    description       TEXT,
    referring_hpi     TEXT,
    performing_hpi    TEXT,
    status            TEXT        NOT NULL DEFAULT 'registered',
    num_series        INT         NOT NULL DEFAULT 0,
    num_instances     INT         NOT NULL DEFAULT 0,
    fhir_resource     BYTEA,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, study_instance_uid)
);

CREATE INDEX IF NOT EXISTS idx_imaging_studies_tenant_patient ON imaging_studies (tenant_id, patient_nhi);
CREATE INDEX IF NOT EXISTS idx_imaging_studies_study_date     ON imaging_studies (tenant_id, study_date DESC);
CREATE INDEX IF NOT EXISTS idx_imaging_studies_status         ON imaging_studies (tenant_id, status);

CREATE TABLE IF NOT EXISTS radiology_orders (
    id               TEXT        PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id        TEXT        NOT NULL,
    patient_nhi      TEXT        NOT NULL,
    imaging_study_id TEXT        REFERENCES imaging_studies(id),
    accession_number TEXT,
    modality         TEXT        NOT NULL,
    body_part        TEXT,
    clinical_info    TEXT,
    priority         TEXT        NOT NULL DEFAULT 'routine',
    status           TEXT        NOT NULL DEFAULT 'draft',
    referring_hpi    TEXT        NOT NULL,
    requested_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    scheduled_at     TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    loinc_code       TEXT,
    loinc_display    TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_radiology_orders_tenant_patient ON radiology_orders (tenant_id, patient_nhi);
CREATE INDEX IF NOT EXISTS idx_radiology_orders_status         ON radiology_orders (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_radiology_orders_requested_at   ON radiology_orders (tenant_id, requested_at DESC);

CREATE TABLE IF NOT EXISTS radiology_reports (
    id               TEXT        PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id        TEXT        NOT NULL,
    patient_nhi      TEXT        NOT NULL,
    imaging_study_id TEXT        REFERENCES imaging_studies(id),
    order_id         TEXT        REFERENCES radiology_orders(id),
    radiologist_hpi  TEXT        NOT NULL,
    status           TEXT        NOT NULL DEFAULT 'draft',
    findings         BYTEA,
    impression       BYTEA,
    fhir_resource    BYTEA,
    signed_at        TIMESTAMPTZ,
    amended_at       TIMESTAMPTZ,
    amendment_reason TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_radiology_reports_tenant_patient ON radiology_reports (tenant_id, patient_nhi);
CREATE INDEX IF NOT EXISTS idx_radiology_reports_study          ON radiology_reports (imaging_study_id);
CREATE INDEX IF NOT EXISTS idx_radiology_reports_order          ON radiology_reports (order_id);
CREATE INDEX IF NOT EXISTS idx_radiology_reports_status         ON radiology_reports (tenant_id, status);

CREATE TABLE IF NOT EXISTS imaging_share_tokens (
    id               TEXT        PRIMARY KEY DEFAULT gen_random_uuid()::text,
    tenant_id        TEXT        NOT NULL,
    imaging_study_id TEXT        NOT NULL REFERENCES imaging_studies(id),
    created_by_hpi   TEXT        NOT NULL,
    recipient_email  TEXT,
    recipient_npi    TEXT,
    token_hash       TEXT        NOT NULL UNIQUE,
    expires_at       TIMESTAMPTZ NOT NULL,
    accessed_at      TIMESTAMPTZ,
    revoked_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_imaging_share_tokens_study ON imaging_share_tokens (imaging_study_id);
CREATE INDEX IF NOT EXISTS idx_imaging_share_tokens_hash  ON imaging_share_tokens (token_hash);
