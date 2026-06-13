-- +goose Up
-- +goose StatementBegin

-- Revert the sign flip on liability account entries.
-- Previously, we were inverting signs when storing liability transactions from Teller.
-- Now we store exactly as Teller reports: positive = debt increased, negative = debt decreased.
-- This migration flips the signs back to match Teller's original values.

-- Flip signs on all entries for liability accounts
UPDATE entries e
SET amount_cents = -amount_cents
FROM accounts a
WHERE e.account_id = a.id
  AND a.type = 'liability';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Reverse: flip signs back (same operation since -(-x) = x)
UPDATE entries e
SET amount_cents = -amount_cents
FROM accounts a
WHERE e.account_id = a.id
  AND a.type = 'liability';

-- +goose StatementEnd

