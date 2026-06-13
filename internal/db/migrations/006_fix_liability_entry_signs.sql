-- +goose Up
-- +goose StatementBegin

-- Fix liability account entry signs for NON-TRANSFER transactions
-- 
-- Problem: Entries on liability accounts were created with wrong sign semantics.
-- Teller reports amounts from a balance perspective:
--   - Positive = balance/debt increased (purchase)
--   - Negative = balance/debt decreased (payment)
-- 
-- But accounting requires debit/credit perspective for liabilities:
--   - Debit (+) = liability DECREASES
--   - Credit (-) = liability INCREASES
--
-- Solution: Negate entry amounts for liability account transactions.
-- We only fix non-transfer transactions to avoid complicating already-matched transfers.
-- For transfers, users should either:
--   1. Unlink and re-match transfers, or
--   2. Run a full resync on affected liability accounts
--
-- Note: This assumes each transaction has exactly 2 entries (primary + contra)

-- Step 1: Negate entries on liability accounts for non-transfer transactions
UPDATE entries e
SET amount_cents = -amount_cents
FROM accounts a, transactions t
WHERE e.account_id = a.id
  AND e.transaction_id = t.id
  AND a.type = 'liability'
  AND t.is_transfer = false;

-- Step 2: Negate the contra entries (expense/income) for those same transactions
-- This maintains the zero-sum balance requirement
UPDATE entries e
SET amount_cents = -amount_cents
FROM transactions t
WHERE e.transaction_id = t.id
  AND t.is_transfer = false
  AND e.account_id IN (
      SELECT id FROM accounts WHERE type IN ('expense', 'income')
  )
  AND e.transaction_id IN (
      -- Only transactions that have a liability account entry
      SELECT DISTINCT e2.transaction_id
      FROM entries e2
      JOIN accounts a ON e2.account_id = a.id
      WHERE a.type = 'liability'
  );

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Reverse the sign changes (same operation - negate again)
UPDATE entries e
SET amount_cents = -amount_cents
FROM accounts a, transactions t
WHERE e.account_id = a.id
  AND e.transaction_id = t.id
  AND a.type = 'liability'
  AND t.is_transfer = false;

UPDATE entries e
SET amount_cents = -amount_cents
FROM transactions t
WHERE e.transaction_id = t.id
  AND t.is_transfer = false
  AND e.account_id IN (
      SELECT id FROM accounts WHERE type IN ('expense', 'income')
  )
  AND e.transaction_id IN (
      SELECT DISTINCT e2.transaction_id
      FROM entries e2
      JOIN accounts a ON e2.account_id = a.id
      WHERE a.type = 'liability'
  );

-- +goose StatementEnd

