-- +goose Up
-- +goose StatementBegin

-- Drop the old unique constraint that only allowed one pattern per merchant
-- We need to allow multiple patterns per merchant (different amount buckets)
DROP INDEX IF EXISTS idx_detected_recurring_ledger_merchant;

-- Add amount_bucket column to help with deduplication
-- This stores a bucketed/rounded amount to identify distinct recurring charges
ALTER TABLE detected_recurring ADD COLUMN IF NOT EXISTS amount_bucket BIGINT;

-- Update existing records to set amount_bucket from avg_amount_cents
UPDATE detected_recurring SET amount_bucket = avg_amount_cents WHERE amount_bucket IS NULL;

-- Create new unique constraint on (ledger_id, merchant_id, amount_bucket)
-- This allows multiple patterns per merchant with different amounts
CREATE UNIQUE INDEX idx_detected_recurring_ledger_merchant_amount 
    ON detected_recurring(ledger_id, merchant_id, amount_bucket) 
    WHERE merchant_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_detected_recurring_ledger_merchant_amount;
ALTER TABLE detected_recurring DROP COLUMN IF EXISTS amount_bucket;

-- Restore old unique constraint
CREATE UNIQUE INDEX idx_detected_recurring_ledger_merchant ON detected_recurring(ledger_id, merchant_id) 
    WHERE merchant_id IS NOT NULL;

-- +goose StatementEnd
