-- 006_queue.sql
-- Extend appointments with two-level push reminder tracking.
ALTER TABLE appointments
    ADD COLUMN IF NOT EXISTS reminder_24h_sent BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS reminder_1h_sent  BOOLEAN NOT NULL DEFAULT false;


-- Virtual waiting list, appointment push notifications, and FHIR subscription persistence.

-- FHIR subscriptions: persisted to survive restarts (replaces in-memory store in interop/api/subscription.go)
CREATE TABLE IF NOT EXISTS fhir_subscriptions (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL REFERENCES tenants(id),
    topic           TEXT        NOT NULL,
    channel_type    TEXT        NOT NULL CHECK (channel_type IN ('rest-hook', 'websocket', 'email')),
    endpoint        TEXT,
    headers         JSONB       NOT NULL DEFAULT '[]',
    criteria        TEXT,
    heartbeat_period INT,
    status          TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'off', 'error')),
    reason          TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_fhir_subscriptions_tenant ON fhir_subscriptions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_fhir_subscriptions_topic  ON fhir_subscriptions(topic);

-- VAPID Web Push subscriptions (one per browser per patient)
CREATE TABLE IF NOT EXISTS push_subscriptions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id  UUID        NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    tenant_id   UUID        NOT NULL REFERENCES tenants(id),
    endpoint    TEXT        NOT NULL,
    p256dh      TEXT        NOT NULL,
    auth        TEXT        NOT NULL,
    user_agent  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (patient_id, endpoint)
);

CREATE INDEX IF NOT EXISTS idx_push_subscriptions_patient ON push_subscriptions(patient_id);

-- Daily queue per clinic (one row per practice per day)
CREATE TABLE IF NOT EXISTS queues (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(id),
    name        TEXT        NOT NULL,
    date        DATE        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'open'
                CHECK (status IN ('open', 'paused', 'closed')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, date, name)
);

CREATE INDEX IF NOT EXISTS idx_queues_tenant_date ON queues(tenant_id, date);

-- Individual queue entries (one per patient check-in)
CREATE TABLE IF NOT EXISTS queue_entries (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    queue_id        UUID        NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    patient_id      UUID        REFERENCES patients(id),
    patient_nhi     TEXT        NOT NULL,   -- encrypted index (same pattern as patients.nhi_index)
    appointment_id  UUID        REFERENCES appointments(id),
    position        INT         NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'waiting'
                    CHECK (status IN ('waiting', 'called', 'in_progress', 'done', 'skipped', 'left')),
    checked_in_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    called_at       TIMESTAMPTZ,
    done_at         TIMESTAMPTZ,
    wait_minutes    INT,                    -- computed on status → done
    check_in_method TEXT        CHECK (check_in_method IN ('portal', 'kiosk', 'staff', 'emergency')),
    room_hint       TEXT,                   -- e.g. "Room 2", "Bay 3" — set by staff on call-next
    notes           TEXT
);

CREATE INDEX IF NOT EXISTS idx_queue_entries_queue    ON queue_entries(queue_id, position);
CREATE INDEX IF NOT EXISTS idx_queue_entries_patient  ON queue_entries(patient_id) WHERE patient_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_queue_entries_status   ON queue_entries(queue_id, status);

-- Ephemeral location — only exists while entry status is waiting or called.
-- Deleted in the same transaction that sets a terminal status (HIPC Rules 6 + 10).
CREATE TABLE IF NOT EXISTS queue_entry_locations (
    entry_id    UUID            PRIMARY KEY REFERENCES queue_entries(id) ON DELETE CASCADE,
    lat         DOUBLE PRECISION NOT NULL,
    lng         DOUBLE PRECISION NOT NULL,
    accuracy_m  REAL,
    updated_at  TIMESTAMPTZ     NOT NULL DEFAULT now()
);
