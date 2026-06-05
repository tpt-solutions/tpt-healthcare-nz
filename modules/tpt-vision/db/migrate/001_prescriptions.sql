-- tpt-vision: Refraction Prescriptions
-- Maps to FHIR Observation resources with NZ vision extensions

CREATE TABLE IF NOT EXISTS vision_prescriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    patient_nhi     VARCHAR(7) NOT NULL,
    clinician_id    UUID NOT NULL REFERENCES practitioners(id),
    practice_id     UUID NOT NULL REFERENCES practices(id),
    type            VARCHAR(20) NOT NULL CHECK (type IN ('spectacle', 'contact')),
    distance        VARCHAR(20) NOT NULL CHECK (distance IN ('distance', 'near', 'intermediate')),
    
    -- Right eye
    right_sphere        NUMERIC(4,2) NOT NULL,
    right_cylinder      NUMERIC(4,2) DEFAULT 0,
    right_axis          SMALLINT,
    right_prism         NUMERIC(4,2) DEFAULT 0,
    right_prism_dir     VARCHAR(2) CHECK (right_prism_dir IN ('BU', 'BD', 'BI', 'BO')),
    right_add           NUMERIC(4,2) DEFAULT 0,
    right_visual_acuity VARCHAR(10),
    right_method        VARCHAR(30) NOT NULL,
    right_notes         TEXT,
    
    -- Left eye
    left_sphere         NUMERIC(4,2) NOT NULL,
    left_cylinder       NUMERIC(4,2) DEFAULT 0,
    left_axis           SMALLINT,
    left_prism          NUMERIC(4,2) DEFAULT 0,
    left_prism_dir      VARCHAR(2) CHECK (left_prism_dir IN ('BU', 'BD', 'BI', 'BO')),
    left_add            NUMERIC(4,2) DEFAULT 0,
    left_visual_acuity  VARCHAR(10),
    left_method         VARCHAR(30) NOT NULL,
    left_notes          TEXT,
    
    issued_date       TIMESTAMPTZ NOT NULL,
    expiry_date       TIMESTAMPTZ NOT NULL,
    is_current        BOOLEAN NOT NULL DEFAULT TRUE,
    
    -- FHIR resource storage
    fhir_resource     JSONB NOT NULL,
    fhir_version      INTEGER NOT NULL DEFAULT 1,
    
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT valid_right_axis CHECK (right_axis IS NULL OR (right_axis >= 1 AND right_axis <= 180)),
    CONSTRAINT valid_left_axis CHECK (left_axis IS NULL OR (left_axis >= 1 AND left_axis <= 180)),
    CONSTRAINT valid_right_cylinder CHECK (right_cylinder = 0 OR right_axis IS NOT NULL),
    CONSTRAINT valid_left_cylinder CHECK (left_cylinder = 0 OR left_axis IS NOT NULL),
    CONSTRAINT valid_right_prism CHECK (right_prism = 0 OR right_prism_dir IS NOT NULL),
    CONSTRAINT valid_left_prism CHECK (left_prism = 0 OR left_prism_dir IS NOT NULL)
);

CREATE INDEX idx_vision_prescriptions_patient ON vision_prescriptions(patient_nhi);
CREATE INDEX idx_vision_prescriptions_clinician ON vision_prescriptions(clinician_id);
CREATE INDEX idx_vision_prescriptions_practice ON vision_prescriptions(practice_id);
CREATE INDEX idx_vision_prescriptions_issued ON vision_prescriptions(issued_date DESC);
CREATE INDEX idx_vision_prescriptions_current ON vision_prescriptions(patient_nhi, is_current) WHERE is_current = TRUE;
CREATE INDEX idx_vision_prescriptions_fhir ON vision_prescriptions USING GIN (fhir_resource);

-- Trigger to maintain single current prescription per patient per type/distance
CREATE OR REPLACE FUNCTION vision_ensure_single_current()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.is_current THEN
        UPDATE vision_prescriptions
        SET is_current = FALSE, updated_at = NOW()
        WHERE patient_nhi = NEW.patient_nhi
          AND type = NEW.type
          AND distance = NEW.distance
          AND id != NEW.id
          AND is_current = TRUE;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_vision_prescriptions_single_current
BEFORE INSERT OR UPDATE ON vision_prescriptions
FOR EACH ROW EXECUTE FUNCTION vision_ensure_single_current();

-- Updated at trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_vision_prescriptions_updated_at
BEFORE UPDATE ON vision_prescriptions
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();