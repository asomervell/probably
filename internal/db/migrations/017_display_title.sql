-- +goose Up
ALTER TABLE transactions ADD COLUMN display_title TEXT;

-- +goose Down
ALTER TABLE transactions DROP COLUMN display_title;
