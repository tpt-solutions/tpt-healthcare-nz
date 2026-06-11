-- Cardiology outpatient appointments: new referrals, review clinics, and follow-up.

CREATE TABLE IF NOT EXISTS cardiology_appointments (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi         TEXT        NOT NULL DEFAULT '',
    clinician_hpi       TEXT        NOT NULL DEFAULT '',
    appointment_type    TEXT        NOT NULL DEFAULT 'new',
    -- new | review | follow-up | urgent
    status              TEXT        NOT NULL DEFAULT 'scheduled',
    -- scheduled | arrived | in-progress | completed | did-not-attend | cancelled
    referral_source     TEXT        NOT NULL DEFAULT '',
    referral_date       DATE,
    indication          TEXT        NOT NULL DEFAULT '',
    primary_diagnosis   TEXT        NOT NULL DEFAULT '',
    management_plan     TEXT        NOT NULL DEFAULT '',
    follow_up_weeks     INT,
    notes               TEXT,
    tenant_id           UUID        NOT NULL,
    scheduled_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cardiology_appts_tenant_status ON cardiology_appointments (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_cardiology_appts_patient      ON cardiology_appointments (patient_nhi, scheduled_at DESC);
