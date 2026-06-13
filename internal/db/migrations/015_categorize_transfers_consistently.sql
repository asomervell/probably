-- +goose Up
-- Fix existing transfer pairs to use consistent "Internal Transfer" categorization.
-- Both sides of a transfer should use Internal Transfer (income or expense type),
-- which will net to $0 when both sides are visible.

-- Step 1: Create Internal Transfer accounts for each ledger that has transfers
INSERT INTO accounts (id, ledger_id, name, type, is_active, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    ledger_id,
    'Internal Transfer',
    'income',
    true,
    NOW(),
    NOW()
FROM (SELECT DISTINCT t.ledger_id FROM transactions t WHERE t.is_transfer = true) AS ledgers
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a 
    WHERE a.ledger_id = ledgers.ledger_id 
    AND a.name = 'Internal Transfer' 
    AND a.type = 'income'
);

INSERT INTO accounts (id, ledger_id, name, type, is_active, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    ledger_id,
    'Internal Transfer',
    'expense',
    true,
    NOW(),
    NOW()
FROM (SELECT DISTINCT t.ledger_id FROM transactions t WHERE t.is_transfer = true) AS ledgers
WHERE NOT EXISTS (
    SELECT 1 FROM accounts a 
    WHERE a.ledger_id = ledgers.ledger_id 
    AND a.name = 'Internal Transfer' 
    AND a.type = 'expense'
);

-- Step 2: Update contra entries for all transfer transactions
-- Change income-type contra entries to Internal Transfer (income)
UPDATE entries e
SET account_id = (
    SELECT a2.id 
    FROM accounts a2 
    JOIN transactions t ON t.ledger_id = a2.ledger_id
    WHERE t.id = e.transaction_id
    AND a2.name = 'Internal Transfer' 
    AND a2.type = 'income'
    LIMIT 1
)
FROM accounts a, transactions t
WHERE e.account_id = a.id
AND e.transaction_id = t.id
AND t.is_transfer = true
AND a.type = 'income'
AND a.name != 'Internal Transfer';

-- Change expense-type contra entries to Internal Transfer (expense)
UPDATE entries e
SET account_id = (
    SELECT a2.id 
    FROM accounts a2 
    JOIN transactions t ON t.ledger_id = a2.ledger_id
    WHERE t.id = e.transaction_id
    AND a2.name = 'Internal Transfer' 
    AND a2.type = 'expense'
    LIMIT 1
)
FROM accounts a, transactions t
WHERE e.account_id = a.id
AND e.transaction_id = t.id
AND t.is_transfer = true
AND a.type = 'expense'
AND a.name != 'Internal Transfer';

-- +goose Down
-- Revert Internal Transfer entries back to Uncategorized
UPDATE entries e
SET account_id = (
    SELECT a2.id 
    FROM accounts a2 
    JOIN transactions t ON t.ledger_id = a2.ledger_id
    WHERE t.id = e.transaction_id
    AND a2.name = 'Uncategorized Income' 
    AND a2.type = 'income'
    LIMIT 1
)
FROM accounts a, transactions t
WHERE e.account_id = a.id
AND e.transaction_id = t.id
AND t.is_transfer = true
AND a.name = 'Internal Transfer'
AND a.type = 'income';

UPDATE entries e
SET account_id = (
    SELECT a2.id 
    FROM accounts a2 
    JOIN transactions t ON t.ledger_id = a2.ledger_id
    WHERE t.id = e.transaction_id
    AND a2.name = 'Uncategorized Expenses' 
    AND a2.type = 'expense'
    LIMIT 1
)
FROM accounts a, transactions t
WHERE e.account_id = a.id
AND e.transaction_id = t.id
AND t.is_transfer = true
AND a.name = 'Internal Transfer'
AND a.type = 'expense';

