-- +goose Up
-- +goose StatementBegin

-- Create entities table - unified model for persons, businesses, trusts, partnerships, government
CREATE TABLE entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(50) NOT NULL CHECK (type IN ('person', 'business', 'trust', 'partnership', 'government')),
    subtype VARCHAR(50), -- financial_institution, retailer, individual, etc.
    name VARCHAR(500) NOT NULL,
    slug VARCHAR(500),
    logo_url VARCHAR(500),
    website VARCHAR(500),
    description TEXT,
    external_id VARCHAR(255),
    external_source VARCHAR(50), -- teller, bud, manual
    metadata JSONB DEFAULT '{}',
    user_verified BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_entities_type_subtype ON entities(type, subtype);
CREATE INDEX idx_entities_external ON entities(external_source, external_id);
CREATE INDEX idx_entities_slug ON entities(slug);
CREATE INDEX idx_entities_name ON entities(name);

-- Migrate merchants to entities (all as business, subtype=retailer as default)
-- Use the same UUID so foreign keys continue to work
INSERT INTO entities (id, type, subtype, name, slug, logo_url, website, description, external_id, external_source, user_verified, created_at, updated_at)
SELECT id, 'business', 'retailer', display_name, slug, logo_url, website, description, bud_merchant_id, 'bud', user_verified, created_at, updated_at
FROM merchants;

-- Add entity columns to transactions
ALTER TABLE transactions ADD COLUMN entity_id UUID REFERENCES entities(id);
ALTER TABLE transactions ADD COLUMN counterparty_entity_id UUID REFERENCES entities(id);
ALTER TABLE transactions ADD COLUMN intermediary_entity_id UUID REFERENCES entities(id);

CREATE INDEX idx_transactions_entity_id ON transactions(entity_id);
CREATE INDEX idx_transactions_counterparty_entity_id ON transactions(counterparty_entity_id);
CREATE INDEX idx_transactions_intermediary_entity_id ON transactions(intermediary_entity_id);

-- Migrate merchant_id to entity_id in transactions
UPDATE transactions SET entity_id = merchant_id WHERE merchant_id IS NOT NULL;

-- Add entity_id to recurring_patterns
ALTER TABLE recurring_patterns ADD COLUMN entity_id UUID REFERENCES entities(id);
CREATE INDEX idx_recurring_patterns_entity_id ON recurring_patterns(entity_id);

-- Migrate merchant_id to entity_id in recurring_patterns
UPDATE recurring_patterns SET entity_id = merchant_id WHERE merchant_id IS NOT NULL;

-- Drop merchant_id foreign key and column from transactions
ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_merchant_id_fkey;
ALTER TABLE transactions DROP COLUMN merchant_id;

-- Drop merchant_id foreign key and column from recurring_patterns
ALTER TABLE recurring_patterns DROP CONSTRAINT IF EXISTS recurring_patterns_merchant_id_fkey;
ALTER TABLE recurring_patterns DROP COLUMN merchant_id;

-- Drop the merchants table (data has been migrated to entities)
-- Use CASCADE to drop dependent objects (BM25 index from migration 022, etc.)
DROP TABLE merchants CASCADE;

-- Create entity relationships table
CREATE TABLE entity_relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    entity_a_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    entity_b_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    relationship_type VARCHAR(50) NOT NULL, -- spouse, partner, family, trustee, beneficiary, employer, self
    valid_from TIMESTAMPTZ,
    valid_to TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_entity_relationships_ledger ON entity_relationships(ledger_id);
CREATE INDEX idx_entity_relationships_entity_a ON entity_relationships(entity_a_id);
CREATE INDEX idx_entity_relationships_entity_b ON entity_relationships(entity_b_id);

-- Create account entity ownership table
CREATE TABLE account_entity_ownership (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    ownership_percentage DECIMAL(5,2) DEFAULT 100.00,
    role VARCHAR(50) DEFAULT 'owner', -- owner, trustee, beneficiary
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(account_id, entity_id)
);

CREATE INDEX idx_account_entity_ownership_account ON account_entity_ownership(account_id);
CREATE INDEX idx_account_entity_ownership_entity ON account_entity_ownership(entity_id);

-- Add fixed asset fields to accounts
ALTER TABLE accounts ADD COLUMN asset_subtype VARCHAR(50); -- real_estate, vehicle, equipment, collectible, other
ALTER TABLE accounts ADD COLUMN purchase_date DATE;
ALTER TABLE accounts ADD COLUMN purchase_price_cents BIGINT;
ALTER TABLE accounts ADD COLUMN current_value_cents BIGINT;
ALTER TABLE accounts ADD COLUMN depreciation_method VARCHAR(50); -- none, straight_line, declining_balance

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove fixed asset fields from accounts
ALTER TABLE accounts DROP COLUMN IF EXISTS asset_subtype;
ALTER TABLE accounts DROP COLUMN IF EXISTS purchase_date;
ALTER TABLE accounts DROP COLUMN IF EXISTS purchase_price_cents;
ALTER TABLE accounts DROP COLUMN IF EXISTS current_value_cents;
ALTER TABLE accounts DROP COLUMN IF EXISTS depreciation_method;

-- Drop account entity ownership
DROP TABLE IF EXISTS account_entity_ownership;

-- Drop entity relationships
DROP TABLE IF EXISTS entity_relationships;

-- Recreate merchants table from entities
CREATE TABLE merchants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bud_merchant_id VARCHAR(255),
    slug VARCHAR(500),
    display_name VARCHAR(500) NOT NULL,
    logo_url VARCHAR(500),
    website VARCHAR(500),
    description TEXT,
    user_verified BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Restore merchants from entities (only business type)
INSERT INTO merchants (id, bud_merchant_id, slug, display_name, logo_url, website, description, user_verified, created_at, updated_at)
SELECT id, external_id, slug, name, logo_url, website, description, user_verified, created_at, updated_at
FROM entities WHERE type = 'business';

-- Add merchant_id back to recurring_patterns
ALTER TABLE recurring_patterns ADD COLUMN merchant_id UUID REFERENCES merchants(id);
UPDATE recurring_patterns SET merchant_id = entity_id WHERE entity_id IS NOT NULL;
ALTER TABLE recurring_patterns DROP COLUMN entity_id;

-- Add merchant_id back to transactions
ALTER TABLE transactions ADD COLUMN merchant_id UUID REFERENCES merchants(id);
UPDATE transactions SET merchant_id = entity_id WHERE entity_id IS NOT NULL;

-- Remove entity columns from transactions
ALTER TABLE transactions DROP COLUMN IF EXISTS intermediary_entity_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS counterparty_entity_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS entity_id;

-- Drop entities table
DROP TABLE IF EXISTS entities;

-- +goose StatementEnd
