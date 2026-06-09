-- Cardiac rehabilitation programmes (Phase I–IV) and individual session records.
-- Aligned with the NZ Cardiac Rehabilitation Guidelines.

CREATE TABLE IF NOT EXISTS cardiac_rehab_programmes (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    indication              TEXT        NOT NULL DEFAULT '',
    -- post-MI | post-CABG | post-PCI | heart-failure | stable-angina | post-TAVI | other
    phase                   TEXT        NOT NULL DEFAULT '2',
    -- 1 | 2 | 3 | 4
    status                  TEXT        NOT NULL DEFAULT 'referred',
    -- referred | enrolled | active | completed | withdrawn
    risk_level              TEXT        NOT NULL DEFAULT 'moderate',
    -- low | moderate | high
    target_hr_min           SMALLINT,
    target_hr_max           SMALLINT,
    baseline_mets           NUMERIC(4,1),
    goal_mets               NUMERIC(4,1),
    sessions_planned        SMALLINT,
    sessions_completed      SMALLINT    NOT NULL DEFAULT 0,
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    referred_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at              TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cardiac_rehab_tenant_status ON cardiac_rehab_programmes (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_cardiac_rehab_patient       ON cardiac_rehab_programmes (patient_nhi);

CREATE TABLE IF NOT EXISTS cardiac_rehab_sessions (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    programme_id            UUID        NOT NULL REFERENCES cardiac_rehab_programmes (id),
    clinician_hpi           TEXT        NOT NULL DEFAULT '',
    session_type            TEXT        NOT NULL DEFAULT 'group',
    -- group | individual | telehealth | home
    session_number          SMALLINT,
    peak_hr_bpm             SMALLINT,
    achieved_mets           NUMERIC(4,1),
    borg_rpe                SMALLINT,   -- Borg RPE 6–20
    pre_systolic_bp         SMALLINT,
    pre_diastolic_bp        SMALLINT,
    post_systolic_bp        SMALLINT,
    post_diastolic_bp       SMALLINT,
    symptoms_during         TEXT        NOT NULL DEFAULT 'none',
    -- none | chest-pain | dyspnoea | palpitations | dizziness | syncope | other
    ecg_changes_noted       BOOLEAN     NOT NULL DEFAULT false,
    session_notes           TEXT,
    duration_minutes        SMALLINT,
    tenant_id               UUID        NOT NULL,
    session_date            TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cardiac_rehab_sessions_programme ON cardiac_rehab_sessions (programme_id, session_date DESC);
