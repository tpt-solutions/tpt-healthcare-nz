-- eMAR barcode/five-rights verification fields and controlled-drug (S8-equivalent) register
ALTER TABLE inpatient_medications
    ADD COLUMN IF NOT EXISTS barcode TEXT,
    ADD COLUMN IF NOT EXISTS is_controlled_drug BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS controlled_drug_schedule TEXT;

ALTER TABLE med_administration_records
    ADD COLUMN IF NOT EXISTS patient_barcode_scanned TEXT,
    ADD COLUMN IF NOT EXISTS med_barcode_scanned TEXT,
    ADD COLUMN IF NOT EXISTS verification_method TEXT NOT NULL DEFAULT 'manual',
    ADD COLUMN IF NOT EXISTS five_rights_confirmed BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS witness_hpi TEXT;

-- Controlled drug (NZ Misuse of Drugs Regulations schedule) register: an
-- append-only, dual-signed running balance per admission+drug, covering
-- administration, wastage, return-to-pharmacy, and stock-receipt events.
CREATE TABLE IF NOT EXISTS controlled_drug_register (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id    UUID NOT NULL REFERENCES hospital_admissions(id),
    medication_id   UUID REFERENCES inpatient_medications(id),
    drug_name       TEXT NOT NULL,
    schedule        TEXT NOT NULL,
    action          TEXT NOT NULL CHECK (action IN ('administered', 'wasted', 'returned', 'stock-received', 'stock-count')),
    quantity        NUMERIC NOT NULL,
    balance_after   NUMERIC NOT NULL,
    administered_by TEXT NOT NULL,
    witness_hpi     TEXT NOT NULL,
    notes           TEXT,
    tenant_id       UUID NOT NULL,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cd_register_admission ON controlled_drug_register (admission_id, drug_name, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_cd_register_tenant    ON controlled_drug_register (tenant_id);
