-- +goose Up
-- +goose StatementBegin

-- Fix existing categorized transactions: move entries from Uncategorized to proper category accounts
--
-- Problem: Transactions have been tagged with categories (via transaction_tags), but their
-- entries still point to "Uncategorized Income" or "Uncategorized Expenses" accounts.
-- This migration creates category-specific accounts and updates entries to point to them.

-- Step 1: Create category accounts for all tags that have categorized transactions
-- For each tag used on a transaction, we may need both expense and income accounts
-- (e.g., "Food Delivery" as expense when you buy food, as income if you work for DoorDash)

-- Create expense accounts for categories where transactions have expense entries
INSERT INTO accounts (id, ledger_id, name, type, is_active, created_at, updated_at)
SELECT DISTINCT
    uuid_generate_v4(),
    t.ledger_id,
    t.name,
    'expense'::account_type,
    true,
    NOW(),
    NOW()
FROM tags t
JOIN transaction_tags tt ON t.id = tt.tag_id
JOIN entries e ON tt.transaction_id = e.transaction_id
JOIN accounts a ON e.account_id = a.id
WHERE a.type = 'expense'::account_type
  AND a.name = 'Uncategorized Expenses'
  AND NOT EXISTS (
      SELECT 1 FROM accounts existing 
      WHERE existing.ledger_id = t.ledger_id 
        AND existing.name = t.name 
        AND existing.type = 'expense'::account_type
  );

-- Create income accounts for categories where transactions have income entries
INSERT INTO accounts (id, ledger_id, name, type, is_active, created_at, updated_at)
SELECT DISTINCT
    uuid_generate_v4(),
    t.ledger_id,
    t.name,
    'income'::account_type,
    true,
    NOW(),
    NOW()
FROM tags t
JOIN transaction_tags tt ON t.id = tt.tag_id
JOIN entries e ON tt.transaction_id = e.transaction_id
JOIN accounts a ON e.account_id = a.id
WHERE a.type = 'income'::account_type
  AND a.name = 'Uncategorized Income'
  AND NOT EXISTS (
      SELECT 1 FROM accounts existing 
      WHERE existing.ledger_id = t.ledger_id 
        AND existing.name = t.name 
        AND existing.type = 'income'::account_type
  );

-- Step 2: Update entries to point to their category accounts
-- For expense entries
UPDATE entries
SET account_id = category_acc.id
FROM entries e
JOIN accounts current_acc ON e.account_id = current_acc.id
JOIN transaction_tags tt ON e.transaction_id = tt.transaction_id
JOIN tags t ON tt.tag_id = t.id
JOIN accounts category_acc ON category_acc.ledger_id = t.ledger_id 
                           AND category_acc.name = t.name 
                           AND category_acc.type = 'expense'::account_type
WHERE entries.id = e.id
  AND current_acc.type = 'expense'::account_type
  AND current_acc.name = 'Uncategorized Expenses';

-- For income entries  
UPDATE entries
SET account_id = category_acc.id
FROM entries e
JOIN accounts current_acc ON e.account_id = current_acc.id
JOIN transaction_tags tt ON e.transaction_id = tt.transaction_id
JOIN tags t ON tt.tag_id = t.id
JOIN accounts category_acc ON category_acc.ledger_id = t.ledger_id 
                           AND category_acc.name = t.name 
                           AND category_acc.type = 'income'::account_type
WHERE entries.id = e.id
  AND current_acc.type = 'income'::account_type
  AND current_acc.name = 'Uncategorized Income';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Note: We can't reliably reverse this because we don't know which entries
-- were originally uncategorized. This down migration moves all categorized
-- entries back to uncategorized, which may affect transactions that were
-- manually created with specific categories.

-- Move expense entries back to Uncategorized Expenses
UPDATE entries
SET account_id = uncat.id
FROM entries e
JOIN accounts a ON e.account_id = a.id
JOIN accounts uncat ON a.ledger_id = uncat.ledger_id 
                    AND uncat.name = 'Uncategorized Expenses' 
                    AND uncat.type = 'expense'::account_type
WHERE entries.id = e.id
  AND a.type = 'expense'::account_type
  AND a.name != 'Uncategorized Expenses'
  AND EXISTS (SELECT 1 FROM transaction_tags tt WHERE tt.transaction_id = e.transaction_id);

-- Move income entries back to Uncategorized Income
UPDATE entries
SET account_id = uncat.id
FROM entries e
JOIN accounts a ON e.account_id = a.id
JOIN accounts uncat ON a.ledger_id = uncat.ledger_id 
                    AND uncat.name = 'Uncategorized Income' 
                    AND uncat.type = 'income'::account_type
WHERE entries.id = e.id
  AND a.type = 'income'::account_type
  AND a.name != 'Uncategorized Income'
  AND EXISTS (SELECT 1 FROM transaction_tags tt WHERE tt.transaction_id = e.transaction_id);

-- Note: We don't delete the category accounts in down because they might be used elsewhere

-- +goose StatementEnd

