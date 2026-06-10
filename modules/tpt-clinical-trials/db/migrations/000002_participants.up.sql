-- 000002_participants.up.sql
-- Participant screening, enrolment, randomisation, consent, and status tracking.
-- All NHI values are deterministically encrypted for indexing (HIPC Rule 12).
-- Consent records are immutable append-only entries to support audit requirements
-- under the Health and Disability Commissioner Act 1994.

CREATE TABLE IF NOT EXISTS ct_participants (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    protocol_id         UUID        NOT NULL REFERENCES ct_protocols (id) ON DELETE RESTRICT,
    arm_id              UUID        REFERENCES ct_protocol_arms (id) ON DELETE RESTRICT,
    tenant_id           UUID        NOT NULL,
    -- participant_nhi is the deterministically encrypted NHI for indexing (HIPC Rule 5 / 12).
    participant_nhi     TEXT        NOT NULL DEFAULT '',
    -- fhir_patient is the AES-256-GCM encrypted FHIR Patient JSON.
    fhir_patient        BYTEA,
    -- subject_id is the trial-internal participant identifier (e.g. SITE-001-001).
    subject_id          TEXT        NOT NULL DEFAULT '',
    status              TEXT        NOT NULL DEFAULT 'screened',  -- ParticipantStatus
    -- randomisation_code is the blinded allocation code (NULL until randomised).
    randomisation_code  TEXT,
    randomisation_method TEXT       NOT NULL DEFAULT '',
    screened_at         TIMESTAMPTZ,
    enrolled_at         TIMESTAMPTZ,
    randomised_at       TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    withdrawn_at        TIMESTAMPTZ,
    withdrawal_reason   TEXT        NOT NULL DEFAULT '',
    -- withdrawal_notes is encrypted to protect sensitive clinical context.
    withdrawal_notes    BYTEA,
    referring_hpi       TEXT        NOT NULL DEFAULT '',
    site_investigator_hpi TEXT      NOT NULL DEFAULT '',
    notes               TEXT        NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_participants_protocol_status
    ON ct_participants (protocol_id, status);

CREATE INDEX IF NOT EXISTS idx_ct_participants_nhi
    ON ct_participants (participant_nhi);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ct_participants_subject_id
    ON ct_participants (protocol_id, subject_id)
    WHERE subject_id <> '';

-- ct_screen_log records the outcome of screening eligibility assessments.
-- Kept separate from ct_participants so screen failures still appear in the screening log.
CREATE TABLE IF NOT EXISTS ct_screen_log (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    protocol_id         UUID        NOT NULL REFERENCES ct_protocols (id) ON DELETE RESTRICT,
    participant_id      UUID        REFERENCES ct_participants (id) ON DELETE SET NULL,
    tenant_id           UUID        NOT NULL,
    screened_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    screener_hpi        TEXT        NOT NULL DEFAULT '',
    eligible            BOOLEAN     NOT NULL DEFAULT false,
    screen_fail_reason  TEXT        NOT NULL DEFAULT '',
    -- criteria_results is a JSONB array of { criterion_id, met: bool } per eligibility item.
    criteria_results    JSONB       NOT NULL DEFAULT '[]',
    -- notes is AES-256-GCM encrypted to protect sensitive screening details.
    notes               BYTEA,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_screen_log_protocol
    ON ct_screen_log (protocol_id, screened_at DESC);

-- ct_consent_records is an append-only table capturing each consent event.
-- Re-consent after protocol amendments creates a new row; no updates are permitted.
CREATE TABLE IF NOT EXISTS ct_consent_records (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    participant_id      UUID        NOT NULL REFERENCES ct_participants (id) ON DELETE RESTRICT,
    tenant_id           UUID        NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'obtained',  -- ConsentStatus
    consented_at        TIMESTAMPTZ,
    withdrawn_at        TIMESTAMPTZ,
    -- amendment_id links re-consent to the specific protocol amendment (NULL for initial consent).
    amendment_id        UUID        REFERENCES ct_protocol_amendments (id) ON DELETE RESTRICT,
    obtained_by_hpi     TEXT        NOT NULL DEFAULT '',
    -- consent_form_ref is encrypted storage reference to the signed consent document.
    consent_form_ref    BYTEA,
    -- patient_signature_ref is encrypted reference to the digital or scanned signature.
    patient_signature_ref BYTEA,
    version             TEXT        NOT NULL DEFAULT '',  -- consent form version number
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ct_consent_records_participant
    ON ct_consent_records (participant_id, created_at DESC);
