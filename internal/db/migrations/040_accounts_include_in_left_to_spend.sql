-- +goose Up
-- +goose StatementBegin
ALTER TABLE accounts ADD COLUMN include_in_left_to_spend BOOLEAN;
-- NULL = use smart default, TRUE/FALSE = user override
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE accounts DROP COLUMN IF EXISTS include_in_left_to_spend;
-- +goose StatementEnd
