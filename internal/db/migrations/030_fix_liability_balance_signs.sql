-- +goose Up
-- +goose StatementBegin

-- Fix liability account balance signs.
-- 
-- Problem: Migrations 006 and 013 created inconsistent sign conventions.
-- Some liability accounts now have wrong signs while others are correct.
-- 
-- The current convention should be: positive balance = debt owed
-- Display logic flips signs: positive stored → negative displayed (red)
-- 
-- Detection: Liability accounts with NEGATIVE calculated balance are likely wrong
-- because most credit cards have positive debt (money owed), not credits.
-- 
-- Fix: For liability accounts with negative balance, flip ALL entries
-- for transactions involving that account.

-- Step 1: Find liability accounts with negative balance
CREATE TEMP TABLE liability_accounts_to_fix AS
SELECT a.id as account_id
FROM accounts a
LEFT JOIN entries e ON e.account_id = a.id
WHERE a.type = 'liability'
  AND a.is_active = true
GROUP BY a.id
HAVING COALESCE(SUM(e.amount_cents), 0) < 0;

-- Step 2: Get all transactions that have entries to these accounts
CREATE TEMP TABLE transactions_to_fix AS
SELECT DISTINCT e.transaction_id
FROM entries e
WHERE e.account_id IN (SELECT account_id FROM liability_accounts_to_fix);

-- Step 3: Flip ALL entries in those transactions (both liability and contra)
-- This maintains zero-sum balance while fixing the signs
UPDATE entries e
SET amount_cents = -amount_cents
WHERE e.transaction_id IN (SELECT transaction_id FROM transactions_to_fix);

-- Clean up
DROP TABLE liability_accounts_to_fix;
DROP TABLE transactions_to_fix;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- To reverse, you would need to identify which accounts were fixed.
-- The safest approach is to run a full resync on affected accounts.
-- Since flipping twice returns to original, running this migration again would reverse it
-- if the same accounts are still negative (unlikely after the fix).
SELECT 1;

-- +goose StatementEnd
