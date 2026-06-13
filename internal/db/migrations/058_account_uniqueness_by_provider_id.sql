-- +goose Up
-- +goose StatementBegin
-- Allow multiple accounts with the same display name/type in a ledger.
-- Some institutions (e.g. multiple credit cards) return identical account names.
DROP INDEX IF EXISTS idx_accounts_ledger_name_type_unique;

-- Keep deduplication where it is actually stable: provider account identity.
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_ledger_provider_external_unique
ON accounts (ledger_id, provider, external_account_id)
WHERE provider IS NOT NULL
  AND external_account_id IS NOT NULL
  AND external_account_id != '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_accounts_ledger_provider_external_unique;

-- Restore prior uniqueness behavior.
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_ledger_name_type_unique
ON accounts (ledger_id, name, type);
-- +goose StatementEnd
