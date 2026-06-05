-- tpt-vision: Optical Dispensing Orders
-- Maps to FHIR MedicationDispense (for contact lenses) and Device (for spectacles) resources

CREATE TABLE IF NOT EXISTS vision_dispensing_orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    patient_nhi         VARCHAR(7) NOT NULL,
    clinician_id        UUID NOT NULL REFERENCES practitioners(id),
    dispenser_id        UUID REFERENCES practitioners(id),
    practice_id         UUID NOT NULL REFERENCES practices(id),
    prescription_id     UUID NOT NULL REFERENCES vision_prescriptions(id),
    status              VARCHAR(30) NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'lab_sent', 'in_lab', 'received', 'ready_for_collection',
        'collected', 'cancelled', 'warranty_claim'
    )),
    order_date          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    due_date            TIMESTAMPTZ,
    collected_date      TIMESTAMPTZ,
    
    -- Spectacle details (mutually exclusive with contact lens)
    frame_type          VARCHAR(20) CHECK (frame_type IN (
        'full_rim', 'semi_rimless', 'rimless', 'childrens', 'safety'
    )),
    frame_brand         VARCHAR(100),
    frame_model         VARCHAR(100),
    frame_colour        VARCHAR(50),
    frame_size          VARCHAR(20),
    frame_price         NUMERIC(10,2) DEFAULT 0,
    
    lens_type           VARCHAR(20) CHECK (lens_type IN (
        'single_vision', 'bifocal', 'progressive', 'occupational',
        'photochromic', 'polarised', 'clip_on'
    )),
    lens_index          VARCHAR(10) CHECK (lens_index IN ('1.50', '1.60', '1.67', '1.74')),
    lens_coatings       JSONB NOT NULL DEFAULT '[]'::jsonb,
    lens_tint           VARCHAR(30),
    lens_price          NUMERIC(10,2) DEFAULT 0,
    
    -- Measurements
    pd                  NUMERIC(5,2),
    pd_near             NUMERIC(5,2),
    segment_height      NUMERIC(5,2),
    back_vertex_distance NUMERIC(4,2),
    pantoscopic_tilt    NUMERIC(4,2),
    face_form_angle     NUMERIC(4,2),
    
    -- Contact lens details
    cl_type             VARCHAR(30) CHECK (cl_type IN (
        'daily_disposable', 'weekly', 'bi_weekly', 'monthly', 'quarterly',
        'yearly', 'rgp', 'ortho_k', 'scleral'
    )),
    cl_brand            VARCHAR(100),
    cl_base_curve       NUMERIC(4,2),
    cl_diameter         NUMERIC(4,2),
    cl_power_right      NUMERIC(5,2),
    cl_power_left       NUMERIC(5,2),
    cl_cyl_right        NUMERIC(4,2),
    cl_cyl_left         NUMERIC(4,2),
    cl_axis_right       SMALLINT,
    cl_axis_left        SMALLINT,
    cl_qty              SMALLINT,
    cl_price_per_box    NUMERIC(10,2),
    
    -- Pricing
    total_price         NUMERIC(10,2) NOT NULL DEFAULT 0,
    deposit_paid        NUMERIC(10,2) NOT NULL DEFAULT 0,
    balance_due         NUMERIC(10,2) NOT NULL DEFAULT 0,
    funded_by_acc       BOOLEAN NOT NULL DEFAULT FALSE,
    funded_by_dhb       BOOLEAN NOT NULL DEFAULT FALSE,
    acc_claim_id        UUID REFERENCES vision_acc_claims(id),
    
    -- Warranty
    warranty_months     SMALLINT,
    
    notes               TEXT,
    
    -- FHIR resource storage
    fhir_resource       JSONB NOT NULL,
    fhir_version        INTEGER NOT NULL DEFAULT 1,
    
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT valid_spectacle_or_cl CHECK (
        (frame_type IS NOT NULL AND lens_type IS NOT NULL AND cl_type IS NULL) OR
        (frame_type IS NULL AND lens_type IS NULL AND cl_type IS NOT NULL)
    ),
    CONSTRAINT valid_cl_axis CHECK (
        cl_cyl_right = 0 OR cl_axis_right IS NOT NULL
    ),
    CONSTRAINT valid_cl_axis_left CHECK (
        cl_cyl_left = 0 OR cl_axis_left IS NOT NULL
    )
);

CREATE INDEX idx_vision_dispensing_patient ON vision_dispensing_orders(patient_nhi);
CREATE INDEX idx_vision_dispensing_clinician ON vision_dispensing_orders(clinician_id);
CREATE INDEX idx_vision_dispensing_dispenser ON vision_dispensing_orders(dispenser_id);
CREATE INDEX idx_vision_dispensing_practice ON vision_dispensing_orders(practice_id);
CREATE INDEX idx_vision_dispensing_prescription ON vision_dispensing_orders(prescription_id);
CREATE INDEX idx_vision_dispensing_status ON vision_dispensing_orders(status);
CREATE INDEX idx_vision_dispensing_order_date ON vision_dispensing_orders(order_date DESC);
CREATE INDEX idx_vision_dispensing_fhir ON vision_dispensing_orders USING GIN (fhir_resource);

-- Updated at trigger
CREATE TRIGGER trg_vision_dispensing_updated_at
BEFORE UPDATE ON vision_dispensing_orders
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();