-- Outreach tables
-- Community outreach activities, mobile clinic visits, case management.

CREATE TABLE IF NOT EXISTS outreach_programs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id UUID NOT NULL,
    program_name VARCHAR(100) NOT NULL,
    program_type VARCHAR(50) NOT NULL, -- mobile_clinic, vaccination, screening, health_promotion, chronic_disease_support
    description TEXT,
    target_population TEXT, -- e.g., elderly, chronic_disease, rural
    service_area GEOGRAPHY(POLYGON, 4326), -- geographic service boundary
    status VARCHAR(30) NOT NULL DEFAULT 'active', -- active, paused, completed, discontinued
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ,
    funding_source VARCHAR(50), -- dhb, moh, pho, charity, other
    funding_code VARCHAR(50),
    budget NUMERIC(12,2),
    spent NUMERIC(12,2) DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outreach_programs_practice ON outreach_programs(practice_id);
CREATE INDEX idx_outreach_programs_status ON outreach_programs(status);
CREATE INDEX idx_outreach_programs_type ON outreach_programs(program_type);

CREATE TABLE IF NOT EXISTS outreach_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    program_id UUID NOT NULL REFERENCES outreach_programs(id) ON DELETE CASCADE,
    event_name VARCHAR(100) NOT NULL,
    event_type VARCHAR(50) NOT NULL, -- clinic, screening, education, vaccination
    scheduled_date TIMESTAMPTZ NOT NULL,
    estimated_duration_minutes INTEGER NOT NULL DEFAULT 120,
    location_address TEXT NOT NULL,
    latitude NUMERIC(10, 8),
    longitude NUMERIC(11, 8),
    venue_name VARCHAR(100),
    venue_contact VARCHAR(20),
    target_attendees INTEGER,
    actual_attendees INTEGER,
    clinicians UUID[] NOT NULL, -- array of clinician IDs
    equipment_list TEXT[],
    status VARCHAR(30) NOT NULL DEFAULT 'planned', -- planned, confirmed, in_progress, completed, cancelled
    cancellation_reason TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outreach_events_program ON outreach_events(program_id);
CREATE INDEX idx_outreach_events_date ON outreach_events(scheduled_date);
CREATE INDEX idx_outreach_events_status ON outreach_events(status);

CREATE TABLE IF NOT EXISTS outreach_attendees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES outreach_events(id) ON DELETE CASCADE,
    patient_nhi VARCHAR(7),
    attendee_name VARCHAR(100) NOT NULL,
    attendee_type VARCHAR(30) NOT NULL DEFAULT 'patient', -- patient, carer, community_member, staff
    contact_phone VARCHAR(20),
    contact_email VARCHAR(100),
    demographics JSONB, -- {age, ethnicity, gender}
    nhi_provided BOOLEAN NOT NULL DEFAULT FALSE,
    registration_method VARCHAR(30), -- walk_in, booked, referral
    attended_at TIMESTAMPTZ,
    services_received TEXT[], -- e.g., blood_pressure, diabetes_screen, vaccination, whanau_ora
    consent_given BOOLEAN NOT NULL DEFAULT FALSE,
    follow_up_required BOOLEAN NOT NULL DEFAULT FALSE,
    follow_up_details TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outreach_attendees_event ON outreach_attendees(event_id);
CREATE INDEX idx_outreach_attendees_nhi ON outreach_attendees(patient_nhi);
CREATE INDEX idx_outreach_attendees_follow_up ON outreach_attendees(follow_up_required);

CREATE TABLE IF NOT EXISTS outreach_referrals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES outreach_events(id) ON DELETE CASCADE,
    patient_nhi VARCHAR(7) NOT NULL,
    referred_by UUID NOT NULL, -- clinician_id
    referral_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    referral_type VARCHAR(50) NOT NULL, -- gp, specialist, mental_health, social_services, housing, other
    referral_reason TEXT NOT NULL,
    urgency VARCHAR(20) NOT NULL DEFAULT 'routine', -- routine, urgent, emergency
    status VARCHAR(30) NOT NULL DEFAULT 'pending', -- pending, accepted, declined, in_progress, completed
    outcome TEXT,
    outcome_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outreach_referrals_event ON outreach_referrals(event_id);
CREATE INDEX idx_outreach_referrals_patient ON outreach_referrals(patient_nhi);
CREATE INDEX idx_outreach_referrals_status ON outreach_referrals(status);
CREATE INDEX idx_outreach_referrals_type ON outreach_referrals(referral_type);

CREATE TABLE IF NOT EXISTS outreach_screenings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES outreach_events(id) ON DELETE CASCADE,
    patient_nhi VARCHAR(7) NOT NULL,
    clinician_id UUID NOT NULL,
    screening_type VARCHAR(50) NOT NULL, -- blood_pressure, diabetes, cervical, bowel, hearing, vision
    screening_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    result_category VARCHAR(30) NOT NULL, -- normal, abnormal, borderline, inconclusive
    result_value TEXT,
    interpretation TEXT,
    consent_given BOOLEAN NOT NULL DEFAULT FALSE,
    follow_up_required BOOLEAN NOT NULL DEFAULT FALSE,
    follow_up_details TEXT,
    referral_id UUID REFERENCES outreach_referrals(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outreach_screenings_event ON outreach_screenings(event_id);
CREATE INDEX idx_outreach_screenings_patient ON outreach_screenings(patient_nhi);
CREATE INDEX idx_outreach_screenings_type ON outreach_screenings(screening_type);
