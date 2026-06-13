-- +goose Up
-- Migration: Transform rules from pattern-matching to LLM-based prompts
-- This allows natural language rules like "Pet stores should be Pet Supplies"

-- Add new prompt column for LLM instructions
ALTER TABLE categorization_rules ADD COLUMN IF NOT EXISTS prompt TEXT;

-- Add example column for providing transaction examples (optional)
ALTER TABLE categorization_rules ADD COLUMN IF NOT EXISTS examples TEXT;

-- Migrate existing pattern rules to prompts (if any exist)
UPDATE categorization_rules 
SET prompt = CASE 
    WHEN is_regex THEN 'Transactions matching the pattern "' || match_pattern || '" should be categorized appropriately'
    ELSE 'Transactions containing "' || match_pattern || '" in the description should be categorized appropriately'
END
WHERE prompt IS NULL AND match_pattern IS NOT NULL AND match_pattern != '';

-- Index for faster rule lookups
CREATE INDEX IF NOT EXISTS idx_categorization_rules_active ON categorization_rules(ledger_id, is_active) WHERE is_active = true;

-- +goose Down
DROP INDEX IF EXISTS idx_categorization_rules_active;
ALTER TABLE categorization_rules DROP COLUMN IF EXISTS examples;
ALTER TABLE categorization_rules DROP COLUMN IF EXISTS prompt;

