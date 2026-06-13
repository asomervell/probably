-- +goose Up
-- Add reviewed_at timestamp to mark transactions as manually reviewed
-- Transactions with reviewed_at set won't appear in "needs review" filter
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

-- Index for efficient filtering of unreviewed transactions
CREATE INDEX IF NOT EXISTS idx_transactions_reviewed_at ON transactions (reviewed_at) WHERE reviewed_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_transactions_reviewed_at;
ALTER TABLE transactions DROP COLUMN reviewed_at;
