-- +goose Up
-- Add retry logic to enrichment processing

-- Track enrichment attempts (like categorization does)
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS enrichment_attempts INTEGER NOT NULL DEFAULT 0;

-- Reset failed enrichments so they get retried with the new logic
UPDATE transactions 
SET enrichment_status = 'pending', enrichment_attempts = 0
WHERE enrichment_status = 'failed';

-- +goose Down
ALTER TABLE transactions DROP COLUMN IF EXISTS enrichment_attempts;
