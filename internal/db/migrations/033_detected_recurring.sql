-- +goose Up
-- +goose StatementBegin

-- Core recurring detection table
-- Detects recurring charges from transaction patterns
CREATE TABLE detected_recurring (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    merchant_id UUID REFERENCES merchants(id) ON DELETE SET NULL,
    description_pattern TEXT,  -- Pattern matched (for non-merchant based detection)
    
    -- Detection results
    frequency TEXT,             -- weekly, biweekly, monthly, quarterly, annual
    interval_days INT,          -- Average days between occurrences
    avg_amount_cents BIGINT,    -- Average transaction amount
    amount_variance_cents BIGINT, -- How much the amount varies
    confidence_score INT,       -- 0-100 confidence in detection
    occurrence_count INT DEFAULT 0, -- How many times we've seen this pattern
    
    -- Prediction
    last_occurrence_date DATE,
    next_expected_date DATE,
    next_expected_amount_cents BIGINT,
    
    -- User overrides
    is_active BOOLEAN DEFAULT true,
    user_adjusted_amount_cents BIGINT,
    user_adjusted_date DATE,
    
    -- Classification
    bill_type TEXT,             -- subscription, utility, rent, insurance, other
    is_subscription BOOLEAN DEFAULT false,  -- True for fixed-amount subscriptions
    
    -- Price change tracking
    previous_amount_cents BIGINT,
    amount_changed_at DATE,
    price_change_acknowledged BOOLEAN DEFAULT false,
    
    -- Subscription management
    cancelled_at DATE,
    tracking_paused BOOLEAN DEFAULT false,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_detected_recurring_ledger_id ON detected_recurring(ledger_id);
CREATE INDEX idx_detected_recurring_merchant_id ON detected_recurring(merchant_id);
CREATE INDEX idx_detected_recurring_is_subscription ON detected_recurring(ledger_id, is_subscription) WHERE is_subscription = true;
CREATE INDEX idx_detected_recurring_price_change ON detected_recurring(ledger_id, price_change_acknowledged) 
    WHERE price_change_acknowledged = false AND previous_amount_cents IS NOT NULL;
CREATE INDEX idx_detected_recurring_next_date ON detected_recurring(ledger_id, next_expected_date);

-- Unique constraint: one recurring pattern per merchant per ledger
CREATE UNIQUE INDEX idx_detected_recurring_ledger_merchant ON detected_recurring(ledger_id, merchant_id) 
    WHERE merchant_id IS NOT NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_detected_recurring_ledger_merchant;
DROP INDEX IF EXISTS idx_detected_recurring_next_date;
DROP INDEX IF EXISTS idx_detected_recurring_price_change;
DROP INDEX IF EXISTS idx_detected_recurring_is_subscription;
DROP INDEX IF EXISTS idx_detected_recurring_merchant_id;
DROP INDEX IF EXISTS idx_detected_recurring_ledger_id;
DROP TABLE IF EXISTS detected_recurring;

-- +goose StatementEnd
