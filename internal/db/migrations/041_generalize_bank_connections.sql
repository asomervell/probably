-- +goose Up
-- +goose StatementBegin

-- Create provider enum type
CREATE TYPE provider_type AS ENUM ('teller', 'plaid', 'akahu');

-- Add provider column to accounts
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS provider provider_type;

-- Add generic connection fields (keeping teller_* columns for backward compatibility)
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS external_account_id VARCHAR(255);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS connection_id VARCHAR(255);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS access_token TEXT; -- Encrypted, replaces teller_access_token
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS account_subtype VARCHAR(100); -- Replaces teller_subtype
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS account_status VARCHAR(50); -- Replaces teller_status
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS last_synced_at TIMESTAMPTZ; -- Replaces teller_last_synced_at

-- Add provider-specific metadata as JSONB
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS provider_metadata JSONB;

-- Add indexes for new columns
CREATE INDEX IF NOT EXISTS idx_accounts_provider ON accounts(provider);
CREATE INDEX IF NOT EXISTS idx_accounts_external_account_id ON accounts(external_account_id);
CREATE INDEX IF NOT EXISTS idx_accounts_connection_id ON accounts(connection_id);

-- Migrate existing Teller data to new columns
-- This sets provider='teller' and copies teller_* fields to generic fields
UPDATE accounts
SET 
    provider = 'teller',
    external_account_id = teller_account_id,
    connection_id = teller_enrollment_id,
    access_token = teller_access_token,
    account_subtype = teller_subtype,
    account_status = teller_status,
    last_synced_at = teller_last_synced_at,
    provider_metadata = jsonb_build_object(
        'teller_account_id', teller_account_id,
        'teller_enrollment_id', teller_enrollment_id,
        'teller_subtype', teller_subtype,
        'teller_status', teller_status
    )
WHERE teller_account_id IS NOT NULL
  AND provider IS NULL;

-- Set default provider for accounts without any provider connection
UPDATE accounts
SET provider = NULL
WHERE provider IS NULL;

-- Add generic external_transaction_id to transactions (keeping teller_transaction_id for backward compatibility)
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS external_transaction_id VARCHAR(255);

-- Migrate existing Teller transaction IDs
UPDATE transactions
SET external_transaction_id = teller_transaction_id
WHERE teller_transaction_id IS NOT NULL
  AND external_transaction_id IS NULL;

-- Add index for external_transaction_id
CREATE INDEX IF NOT EXISTS idx_transactions_external_transaction_id ON transactions(external_transaction_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Drop indexes
DROP INDEX IF EXISTS idx_transactions_external_transaction_id;
DROP INDEX IF EXISTS idx_accounts_connection_id;
DROP INDEX IF EXISTS idx_accounts_external_account_id;
DROP INDEX IF EXISTS idx_accounts_provider;

-- Drop new columns
ALTER TABLE transactions DROP COLUMN IF EXISTS external_transaction_id;
ALTER TABLE accounts DROP COLUMN IF EXISTS provider_metadata;
ALTER TABLE accounts DROP COLUMN IF EXISTS last_synced_at;
ALTER TABLE accounts DROP COLUMN IF EXISTS account_status;
ALTER TABLE accounts DROP COLUMN IF EXISTS account_subtype;
ALTER TABLE accounts DROP COLUMN IF EXISTS access_token;
ALTER TABLE accounts DROP COLUMN IF EXISTS connection_id;
ALTER TABLE accounts DROP COLUMN IF EXISTS external_account_id;
ALTER TABLE accounts DROP COLUMN IF EXISTS provider;

-- Drop enum type
DROP TYPE IF EXISTS provider_type;

-- +goose StatementEnd
