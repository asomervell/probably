-- +goose Up
-- +goose StatementBegin

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table (for Authboss)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    
    -- Authboss fields
    confirmed BOOLEAN NOT NULL DEFAULT FALSE,
    confirm_selector VARCHAR(255),
    confirm_verifier VARCHAR(255),
    
    recover_selector VARCHAR(255),
    recover_verifier VARCHAR(255),
    recover_token_expiry TIMESTAMP,
    
    locked BOOLEAN NOT NULL DEFAULT FALSE,
    lock_reason VARCHAR(255),
    
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_attempt TIMESTAMP,
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_confirm_selector ON users(confirm_selector);
CREATE INDEX idx_users_recover_selector ON users(recover_selector);

-- Sessions table (for Authboss remember me)
CREATE TABLE sessions (
    token VARCHAR(255) PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Ledgers (one per user, but could have multiple for different currencies)
CREATE TABLE ledgers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ledgers_user_id ON ledgers(user_id);

-- Account types enum
CREATE TYPE account_type AS ENUM ('asset', 'liability', 'income', 'expense', 'equity');

-- Accounts
CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type account_type NOT NULL,
    institution_name VARCHAR(255),
    
    -- Teller integration
    teller_account_id VARCHAR(255),
    teller_enrollment_id VARCHAR(255),
    teller_access_token TEXT, -- Encrypted
    
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_ledger_id ON accounts(ledger_id);
CREATE INDEX idx_accounts_teller_account_id ON accounts(teller_account_id);

-- Transactions (the header record)
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    description TEXT NOT NULL,
    notes TEXT,
    
    -- Teller integration
    teller_transaction_id VARCHAR(255),
    
    -- Transfer tracking
    is_transfer BOOLEAN NOT NULL DEFAULT FALSE,
    transfer_pair_id UUID REFERENCES transactions(id),
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_ledger_id ON transactions(ledger_id);
CREATE INDEX idx_transactions_date ON transactions(date);
CREATE INDEX idx_transactions_teller_transaction_id ON transactions(teller_transaction_id);
CREATE INDEX idx_transactions_transfer_pair_id ON transactions(transfer_pair_id);

-- Entries (the double-entry lines)
CREATE TABLE entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL, -- Positive = debit, Negative = credit
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entries_transaction_id ON entries(transaction_id);
CREATE INDEX idx_entries_account_id ON entries(account_id);

-- Tags for categorization
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES tags(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    color VARCHAR(7) NOT NULL DEFAULT '#6366f1', -- Hex color
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    
    UNIQUE(ledger_id, name)
);

CREATE INDEX idx_tags_ledger_id ON tags(ledger_id);
CREATE INDEX idx_tags_parent_id ON tags(parent_id);

-- Transaction-to-tag mapping (many-to-many)
CREATE TABLE transaction_tags (
    transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (transaction_id, tag_id)
);

CREATE INDEX idx_transaction_tags_tag_id ON transaction_tags(tag_id);

-- Categorization rules
CREATE TABLE categorization_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    match_pattern TEXT NOT NULL, -- Can be regex or simple string
    is_regex BOOLEAN NOT NULL DEFAULT FALSE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL DEFAULT 0, -- Higher = evaluated first
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_categorization_rules_ledger_id ON categorization_rules(ledger_id);
CREATE INDEX idx_categorization_rules_priority ON categorization_rules(priority DESC);

-- Teller enrollments (bank connections)
CREATE TABLE teller_enrollments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    enrollment_id VARCHAR(255) NOT NULL UNIQUE,
    access_token TEXT NOT NULL, -- Encrypted
    institution_id VARCHAR(255) NOT NULL,
    institution_name VARCHAR(255) NOT NULL,
    last_synced_at TIMESTAMP,
    status VARCHAR(50) NOT NULL DEFAULT 'connected',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_teller_enrollments_ledger_id ON teller_enrollments(ledger_id);
CREATE INDEX idx_teller_enrollments_enrollment_id ON teller_enrollments(enrollment_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS teller_enrollments;
DROP TABLE IF EXISTS categorization_rules;
DROP TABLE IF EXISTS transaction_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS ledgers;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS account_type;

-- +goose StatementEnd

