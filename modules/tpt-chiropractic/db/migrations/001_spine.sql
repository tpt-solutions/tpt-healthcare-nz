-- Chiropractic spinal chart entries.
CREATE TABLE IF NOT EXISTS chiropractic_spine_charts (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    patient_nhi     TEXT NOT NULL,
    clinician_id    TEXT NOT NULL,
    practice_id     TEXT NOT NULL DEFAULT '',
    visit_id        TEXT NOT NULL DEFAULT '',
    chart_date      BIGINT NOT NULL DEFAULT 0,
    created_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint,
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_chiropractic_spine_patient ON chiropractic_spine_charts(patient_nhi);

-- Individual vertebra entries linked to a chart.
CREATE TABLE IF NOT EXISTS chiropractic_vertebra_entries (
    id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    chart_id        TEXT NOT NULL REFERENCES chiropractic_spine_charts(id) ON DELETE CASCADE,
    segment         TEXT NOT NULL,
    region          TEXT NOT NULL DEFAULT '',
    fixation        BOOLEAN NOT NULL DEFAULT FALSE,
    subluxation     BOOLEAN NOT NULL DEFAULT FALSE,
    misalignment    TEXT NOT NULL DEFAULT '',
    mobility        TEXT NOT NULL DEFAULT 'normal',
    tenderness      TEXT NOT NULL DEFAULT 'none',
    muscle_tone     TEXT NOT NULL DEFAULT 'normal',
    x_ray_findings  TEXT NOT NULL DEFAULT '',
    adjustment      TEXT NOT NULL DEFAULT '',
    note            TEXT NOT NULL DEFAULT '',
    updated_at      BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM now()) * 1000)::bigint
);

CREATE INDEX IF NOT EXISTS idx_chiropractic_vertebra_chart ON chiropractic_vertebra_entries(chart_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chiropractic_vertebra_segment ON chiropractic_vertebra_entries(chart_id, segment);
