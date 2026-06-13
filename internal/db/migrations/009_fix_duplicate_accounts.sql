-- +goose Up
-- +goose StatementBegin

-- Fix duplicate accounts created by broken migration 007
-- The bug: SELECT DISTINCT with uuid_generate_v4() generates a unique UUID per row
-- BEFORE DISTINCT is applied, so it creates duplicate accounts

-- Step 1: Update entries to point to the canonical account (lowest ID for each ledger_id, name, type)
UPDATE entries e
SET account_id = canonical.id
FROM accounts dup
JOIN (
    -- Find the "canonical" account for each (ledger_id, name, type) combination
    -- We pick the one with the lowest created_at (first created)
    SELECT DISTINCT ON (ledger_id, name, type) 
           id, ledger_id, name, type
    FROM accounts
    ORDER BY ledger_id, name, type, created_at ASC
) canonical ON dup.ledger_id = canonical.ledger_id 
           AND dup.name = canonical.name 
           AND dup.type = canonical.type
WHERE e.account_id = dup.id
  AND dup.id != canonical.id;

-- Step 2: Delete duplicate accounts (keep only the canonical one)
DELETE FROM accounts
WHERE id IN (
    SELECT a.id
    FROM accounts a
    JOIN (
        SELECT DISTINCT ON (ledger_id, name, type) 
               id as canonical_id, ledger_id, name, type
        FROM accounts
        ORDER BY ledger_id, name, type, created_at ASC
    ) canonical ON a.ledger_id = canonical.ledger_id 
               AND a.name = canonical.name 
               AND a.type = canonical.type
    WHERE a.id != canonical.canonical_id
);

-- Step 3: Add unique constraint to prevent future duplicates
-- This ensures (ledger_id, name, type) is unique
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_ledger_name_type_unique 
ON accounts (ledger_id, name, type);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove the unique constraint
DROP INDEX IF EXISTS idx_accounts_ledger_name_type_unique;

-- Note: Cannot restore deleted duplicate accounts

-- +goose StatementEnd

