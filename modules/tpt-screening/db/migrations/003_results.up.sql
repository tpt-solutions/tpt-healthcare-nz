-- Screening results recorded against an enrolment episode.
-- result_category: normal | abnormal | inadequate | unsatisfactory | pending
CREATE TABLE IF NOT EXISTS screening_results (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    enrolment_id            UUID        NOT NULL REFERENCES screening_enrolments(id),
    patient_nhi             TEXT        NOT NULL DEFAULT '',
    programme_type          TEXT        NOT NULL,
    screen_date             DATE        NOT NULL,
    result_category         TEXT        NOT NULL DEFAULT 'pending',
    result_detail           TEXT        NOT NULL DEFAULT '',
    reported_by_hpi         TEXT        NOT NULL DEFAULT '',
    external_reference_id   TEXT,
    follow_up_required      BOOLEAN     NOT NULL DEFAULT FALSE,
    follow_up_action        TEXT,
    follow_up_due_date      DATE,
    follow_up_completed_at  TIMESTAMPTZ,
    next_due_date           DATE,
    notes                   TEXT,
    tenant_id               UUID        NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_screening_results_enrolment      ON screening_results (enrolment_id);
CREATE INDEX IF NOT EXISTS idx_screening_results_tenant_type    ON screening_results (tenant_id, programme_type);
CREATE INDEX IF NOT EXISTS idx_screening_results_tenant_category ON screening_results (tenant_id, result_category);
-- Partial index for outstanding follow-up actions.
CREATE INDEX IF NOT EXISTS idx_screening_results_followup       ON screening_results (tenant_id, follow_up_due_date)
    WHERE follow_up_required = TRUE AND follow_up_completed_at IS NULL;
