-- Screening programme enrolments for NZ national screening programmes.
-- Covers: cervical (NCSP), bowel, breast (BreastScreen Aotearoa),
-- newborn metabolic, newborn hearing, antenatal HIV, antenatal syphilis.
CREATE TABLE IF NOT EXISTS screening_enrolments (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi        TEXT        NOT NULL DEFAULT '',
    programme_type     TEXT        NOT NULL,
    status             TEXT        NOT NULL DEFAULT 'eligible',
    enrolled_by_hpi    TEXT        NOT NULL DEFAULT '',
    registry_reference TEXT,
    last_screen_date   DATE,
    next_due_date      DATE,
    notes              TEXT,
    tenant_id          UUID        NOT NULL,
    enrolled_at        TIMESTAMPTZ,
    suspended_at       TIMESTAMPTZ,
    withdrawn_at       TIMESTAMPTZ,
    completed_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_screening_enrolments_tenant_type   ON screening_enrolments (tenant_id, programme_type);
CREATE INDEX IF NOT EXISTS idx_screening_enrolments_tenant_status ON screening_enrolments (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_screening_enrolments_next_due      ON screening_enrolments (tenant_id, next_due_date) WHERE status = 'active';
