-- +goose Up
-- After Plaid reconnect, generic Chase account names (no •••• disambiguation) can remain as empty
-- shells while new rows hold the real link. Remove only when there is no ledger history and no
-- active Plaid credentials on the row, scoped to Chase / JPMorgan institution names.
DELETE FROM accounts a
WHERE TRIM(a.name) IN ('TOTAL CHECKING', 'CREDIT CARD')
  AND (
    a.institution_name ILIKE '%chase%'
    OR a.institution_name ILIKE '%jpmorgan%'
  )
  AND NOT EXISTS (SELECT 1 FROM entries e WHERE e.account_id = a.id)
  AND COALESCE(a.external_account_id, '') = ''
  AND COALESCE(a.connection_id, '') = ''
  AND COALESCE(a.access_token, '') = '';

-- +goose Down
SELECT 1;
