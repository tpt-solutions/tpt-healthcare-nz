-- Surgical theatre bookings (FHIR R5 Appointment + Schedule)
CREATE TABLE IF NOT EXISTS theatre_bookings (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id            UUID NOT NULL,
    patient_nhi           TEXT NOT NULL DEFAULT '',
    admission_id          UUID,
    surgeon_hpi           TEXT NOT NULL,
    assistant_hpi         TEXT,
    anaesthetist_hpi      TEXT,
    scrub_nurse_hpi       TEXT,
    theatre_id            UUID NOT NULL,
    status                TEXT NOT NULL DEFAULT 'planned',
    procedure_name        TEXT NOT NULL,
    procedure_codes       TEXT[] NOT NULL DEFAULT '{}',
    anaesthesia_type      TEXT NOT NULL DEFAULT 'general',
    planned_duration_mins INT  NOT NULL DEFAULT 60,
    actual_duration_mins  INT,
    scheduled_at          TIMESTAMPTZ NOT NULL,
    started_at            TIMESTAMPTZ,
    completed_at          TIMESTAMPTZ,
    operative_notes       TEXT,
    post_op_notes         TEXT,
    complications         TEXT[] NOT NULL DEFAULT '{}',
    tenant_id             UUID NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_theatre_bookings_tenant_status    ON theatre_bookings (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_theatre_bookings_scheduled_at     ON theatre_bookings (tenant_id, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_theatre_bookings_theatre_date     ON theatre_bookings (theatre_id, scheduled_at::date) WHERE status != 'cancelled';

-- Pre-admission assessments (PAC clinic)
CREATE TABLE IF NOT EXISTS preadmission_assessments (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id             UUID NOT NULL,
    patient_nhi            TEXT NOT NULL DEFAULT '',
    theatre_booking_id     UUID,
    assessor_hpi           TEXT NOT NULL,
    status                 TEXT NOT NULL DEFAULT 'scheduled',
    planned_procedure      TEXT NOT NULL,
    planned_anaesthesia    TEXT NOT NULL DEFAULT 'general',
    asa_grade              TEXT,
    allergies_reviewed     BOOLEAN NOT NULL DEFAULT false,
    medications_reviewed   BOOLEAN NOT NULL DEFAULT false,
    blood_group_confirmed  BOOLEAN NOT NULL DEFAULT false,
    consent_obtained       BOOLEAN NOT NULL DEFAULT false,
    fasting_instructions   TEXT,
    special_instructions   TEXT,
    clinical_notes         TEXT,
    deferral_reason        TEXT,
    investigations_required TEXT[] NOT NULL DEFAULT '{}',
    tenant_id              UUID NOT NULL,
    assessed_at            TIMESTAMPTZ,
    approved_at            TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_preadmission_assessments_tenant  ON preadmission_assessments (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_preadmission_assessments_booking ON preadmission_assessments (theatre_booking_id) WHERE theatre_booking_id IS NOT NULL;
