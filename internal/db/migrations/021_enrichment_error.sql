-- +goose Up
-- Add enrichment_error column to track error messages during processing

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS enrichment_error TEXT;

-- +goose Down
ALTER TABLE transactions DROP COLUMN IF EXISTS enrichment_error;
