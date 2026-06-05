-- tpt-vision: Ophthalmic Examinations
-- Maps to FHIR DiagnosticReport resources with NZ ophthalmology extensions

CREATE TABLE IF NOT EXISTS vision_ophthalmic_exams (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    patient_nhi     VARCHAR(7) NOT NULL,
    clinician_id    UUID NOT NULL REFERENCES practitioners(id),
    practice_id     UUID NOT NULL REFERENCES practices(id),
    exam_type       VARCHAR(30) NOT NULL CHECK (exam_type IN (
        'comprehensive', 'follow_up', 'glaucoma', 'retina', 'cataract',
        'cornea', 'neuro_ophthalmic', 'paediatric', 'emergency'
    )),
    exam_date       TIMESTAMPTZ NOT NULL,
    
    -- Visual acuity
    va_distance_right   VARCHAR(10),
    va_distance_left    VARCHAR(10),
    va_near_right       VARCHAR(10),
    va_near_left        VARCHAR(10),
    pinhole_right       VARCHAR(10),
    pinhole_left        VARCHAR(10),
    
    -- Refraction (objective)
    auto_refraction_right   VARCHAR(50),
    auto_refraction_left    VARCHAR(50),
    
    -- Tonometry / IOP (stored as JSONB array)
    iop_readings        JSONB NOT NULL DEFAULT '[]'::jsonb,
    
    -- Anterior segment
    lens_right          VARCHAR(30) NOT NULL CHECK (lens_right IN (
        'phakic', 'posterior_subcapsular', 'nuclear_sclerotic', 'cortical',
        'pc_iol', 'ac_iol', 'aphakic', 'pseudophakic'
    )),
    lens_left           VARCHAR(30) NOT NULL CHECK (lens_left IN (
        'phakic', 'posterior_subcapsular', 'nuclear_sclerotic', 'cortical',
        'pc_iol', 'ac_iol', 'aphakic', 'pseudophakic'
    )),
    cataract_grade      SMALLINT NOT NULL DEFAULT 0 CHECK (cataract_grade BETWEEN 0 AND 4),
    cornea_clear        BOOLEAN NOT NULL DEFAULT TRUE,
    anterior_chamber    VARCHAR(50),
    
    -- Posterior segment
    disc_right          VARCHAR(30) NOT NULL CHECK (disc_right IN (
        'normal', 'cupped', 'pale', 'oedematous', 'drusen', 'tilted'
    )),
    disc_left           VARCHAR(30) NOT NULL CHECK (disc_left IN (
        'normal', 'cupped', 'pale', 'oedematous', 'drusen', 'tilted'
    )),
    cd_ratio_right      NUMERIC(3,2) NOT NULL DEFAULT 0 CHECK (cd_ratio_right BETWEEN 0 AND 1),
    cd_ratio_left       NUMERIC(3,2) NOT NULL DEFAULT 0 CHECK (cd_ratio_left BETWEEN 0 AND 1),
    macula_right        VARCHAR(40) NOT NULL CHECK (macula_right IN (
        'normal', 'macular_oedema', 'macular_drusen', 'choroidal_neovascularisation',
        'macular_hole', 'epiretinal_membrane', 'macular_scar'
    )),
    macula_left         VARCHAR(40) NOT NULL CHECK (macula_left IN (
        'normal', 'macular_oedema', 'macular_drusen', 'choroidal_neovascularisation',
        'macular_hole', 'epiretinal_membrane', 'macular_scar'
    )),
    
    -- Visual fields
    visual_fields_right VARCHAR(100),
    visual_fields_left  VARCHAR(100),
    
    -- OCT imaging
    oct_right           TEXT,
    oct_left            TEXT,
    
    -- Diagnosis / impression
    diagnosis           TEXT,
    plan                TEXT,
    referral_required   BOOLEAN NOT NULL DEFAULT FALSE,
    follow_up_days      SMALLINT,
    
    -- FHIR resource storage
    fhir_resource       JSONB NOT NULL,
    fhir_version        INTEGER NOT NULL DEFAULT 1,
    
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vision_exams_patient ON vision_ophthalmic_exams(patient_nhi);
CREATE INDEX idx_vision_exams_clinician ON vision_ophthalmic_exams(clinician_id);
CREATE INDEX idx_vision_exams_practice ON vision_ophthalmic_exams(practice_id);
CREATE INDEX idx_vision_exams_date ON vision_ophthalmic_exams(exam_date DESC);
CREATE INDEX idx_vision_exams_type ON vision_ophthalmic_exams(exam_type);
CREATE INDEX idx_vision_exams_fhir ON vision_ophthalmic_exams USING GIN (fhir_resource);
CREATE INDEX idx_vision_exams_iop ON vision_ophthalmic_exams USING GIN (iop_readings);

-- Updated at trigger
CREATE TRIGGER trg_vision_exams_updated_at
BEFORE UPDATE ON vision_ophthalmic_exams
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();