-- Recall notifications for active screening enrolments.
-- Tracks the full lifecycle from scheduled to completed or lapsed.
CREATE TABLE IF NOT EXISTS screening_recalls (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    enrolment_id    UUID        NOT NULL REFERENCES screening_enrolments(id),
    patient_nhi     TEXT        NOT NULL DEFAULT '',
    recall_type     TEXT        NOT NULL DEFAULT 'routine',
    contact_method  TEXT        NOT NULL DEFAULT 'letter',
    due_date        DATE        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending',
    notes           TEXT,
    tenant_id       UUID        NOT NULL,
    sent_at         TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_screening_recalls_enrolment    ON screening_recalls (enrolment_id);
CREATE INDEX IF NOT EXISTS idx_screening_recalls_tenant_status ON screening_recalls (tenant_id, status);
-- Partial index for overdue recall lookup (pending/sent only).
CREATE INDEX IF NOT EXISTS idx_screening_recalls_tenant_due   ON screening_recalls (tenant_id, due_date) WHERE status IN ('pending', 'sent');
