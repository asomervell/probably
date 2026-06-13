-- +goose Up
-- +goose StatementBegin

-- Pattern detection results for transactions
-- This allows transactions to be analyzed for patterns (salary, recurring bills, transfers, investments)
-- and tracked through the detection process

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS pattern_type VARCHAR(50);
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS pattern_metadata JSONB;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS pattern_detection_status VARCHAR(20) DEFAULT 'pending';
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS pattern_detection_attempts INTEGER DEFAULT 0;
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS pattern_detection_error TEXT;

-- Index for pattern detection queue
CREATE INDEX IF NOT EXISTS idx_transactions_pattern_detection 
ON transactions (ledger_id, pattern_detection_status, date DESC)
WHERE pattern_detection_status IN ('pending', 'queued', 'failed');

-- Index for querying by pattern type
CREATE INDEX IF NOT EXISTS idx_transactions_pattern_type 
ON transactions (ledger_id, pattern_type, date DESC)
WHERE pattern_type IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_transactions_pattern_type;
DROP INDEX IF EXISTS idx_transactions_pattern_detection;

ALTER TABLE transactions DROP COLUMN IF EXISTS pattern_detection_error;
ALTER TABLE transactions DROP COLUMN IF EXISTS pattern_detection_attempts;
ALTER TABLE transactions DROP COLUMN IF EXISTS pattern_detection_status;
ALTER TABLE transactions DROP COLUMN IF EXISTS pattern_metadata;
ALTER TABLE transactions DROP COLUMN IF EXISTS pattern_type;

-- +goose StatementEnd
