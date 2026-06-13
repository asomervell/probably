-- +goose Up
-- +goose StatementBegin

-- Add Teller enrichment fields to transactions for transfer detection and categorization
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS counterparty_name VARCHAR(255);
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS counterparty_type VARCHAR(50);
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS teller_type VARCHAR(50);
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS teller_category VARCHAR(100);
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS teller_status VARCHAR(50);
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS running_balance_cents BIGINT;

-- Index for efficient transfer candidate lookups
CREATE INDEX IF NOT EXISTS idx_transactions_transfer_lookup ON transactions(ledger_id, date, teller_transaction_id);

-- Pending transfer matches for user review
CREATE TABLE IF NOT EXISTS pending_transfer_matches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    candidate_transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    confidence_score DECIMAL(3,2) NOT NULL, -- 0.00 to 1.00
    match_reasons TEXT[], -- Array of reasons for the match
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, confirmed, rejected
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMP,
    
    -- Ensure we don't have duplicate matches (in either direction)
    UNIQUE(transaction_id, candidate_transaction_id),
    
    -- Prevent self-matching
    CHECK (transaction_id != candidate_transaction_id)
);

CREATE INDEX IF NOT EXISTS idx_pending_matches_status ON pending_transfer_matches(status);
CREATE INDEX IF NOT EXISTS idx_pending_matches_transaction ON pending_transfer_matches(transaction_id);
CREATE INDEX IF NOT EXISTS idx_pending_matches_candidate ON pending_transfer_matches(candidate_transaction_id);

-- Add extended Teller data to accounts
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS last_four VARCHAR(4);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS teller_subtype VARCHAR(100);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS teller_status VARCHAR(50);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS account_number_masked VARCHAR(50);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS routing_number_ach VARCHAR(20);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS routing_number_wire VARCHAR(20);

-- Store account owner/identity information from Teller
CREATE TABLE IF NOT EXISTS account_owners (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    date_of_birth DATE,
    
    -- Primary address
    address_street VARCHAR(255),
    address_city VARCHAR(100),
    address_region VARCHAR(100),
    address_postal_code VARCHAR(20),
    address_country VARCHAR(100),
    
    -- Primary contact info
    phone_number VARCHAR(50),
    email VARCHAR(255),
    
    -- Raw JSON for additional addresses/phones/emails
    additional_data JSONB,
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_owners_account_id ON account_owners(account_id);

-- Add index for category-based queries (useful for AI training/analysis)
CREATE INDEX IF NOT EXISTS idx_transactions_teller_category ON transactions(teller_category);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_transactions_teller_category;
DROP TABLE IF EXISTS account_owners;
ALTER TABLE accounts DROP COLUMN IF EXISTS routing_number_wire;
ALTER TABLE accounts DROP COLUMN IF EXISTS routing_number_ach;
ALTER TABLE accounts DROP COLUMN IF EXISTS account_number_masked;
ALTER TABLE accounts DROP COLUMN IF EXISTS teller_status;
ALTER TABLE accounts DROP COLUMN IF EXISTS teller_subtype;
ALTER TABLE accounts DROP COLUMN IF EXISTS last_four;
DROP INDEX IF EXISTS idx_pending_matches_candidate;
DROP INDEX IF EXISTS idx_pending_matches_transaction;
DROP INDEX IF EXISTS idx_pending_matches_status;
DROP TABLE IF EXISTS pending_transfer_matches;
DROP INDEX IF EXISTS idx_transactions_transfer_lookup;
ALTER TABLE transactions DROP COLUMN IF EXISTS running_balance_cents;
ALTER TABLE transactions DROP COLUMN IF EXISTS teller_status;
ALTER TABLE transactions DROP COLUMN IF EXISTS teller_category;
ALTER TABLE transactions DROP COLUMN IF EXISTS teller_type;
ALTER TABLE transactions DROP COLUMN IF EXISTS counterparty_type;
ALTER TABLE transactions DROP COLUMN IF EXISTS counterparty_name;

-- +goose StatementEnd

