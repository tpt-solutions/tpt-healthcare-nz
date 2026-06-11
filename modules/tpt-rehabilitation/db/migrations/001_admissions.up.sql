-- Inpatient rehabilitation admissions and functional assessment episodes.

CREATE TABLE IF NOT EXISTS rehab_admissions (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    ward                    TEXT        NOT NULL DEFAULT '',
    admission_type          TEXT        NOT NULL DEFAULT 'inpatient',
    -- inpatient | day-rehab
    admission_source        TEXT        NOT NULL DEFAULT 'hospital',
    -- hospital | community | ACC | NASC
    primary_diagnosis       TEXT        NOT NULL DEFAULT '',
    secondary_diagnoses     TEXT        NOT NULL DEFAULT '',
    status                  TEXT        NOT NULL DEFAULT 'admitted',
    -- admitted | active | discharge-planning | discharged | transferred
    mobility_on_admission   TEXT        NOT NULL DEFAULT '',
    cognitive_status        TEXT        NOT NULL DEFAULT '',
    goals_set_at            TIMESTAMPTZ,
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    admitted_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    discharged_at           TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_rehab_admissions_tenant_status ON rehab_admissions (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_rehab_admissions_patient       ON rehab_admissions (patient_nhi);
CREATE INDEX IF NOT EXISTS idx_rehab_admissions_admitted_at   ON rehab_admissions (tenant_id, admitted_at DESC);
