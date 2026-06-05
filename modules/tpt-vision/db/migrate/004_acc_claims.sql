-- tpt-vision: ACC Vision Claims
-- Maps to FHIR Claim resources with NZ ACC vision extensions

CREATE TABLE IF NOT EXISTS vision_acc_claims (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    patient_nhi         VARCHAR(7) NOT NULL,
    clinician_id        UUID NOT NULL REFERENCES practitioners(id),
    practice_id         UUID NOT NULL REFERENCES practices(id),
    claim_type          VARCHAR(40) NOT NULL CHECK (claim_type IN (
        'eye_examination', 'spectacle_after_injury', 'contact_lens_injury',
        'surgical_correction', 'follow_up'
    )),
    status              VARCHAR(30) NOT NULL DEFAULT 'draft' CHECK (status IN (
        'draft', 'ready_to_submit', 'submitted', 'accepted', 'partially_paid',
        'declined', 'requires_info', 'appealed'
    )),
    provider            VARCHAR(30) NOT NULL CHECK (provider IN (
        'optometrist', 'ophthalmologist', 'optical_dispenser', 'gp'
    )),
    
    -- Linked records
    prescription_id     UUID REFERENCES vision_prescriptions(id),
    exam_id             UUID REFERENCES vision_ophthalmic_exams(id),
    dispensing_id       UUID REFERENCES vision_dispensing_orders(id),
    
    -- Injury details
    accident_date       TIMESTAMPTZ NOT NULL,
    injury_type         VARCHAR(100) NOT NULL,
    injury_cause        TEXT NOT NULL,
    acc_number          VARCHAR(20),
    lodged_by           VARCHAR(100) NOT NULL,
    
    -- Claim items (stored as JSONB array)
    items               JSONB NOT NULL DEFAULT '[]'::jsonb,
    
    -- Financials
    total_claimed       NUMERIC(12,2) NOT NULL DEFAULT 0,
    gst_total           NUMERIC(12,2) NOT NULL DEFAULT 0,
    total_inc_gst       NUMERIC(12,2) NOT NULL DEFAULT 0,
    amount_paid         NUMERIC(12,2) NOT NULL DEFAULT 0,
    outstanding         NUMERIC(12,2) NOT NULL DEFAULT 0,
    
    -- Submission tracking
    submitted_date      TIMESTAMPTZ,
    response_date       TIMESTAMPTZ,
    decline_reason      TEXT,
    notes               TEXT,
    
    -- FHIR resource storage
    fhir_resource       JSONB NOT NULL,
    fhir_version        INTEGER NOT NULL DEFAULT 1,
    
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vision_acc_patient ON vision_acc_claims(patient_nhi);
CREATE INDEX idx_vision_acc_clinician ON vision_acc_claims(clinician_id);
CREATE INDEX idx_vision_acc_practice ON vision_acc_claims(practice_id);
CREATE INDEX idx_vision_acc_status ON vision_acc_claims(status);
CREATE INDEX idx_vision_acc_type ON vision_acc_claims(claim_type);
CREATE INDEX idx_vision_acc_accident_date ON vision_acc_claims(accident_date DESC);
CREATE INDEX idx_vision_acc_fhir ON vision_acc_claims USING GIN (fhir_resource);
CREATE INDEX idx_vision_acc_items ON vision_acc_claims USING GIN (items);

-- Updated at trigger
CREATE TRIGGER trg_vision_acc_updated_at
BEFORE UPDATE ON vision_acc_claims
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();