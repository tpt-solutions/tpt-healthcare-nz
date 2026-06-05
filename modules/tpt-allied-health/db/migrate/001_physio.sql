-- Physiotherapy tables
CREATE TABLE IF NOT EXISTS physio_treatment_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    acc_number VARCHAR(20),
    referral_source VARCHAR(50) NOT NULL,
    diagnosis TEXT NOT NULL,
    icd10_code VARCHAR(10),
    start_date TIMESTAMPTZ NOT NULL,
    review_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_physio_treatment_plans_patient ON physio_treatment_plans(patient_nhi);
CREATE INDEX idx_physio_treatment_plans_clinician ON physio_treatment_plans(clinician_id);
CREATE INDEX idx_physio_treatment_plans_status ON physio_treatment_plans(status);
CREATE INDEX idx_physio_treatment_plans_acc ON physio_treatment_plans(acc_number);

CREATE TABLE IF NOT EXISTS physio_treatment_goals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    treatment_plan_id UUID NOT NULL REFERENCES physio_treatment_plans(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    target_date TIMESTAMPTZ NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'not_started',
    outcome TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_physio_treatment_goals_plan ON physio_treatment_goals(treatment_plan_id);

CREATE TABLE IF NOT EXISTS physio_interventions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    treatment_plan_id UUID NOT NULL REFERENCES physio_treatment_plans(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    body_region VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    frequency VARCHAR(50) NOT NULL,
    duration VARCHAR(50) NOT NULL,
    intensity VARCHAR(50),
    parameters JSONB,
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'planned',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_physio_interventions_plan ON physio_interventions(treatment_plan_id);

CREATE TABLE IF NOT EXISTS physio_outcome_measures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    treatment_plan_id UUID NOT NULL REFERENCES physio_treatment_plans(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    score DECIMAL(10,2) NOT NULL,
    max_score DECIMAL(10,2) NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    interpretation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_physio_outcome_measures_plan ON physio_outcome_measures(treatment_plan_id);

CREATE TABLE IF NOT EXISTS physio_session_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    treatment_plan_id UUID REFERENCES physio_treatment_plans(id) ON DELETE SET NULL,
    session_date TIMESTAMPTZ NOT NULL,
    session_number INTEGER NOT NULL,
    subjective TEXT,
    objective TEXT,
    assessment TEXT,
    plan TEXT,
    duration_minutes INTEGER NOT NULL,
    charge_code VARCHAR(20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_physio_session_notes_patient ON physio_session_notes(patient_nhi);
CREATE INDEX idx_physio_session_notes_plan ON physio_session_notes(treatment_plan_id);
CREATE INDEX idx_physio_session_notes_date ON physio_session_notes(session_date);

CREATE TABLE IF NOT EXISTS physio_session_interventions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_note_id UUID NOT NULL REFERENCES physio_session_notes(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    body_region VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    frequency VARCHAR(50),
    duration VARCHAR(50),
    intensity VARCHAR(50),
    parameters JSONB,
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'planned',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_physio_session_interventions_note ON physio_session_interventions(session_note_id);

CREATE TABLE IF NOT EXISTS physio_session_outcome_measures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_note_id UUID NOT NULL REFERENCES physio_session_notes(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    score DECIMAL(10,2) NOT NULL,
    max_score DECIMAL(10,2) NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    interpretation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_physio_session_outcome_measures_note ON physio_session_outcome_measures(session_note_id);