-- +goose Up
-- +goose StatementBegin

-- Add institution_id to accounts table to enable showing institution logos
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS institution_id VARCHAR(255);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE accounts DROP COLUMN IF EXISTS institution_id;

-- +goose StatementEnd
