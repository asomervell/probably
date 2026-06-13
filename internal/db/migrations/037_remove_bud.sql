-- +goose Up
-- Remove Bud integration (no longer used)

-- Drop Bud-specific tables
DROP TABLE IF EXISTS bud_category_mappings;
DROP TABLE IF EXISTS bud_customers;

-- Remove Bud-specific columns from transactions
ALTER TABLE transactions
    DROP COLUMN IF EXISTS bud_category_l1,
    DROP COLUMN IF EXISTS bud_category_l2,
    DROP COLUMN IF EXISTS bud_location_city,
    DROP COLUMN IF EXISTS bud_location_region;

-- Note: merchants table was removed in 036_entities.sql migration
-- The bud_merchant_id was migrated to entities.external_id with external_source='bud'
-- Clear the Bud external references from entities
UPDATE entities SET external_id = NULL, external_source = NULL WHERE external_source = 'bud';

-- Remove bud_group_label from recurring_patterns
-- (keeping the table but removing the Bud-specific column)
ALTER TABLE recurring_patterns
    DROP COLUMN IF EXISTS bud_group_label;

-- Drop the unique constraint on bud_group_label if it exists
DROP INDEX IF EXISTS recurring_patterns_ledger_id_bud_group_label_key;

-- +goose Down
-- Restore Bud tables and columns

-- Recreate Bud-specific tables
CREATE TABLE IF NOT EXISTS bud_customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID UNIQUE NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    bud_customer_id TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bud_category_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    bud_category_l1 TEXT NOT NULL,
    bud_category_l2 TEXT NOT NULL,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(ledger_id, bud_category_l1, bud_category_l2)
);

CREATE INDEX IF NOT EXISTS idx_bud_category_mappings_ledger ON bud_category_mappings(ledger_id);

-- Restore columns on transactions
ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS bud_category_l1 TEXT,
    ADD COLUMN IF NOT EXISTS bud_category_l2 TEXT,
    ADD COLUMN IF NOT EXISTS bud_location_city TEXT,
    ADD COLUMN IF NOT EXISTS bud_location_region TEXT;

-- Note: merchants table recreation is handled by 036_entities.sql down migration
-- Just restore the external_source='bud' references if needed (no-op since we can't restore the original values)

-- Restore bud_group_label on recurring_patterns
ALTER TABLE recurring_patterns
    ADD COLUMN IF NOT EXISTS bud_group_label TEXT,
    ADD CONSTRAINT recurring_patterns_ledger_id_bud_group_label_key UNIQUE(ledger_id, bud_group_label);
