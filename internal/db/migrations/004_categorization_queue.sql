-- +goose Up
-- Migration: Add categorization queue status to transactions
-- This allows transactions to be queued for AI categorization and processed in batches

-- Categorization status enum:
-- 'pending' - new transaction, needs categorization
-- 'queued' - explicitly queued for AI categorization  
-- 'processing' - currently being processed by the worker
-- 'done' - categorization complete (has tags or skipped)
-- 'skipped' - skipped (e.g., transfers, already has tags)
-- 'failed' - AI categorization failed (will retry)

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS categorization_status VARCHAR(20) DEFAULT 'pending';
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS categorization_error TEXT;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS categorization_attempts INTEGER DEFAULT 0;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS categorization_queued_at TIMESTAMP;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS categorization_completed_at TIMESTAMP;

-- Index for efficient queue queries
CREATE INDEX IF NOT EXISTS idx_transactions_categorization_queue 
ON transactions (ledger_id, categorization_status, categorization_queued_at)
WHERE categorization_status IN ('pending', 'queued', 'failed');

-- Mark existing transactions with tags as 'done'
UPDATE transactions t
SET categorization_status = 'done'
WHERE EXISTS (
    SELECT 1 FROM transaction_tags tt WHERE tt.transaction_id = t.id
);

-- Mark transfers as 'skipped'
UPDATE transactions 
SET categorization_status = 'skipped'
WHERE is_transfer = true AND categorization_status = 'pending';

-- +goose Down
DROP INDEX IF EXISTS idx_transactions_categorization_queue;
ALTER TABLE transactions DROP COLUMN IF EXISTS categorization_completed_at;
ALTER TABLE transactions DROP COLUMN IF EXISTS categorization_queued_at;
ALTER TABLE transactions DROP COLUMN IF EXISTS categorization_attempts;
ALTER TABLE transactions DROP COLUMN IF EXISTS categorization_error;
ALTER TABLE transactions DROP COLUMN IF EXISTS categorization_status;

