-- HSD reporting support: extend dispensing records for de-identified statistics
ALTER TABLE pharmacy_dispensing_records
    ADD COLUMN IF NOT EXISTS nzmt_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS formulary_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS quantity NUMERIC(10,3) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS unit TEXT NOT NULL DEFAULT 'tablet',
    ADD COLUMN IF NOT NULL ADD COLUMN subsidy_amount_nzd NUMERIC(10,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT NULL ADD COLUMN dispensed_date DATE NOT NULL DEFAULT CURRENT_DATE;

-- HSD reports: persisted reports for audit trail
CREATE TABLE IF NOT EXISTS pharmacy_hsd_reports (
    id                  TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    pharmacy_hsp_no     TEXT NOT NULL,
    report_period_start DATE NOT NULL,
    report_period_end   DATE NOT NULL,
    total_dispenses     INTEGER NOT NULL DEFAULT 0,
    generated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hsd_reports_pharmacy ON pharmacy_hsd_reports(pharmacy_hsp_no);
CREATE INDEX IF NOT EXISTS idx_hsd_reports_period ON pharmacy_hsd_reports(report_period_start, report_period_end);