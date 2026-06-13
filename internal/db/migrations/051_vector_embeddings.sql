-- +goose Up
-- +goose StatementBegin

-- Vector embeddings for similarity search without pgvector
-- Uses native PostgreSQL FLOAT4[] arrays with custom similarity functions

-- =============================================================================
-- PART 1: Remove legacy detected_recurring system
-- Patterns are now stored directly on transactions (pattern_type, pattern_metadata)
-- =============================================================================

-- Remove FK constraint from transactions first
ALTER TABLE transactions DROP COLUMN IF EXISTS detected_recurring_id;

-- Drop subscription_price_history (references detected_recurring)
DROP TABLE IF EXISTS subscription_price_history;

-- Drop detected_recurring table
DROP TABLE IF EXISTS detected_recurring;

-- =============================================================================
-- PART 2: Add embeddings to entities
-- =============================================================================

-- Add embedding column to entities for semantic search
-- Using FLOAT4[] (real[]) for embeddings - typically 768 dimensions
ALTER TABLE entities ADD COLUMN IF NOT EXISTS embedding FLOAT4[];
ALTER TABLE entities ADD COLUMN IF NOT EXISTS embedding_model VARCHAR(100);
ALTER TABLE entities ADD COLUMN IF NOT EXISTS embedding_updated_at TIMESTAMPTZ;

-- Index on entities that have embeddings (for filtering)
CREATE INDEX IF NOT EXISTS idx_entities_has_embedding 
ON entities ((embedding IS NOT NULL)) 
WHERE embedding IS NOT NULL;

-- =============================================================================
-- PART 3: Add embeddings to transactions
-- =============================================================================

-- Add embedding column to transactions for semantic search
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS embedding FLOAT4[];
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS embedding_model VARCHAR(100);
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS embedding_updated_at TIMESTAMPTZ;

-- Index on transactions that have embeddings
CREATE INDEX IF NOT EXISTS idx_transactions_has_embedding 
ON transactions ((embedding IS NOT NULL)) 
WHERE embedding IS NOT NULL;

-- =============================================================================
-- PART 4: Vector similarity functions
-- =============================================================================

-- Cosine similarity function for FLOAT4[] vectors
-- Returns similarity score between -1 and 1 (1 = identical, 0 = orthogonal, -1 = opposite)
CREATE OR REPLACE FUNCTION cosine_similarity(a FLOAT4[], b FLOAT4[]) 
RETURNS FLOAT4 AS $$
DECLARE
    dot_product FLOAT4 := 0;
    norm_a FLOAT4 := 0;
    norm_b FLOAT4 := 0;
    i INT;
BEGIN
    -- Vectors must be same length
    IF array_length(a, 1) IS NULL OR array_length(b, 1) IS NULL THEN
        RETURN NULL;
    END IF;
    IF array_length(a, 1) != array_length(b, 1) THEN
        RETURN NULL;
    END IF;
    
    -- Calculate dot product and norms in single pass
    FOR i IN 1..array_length(a, 1) LOOP
        dot_product := dot_product + (a[i] * b[i]);
        norm_a := norm_a + (a[i] * a[i]);
        norm_b := norm_b + (b[i] * b[i]);
    END LOOP;
    
    -- Avoid division by zero
    IF norm_a = 0 OR norm_b = 0 THEN
        RETURN 0;
    END IF;
    
    RETURN dot_product / (sqrt(norm_a) * sqrt(norm_b));
END;
$$ LANGUAGE plpgsql IMMUTABLE STRICT;

-- Euclidean distance function (L2 distance)
-- Smaller = more similar
CREATE OR REPLACE FUNCTION euclidean_distance(a FLOAT4[], b FLOAT4[]) 
RETURNS FLOAT4 AS $$
DECLARE
    sum_sq FLOAT4 := 0;
    i INT;
BEGIN
    IF array_length(a, 1) IS NULL OR array_length(b, 1) IS NULL THEN
        RETURN NULL;
    END IF;
    IF array_length(a, 1) != array_length(b, 1) THEN
        RETURN NULL;
    END IF;
    
    FOR i IN 1..array_length(a, 1) LOOP
        sum_sq := sum_sq + ((a[i] - b[i]) * (a[i] - b[i]));
    END LOOP;
    
    RETURN sqrt(sum_sq);
END;
$$ LANGUAGE plpgsql IMMUTABLE STRICT;

-- =============================================================================
-- Comments
-- =============================================================================

COMMENT ON COLUMN entities.embedding IS 'Semantic embedding vector from LLM embedding model (e.g., text-embedding-004)';
COMMENT ON COLUMN entities.embedding_model IS 'Model used to generate the embedding (e.g., text-embedding-004)';
COMMENT ON COLUMN transactions.embedding IS 'Transaction embedding for similarity search and pattern detection';
COMMENT ON COLUMN transactions.embedding_model IS 'Model used to generate the embedding';
COMMENT ON FUNCTION cosine_similarity(FLOAT4[], FLOAT4[]) IS 'Compute cosine similarity between two vectors (-1 to 1)';
COMMENT ON FUNCTION euclidean_distance(FLOAT4[], FLOAT4[]) IS 'Compute Euclidean (L2) distance between two vectors';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove transaction embeddings
DROP INDEX IF EXISTS idx_transactions_has_embedding;
ALTER TABLE transactions DROP COLUMN IF EXISTS embedding_updated_at;
ALTER TABLE transactions DROP COLUMN IF EXISTS embedding_model;
ALTER TABLE transactions DROP COLUMN IF EXISTS embedding;

-- Remove entity embeddings
DROP INDEX IF EXISTS idx_entities_has_embedding;
ALTER TABLE entities DROP COLUMN IF EXISTS embedding_updated_at;
ALTER TABLE entities DROP COLUMN IF EXISTS embedding_model;
ALTER TABLE entities DROP COLUMN IF EXISTS embedding;

-- Drop vector functions
DROP FUNCTION IF EXISTS euclidean_distance(FLOAT4[], FLOAT4[]);
DROP FUNCTION IF EXISTS cosine_similarity(FLOAT4[], FLOAT4[]);

-- Note: detected_recurring and subscription_price_history tables are NOT recreated
-- in the down migration as the data has been migrated to transaction columns.
-- If you need to restore them, use migrations 033 and 034.

-- +goose StatementEnd
