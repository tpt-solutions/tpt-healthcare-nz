-- Clinical coding: ICD-10-AM diagnoses and ACHI procedures
CREATE TABLE IF NOT EXISTS clinical_codes (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id UUID NOT NULL REFERENCES hospital_admissions(id),
    system       TEXT NOT NULL CHECK (system IN ('ICD-10-AM', 'ACHI')),
    code         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    code_type    TEXT NOT NULL,
    sequence     SMALLINT NOT NULL DEFAULT 1,
    coder_hpi    TEXT,
    tenant_id    UUID NOT NULL,
    coded_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_clinical_codes_admission ON clinical_codes (admission_id, code_type);
CREATE INDEX IF NOT EXISTS idx_clinical_codes_tenant    ON clinical_codes (tenant_id);
