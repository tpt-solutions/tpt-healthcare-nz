-- Outpatient specialist clinics, appointments, and waitlist
CREATE TABLE IF NOT EXISTS outpatient_clinics (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                TEXT NOT NULL,
    specialty           TEXT NOT NULL,
    lead_clinician_hpi  TEXT,
    location            TEXT,
    active              BOOLEAN NOT NULL DEFAULT true,
    tenant_id           UUID NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_outpatient_clinics_tenant_specialty ON outpatient_clinics (tenant_id, specialty) WHERE active;

CREATE TABLE IF NOT EXISTS outpatient_appointments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id     UUID NOT NULL REFERENCES outpatient_clinics(id),
    patient_id    UUID NOT NULL,
    patient_nhi   TEXT NOT NULL DEFAULT '',
    clinician_hpi TEXT,
    status        TEXT NOT NULL DEFAULT 'booked',
    referral_id   UUID,
    reason        TEXT NOT NULL,
    notes         TEXT,
    scheduled_at  TIMESTAMPTZ NOT NULL,
    attended_at   TIMESTAMPTZ,
    tenant_id     UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_outpatient_appointments_clinic   ON outpatient_appointments (clinic_id, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_outpatient_appointments_patient  ON outpatient_appointments (patient_id);
CREATE INDEX IF NOT EXISTS idx_outpatient_appointments_status   ON outpatient_appointments (tenant_id, status);

CREATE TABLE IF NOT EXISTS outpatient_waitlist (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id       UUID NOT NULL REFERENCES outpatient_clinics(id),
    patient_id      UUID NOT NULL,
    patient_nhi     TEXT NOT NULL DEFAULT '',
    priority        TEXT NOT NULL DEFAULT 'routine',
    reason          TEXT NOT NULL,
    referral_id     UUID,
    added_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    target_date     TIMESTAMPTZ,
    appointment_id  UUID,
    tenant_id       UUID NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_outpatient_waitlist_clinic    ON outpatient_waitlist (clinic_id, priority) WHERE appointment_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_outpatient_waitlist_patient   ON outpatient_waitlist (patient_id);

-- Hospital in the Home (HITH) episodes and visits
CREATE TABLE IF NOT EXISTS hith_episodes (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id            UUID NOT NULL,
    patient_nhi           TEXT NOT NULL DEFAULT '',
    linked_admission_id   UUID,
    lead_clinician_hpi    TEXT NOT NULL,
    status                TEXT NOT NULL DEFAULT 'active',
    diagnosis             TEXT NOT NULL,
    care_goals            TEXT[] NOT NULL DEFAULT '{}',
    daily_visit_frequency TEXT NOT NULL DEFAULT 'once',
    home_address          TEXT NOT NULL,
    emergency_contact     TEXT,
    patient_consented     BOOLEAN NOT NULL DEFAULT false,
    tenant_id             UUID NOT NULL,
    start_date            TIMESTAMPTZ NOT NULL DEFAULT now(),
    expected_end_date     TIMESTAMPTZ,
    actual_end_date       TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hith_episodes_tenant_status ON hith_episodes (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_hith_episodes_patient       ON hith_episodes (patient_id);

CREATE TABLE IF NOT EXISTS hith_visits (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    episode_id       UUID NOT NULL REFERENCES hith_episodes(id),
    clinician_hpi    TEXT NOT NULL,
    visit_type       TEXT NOT NULL DEFAULT 'nursing',
    vitals           JSONB NOT NULL DEFAULT '{}',
    clinical_notes   TEXT,
    escalated        BOOLEAN NOT NULL DEFAULT false,
    escalation_note  TEXT,
    next_visit_date  TIMESTAMPTZ,
    tenant_id        UUID NOT NULL,
    visited_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hith_visits_episode ON hith_visits (episode_id, visited_at DESC);
