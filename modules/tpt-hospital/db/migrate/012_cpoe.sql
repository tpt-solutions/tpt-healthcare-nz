-- CPOE: Computerised Provider Order Entry
-- Links lab/imaging/consult orders to hospital admissions.

CREATE TABLE clinical_orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admission_id        UUID NOT NULL REFERENCES hospital_admissions(id) ON DELETE CASCADE,
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    patient_nhi         VARCHAR(7) NOT NULL,
    order_type          TEXT NOT NULL,
    priority            TEXT NOT NULL DEFAULT 'routine',
    status              TEXT NOT NULL DEFAULT 'pending',
    order_code          TEXT NOT NULL,
    order_text          TEXT NOT NULL,
    clinical_indication TEXT NOT NULL DEFAULT '',
    ordered_by          VARCHAR(16) NOT NULL,
    ordered_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    scheduled_for       TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    cancel_reason       TEXT,
    comments            TEXT,
    -- lab-specific (nullable)
    specimen_type       TEXT,
    container_type      TEXT,
    fasting_required    BOOLEAN NOT NULL DEFAULT FALSE,
    volume_required     TEXT,
    -- radiology-specific (nullable)
    body_site           TEXT,
    modality            TEXT,
    contrast            TEXT,
    pregnancy_status    TEXT,
    sedation_required   BOOLEAN NOT NULL DEFAULT FALSE,
    transport_mode      TEXT,
    -- result linkage
    result_id           UUID,
    result_at           TIMESTAMPTZ,
    -- HL7 dispatch tracking
    hl7_placer_order_id TEXT,
    hl7_filler_order_id TEXT,
    hl7_dispatched_at   TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_clinical_orders_admission ON clinical_orders(admission_id);
CREATE INDEX idx_clinical_orders_tenant ON clinical_orders(tenant_id);
CREATE INDEX idx_clinical_orders_patient ON clinical_orders(patient_nhi);
CREATE INDEX idx_clinical_orders_status ON clinical_orders(status);
CREATE INDEX idx_clinical_orders_type ON clinical_orders(order_type);
CREATE INDEX idx_clinical_orders_hl7_placer ON clinical_orders(hl7_placer_order_id) WHERE hl7_placer_order_id IS NOT NULL;
