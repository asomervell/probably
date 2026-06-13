-- +goose Up
-- +goose StatementBegin

-- Add institution_logo_url to store downloaded institution logos
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS institution_logo_url VARCHAR(500);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE accounts DROP COLUMN IF EXISTS institution_logo_url;

-- +goose StatementEnd
