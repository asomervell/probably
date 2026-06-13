-- +goose Up
-- +goose StatementBegin

-- Fix teller_last_synced_at to use timezone-aware timestamp
-- The column was created without timezone, causing 8-hour offset bugs
ALTER TABLE accounts 
ALTER COLUMN teller_last_synced_at TYPE TIMESTAMPTZ 
USING teller_last_synced_at AT TIME ZONE 'America/Los_Angeles';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE accounts 
ALTER COLUMN teller_last_synced_at TYPE TIMESTAMP WITHOUT TIME ZONE;

-- +goose StatementEnd
