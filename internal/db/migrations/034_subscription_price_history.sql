-- +goose Up
-- +goose StatementBegin

-- Price history for subscriptions
-- Tracks every price point for a subscription over time
CREATE TABLE subscription_price_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recurring_id UUID NOT NULL REFERENCES detected_recurring(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL,
    effective_from DATE NOT NULL,
    effective_to DATE,  -- NULL = current price
    occurrence_count INT DEFAULT 1, -- How many times we saw this price
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_subscription_price_history_recurring_id ON subscription_price_history(recurring_id);
CREATE INDEX idx_subscription_price_history_current ON subscription_price_history(recurring_id, effective_to) 
    WHERE effective_to IS NULL;

-- Link transactions to their detected recurring pattern
-- This helps us track which transactions contributed to detection
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS detected_recurring_id UUID REFERENCES detected_recurring(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_transactions_detected_recurring_id ON transactions(detected_recurring_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_transactions_detected_recurring_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS detected_recurring_id;
DROP INDEX IF EXISTS idx_subscription_price_history_current;
DROP INDEX IF EXISTS idx_subscription_price_history_recurring_id;
DROP TABLE IF EXISTS subscription_price_history;

-- +goose StatementEnd
