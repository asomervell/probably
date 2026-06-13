-- +goose Up
ALTER TABLE accounts 
ADD COLUMN connection_status TEXT;

CREATE INDEX idx_accounts_connection_status 
ON accounts(ledger_id, connection_status) 
WHERE connection_status IS NOT NULL;

COMMENT ON COLUMN accounts.connection_status IS 
'Status indicating if connection needs update: update_required, pending_expiration, pending_disconnect, new_accounts_available';

-- +goose Down
DROP INDEX IF EXISTS idx_accounts_connection_status;
ALTER TABLE accounts DROP COLUMN IF EXISTS connection_status;
