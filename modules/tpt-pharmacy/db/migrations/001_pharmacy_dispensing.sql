-- Pharmacy dispensing records: tracks MedicationRequest through the dispensing workflow.
CREATE TABLE IF NOT EXISTS pharmacy_dispensing_records (
    id                  TEXT PRIMARY KEY,
    medication_request_id TEXT NOT NULL,
    patient_nhi         TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending',
    is_schedule2        BOOLEAN NOT NULL DEFAULT FALSE,
    pharmacist_hpi_cpn  TEXT,
    second_pharmacist_id TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pharmacy_dispensing_status ON pharmacy_dispensing_records(status);
CREATE INDEX IF NOT EXISTS idx_pharmacy_dispensing_patient ON pharmacy_dispensing_records(patient_nhi);

-- Pharmacy PHARMAC claims: consolidated subsidy claims.
CREATE TABLE IF NOT EXISTS pharmacy_pharmac_claims (
    id                    TEXT PRIMARY KEY,
    status                TEXT NOT NULL DEFAULT 'draft',
    pharmacy_hsp_no       TEXT NOT NULL,
    claim_period_start    TIMESTAMPTZ NOT NULL,
    claim_period_end      TIMESTAMPTZ NOT NULL,
    dispense_ids          JSONB NOT NULL DEFAULT '[]',
    total_subsidy_amount  NUMERIC(12,2) NOT NULL DEFAULT 0,
    submitted_at          TIMESTAMPTZ,
    pharmac_reference_no  TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_pharmacy_claims_status ON pharmacy_pharmac_claims(status);
CREATE INDEX IF NOT EXISTS idx_pharmacy_claims_pharmacy ON pharmacy_pharmac_claims(pharmacy_hsp_no);
