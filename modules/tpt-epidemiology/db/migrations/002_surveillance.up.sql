-- Notifiable disease surveillance cases for EpiSurv/ESR reporting.
-- Notifiable diseases are defined in the Health (Infectious and Notifiable Diseases)
-- Regulations 2016 under the Health Act 1956.
-- patient_nhi is stored AES-256-GCM encrypted (HIPC Rule 5).
-- notification_status: draft | submitted | acknowledged | closed
CREATE TABLE IF NOT EXISTS surveillance_cases (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi          TEXT        NOT NULL DEFAULT '',
    disease_code         TEXT        NOT NULL,
    disease_name         TEXT        NOT NULL DEFAULT '',
    diagnosis_date       DATE        NOT NULL,
    reporting_hpi        TEXT        NOT NULL DEFAULT '',
    notification_status  TEXT        NOT NULL DEFAULT 'draft',
    episurv_reference    TEXT,
    clinical_notes       TEXT,
    exposure_details     TEXT,
    outbreak_id          UUID        REFERENCES outbreak_investigations (id),
    tenant_id            UUID        NOT NULL,
    submitted_at         TIMESTAMPTZ,
    acknowledged_at      TIMESTAMPTZ,
    closed_at            TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_surv_cases_tenant_status    ON surveillance_cases (tenant_id, notification_status);
CREATE INDEX IF NOT EXISTS idx_surv_cases_tenant_disease   ON surveillance_cases (tenant_id, disease_code);
CREATE INDEX IF NOT EXISTS idx_surv_cases_tenant_diagnosis ON surveillance_cases (tenant_id, diagnosis_date DESC);
CREATE INDEX IF NOT EXISTS idx_surv_cases_outbreak         ON surveillance_cases (outbreak_id) WHERE outbreak_id IS NOT NULL;
