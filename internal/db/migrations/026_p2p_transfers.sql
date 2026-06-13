-- +goose Up
-- +goose StatementBegin

-- Add transfer_type to transactions for P2P categorization
-- Values: 'internal' (between own accounts), 'household' (family/spouse), 
--         'person_payment' (outbound P2P), 'person_receipt' (inbound P2P), 
--         'reimbursement' (friend paying back), NULL (not a transfer or unknown)
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS transfer_type VARCHAR(50);

-- Add index for filtering by transfer type
CREATE INDEX IF NOT EXISTS idx_transactions_transfer_type ON transactions(transfer_type);

-- Add is_p2p flag to merchants for tracking person vs business
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS is_person BOOLEAN NOT NULL DEFAULT FALSE;

-- Create household_members table for user-defined household relationships
CREATE TABLE IF NOT EXISTS household_members (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    name_pattern VARCHAR(255) NOT NULL, -- Pattern to match counterparty names
    relationship VARCHAR(100), -- 'spouse', 'self', 'family', 'partner', etc.
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    UNIQUE(ledger_id, name_pattern)
);

CREATE INDEX IF NOT EXISTS idx_household_members_ledger_id ON household_members(ledger_id);

-- Create p2p_rules table for user-defined P2P categorization rules
CREATE TABLE IF NOT EXISTS p2p_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    name_pattern VARCHAR(255) NOT NULL, -- Pattern to match counterparty/description
    transfer_type VARCHAR(50) NOT NULL, -- What to categorize as
    tag_id UUID REFERENCES tags(id) ON DELETE SET NULL, -- Optional tag to apply
    priority INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_p2p_rules_ledger_id ON p2p_rules(ledger_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS p2p_rules;
DROP TABLE IF EXISTS household_members;
DROP INDEX IF EXISTS idx_transactions_transfer_type;
ALTER TABLE transactions DROP COLUMN IF EXISTS transfer_type;
ALTER TABLE merchants DROP COLUMN IF EXISTS is_person;

-- +goose StatementEnd
