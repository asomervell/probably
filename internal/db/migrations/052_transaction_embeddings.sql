-- +goose Up
-- +goose StatementBegin

-- Add embedding columns to transactions for semantic search
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS embedding FLOAT4[];
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS embedding_model VARCHAR(100);
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS embedding_updated_at TIMESTAMPTZ;

-- Index on transactions that have embeddings
CREATE INDEX IF NOT EXISTS idx_transactions_has_embedding 
ON transactions ((embedding IS NOT NULL)) 
WHERE embedding IS NOT NULL;

-- Remove legacy detected_recurring system if it still exists
-- (Patterns are now stored directly on transactions via pattern_type, pattern_metadata)
ALTER TABLE transactions DROP COLUMN IF EXISTS detected_recurring_id;
DROP TABLE IF EXISTS subscription_price_history;
DROP TABLE IF EXISTS detected_recurring;

COMMENT ON COLUMN transactions.embedding IS 'Transaction embedding for similarity search and pattern detection';
COMMENT ON COLUMN transactions.embedding_model IS 'Model used to generate the embedding';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_transactions_has_embedding;
ALTER TABLE transactions DROP COLUMN IF EXISTS embedding_updated_at;
ALTER TABLE transactions DROP COLUMN IF EXISTS embedding_model;
ALTER TABLE transactions DROP COLUMN IF EXISTS embedding;

-- Note: detected_recurring tables are NOT recreated in down migration

-- +goose StatementEnd
