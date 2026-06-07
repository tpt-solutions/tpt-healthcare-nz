-- Hospital admissions (FHIR R5 Encounter — inpatient)
CREATE TABLE IF NOT EXISTS hospital_admissions (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id               UUID NOT NULL,
    patient_nhi              TEXT NOT NULL DEFAULT '',
    admitting_clinician_hpi  TEXT NOT NULL,
    responsible_clinician_hpi TEXT,
    admission_type           TEXT NOT NULL DEFAULT 'emergency',
    status                   TEXT NOT NULL DEFAULT 'admitted',
    ward_id                  UUID,
    bed_id                   UUID,
    admission_reason         TEXT NOT NULL DEFAULT '',
    primary_diagnosis        TEXT,
    acc_claim_number         TEXT,
    referring_facility_hpi   TEXT,
    discharge_destination    TEXT,
    discharge_notes          TEXT,
    tenant_id                UUID NOT NULL,
    admitted_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    discharged_at            TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hospital_admissions_tenant_status ON hospital_admissions (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_hospital_admissions_patient       ON hospital_admissions (patient_id);
CREATE INDEX IF NOT EXISTS idx_hospital_admissions_ward          ON hospital_admissions (ward_id) WHERE ward_id IS NOT NULL;

-- Discharge summaries
CREATE TABLE IF NOT EXISTS hospital_discharge_summaries (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id          UUID NOT NULL REFERENCES hospital_admissions(id),
    patient_id            UUID NOT NULL,
    author_hpi            TEXT NOT NULL,
    admission_date        TIMESTAMPTZ NOT NULL,
    discharge_date        TIMESTAMPTZ NOT NULL,
    primary_diagnosis     TEXT NOT NULL DEFAULT '',
    secondary_diagnoses   TEXT[] NOT NULL DEFAULT '{}',
    procedures_performed  TEXT[] NOT NULL DEFAULT '{}',
    clinical_summary      TEXT NOT NULL DEFAULT '',
    discharge_condition   TEXT NOT NULL DEFAULT '',
    follow_up_plan        TEXT NOT NULL DEFAULT '',
    medications           TEXT[] NOT NULL DEFAULT '{}',
    gp_notified           BOOLEAN NOT NULL DEFAULT false,
    gp_notified_at        TIMESTAMPTZ,
    tenant_id             UUID NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_discharge_summaries_admission ON hospital_discharge_summaries (admission_id);
CREATE INDEX IF NOT EXISTS idx_discharge_summaries_tenant    ON hospital_discharge_summaries (tenant_id);
