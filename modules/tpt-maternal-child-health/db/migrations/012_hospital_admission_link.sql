-- Link NICU, PICU, and paediatric admissions back to the hospital admission model.
-- This enables cross-module queries: e.g. "show me all ICU/PICU/NICU records
-- for this hospital admission" without relying solely on the local admission tables.
-- The FK is intentionally nullable so that neonates admitted directly to NICU
-- (without a separate hospital_admissions row) are not blocked.

ALTER TABLE nicu_admissions
    ADD COLUMN IF NOT EXISTS hospital_admission_id UUID;

CREATE INDEX IF NOT EXISTS idx_nicu_admissions_hospital_admission
    ON nicu_admissions (hospital_admission_id)
    WHERE hospital_admission_id IS NOT NULL;

ALTER TABLE picu_admissions
    ADD COLUMN IF NOT EXISTS hospital_admission_id UUID;

CREATE INDEX IF NOT EXISTS idx_picu_admissions_hospital_admission
    ON picu_admissions (hospital_admission_id)
    WHERE hospital_admission_id IS NOT NULL;

ALTER TABLE paediatric_admissions
    ADD COLUMN IF NOT EXISTS hospital_admission_id UUID;

CREATE INDEX IF NOT EXISTS idx_paed_admissions_hospital_admission
    ON paediatric_admissions (hospital_admission_id)
    WHERE hospital_admission_id IS NOT NULL;
