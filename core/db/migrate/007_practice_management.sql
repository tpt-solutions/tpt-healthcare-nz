-- Migration 007: Practice Management & Operations
-- Covers: departments, RBAC role_assignments, staff roster, room bookings,
-- leave management, inventory, purchase orders, cold chain, cost centres,
-- budgets, patient invoices, payments, and onboarding wizard state.
-- All tenant-scoped; uses gen_random_uuid() for ID generation.

-- ============================================================
-- RBAC: Departments and Role Assignments
-- ============================================================

CREATE TABLE IF NOT EXISTS departments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        TEXT NOT NULL,
    code        TEXT NOT NULL,
    parent_id   UUID REFERENCES departments(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,
    UNIQUE (tenant_id, code)
);

CREATE INDEX IF NOT EXISTS idx_departments_tenant ON departments (tenant_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS role_assignments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    principal_id  TEXT NOT NULL,
    role          TEXT NOT NULL,
    department_id UUID REFERENCES departments(id),
    granted_by    TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at    TIMESTAMPTZ,
    -- Unique active assignment per principal+role+dept (NULL dept = tenant-wide)
    UNIQUE (tenant_id, principal_id, role,
            COALESCE(department_id, '00000000-0000-0000-0000-000000000000'))
        DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX IF NOT EXISTS idx_role_assignments_principal
    ON role_assignments (tenant_id, principal_id)
    WHERE revoked_at IS NULL;

-- ============================================================
-- Staff Roster & Rooms
-- ============================================================

CREATE TABLE IF NOT EXISTS staff_shifts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    principal_id    TEXT NOT NULL,          -- auth.Principal.ID (JWT sub)
    department_id   UUID REFERENCES departments(id),
    shift_start     TIMESTAMPTZ NOT NULL,
    shift_end       TIMESTAMPTZ NOT NULL,
    shift_type      TEXT NOT NULL DEFAULT 'ordinary',  -- ordinary | on_call | overtime
    notes           TEXT,
    payroll_push_at TIMESTAMPTZ,            -- when the shift was pushed to payroll
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_staff_shifts_tenant_start
    ON staff_shifts (tenant_id, shift_start);

CREATE TABLE IF NOT EXISTS on_call_slots (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    principal_id  TEXT NOT NULL,
    department_id UUID REFERENCES departments(id),
    slot_start    TIMESTAMPTZ NOT NULL,
    slot_end      TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rooms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL,
    name        TEXT NOT NULL,
    location    TEXT,
    capacity    INT  NOT NULL DEFAULT 1,
    active      BOOL NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS room_bookings (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    room_id       UUID NOT NULL REFERENCES rooms(id),
    booked_by     TEXT NOT NULL,
    start_time    TIMESTAMPTZ NOT NULL,
    end_time      TIMESTAMPTZ NOT NULL,
    appointment_ref TEXT,                  -- optional link to appointment/encounter
    encounter_ref   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    EXCLUDE USING gist (
        room_id WITH =,
        tstzrange(start_time, end_time) WITH &&
    )  -- prevents double-booking
);

-- ============================================================
-- Leave Management
-- ============================================================

CREATE TABLE IF NOT EXISTS leave_requests (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id             UUID NOT NULL,
    principal_id          TEXT NOT NULL,
    leave_type            TEXT NOT NULL,  -- annual | sick | lieu | bereavement | parental | other
    start_date            DATE NOT NULL,
    end_date              DATE NOT NULL,
    status                TEXT NOT NULL DEFAULT 'pending',  -- pending | approved | declined | cancelled
    notes                 TEXT,
    approved_by           TEXT,
    approved_at           TIMESTAMPTZ,
    payroll_leave_id      TEXT,           -- ID returned by payroll provider
    payroll_submitted_at  TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_leave_requests_tenant_principal
    ON leave_requests (tenant_id, principal_id);

-- ============================================================
-- Inventory
-- ============================================================

CREATE TABLE IF NOT EXISTS stock_items (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID NOT NULL,
    sku                      TEXT NOT NULL,
    name                     TEXT NOT NULL,
    category                 TEXT NOT NULL,  -- vaccine | medication | consumable | equipment
    unit                     TEXT NOT NULL DEFAULT 'unit',
    quantity_on_hand         BIGINT NOT NULL DEFAULT 0,
    reorder_point            BIGINT NOT NULL DEFAULT 0,
    storage_temp_min_c       NUMERIC(5,2),
    storage_temp_max_c       NUMERIC(5,2),
    fhir_supply_delivery_ref TEXT,
    expiry_date              DATE,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, sku)
);

CREATE INDEX IF NOT EXISTS idx_stock_items_tenant ON stock_items (tenant_id);
CREATE INDEX IF NOT EXISTS idx_stock_items_expiry ON stock_items (tenant_id, expiry_date)
    WHERE expiry_date IS NOT NULL;

CREATE TABLE IF NOT EXISTS stock_movements (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    stock_item_id   UUID NOT NULL REFERENCES stock_items(id),
    type            TEXT NOT NULL,    -- receive | consume | adjust | transfer | discard_expired
    quantity_delta  BIGINT NOT NULL,
    performed_by    TEXT NOT NULL,
    notes           TEXT,
    encounter_ref   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_stock_movements_item ON stock_movements (stock_item_id, created_at DESC);

-- Trigger: keep quantity_on_hand in sync with movements
CREATE OR REPLACE FUNCTION update_stock_on_hand()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    UPDATE stock_items
    SET quantity_on_hand = quantity_on_hand + NEW.quantity_delta,
        updated_at = NOW()
    WHERE id = NEW.stock_item_id;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_stock_on_hand
AFTER INSERT ON stock_movements
FOR EACH ROW EXECUTE FUNCTION update_stock_on_hand();

CREATE TABLE IF NOT EXISTS purchase_orders (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    supplier_name TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'draft',  -- draft | sent | partially_received | received | cancelled
    ordered_at    TIMESTAMPTZ,
    expected_at   TIMESTAMPTZ,
    received_at   TIMESTAMPTZ,
    notes         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS purchase_order_lines (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_order_id UUID NOT NULL REFERENCES purchase_orders(id) ON DELETE CASCADE,
    stock_item_id     UUID NOT NULL REFERENCES stock_items(id),
    quantity_ordered  BIGINT NOT NULL,
    quantity_received BIGINT NOT NULL DEFAULT 0,
    unit_cost_cents   BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS cold_chain_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    stock_item_id UUID NOT NULL REFERENCES stock_items(id),
    temp_c        NUMERIC(6,3) NOT NULL,
    recorded_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    breach        BOOL NOT NULL DEFAULT false,
    alarm_sent    BOOL NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_cold_chain_breach
    ON cold_chain_logs (tenant_id, recorded_at DESC)
    WHERE breach = true;

-- ============================================================
-- Cost Centres & Budgets
-- ============================================================

CREATE TABLE IF NOT EXISTS cost_centres (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    department_id UUID REFERENCES departments(id),
    name          TEXT NOT NULL,
    code          TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, code)
);

CREATE TABLE IF NOT EXISTS budgets (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL,
    cost_centre_id UUID NOT NULL REFERENCES cost_centres(id),
    financial_year INT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cost_centre_id, financial_year)
);

CREATE TABLE IF NOT EXISTS budget_lines (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id     UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    month         INT NOT NULL CHECK (month BETWEEN 1 AND 12),
    category      TEXT NOT NULL,
    planned_cents BIGINT NOT NULL DEFAULT 0,
    actual_cents  BIGINT NOT NULL DEFAULT 0,
    external_ref  TEXT,   -- accounting provider account code for actual sync
    UNIQUE (budget_id, month, category)
);

-- ============================================================
-- Patient Invoices & Payments
-- ============================================================

CREATE TABLE IF NOT EXISTS patient_invoices (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id              UUID NOT NULL,
    patient_nhi            TEXT NOT NULL,
    encounter_ref          TEXT,
    funding_type           TEXT NOT NULL,   -- acc | pho | dhb | private | vac
    total_cents            BIGINT NOT NULL DEFAULT 0,
    subsidy_cents          BIGINT NOT NULL DEFAULT 0,
    patient_amount_cents   BIGINT NOT NULL DEFAULT 0,
    status                 TEXT NOT NULL DEFAULT 'draft',  -- draft | issued | overdue | paid | cancelled
    issued_at              TIMESTAMPTZ,
    due_date               DATE,
    paid_at                TIMESTAMPTZ,
    payment_plan           BOOL NOT NULL DEFAULT false,
    accounting_external_id TEXT,           -- ID in Xero/QBO/FreshBooks
    accounting_synced_at   TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_patient_invoices_tenant_status
    ON patient_invoices (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_patient_invoices_nhi
    ON patient_invoices (patient_nhi);

CREATE TABLE IF NOT EXISTS patient_payments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    invoice_id    UUID NOT NULL REFERENCES patient_invoices(id),
    amount_cents  BIGINT NOT NULL,
    paid_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payment_ref   TEXT,
    provider      TEXT,                     -- windcave | stripe | paymark | eway | poli | humm
    external_id   TEXT,                     -- payment gateway transaction ID
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- Accounting & Payroll Sync Logs
-- ============================================================

CREATE TABLE IF NOT EXISTS accounting_sync_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    provider      TEXT NOT NULL,
    entity_type   TEXT NOT NULL,   -- invoice | payment | contact | journal
    entity_id     UUID NOT NULL,
    external_id   TEXT,
    status        TEXT NOT NULL,   -- success | failed
    error_text    TEXT,
    synced_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payroll_sync_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,
    provider      TEXT NOT NULL,
    entity_type   TEXT NOT NULL,   -- employee | timesheet | leave_request
    entity_ref    TEXT NOT NULL,
    external_id   TEXT,
    status        TEXT NOT NULL,
    error_text    TEXT,
    synced_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- Onboarding Wizard
-- ============================================================

CREATE TABLE IF NOT EXISTS onboarding_wizard (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL UNIQUE,
    step        INT NOT NULL DEFAULT 1,   -- current step (1–7)
    -- Per-step completion timestamps
    step1_at    TIMESTAMPTZ,              -- practice details
    step2_at    TIMESTAMPTZ,              -- departments
    step3_at    TIMESTAMPTZ,              -- staff & roles
    step4_at    TIMESTAMPTZ,              -- accounting connection
    step5_at    TIMESTAMPTZ,              -- payroll connection
    step6_at    TIMESTAMPTZ,              -- inventory setup
    step7_at    TIMESTAMPTZ,              -- review & launch
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add wizard_complete flag to tenants if the column doesn't already exist.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'tenants' AND column_name = 'wizard_complete'
    ) THEN
        ALTER TABLE tenants ADD COLUMN wizard_complete BOOLEAN NOT NULL DEFAULT false;
    END IF;
END
$$;
