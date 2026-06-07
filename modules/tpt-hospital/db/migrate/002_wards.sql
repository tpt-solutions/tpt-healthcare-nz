-- Hospital wards and beds
CREATE TABLE IF NOT EXISTS hospital_wards (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL,
    code             TEXT NOT NULL,
    ward_type        TEXT NOT NULL DEFAULT 'general',
    floor            TEXT,
    building         TEXT,
    total_beds       INT  NOT NULL DEFAULT 0,
    available_beds   INT  NOT NULL DEFAULT 0,
    tenant_id        UUID NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hospital_wards_tenant_type ON hospital_wards (tenant_id, ward_type);

CREATE TABLE IF NOT EXISTS hospital_beds (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ward_id      UUID NOT NULL REFERENCES hospital_wards(id),
    bed_number   TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'available',
    admission_id UUID,
    tenant_id    UUID NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hospital_beds_ward   ON hospital_beds (ward_id);
CREATE INDEX IF NOT EXISTS idx_hospital_beds_status ON hospital_beds (tenant_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_hospital_beds_ward_number ON hospital_beds (ward_id, bed_number);
