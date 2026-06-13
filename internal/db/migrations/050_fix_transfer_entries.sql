-- +goose Up
-- Fix transfer entries to use equity account instead of income/expense
-- This ensures transfers don't pollute income/expense reports

-- Step 1: Create equity "Internal Transfer" account for each ledger that has transfers
INSERT INTO accounts (id, ledger_id, name, type, is_active, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    l.id,
    'Internal Transfer',
    'equity',
    true,
    NOW(),
    NOW()
FROM ledgers l
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a 
    WHERE a.ledger_id = l.id 
    AND a.name = 'Internal Transfer' 
    AND a.type = 'equity'
);

-- Step 2: Move all transfer entries from old Internal Transfer (income/expense) to new equity account
-- For each ledger, update entries that:
--   1. Belong to a transaction marked as a transfer
--   2. Currently point to an "Internal Transfer" income or expense account
UPDATE entries
SET account_id = equity_acc.id
FROM transactions t, accounts old_acc, accounts equity_acc
WHERE entries.transaction_id = t.id
    AND entries.account_id = old_acc.id
    AND t.is_transfer = true
    AND old_acc.name = 'Internal Transfer'
    AND old_acc.type IN ('income', 'expense')
    AND equity_acc.ledger_id = t.ledger_id 
    AND equity_acc.name = 'Internal Transfer' 
    AND equity_acc.type = 'equity';

-- Step 3: Deactivate (soft delete) the old income/expense Internal Transfer accounts
-- Don't hard delete in case there are references we missed
UPDATE accounts
SET is_active = false, updated_at = NOW()
WHERE name = 'Internal Transfer'
    AND type IN ('income', 'expense');

-- +goose Down
-- Restore entries to original Internal Transfer income/expense accounts
-- This is a best-effort reversal - we recreate the accounts and move entries back

-- Step 1: Reactivate the old income/expense Internal Transfer accounts
UPDATE accounts
SET is_active = true, updated_at = NOW()
WHERE name = 'Internal Transfer'
    AND type IN ('income', 'expense');

-- Step 2: Create income/expense Internal Transfer accounts if they don't exist
INSERT INTO accounts (id, ledger_id, name, type, is_active, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    l.id,
    'Internal Transfer',
    'income',
    true,
    NOW(),
    NOW()
FROM ledgers l
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a 
    WHERE a.ledger_id = l.id 
    AND a.name = 'Internal Transfer' 
    AND a.type = 'income'
);

INSERT INTO accounts (id, ledger_id, name, type, is_active, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    l.id,
    'Internal Transfer',
    'expense',
    true,
    NOW(),
    NOW()
FROM ledgers l
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a 
    WHERE a.ledger_id = l.id 
    AND a.name = 'Internal Transfer' 
    AND a.type = 'expense'
);

-- Step 3: Move entries back based on sign
-- Positive amounts (money arriving) go to income, negative (money leaving) go to expense
UPDATE entries
SET account_id = CASE 
    WHEN entries.amount_cents > 0 THEN income_acc.id
    ELSE expense_acc.id
END
FROM transactions t, accounts equity_acc, accounts income_acc, accounts expense_acc
WHERE entries.transaction_id = t.id
    AND entries.account_id = equity_acc.id
    AND t.is_transfer = true
    AND equity_acc.name = 'Internal Transfer'
    AND equity_acc.type = 'equity'
    AND income_acc.ledger_id = t.ledger_id 
    AND income_acc.name = 'Internal Transfer' 
    AND income_acc.type = 'income'
    AND expense_acc.ledger_id = t.ledger_id 
    AND expense_acc.name = 'Internal Transfer' 
    AND expense_acc.type = 'expense';

-- Step 4: Deactivate the equity Internal Transfer accounts
UPDATE accounts
SET is_active = false, updated_at = NOW()
WHERE name = 'Internal Transfer'
    AND type = 'equity';
