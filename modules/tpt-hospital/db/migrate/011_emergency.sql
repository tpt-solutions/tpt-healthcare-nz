-- Emergency & Disaster Management (CIMS-based)
-- Covers: incident command, MCI triage, surge capacity, CBRN decontamination.

CREATE TYPE emergency_incident_type AS ENUM (
    'mci', 'cbrn', 'fire', 'flood', 'cyber', 'pandemic', 'other'
);

CREATE TYPE emergency_incident_status AS ENUM (
    'declared', 'activated', 'escalated', 'stand_down', 'closed'
);

CREATE TYPE cims_role AS ENUM (
    'incident_commander', 'deputy_ic', 'safety_officer',
    'operations_chief', 'logistics_chief', 'planning_chief', 'finance_chief',
    'medical_director', 'liaison_officer', 'public_info_officer', 'zone_leader'
);

CREATE TYPE mci_triage_category AS ENUM (
    'immediate', 'delayed', 'minor', 'expectant', 'deceased'
);

CREATE TYPE mci_triage_method AS ENUM ('start', 'jumpstart');

CREATE TYPE resource_request_status AS ENUM ('requested', 'fulfilled', 'cancelled');

-- ── Incidents ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS emergency_incidents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    title           TEXT NOT NULL,
    type            emergency_incident_type NOT NULL,
    status          emergency_incident_status NOT NULL DEFAULT 'declared',
    description     TEXT,
    location        TEXT,
    cbrn_agent      TEXT,                           -- only populated for type='cbrn'
    surge_level     INT NOT NULL DEFAULT 0,         -- 0=normal 1=expanded 2=crisis 3=catastrophic
    declared_by     TEXT NOT NULL,                  -- auth principal ID
    ic_principal_id TEXT,                           -- assigned IC (may differ from declarer)
    declared_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at    TIMESTAMPTZ,
    escalated_at    TIMESTAMPTZ,
    stand_down_at   TIMESTAMPTZ,
    closed_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS emergency_incidents_tenant_status_idx
    ON emergency_incidents (tenant_id, status);
CREATE INDEX IF NOT EXISTS emergency_incidents_tenant_declared_idx
    ON emergency_incidents (tenant_id, declared_at DESC);

-- ── CIMS command structure ────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS incident_command_assignments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id  UUID NOT NULL REFERENCES emergency_incidents(id),
    tenant_id    UUID NOT NULL,
    cims_role    cims_role NOT NULL,
    principal_id TEXT NOT NULL,
    assigned_by  TEXT NOT NULL,
    assigned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    relieved_at  TIMESTAMPTZ
);

-- Partial unique index: only one active holder per CIMS role per incident.
CREATE UNIQUE INDEX IF NOT EXISTS incident_command_active_role_idx
    ON incident_command_assignments (incident_id, cims_role)
    WHERE relieved_at IS NULL;

CREATE INDEX IF NOT EXISTS incident_command_incident_idx
    ON incident_command_assignments (incident_id);

-- ── Append-only incident command log ─────────────────────────────────────────

CREATE TABLE IF NOT EXISTS incident_log_entries (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES emergency_incidents(id),
    tenant_id   UUID NOT NULL,
    author_id   TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT 'general', -- decision/resource/clinical/comms/safety/general
    message     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
    -- no updated_at: append-only
);

CREATE INDEX IF NOT EXISTS incident_log_entries_incident_idx
    ON incident_log_entries (incident_id, created_at ASC);

-- ── MCI patient tagging and triage ───────────────────────────────────────────

CREATE TABLE IF NOT EXISTS mci_patients (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id          UUID NOT NULL REFERENCES emergency_incidents(id),
    tenant_id            UUID NOT NULL,
    tag_number           INT NOT NULL CHECK (tag_number BETWEEN 1 AND 999),
    nhi_encrypted        BYTEA,
    nhi_masked           TEXT,        -- last 3 chars for display (e.g. "***7AF")
    triage_category      mci_triage_category NOT NULL,
    triage_method        mci_triage_method NOT NULL DEFAULT 'start',
    is_paediatric        BOOLEAN NOT NULL DEFAULT false,
    age_years_approx     INT,
    sex                  TEXT,
    presenting_complaint TEXT,
    allocated_zone       TEXT,        -- e.g. "hot", "warm", "cold", "treatment"
    last_reassessed_at   TIMESTAMPTZ,
    reassessed_by        TEXT,
    notes                TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (incident_id, tag_number)
);

CREATE INDEX IF NOT EXISTS mci_patients_incident_category_idx
    ON mci_patients (incident_id, triage_category);
CREATE INDEX IF NOT EXISTS mci_patients_incident_tag_idx
    ON mci_patients (incident_id, tag_number);

-- ── Surge capacity snapshots ──────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS surge_capacity_snapshots (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id          UUID NOT NULL REFERENCES emergency_incidents(id),
    tenant_id            UUID NOT NULL,
    surge_level          INT NOT NULL,
    total_beds           INT NOT NULL DEFAULT 0,
    occupied_beds        INT NOT NULL DEFAULT 0,
    surge_beds_activated INT NOT NULL DEFAULT 0,
    icu_total            INT NOT NULL DEFAULT 0,
    icu_occupied         INT NOT NULL DEFAULT 0,
    ed_waiting           INT NOT NULL DEFAULT 0,
    recorded_by          TEXT NOT NULL,
    recorded_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS surge_snapshots_incident_idx
    ON surge_capacity_snapshots (incident_id, recorded_at DESC);

-- ── CBRN decontamination log ─────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS cbrn_decon_log (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mci_patient_id          UUID NOT NULL UNIQUE REFERENCES mci_patients(id),
    incident_id             UUID NOT NULL,
    tenant_id               UUID NOT NULL,
    contamination_suspected BOOLEAN NOT NULL DEFAULT true,
    contaminant_type        TEXT,    -- chemical/biological/radiological/nuclear/unknown
    decon_started_at        TIMESTAMPTZ,
    decon_complete_at       TIMESTAMPTZ,
    decon_method            TEXT,    -- dry/wet/gross-decon
    decon_by                TEXT,
    ppe_level_used          TEXT,    -- level-a/level-b/level-c/level-d
    cleared_for_treatment   BOOLEAN NOT NULL DEFAULT false,
    notes                   TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS cbrn_decon_incident_idx
    ON cbrn_decon_log (incident_id);
CREATE INDEX IF NOT EXISTS cbrn_decon_patient_idx
    ON cbrn_decon_log (mci_patient_id);

-- ── Resource requests ────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS resource_requests (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id  UUID NOT NULL REFERENCES emergency_incidents(id),
    tenant_id    UUID NOT NULL,
    category     TEXT NOT NULL,    -- staff/equipment/blood/medication/transport/other
    description  TEXT NOT NULL,
    quantity     INT NOT NULL DEFAULT 1,
    status       resource_request_status NOT NULL DEFAULT 'requested',
    priority     TEXT NOT NULL DEFAULT 'normal', -- urgent/high/normal/low
    requested_by TEXT NOT NULL,
    fulfilled_by TEXT,
    notes        TEXT,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    fulfilled_at TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS resource_requests_incident_status_idx
    ON resource_requests (incident_id, status);
