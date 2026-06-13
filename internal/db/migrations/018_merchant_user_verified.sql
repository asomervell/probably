-- +goose Up
-- Add description and user_verified fields to merchants
-- user_verified prevents automatic enrichment from overriding user selections

ALTER TABLE merchants ADD COLUMN IF NOT EXISTS description TEXT;
ALTER TABLE merchants ADD COLUMN IF NOT EXISTS user_verified BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE merchants DROP COLUMN IF EXISTS description;
ALTER TABLE merchants DROP COLUMN IF EXISTS user_verified;
