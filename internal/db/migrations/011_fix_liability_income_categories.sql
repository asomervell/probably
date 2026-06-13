-- +goose Up
-- Fix category accounts that were incorrectly created as "income" type
-- when they should be "expense" type (due to bug with liability account handling)
--
-- The bug: purchases on credit cards (liability accounts) were categorized
-- into income accounts instead of expense accounts.

-- Step 1: For income accounts that have a matching expense account with same name,
-- move entries from income to expense account
UPDATE entries e
SET account_id = expense_acc.id
FROM accounts income_acc
JOIN accounts expense_acc ON expense_acc.ledger_id = income_acc.ledger_id 
    AND expense_acc.name = income_acc.name 
    AND expense_acc.type = 'expense'
WHERE e.account_id = income_acc.id
AND income_acc.type = 'income'
AND income_acc.name != 'Uncategorized Income'
-- Only fix accounts that have entries from liability-originated transactions
AND EXISTS (
    SELECT 1 FROM entries e2
    JOIN accounts a2 ON e2.account_id = a2.id
    WHERE e2.transaction_id IN (SELECT transaction_id FROM entries WHERE account_id = income_acc.id)
    AND a2.type = 'liability'
);

-- Step 2: Delete the now-empty income accounts that had matching expense accounts
DELETE FROM accounts a
WHERE a.type = 'income'
AND a.name != 'Uncategorized Income'
AND EXISTS (
    SELECT 1 FROM accounts expense_acc 
    WHERE expense_acc.ledger_id = a.ledger_id 
    AND expense_acc.name = a.name 
    AND expense_acc.type = 'expense'
)
AND NOT EXISTS (SELECT 1 FROM entries WHERE account_id = a.id);

-- Step 3: For income accounts WITHOUT a matching expense account,
-- simply change their type to expense
UPDATE accounts a
SET type = 'expense', updated_at = NOW()
WHERE a.type = 'income'
AND a.name != 'Uncategorized Income'
-- Only fix accounts that have entries from liability-originated transactions
AND EXISTS (
    SELECT 1 FROM entries e
    JOIN entries e2 ON e2.transaction_id = e.transaction_id AND e2.account_id != a.id
    JOIN accounts a2 ON e2.account_id = a2.id
    WHERE e.account_id = a.id
    AND a2.type = 'liability'
)
-- And don't have a matching expense account
AND NOT EXISTS (
    SELECT 1 FROM accounts expense_acc 
    WHERE expense_acc.ledger_id = a.ledger_id 
    AND expense_acc.name = a.name 
    AND expense_acc.type = 'expense'
);

-- +goose Down
-- Note: This is a data fix migration. Rolling back would require knowing
-- which accounts were changed, which we don't track. No-op for safety.
