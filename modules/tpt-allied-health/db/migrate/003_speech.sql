-- Speech-Language Therapy tables
CREATE TABLE IF NOT EXISTS speech_assessments (
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
    status VARCHAR(30) NOT NULL DEFAULT 'scheduled',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_speech_assessments_patient ON speech_assessments(patient_nhi);
CREATE INDEX idx_speech_assessments_clinician ON speech_assessments(clinician_id);
CREATE INDEX idx_speech_assessments_type ON speech_assessments(type);
CREATE INDEX idx_speech_assessments_status ON speech_assessments(status);
CREATE INDEX idx_speech_assessments_acc ON speech_assessments(acc_number);

CREATE TABLE IF NOT EXISTS speech_recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id UUID NOT NULL REFERENCES speech_assessments(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    priority VARCHAR(20) NOT NULL DEFAULT 'routine',
    type VARCHAR(50),
    frequency VARCHAR(50),
    duration VARCHAR(50),
    setting VARCHAR(50),
    equipment TEXT,
    funding_source VARCHAR(50),
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_speech_recommendations_assessment ON speech_recommendations(assessment_id);

CREATE TABLE IF NOT EXISTS speech_assessment_outcome_measures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id UUID NOT NULL REFERENCES speech_assessments(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    domain VARCHAR(50) NOT NULL,
    score DECIMAL(10,2) NOT NULL,
    max_score DECIMAL(10,2),
    percentile DECIMAL(5,2),
    age_equivalent VARCHAR(20),
    date TIMESTAMPTZ NOT NULL,
    interpretation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_speech_assessment_outcome_measures_assessment ON speech_assessment_outcome_measures(assessment_id);

CREATE TABLE IF NOT EXISTS speech_therapy_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    assessment_id UUID REFERENCES speech_assessments(id) ON DELETE SET NULL,
    acc_number VARCHAR(20),
    start_date TIMESTAMPTZ NOT NULL,
    review_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_speech_therapy_plans_patient ON speech_therapy_plans(patient_nhi);
CREATE INDEX idx_speech_therapy_plans_clinician ON speech_therapy_plans(clinician_id);
CREATE INDEX idx_speech_therapy_plans_status ON speech_therapy_plans(status);
CREATE INDEX idx_speech_therapy_plans_assessment ON speech_therapy_plans(assessment_id);

CREATE TABLE IF NOT EXISTS speech_therapy_goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    therapy_plan_id UUID NOT NULL REFERENCES speech_therapy_plans(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    domain VARCHAR(50) NOT NULL,
    target_date TIMESTAMPTZ NOT NULL,
    criteria TEXT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'not_started',
    outcome TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_speech_therapy_goals_plan ON speech_therapy_goals(therapy_plan_id);

CREATE TABLE IF NOT EXISTS speech_planned_interventions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    therapy_plan_id UUID NOT NULL REFERENCES speech_therapy_plans(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    frequency VARCHAR(50) NOT NULL,
    duration VARCHAR(50) NOT NULL,
    setting VARCHAR(50) NOT NULL,
    techniques TEXT,
    materials TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'planned',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_speech_planned_interventions_plan ON speech_planned_interventions(therapy_plan_id);

CREATE TABLE IF NOT EXISTS speech_session_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    therapy_plan_id UUID REFERENCES speech_therapy_plans(id) ON DELETE SET NULL,
    session_date TIMESTAMPTZ NOT NULL,
    session_number INTEGER NOT NULL,
    setting VARCHAR(50) NOT NULL,
    subjective TEXT,
    objective TEXT,
    assessment TEXT,
    plan TEXT,
    duration_minutes INTEGER NOT NULL,
    charge_code VARCHAR(20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_speech_session_notes_patient ON speech_session_notes(patient_nhi);
CREATE INDEX idx_speech_session_notes_plan ON speech_session_notes(therapy_plan_id);
CREATE INDEX idx_speech_session_notes_date ON speech_session_notes(session_date);

CREATE TABLE IF NOT EXISTS speech_session_interventions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_note_id UUID NOT NULL REFERENCES speech_session_notes(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    frequency VARCHAR(50),
    duration VARCHAR(50),
    setting VARCHAR(50),
    techniques TEXT,
    materials TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'planned',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_speech_session_interventions_note ON speech_session_interventions(session_note_id);

CREATE TABLE IF NOT EXISTS speech_session_outcome_measures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_note_id UUID NOT NULL REFERENCES speech_session_notes(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    domain VARCHAR(50) NOT NULL,
    score DECIMAL(10,2) NOT NULL,
    max_score DECIMAL(10,2),
    percentile DECIMAL(5,2),
    age_equivalent VARCHAR(20),
    date TIMESTAMPTZ NOT NULL,
    interpretation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_speech_session_outcome_measures_note ON speech_session_outcome_measures(session_note_id);

-- Swallowing assessments (specialised)
CREATE TABLE IF NOT EXISTS speech_swallowing_assessments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    reason TEXT NOT NULL,
    oral_mechanism TEXT,
    clinical_findings TEXT,
    instrumental_exam VARCHAR(50), -- VFSS, FEES
    diet_recommendations TEXT NOT NULL, -- IDDSI levels
    strategies TEXT,
    referrals TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'scheduled',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_speech_swallowing_assessments_patient ON speech_swallowing_assessments(patient_nhi);
CREATE INDEX idx_speech_swallowing_assessments_clinician ON speech_swallowing_assessments(clinician_id);
CREATE INDEX idx_speech_swallowing_assessments_status ON speech_swallowing_assessments(status);