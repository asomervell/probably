-- +goose Up
-- +goose StatementBegin

-- Workflow state tracking for merchant/entity enrichment
-- This allows tracking of enrichment progress (searching, extracting, fetching logo, etc.)
-- Similar to how transactions have categorization_status

ALTER TABLE entities ADD COLUMN IF NOT EXISTS enrichment_status VARCHAR(50) DEFAULT 'pending';
ALTER TABLE entities ADD COLUMN IF NOT EXISTS enrichment_steps JSONB DEFAULT '[]';
ALTER TABLE entities ADD COLUMN IF NOT EXISTS enrichment_error TEXT;
ALTER TABLE entities ADD COLUMN IF NOT EXISTS enrichment_started_at TIMESTAMP;
ALTER TABLE entities ADD COLUMN IF NOT EXISTS enrichment_completed_at TIMESTAMP;

-- Index for enrichment queue
CREATE INDEX IF NOT EXISTS idx_entities_enrichment_status 
ON entities (enrichment_status, created_at)
WHERE enrichment_status IN ('pending', 'processing', 'failed');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_entities_enrichment_status;

ALTER TABLE entities DROP COLUMN IF EXISTS enrichment_completed_at;
ALTER TABLE entities DROP COLUMN IF EXISTS enrichment_started_at;
ALTER TABLE entities DROP COLUMN IF EXISTS enrichment_error;
ALTER TABLE entities DROP COLUMN IF EXISTS enrichment_steps;
ALTER TABLE entities DROP COLUMN IF EXISTS enrichment_status;

-- +goose StatementEnd
