-- +goose Up
-- Fix entries that were corrupted by the buggy SetTransferPair logic.
-- The old code was rewriting entry account_ids to point to the paired transaction's
-- asset/liability account, which corrupted the ledger and caused incorrect balances.
--
-- This migration identifies corrupted entries and restores them to point to the
-- appropriate Uncategorized Income or Uncategorized Expense account.

-- Step 1: Identify corrupted entries
-- A corrupted entry is one where:
-- 1. The transaction is marked as a transfer (is_transfer = true)
-- 2. The entry points to an asset or liability account
-- 3. BUT there's another entry in the same transaction that ALSO points to an asset/liability account
--    (normally one entry points to asset/liability, the other to income/expense)

-- Create a temp table to hold entries that need to be fixed
CREATE TEMP TABLE entries_to_fix AS
WITH transfer_entries AS (
    -- Get all entries for transfer transactions with their account types
    SELECT 
        e.id as entry_id,
        e.transaction_id,
        e.account_id,
        e.amount_cents,
        a.type as account_type,
        a.ledger_id,
        t.transfer_pair_id
    FROM entries e
    JOIN transactions t ON e.transaction_id = t.id
    JOIN accounts a ON e.account_id = a.id
    WHERE t.is_transfer = true
),
problematic_transactions AS (
    -- Find transactions where BOTH entries point to asset/liability accounts
    -- This is the corruption signature
    SELECT 
        te.transaction_id,
        COUNT(*) FILTER (WHERE te.account_type IN ('asset', 'liability')) as asset_liability_count
    FROM transfer_entries te
    GROUP BY te.transaction_id
    HAVING COUNT(*) FILTER (WHERE te.account_type IN ('asset', 'liability')) > 1
)
SELECT 
    te.entry_id,
    te.transaction_id,
    te.account_id,
    te.amount_cents,
    te.account_type,
    te.ledger_id,
    te.transfer_pair_id
FROM transfer_entries te
JOIN problematic_transactions pt ON te.transaction_id = pt.transaction_id;

-- Step 2: For each corrupted transaction, identify which entry is the "main" one
-- and which one is the "contra" that got corrupted
-- The "main" entry is the one that matches an entry in the paired transaction's account
-- The "contra" entry is the one that should point to income/expense

-- Create temp table to map corrupted entries to their correct target accounts
CREATE TEMP TABLE entry_fixes AS
WITH paired_accounts AS (
    -- For each entry, check if its account_id matches any entry in the paired transaction
    SELECT 
        etf.entry_id,
        etf.transaction_id,
        etf.account_id,
        etf.amount_cents,
        etf.ledger_id,
        etf.transfer_pair_id,
        -- Check if this entry's account appears in the paired transaction
        EXISTS (
            SELECT 1 FROM entries e2 
            WHERE e2.transaction_id = etf.transfer_pair_id 
            AND e2.account_id = etf.account_id
        ) as is_cross_reference
    FROM entries_to_fix etf
    WHERE etf.transfer_pair_id IS NOT NULL
),
uncategorized_accounts AS (
    -- Get the uncategorized income/expense accounts for each ledger
    SELECT 
        id as account_id,
        ledger_id,
        type as account_type,
        name
    FROM accounts
    WHERE name IN ('Uncategorized Income', 'Uncategorized Expenses')
)
SELECT 
    pa.entry_id,
    pa.transaction_id,
    pa.amount_cents,
    pa.ledger_id,
    -- If this is a cross-reference (points to the paired account), it's the corrupted one
    -- Restore it based on amount: positive = expense, negative = income (for contra entries)
    CASE 
        WHEN pa.is_cross_reference AND pa.amount_cents > 0 THEN 
            (SELECT account_id FROM uncategorized_accounts ua WHERE ua.ledger_id = pa.ledger_id AND ua.account_type = 'expense' LIMIT 1)
        WHEN pa.is_cross_reference AND pa.amount_cents < 0 THEN 
            (SELECT account_id FROM uncategorized_accounts ua WHERE ua.ledger_id = pa.ledger_id AND ua.account_type = 'income' LIMIT 1)
        ELSE NULL
    END as new_account_id
FROM paired_accounts pa
WHERE pa.is_cross_reference = true;

-- Step 3: Apply the fixes
UPDATE entries e
SET account_id = ef.new_account_id
FROM entry_fixes ef
WHERE e.id = ef.entry_id
AND ef.new_account_id IS NOT NULL;

-- Step 4: Clean up temp tables
DROP TABLE IF EXISTS entries_to_fix;
DROP TABLE IF EXISTS entry_fixes;

-- Step 5: Delete any orphaned Opening Balance transactions that now have incorrect amounts
-- After fixing entries, opening balances may be wrong. We'll delete them so they get recalculated on resync.
DELETE FROM entries 
WHERE transaction_id IN (
    SELECT t.id FROM transactions t WHERE t.description = 'Opening Balance'
);

DELETE FROM transactions WHERE description = 'Opening Balance';

-- +goose Down
-- Cannot reliably reverse this migration as it fixes data corruption
-- A resync would be needed to restore correct state
SELECT 1;

