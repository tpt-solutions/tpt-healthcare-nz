-- Cath lab: procedure booking, documentation, and post-procedure care.

CREATE TABLE IF NOT EXISTS cath_procedures (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi                 TEXT        NOT NULL DEFAULT '',
    operator_clinician_hpi      TEXT        NOT NULL DEFAULT '',
    procedure_type              TEXT        NOT NULL DEFAULT 'coronary-angiogram',
    -- coronary-angiogram | PCI | RHC | TAVI | pericardiocentesis | electrophysiology | other
    status                      TEXT        NOT NULL DEFAULT 'booked',
    -- booked | in-progress | completed | cancelled
    indication                  TEXT        NOT NULL DEFAULT '',
    access_site                 TEXT        NOT NULL DEFAULT 'radial-arterial',
    anaesthesia_type            TEXT        NOT NULL DEFAULT 'local',
    contrast_volume_ml          NUMERIC(5,1),
    radiation_dose_gy           NUMERIC(5,3),
    fluoroscopy_time_minutes    NUMERIC(5,1),
    lesions_treated             TEXT,
    stents_placed               TEXT,
    timi_flow_post              SMALLINT,
    complications               TEXT        NOT NULL DEFAULT 'none',
    notes                       TEXT,
    tenant_id                   UUID        NOT NULL,
    scheduled_at                TIMESTAMPTZ,
    started_at                  TIMESTAMPTZ,
    completed_at                TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cath_procedures_tenant_status ON cath_procedures (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_cath_procedures_patient       ON cath_procedures (patient_nhi, created_at DESC);

CREATE TABLE IF NOT EXISTS cath_post_care (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    procedure_id                UUID        NOT NULL REFERENCES cath_procedures (id),
    nurse_hpi                   TEXT        NOT NULL DEFAULT '',
    haematoma                   TEXT        NOT NULL DEFAULT 'none',
    neurovascular_status        TEXT        NOT NULL DEFAULT 'normal',
    systolic_bp                 SMALLINT,
    diastolic_bp                SMALLINT,
    heart_rate_bpm              SMALLINT,
    sp_o2_percent               SMALLINT,
    ecg_changes                 BOOLEAN     NOT NULL DEFAULT false,
    anticoagulation_reversed    BOOLEAN     NOT NULL DEFAULT false,
    sheath_removed              BOOLEAN     NOT NULL DEFAULT false,
    sheath_removed_at           TIMESTAMPTZ,
    notes                       TEXT,
    tenant_id                   UUID        NOT NULL,
    assessed_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cath_post_care_procedure ON cath_post_care (procedure_id, assessed_at DESC);
