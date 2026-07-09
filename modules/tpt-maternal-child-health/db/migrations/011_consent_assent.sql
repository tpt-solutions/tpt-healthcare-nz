-- 011_consent_assent.sql
-- Consent and assent documentation for maternal and child health records.
-- Covers parent/guardian treatment consent, procedural consent, information-sharing consent,
-- and child assent (typically from age 7+). Complies with HIPC Rules 10 and 11.

CREATE TABLE IF NOT EXISTS mch_consent_forms (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The type of clinical record this consent applies to
    resource_type       TEXT NOT NULL CHECK (resource_type IN (
                            'maternity_episode', 'nicu_admission', 'scbu_admission',
                            'paediatric_admission', 'picu_admission', 'well_child_check'
                        )),
    resource_id         UUID NOT NULL,
    patient_nhi         TEXT NOT NULL,
    -- consent_type distinguishes treatment, procedure, information-sharing, research, and assent
    consent_type        TEXT NOT NULL CHECK (consent_type IN (
                            'treatment', 'procedure', 'information-sharing', 'research', 'assent'
                        )),
    consent_given       BOOLEAN NOT NULL DEFAULT FALSE,
    -- who gave (or withheld) consent
    given_by_nhi        TEXT,           -- NHI of consenting person (guardian or patient)
    given_by_name       TEXT NOT NULL,  -- free-text name for the record
    given_by_relationship TEXT NOT NULL CHECK (given_by_relationship IN (
                            'mother', 'father', 'guardian', 'caregiver', 'self', 'other'
                        )),
    clinician_hpi       TEXT NOT NULL,  -- HPI of the clinician who obtained consent
    description         TEXT NOT NULL,  -- what the consent covers (procedure/purpose description)
    notes               TEXT,
    signed_at           TIMESTAMPTZ,
    withdrawn_at        TIMESTAMPTZ,
    withdrawn_reason    TEXT,
    status              TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'signed', 'declined', 'withdrawn')),
    tenant_id           TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mch_consent_forms_resource
    ON mch_consent_forms (resource_type, resource_id, tenant_id);
CREATE INDEX IF NOT EXISTS idx_mch_consent_forms_patient
    ON mch_consent_forms (patient_nhi, tenant_id);
CREATE INDEX IF NOT EXISTS idx_mch_consent_forms_tenant
    ON mch_consent_forms (tenant_id, created_at DESC);
