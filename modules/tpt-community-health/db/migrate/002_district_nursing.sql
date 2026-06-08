-- District Nursing tables
-- Community nursing care plans, wound care, chronic disease management.

CREATE TABLE IF NOT EXISTS district_nursing_care_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL, -- responsible nurse
    practice_id UUID NOT NULL,
    plan_name VARCHAR(100) NOT NULL,
    plan_type VARCHAR(50) NOT NULL, -- wound_care, palliative, diabetes, heart_failure, copd, post_surgical, post_acute
    status VARCHAR(30) NOT NULL DEFAULT 'active', -- draft, active, under_review, completed, suspended
    start_date TIMESTAMPTZ NOT NULL,
    review_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ,
    goals TEXT[],
    risk_level VARCHAR(20) NOT NULL DEFAULT 'low', -- low, moderate, high, very_high
    consent_given BOOLEAN NOT NULL DEFAULT FALSE,
    consent_date TIMESTAMPTZ,
    dhb_funded BOOLEAN NOT NULL DEFAULT FALSE,
    funding_code VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dn_care_plans_patient ON district_nursing_care_plans(patient_nhi);
CREATE INDEX idx_dn_care_plans_clinician ON district_nursing_care_plans(clinician_id);
CREATE INDEX idx_dn_care_plans_status ON district_nursing_care_plans(status);
CREATE INDEX idx_dn_care_plans_type ON district_nursing_care_plans(plan_type);
CREATE INDEX idx_dn_care_plans_dates ON district_nursing_care_plans(start_date, review_date);

CREATE TABLE IF NOT EXISTS district_nursing_visits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    care_plan_id UUID NOT NULL REFERENCES district_nursing_care_plans(id) ON DELETE CASCADE,
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    visit_date TIMESTAMPTZ NOT NULL,
    visit_type VARCHAR(50) NOT NULL, -- scheduled, unscheduled, urgent
    visit_status VARCHAR(30) NOT NULL DEFAULT 'scheduled', -- scheduled, in_progress, completed, cancelled
    vital_signs JSONB, -- temperature, bp, hr, spo2, pain, weight
    wound_assessments JSONB[], -- array of wound assessments
    medications_administered JSONB[], -- array of administered meds
    observations TEXT,
    patient_education TEXT[],
    equipment_check TEXT[],
    next_visit_date TIMESTAMPTZ,
    next_visit_reason TEXT,
    concerns TEXT[],
    escalations TEXT, -- e.g., referral to GP, acute admission
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dn_visits_plan ON district_nursing_visits(care_plan_id);
CREATE INDEX idx_dn_visits_patient ON district_nursing_visits(patient_nhi);
CREATE INDEX idx_dn_visits_clinician ON district_nursing_visits(clinician_id);
CREATE INDEX idx_dn_visits_date ON district_nursing_visits(visit_date);
CREATE INDEX idx_dn_visits_status ON district_nursing_visits(visit_status);

CREATE TABLE IF NOT EXISTS district_nursing_medication_administration (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    care_plan_id UUID NOT NULL REFERENCES district_nursing_care_plans(id) ON DELETE CASCADE,
    visit_id UUID REFERENCES district_nursing_visits(id) ON DELETE SET NULL,
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    medication_name VARCHAR(100) NOT NULL,
    dose VARCHAR(50) NOT NULL,
    route VARCHAR(30) NOT NULL, -- oral, im, iv, sc, topical, inhalation
    frequency VARCHAR(50) NOT NULL,
    prescribed_by VARCHAR(100), -- prescriber name/GMC number
    administered_at TIMESTAMPTZ,
    administration_status VARCHAR(30) NOT NULL DEFAULT 'scheduled', -- scheduled, administered, refused, omitted, held
    omission_reason TEXT,
    observations TEXT,
    side_effects_noted TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dn_med_admin_plan ON district_nursing_medication_administration(care_plan_id);
CREATE INDEX idx_dn_med_admin_visit ON district_nursing_medication_administration(visit_id);
CREATE INDEX idx_dn_med_admin_patient ON district_nursing_medication_administration(patient_nhi);
CREATE INDEX idx_dn_med_admin_status ON district_nursing_medication_administration(administration_status);
CREATE INDEX idx_dn_med_admin_date ON district_nursing_medication_administration(administered_at);

CREATE TABLE IF NOT EXISTS district_nursing_wound_care (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    care_plan_id UUID NOT NULL REFERENCES district_nursing_care_plans(id) ON DELETE CASCADE,
    visit_id UUID REFERENCES district_nursing_visits(id) ON DELETE SET NULL,
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    wound_site VARCHAR(100) NOT NULL,
    wound_cause VARCHAR(50), -- pressure_injury, venous, arterial, diabetic, surgical, trauma
    wound_dimensions JSONB, -- {length, width, depth, undermining}
    tissue_type VARCHAR(30), -- necrotic, sloughy, granulating, epithelialising
    exudate_amount VARCHAR(20), -- none, low, moderate, high
    exudate_type VARCHAR(30), -- serous, sanguineous, purulent, serosanguineous
    odour BOOLEAN NOT NULL DEFAULT FALSE,
    signs_infection JSONB, -- {erythema, oedema, heat, pain, purulent_exudate, odour}
    periwound_skin VARCHAR(50),
    pain_score INTEGER, -- 0-10
    dressing_applied VARCHAR(100),
    cleansing_solution VARCHAR(100),
    debridement_performed BOOLEAN NOT NULL DEFAULT FALSE,
    debridement_type VARCHAR(30),
    photos_taken BOOLEAN NOT NULL DEFAULT FALSE,
    photo_urls TEXT[],
    next_review_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dn_wound_plan ON district_nursing_wound_care(care_plan_id);
CREATE INDEX idx_dn_wound_visit ON district_nursing_wound_care(visit_id);
CREATE INDEX idx_dn_wound_patient ON district_nursing_wound_care(patient_nhi);
CREATE INDEX idx_dn_wound_cause ON district_nursing_wound_care(wound_cause);
