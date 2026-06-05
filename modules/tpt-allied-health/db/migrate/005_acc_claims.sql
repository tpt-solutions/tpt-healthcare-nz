-- ACC Claims tables (shared across all allied health professions)
CREATE TABLE IF NOT EXISTS acc_claims (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    claim_type VARCHAR(50) NOT NULL, -- physiotherapy, occupational_therapy, speech_language_therapy, podiatry
    acc_number VARCHAR(20) NOT NULL UNIQUE,
    injury_date TIMESTAMPTZ NOT NULL,
    claim_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    diagnosis TEXT NOT NULL,
    icd10_code VARCHAR(10),
    body_region VARCHAR(50) NOT NULL,
    injury_mechanism TEXT,
    referrer VARCHAR(100),
    approved_sessions INTEGER NOT NULL DEFAULT 0,
    used_sessions INTEGER NOT NULL DEFAULT 0,
    start_date TIMESTAMPTZ NOT NULL,
    expiry_date TIMESTAMPTZ NOT NULL,
    last_treatment_date TIMESTAMPTZ,
    next_review_date TIMESTAMPTZ,
    clinical_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_acc_claims_patient ON acc_claims(patient_nhi);
CREATE INDEX idx_acc_claims_clinician ON acc_claims(clinician_id);
CREATE INDEX idx_acc_claims_type ON acc_claims(claim_type);
CREATE INDEX idx_acc_claims_status ON acc_claims(status);
CREATE INDEX idx_acc_claims_acc_number ON acc_claims(acc_number);
CREATE INDEX idx_acc_claims_expiry ON acc_claims(expiry_date);

CREATE TABLE IF NOT EXISTS acc_treatment_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    claim_id UUID NOT NULL REFERENCES acc_claims(id) ON DELETE CASCADE,
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    session_date TIMESTAMPTZ NOT NULL,
    session_number INTEGER NOT NULL,
    duration_minutes INTEGER NOT NULL,
    charge_code VARCHAR(20) NOT NULL,
    charge_amount DECIMAL(10,2) NOT NULL,
    treatment_type VARCHAR(50) NOT NULL,
    body_region VARCHAR(50) NOT NULL,
    subjective TEXT,
    objective TEXT,
    assessment TEXT,
    plan TEXT,
    status VARCHAR(30) NOT NULL DEFAULT 'planned',
    submitted_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_acc_treatment_sessions_claim ON acc_treatment_sessions(claim_id);
CREATE INDEX idx_acc_treatment_sessions_patient ON acc_treatment_sessions(patient_nhi);
CREATE INDEX idx_acc_treatment_sessions_date ON acc_treatment_sessions(session_date);
CREATE INDEX idx_acc_treatment_sessions_status ON acc_treatment_sessions(status);

CREATE TABLE IF NOT EXISTS acc_session_outcome_measures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES acc_treatment_sessions(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    domain VARCHAR(50) NOT NULL,
    score DECIMAL(10,2) NOT NULL,
    max_score DECIMAL(10,2) NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    interpretation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_acc_session_outcome_measures_session ON acc_session_outcome_measures(session_id);

CREATE TABLE IF NOT EXISTS acc_review_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    claim_id UUID NOT NULL REFERENCES acc_claims(id) ON DELETE CASCADE,
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    report_date TIMESTAMPTZ NOT NULL,
    report_type VARCHAR(30) NOT NULL, -- initial, progress, discharge, extension, reassessment
    sessions_since_last_review INTEGER NOT NULL DEFAULT 0,
    progress_summary TEXT NOT NULL,
    current_status TEXT NOT NULL,
    goals_achieved TEXT[],
    goals_ongoing TEXT[],
    goals_not_achieved TEXT[],
    recommendation VARCHAR(30) NOT NULL, -- continue, extend, discharge, refer, investigate
    additional_sessions_requested INTEGER NOT NULL DEFAULT 0,
    proposed_end_date TIMESTAMPTZ,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    submitted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_acc_review_reports_claim ON acc_review_reports(claim_id);
CREATE INDEX idx_acc_review_reports_patient ON acc_review_reports(patient_nhi);
CREATE INDEX idx_acc_review_reports_type ON acc_review_reports(report_type);
CREATE INDEX idx_acc_review_reports_status ON acc_review_reports(status);

CREATE TABLE IF NOT EXISTS acc_review_outcome_measures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    review_id UUID NOT NULL REFERENCES acc_review_reports(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    domain VARCHAR(50) NOT NULL,
    score DECIMAL(10,2) NOT NULL,
    max_score DECIMAL(10,2) NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    interpretation TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_acc_review_outcome_measures_review ON acc_review_outcome_measures(review_id);

-- ACC Charge codes reference table
CREATE TABLE IF NOT EXISTS acc_charge_codes (
    code VARCHAR(20) PRIMARY KEY,
    description TEXT NOT NULL,
    profession VARCHAR(50) NOT NULL, -- physiotherapy, occupational_therapy, speech_language_therapy, podiatry
    unit VARCHAR(20) NOT NULL, -- session, 15min, 30min, 45min, 60min
    rate DECIMAL(10,2) NOT NULL,
    effective_from TIMESTAMPTZ NOT NULL,
    effective_to TIMESTAMPTZ,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_acc_charge_codes_profession ON acc_charge_codes(profession);
CREATE INDEX idx_acc_charge_codes_active ON acc_charge_codes(active);

-- Insert standard charge codes
INSERT INTO acc_charge_codes (code, description, profession, unit, rate, effective_from, active) VALUES
-- Physiotherapy
('PHY001', 'Physiotherapy initial assessment (45 min)', 'physiotherapy', 'session', 85.00, '2024-01-01', TRUE),
('PHY002', 'Physiotherapy follow-up treatment (30 min)', 'physiotherapy', 'session', 55.00, '2024-01-01', TRUE),
('PHY003', 'Physiotherapy extended treatment (45 min)', 'physiotherapy', 'session', 75.00, '2024-01-01', TRUE),
('PHY004', 'Physiotherapy group session (60 min)', 'physiotherapy', 'session', 35.00, '2024-01-01', TRUE),
('PHY005', 'Physiotherapy hydrotherapy (45 min)', 'physiotherapy', 'session', 65.00, '2024-01-01', TRUE),
('PHY006', 'Physiotherapy report writing (per 15 min)', 'physiotherapy', '15min', 22.00, '2024-01-01', TRUE),

-- Occupational Therapy
('OT001', 'OT initial assessment (60 min)', 'occupational_therapy', 'session', 95.00, '2024-01-01', TRUE),
('OT002', 'OT follow-up treatment (45 min)', 'occupational_therapy', 'session', 70.00, '2024-01-01', TRUE),
('OT003', 'OT home visit assessment (90 min)', 'occupational_therapy', 'session', 140.00, '2024-01-01', TRUE),
('OT004', 'OT worksite assessment (120 min)', 'occupational_therapy', 'session', 180.00, '2024-01-01', TRUE),
('OT005', 'OT equipment prescription (30 min)', 'occupational_therapy', 'session', 55.00, '2024-01-01', TRUE),
('OT006', 'OT report writing (per 15 min)', 'occupational_therapy', '15min', 25.00, '2024-01-01', TRUE),

-- Speech-Language Therapy
('SLT001', 'SLT initial assessment (60 min)', 'speech_language_therapy', 'session', 100.00, '2024-01-01', TRUE),
('SLT002', 'SLT follow-up therapy (45 min)', 'speech_language_therapy', 'session', 75.00, '2024-01-01', TRUE),
('SLT003', 'SLT swallowing assessment (60 min)', 'speech_language_therapy', 'session', 110.00, '2024-01-01', TRUE),
('SLT004', 'SLT AAC assessment (90 min)', 'speech_language_therapy', 'session', 150.00, '2024-01-01', TRUE),
('SLT005', 'SLT report writing (per 15 min)', 'speech_language_therapy', '15min', 28.00, '2024-01-01', TRUE),

-- Podiatry
('POD001', 'Podiatry initial assessment (30 min)', 'podiatry', 'session', 65.00, '2024-01-01', TRUE),
('POD002', 'Podiatry follow-up treatment (20 min)', 'podiatry', 'session', 45.00, '2024-01-01', TRUE),
('POD003', 'Podiatry wound care (30 min)', 'podiatry', 'session', 70.00, '2024-01-01', TRUE),
('POD004', 'Podiatry nail surgery (60 min)', 'podiatry', 'session', 180.00, '2024-01-01', TRUE),
('POD005', 'Podiatry diabetic foot assessment (45 min)', 'podiatry', 'session', 85.00, '2024-01-01', TRUE),
('POD006', 'Podiatry orthotic therapy (45 min)', 'podiatry', 'session', 95.00, '2024-01-01', TRUE),
('POD007', 'Podiatry report writing (per 15 min)', 'podiatry', '15min', 20.00, '2024-01-01', TRUE)
ON CONFLICT (code) DO UPDATE SET
    description = EXCLUDED.description,
    profession = EXCLUDED.profession,
    unit = EXCLUDED.unit,
    rate = EXCLUDED.rate,
    effective_from = EXCLUDED.effective_from,
    effective_to = EXCLUDED.effective_to,
    active = EXCLUDED.active,
    updated_at = NOW();