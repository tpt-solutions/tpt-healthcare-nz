-- Home Visits tables
-- Scheduling and documentation for community health visits.

CREATE TABLE IF NOT EXISTS home_visits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    practice_id UUID NOT NULL,
    scheduled_date TIMESTAMPTZ NOT NULL,
    estimated_duration_minutes INTEGER NOT NULL DEFAULT 30,
    actual_start_time TIMESTAMPTZ,
    actual_end_time TIMESTAMPTZ,
    visit_type VARCHAR(50) NOT NULL, -- wound_care, medication_review, post_acute, palliative, assessment, follow_up
    priority VARCHAR(20) NOT NULL DEFAULT 'routine', -- urgent, high, routine, low
    status VARCHAR(30) NOT NULL DEFAULT 'scheduled', -- scheduled, in_transit, arrived, in_progress, completed, cancelled, rescheduled, no_show
    address TEXT NOT NULL,
    latitude NUMERIC(10, 8),
    longitude NUMERIC(11, 8),
    contact_phone VARCHAR(20),
    contact_name VARCHAR(100),
    access_instructions TEXT,
    safety_notes TEXT,
    transport_mode VARCHAR(30), -- car, bike, public_transport, walking
    route_order INTEGER,
    previous_visit_id UUID REFERENCES home_visits(id),
    cancellation_reason VARCHAR(50),
    cancellation_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_home_visits_patient ON home_visits(patient_nhi);
CREATE INDEX idx_home_visits_clinician ON home_visits(clinician_id);
CREATE INDEX idx_home_visits_date ON home_visits(scheduled_date);
CREATE INDEX idx_home_visits_status ON home_visits(status);
CREATE INDEX idx_home_visits_priority ON home_visits(priority);
CREATE INDEX idx_home_visits_type ON home_visits(visit_type);

CREATE TABLE IF NOT EXISTS home_visit_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    home_visit_id UUID NOT NULL REFERENCES home_visits(id) ON DELETE CASCADE,
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    note_type VARCHAR(30) NOT NULL, -- subjective, objective, assessment, plan, supplementary
    narrative TEXT NOT NULL,
    concerns TEXT[], -- array of concerns identified during visit
    actions TEXT[], -- array of actions taken
    follow_up_required BOOLEAN NOT NULL DEFAULT FALSE,
    follow_up_details TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_home_visit_notes_visit ON home_visit_notes(home_visit_id);
CREATE INDEX idx_home_visit_notes_patient ON home_visit_notes(patient_nhi);

CREATE TABLE IF NOT EXISTS home_visit_outcomes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    home_visit_id UUID NOT NULL REFERENCES home_visits(id) ON DELETE CASCADE,
    patient_nhi VARCHAR(7) NOT NULL,
    outcome_category VARCHAR(50) NOT NULL, -- wound_healing, medication_adherence, functional_improvement, safety, referral
    outcome_description TEXT NOT NULL,
    outcome_score INTEGER, -- where applicable, e.g., 1-5 scale
    achieved BOOLEAN,
    review_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_home_visit_outcomes_visit ON home_visit_outcomes(home_visit_id);

CREATE TABLE IF NOT EXISTS home_visit_equipment (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    home_visit_id UUID NOT NULL REFERENCES home_visits(id) ON DELETE CASCADE,
    equipment_name VARCHAR(100) NOT NULL,
    serial_number VARCHAR(100),
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(30) NOT NULL DEFAULT 'functioning', -- functioning, needs_service, broken, missing
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_home_visit_equipment_visit ON home_visit_equipment(home_visit_id);

CREATE TABLE IF NOT EXISTS home_visit_safety_checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    home_visit_id UUID NOT NULL REFERENCES home_visits(id) ON DELETE CASCADE,
    patient_nhi VARCHAR(7) NOT NULL,
    checked_by UUID NOT NULL, -- clinician_id
    check_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    fall_risk INTEGER NOT NULL DEFAULT 0, -- 0-10 scale
    pressure_injury_risk INTEGER NOT NULL DEFAULT 0,
    fire_safety_ok BOOLEAN NOT NULL DEFAULT TRUE,
    smoke_alarms_ok BOOLEAN NOT NULL DEFAULT TRUE,
    medication_storage_ok BOOLEAN NOT NULL DEFAULT TRUE,
    tripping_hazards_noted TEXT[],
    recommendations TEXT[],
    actions_taken TEXT[],
    next_check_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_home_visit_safety_checks_visit ON home_visit_safety_checks(home_visit_id);

-- Clinician daily route for home visits (updated by scheduler/optimiser)
CREATE TABLE IF NOT EXISTS clinician_daily_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinician_id UUID NOT NULL,
    route_date DATE NOT NULL,
    visit_ids UUID[] NOT NULL, -- ordered array of home_visit IDs
    estimated_total_distance_km NUMERIC(8,2),
    estimated_total_duration_minutes INTEGER,
    actual_total_distance_km NUMERIC(8,2),
    actual_total_duration_minutes INTEGER,
    status VARCHAR(30) NOT NULL DEFAULT 'planned', -- planned, in_progress, completed
    optimisation_algorithm VARCHAR(50) DEFAULT 'nearest_neighbour',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(clinician_id, route_date)
);

CREATE INDEX idx_clinician_routes_clinician ON clinician_daily_routes(clinician_id);
CREATE INDEX idx_clinician_routes_date ON clinician_daily_routes(route_date);
