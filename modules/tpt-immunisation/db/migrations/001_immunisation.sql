-- Immunisation records (FHIR R5 Immunization adapted for NZ).
CREATE TABLE IF NOT EXISTS immunisation_records (
    id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi         TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'completed',
    vaccine_code        TEXT NOT NULL DEFAULT '',
    vaccine_display     TEXT NOT NULL DEFAULT '',
    occurrence_datetime TIMESTAMPTZ NOT NULL,
    site_code           TEXT NOT NULL DEFAULT '',
    site_display        TEXT NOT NULL DEFAULT '',
    route_code          TEXT NOT NULL DEFAULT '',
    route_display       TEXT NOT NULL DEFAULT '',
    lot_number          TEXT NOT NULL DEFAULT '',
    expiry_date         TEXT NOT NULL DEFAULT '',
    practitioner_hpi_cpn TEXT NOT NULL DEFAULT '',
    nir_submitted       BOOLEAN NOT NULL DEFAULT FALSE,
    nir_submitted_at    TIMESTAMPTZ,
    nir_reference_id    TEXT NOT NULL DEFAULT '',
    note                TEXT NOT NULL DEFAULT '',
    fhir_resource       JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_immunisation_patient ON immunisation_records(patient_nhi);
CREATE INDEX IF NOT EXISTS idx_immunisation_status ON immunisation_records(status);
CREATE INDEX IF NOT EXISTS idx_immunisation_vaccine ON immunisation_records(vaccine_code);

-- Disease outbreaks for recall tracking.
CREATE TABLE IF NOT EXISTS outbreaks (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    disease         TEXT NOT NULL,
    snomed_code     TEXT NOT NULL DEFAULT '',
    region          TEXT NOT NULL,
    reported_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    active_until    TIMESTAMPTZ NOT NULL,
    cases_count     INTEGER NOT NULL DEFAULT 0,
    contact_email   TEXT NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Immunisation recalls generated from active outbreaks.
CREATE TABLE IF NOT EXISTS recalls (
    id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi         TEXT NOT NULL,
    outbreak_id         TEXT NOT NULL REFERENCES outbreaks(id),
    disease             TEXT NOT NULL,
    missing_vaccines    TEXT[] NOT NULL DEFAULT '{}',
    last_contact_at     TIMESTAMPTZ,
    priority            TEXT NOT NULL DEFAULT 'medium',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_recalls_outbreak ON recalls(outbreak_id);
CREATE INDEX IF NOT EXISTS idx_recalls_patient ON recalls(patient_nhi);
CREATE INDEX IF NOT EXISTS idx_recalls_priority ON recalls(priority);
