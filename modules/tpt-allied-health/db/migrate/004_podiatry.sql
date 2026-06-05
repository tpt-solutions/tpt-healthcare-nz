-- Podiatry tables
CREATE TABLE IF NOT EXISTS podiatry_assessments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    acc_number VARCHAR(20),
    referral_source VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    reason TEXT NOT NULL,
    findings TEXT,
    diagnosis TEXT,
    icd10_code VARCHAR(10),
    risk_category VARCHAR(30),
    vascular_status VARCHAR(30),
    neurological_status VARCHAR(30),
    status VARCHAR(30) NOT NULL DEFAULT 'scheduled',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podiatry_assessments_patient ON podiatry_assessments(patient_nhi);
CREATE INDEX idx_podiatry_assessments_clinician ON podiatry_assessments(clinician_id);
CREATE INDEX idx_podiatry_assessments_type ON podiatry_assessments(type);
CREATE INDEX idx_podiatry_assessments_risk ON podiatry_assessments(risk_category);
CREATE INDEX idx_podiatry_assessments_status ON podiatry_assessments(status);
CREATE INDEX idx_podiatry_assessments_acc ON podiatry_assessments(acc_number);

CREATE TABLE IF NOT EXISTS podiatry_recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id UUID NOT NULL REFERENCES podiatry_assessments(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    priority VARCHAR(20) NOT NULL DEFAULT 'routine',
    type VARCHAR(50),
    frequency VARCHAR(50),
    duration VARCHAR(50),
    equipment TEXT,
    referral_to TEXT,
    funding_source VARCHAR(50),
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podiatry_recommendations_assessment ON podiatry_recommendations(assessment_id);

CREATE TABLE IF NOT EXISTS podiatry_assessment_outcome_measures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id UUID NOT NULL REFERENCES podiatry_assessments(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    domain VARCHAR(50) NOT NULL,
    score DECIMAL(10,2) NOT NULL,
    unit VARCHAR(20),
    date TIMESTAMPTZ NOT NULL,
    interpretation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podiatry_assessment_outcome_measures_assessment ON podiatry_assessment_outcome_measures(assessment_id);

CREATE TABLE IF NOT EXISTS podiatry_treatment_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    assessment_id UUID REFERENCES podiatry_assessments(id) ON DELETE SET NULL,
    acc_number VARCHAR(20),
    start_date TIMESTAMPTZ NOT NULL,
    review_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podiatry_treatment_plans_patient ON podiatry_treatment_plans(patient_nhi);
CREATE INDEX idx_podiatry_treatment_plans_clinician ON podiatry_treatment_plans(clinician_id);
CREATE INDEX idx_podiatry_treatment_plans_status ON podiatry_treatment_plans(status);
CREATE INDEX idx_podiatry_treatment_plans_assessment ON podiatry_treatment_plans(assessment_id);

CREATE TABLE IF NOT EXISTS podiatry_treatment_goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    treatment_plan_id UUID NOT NULL REFERENCES podiatry_treatment_plans(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    domain VARCHAR(50) NOT NULL,
    target_date TIMESTAMPTZ NOT NULL,
    criteria TEXT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'not_started',
    outcome TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podiatry_treatment_goals_plan ON podiatry_treatment_goals(treatment_plan_id);

CREATE TABLE IF NOT EXISTS podiatry_planned_interventions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    treatment_plan_id UUID NOT NULL REFERENCES podiatry_treatment_plans(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    frequency VARCHAR(50) NOT NULL,
    duration VARCHAR(50) NOT NULL,
    location VARCHAR(50) NOT NULL,
    materials TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'planned',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podiatry_planned_interventions_plan ON podiatry_planned_interventions(treatment_plan_id);

CREATE TABLE IF NOT EXISTS podiatry_session_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    treatment_plan_id UUID REFERENCES podiatry_treatment_plans(id) ON DELETE SET NULL,
    session_date TIMESTAMPTZ NOT NULL,
    session_number INTEGER NOT NULL,
    location VARCHAR(50) NOT NULL,
    subjective TEXT,
    objective TEXT,
    assessment TEXT,
    plan TEXT,
    duration_minutes INTEGER NOT NULL,
    charge_code VARCHAR(20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podiatry_session_notes_patient ON podiatry_session_notes(patient_nhi);
CREATE INDEX idx_podiatry_session_notes_plan ON podiatry_session_notes(treatment_plan_id);
CREATE INDEX idx_podiatry_session_notes_date ON podiatry_session_notes(session_date);

CREATE TABLE IF NOT EXISTS podiatry_session_interventions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_note_id UUID NOT NULL REFERENCES podiatry_session_notes(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    frequency VARCHAR(50),
    duration VARCHAR(50),
    location VARCHAR(50),
    materials TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'planned',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podiatry_session_interventions_note ON podiatry_session_interventions(session_note_id);

CREATE TABLE IF NOT EXISTS podiatry_session_outcome_measures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_note_id UUID NOT NULL REFERENCES podiatry_session_notes(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    domain VARCHAR(50) NOT NULL,
    score DECIMAL(10,2) NOT NULL,
    unit VARCHAR(20),
    date TIMESTAMPTZ NOT NULL,
    interpretation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podiatry_session_outcome_measures_note ON podiatry_session_outcome_measures(session_note_id);

-- Wound assessments (specialised)
CREATE TABLE IF NOT EXISTS podiatry_wound_assessments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    location VARCHAR(100) NOT NULL,
    side VARCHAR(20) NOT NULL, -- left, right, bilateral
    wound_type VARCHAR(50) NOT NULL,
    length_cm DECIMAL(6,2),
    width_cm DECIMAL(6,2),
    depth_cm DECIMAL(6,2),
    undermining_cm DECIMAL(6,2),
    sinus_tract_cm DECIMAL(6,2),
    area_cm2 DECIMAL(8,2),
    volume_ml DECIMAL(8,2),
    tissue_type VARCHAR(30),
    exudate_level VARCHAR(20),
    erythema BOOLEAN DEFAULT FALSE,
    oedema BOOLEAN DEFAULT FALSE,
    heat BOOLEAN DEFAULT FALSE,
    pain BOOLEAN DEFAULT FALSE,
    purulent_exudate BOOLEAN DEFAULT FALSE,
    odour BOOLEAN DEFAULT FALSE,
    fever BOOLEAN DEFAULT FALSE,
    wbc_elevated BOOLEAN DEFAULT FALSE,
    crp_elevated BOOLEAN DEFAULT FALSE,
    periwound_skin TEXT,
    pain_score INTEGER CHECK (pain_score >= 0 AND pain_score <= 10),
    treatment TEXT,
    dressing_plan TEXT,
    offloading TEXT,
    referrals TEXT,
    review_date TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'scheduled',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_podiatry_wound_assessments_patient ON podiatry_wound_assessments(patient_nhi);
CREATE INDEX idx_podiatry_wound_assessments_clinician ON podiatry_wound_assessments(clinician_id);
CREATE INDEX idx_podiatry_wound_assessments_type ON podiatry_wound_assessments(wound_type);
CREATE INDEX idx_podiatry_wound_assessments_status ON podiatry_wound_assessments(status);
CREATE INDEX idx_podiatry_wound_assessments_date ON podiatry_wound_assessments(date);