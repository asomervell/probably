-- +goose Up
-- +goose StatementBegin

-- Add teller_last_synced_at to track when each account was last synced
ALTER TABLE accounts ADD COLUMN teller_last_synced_at TIMESTAMP;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE accounts DROP COLUMN IF EXISTS teller_last_synced_at;

-- +goose StatementEnd
