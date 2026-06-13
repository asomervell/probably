-- +goose Up
-- +goose StatementBegin

-- WARNING: This migration removes Teller-specific columns.
-- Only run this AFTER migration 041 has been applied and all data has been migrated.
-- This migration should be run after verifying that all accounts work with the new schema.

-- Remove Teller-specific columns from accounts table
-- These have been replaced by generic provider columns
ALTER TABLE accounts DROP COLUMN IF EXISTS teller_account_id;
ALTER TABLE accounts DROP COLUMN IF EXISTS teller_enrollment_id;
ALTER TABLE accounts DROP COLUMN IF EXISTS teller_access_token;
ALTER TABLE accounts DROP COLUMN IF EXISTS teller_subtype;
ALTER TABLE accounts DROP COLUMN IF EXISTS teller_status;
ALTER TABLE accounts DROP COLUMN IF EXISTS teller_last_synced_at;

-- Remove Teller-specific index
DROP INDEX IF EXISTS idx_accounts_teller_account_id;

-- Note: We keep teller_transaction_id in transactions table for now
-- as it may be referenced by other code. It can be removed in a future migration
-- after verifying all code uses external_transaction_id.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Re-add Teller columns (for rollback)
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS teller_account_id VARCHAR(255);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS teller_enrollment_id VARCHAR(255);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS teller_access_token TEXT;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS teller_subtype VARCHAR(100);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS teller_status VARCHAR(50);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS teller_last_synced_at TIMESTAMPTZ;

-- Re-add index
CREATE INDEX IF NOT EXISTS idx_accounts_teller_account_id ON accounts(teller_account_id);

-- Note: Data migration would need to be done manually if rolling back

-- +goose StatementEnd
