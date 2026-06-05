-- Occupational Therapy tables
CREATE TABLE IF NOT EXISTS ot_assessments (
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
    status VARCHAR(30) NOT NULL DEFAULT 'scheduled',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ot_assessments_patient ON ot_assessments(patient_nhi);
CREATE INDEX idx_ot_assessments_clinician ON ot_assessments(clinician_id);
CREATE INDEX idx_ot_assessments_type ON ot_assessments(type);
CREATE INDEX idx_ot_assessments_status ON ot_assessments(status);
CREATE INDEX idx_ot_assessments_acc ON ot_assessments(acc_number);

CREATE TABLE IF NOT EXISTS ot_recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id UUID NOT NULL REFERENCES ot_assessments(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    priority VARCHAR(20) NOT NULL DEFAULT 'routine',
    type VARCHAR(50),
    equipment TEXT,
    supplier TEXT,
    estimated_cost DECIMAL(10,2),
    funding_source VARCHAR(50),
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ot_recommendations_assessment ON ot_recommendations(assessment_id);

CREATE TABLE IF NOT EXISTS ot_assessment_outcome_measures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assessment_id UUID NOT NULL REFERENCES ot_assessments(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    domain VARCHAR(50) NOT NULL,
    score DECIMAL(10,2) NOT NULL,
    max_score DECIMAL(10,2) NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    interpretation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ot_assessment_outcome_measures_assessment ON ot_assessment_outcome_measures(assessment_id);

CREATE TABLE IF NOT EXISTS ot_intervention_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    assessment_id UUID REFERENCES ot_assessments(id) ON DELETE SET NULL,
    acc_number VARCHAR(20),
    start_date TIMESTAMPTZ NOT NULL,
    review_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ot_intervention_plans_patient ON ot_intervention_plans(patient_nhi);
CREATE INDEX idx_ot_intervention_plans_clinician ON ot_intervention_plans(clinician_id);
CREATE INDEX idx_ot_intervention_plans_status ON ot_intervention_plans(status);
CREATE INDEX idx_ot_intervention_plans_assessment ON ot_intervention_plans(assessment_id);

CREATE TABLE IF NOT EXISTS ot_intervention_goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    intervention_plan_id UUID NOT NULL REFERENCES ot_intervention_plans(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    domain VARCHAR(50) NOT NULL,
    target_date TIMESTAMPTZ NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'not_started',
    outcome TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ot_intervention_goals_plan ON ot_intervention_goals(intervention_plan_id);

CREATE TABLE IF NOT EXISTS ot_planned_interventions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    intervention_plan_id UUID NOT NULL REFERENCES ot_intervention_plans(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    frequency VARCHAR(50) NOT NULL,
    duration VARCHAR(50) NOT NULL,
    location VARCHAR(50) NOT NULL,
    equipment_needed TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'planned',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ot_planned_interventions_plan ON ot_planned_interventions(intervention_plan_id);

CREATE TABLE IF NOT EXISTS ot_session_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    intervention_plan_id UUID REFERENCES ot_intervention_plans(id) ON DELETE SET NULL,
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

CREATE INDEX idx_ot_session_notes_patient ON ot_session_notes(patient_nhi);
CREATE INDEX idx_ot_session_notes_plan ON ot_session_notes(intervention_plan_id);
CREATE INDEX idx_ot_session_notes_date ON ot_session_notes(session_date);

CREATE TABLE IF NOT EXISTS ot_session_interventions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_note_id UUID NOT NULL REFERENCES ot_session_notes(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    frequency VARCHAR(50),
    duration VARCHAR(50),
    location VARCHAR(50),
    equipment_needed TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'planned',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ot_session_interventions_note ON ot_session_interventions(session_note_id);

CREATE TABLE IF NOT EXISTS ot_session_outcome_measures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_note_id UUID NOT NULL REFERENCES ot_session_notes(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    domain VARCHAR(50) NOT NULL,
    score DECIMAL(10,2) NOT NULL,
    max_score DECIMAL(10,2) NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    interpretation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ot_session_outcome_measures_note ON ot_session_outcome_measures(session_note_id);