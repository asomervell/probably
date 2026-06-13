-- +goose Up
-- Add pattern detection hints to entities
-- Entities can exhibit multiple patterns (e.g., Amazon: subscriptions AND salary AND purchases)
-- Stored as JSONB array of pattern hints

ALTER TABLE entities ADD COLUMN IF NOT EXISTS pattern_hints JSONB DEFAULT '[]'::JSONB;

-- Index for querying entities by pattern type using GIN
CREATE INDEX IF NOT EXISTS idx_entities_pattern_hints ON entities USING GIN (pattern_hints);

COMMENT ON COLUMN entities.pattern_hints IS 'Array of learned pattern hints: [{pattern_type, frequency, confidence, occurrence_count, last_updated_at}, ...]';

-- +goose Down
ALTER TABLE entities DROP COLUMN IF EXISTS pattern_hints;
DROP INDEX IF EXISTS idx_entities_pattern_hints;
