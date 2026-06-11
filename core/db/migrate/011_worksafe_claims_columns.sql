-- Add columns to acc_claims to support WorkSafe NZ as an alternative claim
-- destination for work-related injuries. claim_destination routes each claim
-- to the correct external scheme; worksafe_ref_number holds the WorkSafe-
-- assigned reference (analogous to acc_claim_number for ACC claims).
ALTER TABLE acc_claims
    ADD COLUMN IF NOT EXISTS claim_destination    VARCHAR(20) NOT NULL DEFAULT 'acc',
    ADD COLUMN IF NOT EXISTS worksafe_ref_number  TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS employer_nzbn        TEXT        NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS injury_mechanism     TEXT        NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_acc_claims_destination
    ON acc_claims (tenant_id, claim_destination);
