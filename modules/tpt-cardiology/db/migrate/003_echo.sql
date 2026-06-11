-- Echocardiography studies: TTE, TOE, stress echo, and contrast echo.
-- Captures LV function, valve findings, haemodynamic parameters, and pericardial assessment.

CREATE TABLE IF NOT EXISTS echo_studies (
    id                          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_nhi                 TEXT        NOT NULL DEFAULT '',
    ordering_clinician_hpi      TEXT        NOT NULL DEFAULT '',
    reporting_clinician_hpi     TEXT        NOT NULL DEFAULT '',
    study_type                  TEXT        NOT NULL DEFAULT 'TTE',
    -- TTE | TOE | stress | contrast
    status                      TEXT        NOT NULL DEFAULT 'ordered',
    -- ordered | performed | reported | cancelled
    indication                  TEXT        NOT NULL DEFAULT '',

    -- LV systolic function
    lvef_percent                SMALLINT,
    lv_edv_ml                   NUMERIC(5,1),
    lv_esv_ml                   NUMERIC(5,1),
    lv_diastolic_diameter_mm    NUMERIC(4,1),
    lv_systolic_diameter_mm     NUMERIC(4,1),
    lv_posterior_wall_mm        NUMERIC(4,1),
    interventricular_septum_mm  NUMERIC(4,1),
    lv_mass_g                   NUMERIC(5,1),
    wall_motion_abnormality     BOOLEAN     NOT NULL DEFAULT false,
    wall_motion_segments        TEXT,       -- free-text or JSON list of affected segments
    diastolic_function          TEXT        NOT NULL DEFAULT 'normal',
    -- normal | grade1 | grade2 | grade3 | indeterminate

    -- Aortic valve
    aortic_valve_finding        TEXT        NOT NULL DEFAULT 'normal',
    aortic_gradient_mmhg        NUMERIC(5,1),
    aortic_valve_area_cm2       NUMERIC(4,2),

    -- Mitral valve
    mitral_valve_finding        TEXT        NOT NULL DEFAULT 'normal',
    mitral_e_velocity           NUMERIC(4,2),
    mitral_a_velocity           NUMERIC(4,2),
    e_a_ratio                   NUMERIC(4,2),
    mitral_e_prime              NUMERIC(4,2),
    e_e_prime_ratio             NUMERIC(4,1),

    -- Right heart
    tricuspid_valve_finding     TEXT        NOT NULL DEFAULT 'normal',
    tv_regurg_velocity          NUMERIC(4,2),
    rvsp_mmhg                   NUMERIC(5,1),
    pulmonary_valve_finding     TEXT        NOT NULL DEFAULT 'normal',
    rv_function                 TEXT        NOT NULL DEFAULT 'normal',
    -- normal | mildly-reduced | moderately-reduced | severely-reduced

    -- Pericardium and chambers
    pericardial_effusion        TEXT        NOT NULL DEFAULT 'none',
    -- none | trivial | small | moderate | large
    ivc_diameter_mm             NUMERIC(4,1),
    ivc_collapsibility          TEXT        NOT NULL DEFAULT '',
    la_volume_ml                NUMERIC(5,1),
    ra_area_cm2                 NUMERIC(4,1),

    interpretation              TEXT        NOT NULL DEFAULT '',
    notes                       TEXT,
    tenant_id                   UUID        NOT NULL,
    ordered_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    performed_at                TIMESTAMPTZ,
    reported_at                 TIMESTAMPTZ,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_echo_studies_tenant_status ON echo_studies (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_echo_studies_patient       ON echo_studies (patient_nhi, ordered_at DESC);
