-- +goose Up
-- +goose StatementBegin

-- Add default_tag_id to merchants table
-- This allows users to set a default category (tag) for a merchant
-- Note: This is a soft reference since tags are per-ledger and merchants are global
-- The application validates that the tag belongs to the correct ledger
ALTER TABLE merchants ADD COLUMN default_tag_id UUID;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE merchants DROP COLUMN IF EXISTS default_tag_id;

-- +goose StatementEnd
