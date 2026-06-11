-- Implantable cardiac device registry and follow-up interrogations.
-- Covers pacemakers (VVI, DDD, CRT-P), ICDs (single, dual, CRT-D), ILR, and LVAD.

CREATE TABLE IF NOT EXISTS implantable_devices (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi                 TEXT        NOT NULL DEFAULT '',
    implanting_clinician_hpi    TEXT        NOT NULL DEFAULT '',
    follow_up_clinician_hpi     TEXT        NOT NULL DEFAULT '',
    device_type                 TEXT        NOT NULL DEFAULT 'pacemaker-DDD',
    -- pacemaker-VVI | pacemaker-DDD | pacemaker-CRT-P
    -- ICD-single | ICD-dual | ICD-CRT-D | ILR | LVAD
    device_brand                TEXT        NOT NULL DEFAULT '',
    model_name                  TEXT        NOT NULL DEFAULT '',
    serial_number               TEXT        NOT NULL DEFAULT '',
    status                      TEXT        NOT NULL DEFAULT 'active',
    -- active | battery-replacement-due | explanted | lost-to-follow-up
    indication                  TEXT        NOT NULL DEFAULT '',
    -- RV lead
    rv_lead_impedance_ohm       INT,
    rv_pacing_threshold_v       NUMERIC(4,2),
    rv_sensed_amplitude_mv      NUMERIC(4,2),
    -- LV lead (CRT devices only)
    lv_lead_impedance_ohm       INT,
    lv_pacing_threshold_v       NUMERIC(4,2),
    lv_sensed_amplitude_mv      NUMERIC(4,2),
    -- RA lead (dual-chamber and CRT devices)
    ra_lead_impedance_ohm       INT,
    ra_pacing_threshold_v       NUMERIC(4,2),
    ra_sensed_amplitude_mv      NUMERIC(4,2),
    battery_voltage             NUMERIC(4,2),
    estimated_longevity_months  SMALLINT,
    notes                       TEXT,
    tenant_id                   UUID        NOT NULL,
    implanted_at                TIMESTAMPTZ,
    next_follow_up_at           TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_implantable_devices_tenant_status ON implantable_devices (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_implantable_devices_patient       ON implantable_devices (patient_nhi);

-- Device interrogations: routine clinic checks, remote monitoring, and symptomatic reviews.
CREATE TABLE IF NOT EXISTS device_interrogations (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id                   UUID        NOT NULL REFERENCES implantable_devices (id),
    interrogating_clinician_hpi TEXT        NOT NULL DEFAULT '',
    battery_status              TEXT        NOT NULL DEFAULT 'adequate',
    -- adequate | elective-replacement | urgent-replacement
    battery_voltage             NUMERIC(4,2),
    estimated_longevity_months  SMALLINT,
    percent_v_paced             NUMERIC(5,2),
    percent_a_paced             NUMERIC(5,2),
    af_burden_percent           NUMERIC(5,2),
    vt_episodes                 INT,
    vf_episodes                 INT,
    shock_therapy_delivered     BOOLEAN     NOT NULL DEFAULT false,
    shock_count                 SMALLINT,
    atp_episodes                INT,
    programme_changes           TEXT        NOT NULL DEFAULT '',
    clinical_notes              TEXT        NOT NULL DEFAULT '',
    tenant_id                   UUID        NOT NULL,
    interrogated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    next_interrogation_at       TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_device_interrogations_device ON device_interrogations (device_id, interrogated_at DESC);
