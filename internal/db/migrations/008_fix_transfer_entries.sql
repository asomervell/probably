-- +goose Up
-- +goose StatementBegin

-- Unlink all transfers so they can be properly re-matched after liability sign fixes.
-- After this migration, user should do "Full Resync" on liability accounts (credit cards)
-- to re-import transactions with correct signs and re-run transfer matching.

-- Unlink all transfers
UPDATE transactions 
SET is_transfer = false, transfer_pair_id = NULL 
WHERE is_transfer = true;

-- Clear pending transfer matches (if table exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'pending_transfer_matches') THEN
        DELETE FROM pending_transfer_matches;
    END IF;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Cannot reverse - transfers would need to be manually re-matched
SELECT 1;
-- +goose StatementEnd

