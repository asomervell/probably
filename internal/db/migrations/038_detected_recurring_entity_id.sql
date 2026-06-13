-- +goose Up
-- Rename merchant_id to entity_id in detected_recurring table
-- to align with the entities migration (036)

-- Drop the existing unique index that references merchant_id
DROP INDEX IF EXISTS idx_detected_recurring_ledger_merchant_amount;

-- Rename the column
ALTER TABLE detected_recurring RENAME COLUMN merchant_id TO entity_id;

-- Recreate the unique index with the new column name
CREATE UNIQUE INDEX idx_detected_recurring_ledger_entity_amount
    ON detected_recurring(ledger_id, entity_id, amount_bucket)
    WHERE entity_id IS NOT NULL;

-- +goose Down
-- Revert: rename entity_id back to merchant_id

DROP INDEX IF EXISTS idx_detected_recurring_ledger_entity_amount;

ALTER TABLE detected_recurring RENAME COLUMN entity_id TO merchant_id;

CREATE UNIQUE INDEX idx_detected_recurring_ledger_merchant_amount
    ON detected_recurring(ledger_id, merchant_id, amount_bucket)
    WHERE merchant_id IS NOT NULL;
