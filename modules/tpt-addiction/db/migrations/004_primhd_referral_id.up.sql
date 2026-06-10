-- Add PRIMHD referral ID to addiction programmes.
-- The PRIMHDReferralID is the identifier assigned by the PRIMHD reporting system
-- when a referral is opened for a patient accepted into a DHB-funded addiction service.
ALTER TABLE addiction_programmes
    ADD COLUMN IF NOT EXISTS primhd_referral_id VARCHAR(100);
