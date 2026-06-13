-- +goose Up
-- Bud Transaction Enrichment Integration

-- Merchants table (normalized merchant data from Bud)
CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bud_merchant_id TEXT UNIQUE,
    slug TEXT,
    display_name TEXT NOT NULL,
    logo_url TEXT,
    website TEXT,
    default_category_l1 TEXT,
    default_category_l2 TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_merchants_bud_id ON merchants(bud_merchant_id);
CREATE INDEX idx_merchants_slug ON merchants(slug);

-- Recurring patterns table (subscription/regularity detection)
CREATE TABLE recurring_patterns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    merchant_id UUID REFERENCES merchants(id),
    bud_group_label TEXT,
    frequency TEXT,
    predicted_dates JSONB,
    avg_amount_cents BIGINT,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(ledger_id, bud_group_label)
);

CREATE INDEX idx_recurring_patterns_ledger ON recurring_patterns(ledger_id);
CREATE INDEX idx_recurring_patterns_merchant ON recurring_patterns(merchant_id);

-- Bud customers table (one per ledger)
CREATE TABLE bud_customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID UNIQUE NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    bud_customer_id TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Bud category mappings (maps Bud L1/L2 categories to user tags)
CREATE TABLE bud_category_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    bud_category_l1 TEXT NOT NULL,
    bud_category_l2 TEXT NOT NULL,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(ledger_id, bud_category_l1, bud_category_l2)
);

CREATE INDEX idx_bud_category_mappings_ledger ON bud_category_mappings(ledger_id);

-- Transaction table updates for Bud enrichment
ALTER TABLE transactions 
    ADD COLUMN IF NOT EXISTS merchant_id UUID REFERENCES merchants(id),
    ADD COLUMN IF NOT EXISTS recurring_pattern_id UUID REFERENCES recurring_patterns(id),
    ADD COLUMN IF NOT EXISTS bud_category_l1 TEXT,
    ADD COLUMN IF NOT EXISTS bud_category_l2 TEXT,
    ADD COLUMN IF NOT EXISTS bud_location_city TEXT,
    ADD COLUMN IF NOT EXISTS bud_location_region TEXT,
    ADD COLUMN IF NOT EXISTS enrichment_status TEXT DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS enriched_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_transactions_merchant ON transactions(merchant_id);
CREATE INDEX IF NOT EXISTS idx_transactions_recurring ON transactions(recurring_pattern_id);
CREATE INDEX IF NOT EXISTS idx_transactions_enrichment ON transactions(enrichment_status);

-- +goose Down
-- Remove transaction columns
ALTER TABLE transactions 
    DROP COLUMN IF EXISTS merchant_id,
    DROP COLUMN IF EXISTS recurring_pattern_id,
    DROP COLUMN IF EXISTS bud_category_l1,
    DROP COLUMN IF EXISTS bud_category_l2,
    DROP COLUMN IF EXISTS bud_location_city,
    DROP COLUMN IF EXISTS bud_location_region,
    DROP COLUMN IF EXISTS enrichment_status,
    DROP COLUMN IF EXISTS enriched_at;

DROP INDEX IF EXISTS idx_transactions_merchant;
DROP INDEX IF EXISTS idx_transactions_recurring;
DROP INDEX IF EXISTS idx_transactions_enrichment;

-- Drop tables in reverse order (respecting foreign keys)
DROP TABLE IF EXISTS bud_category_mappings;
DROP TABLE IF EXISTS bud_customers;
DROP TABLE IF EXISTS recurring_patterns;
DROP TABLE IF EXISTS merchants;


