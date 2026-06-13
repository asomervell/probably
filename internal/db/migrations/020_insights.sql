-- +goose Up
-- Financial insights and reports tables

-- Cached periodic reports (monthly, quarterly, annual)
CREATE TABLE reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    report_type VARCHAR(20) NOT NULL, -- 'monthly', 'quarterly', 'annual'
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    
    -- Aggregated metrics (for quick access without LLM)
    total_income_cents BIGINT,
    total_expenses_cents BIGINT,
    net_savings_cents BIGINT,
    category_breakdown JSONB, -- {category: amount_cents}
    top_merchants JSONB,      -- [{merchant_id, name, amount_cents, count}]
    
    -- LLM-generated content
    summary TEXT,             -- Natural language summary
    highlights JSONB,         -- Key observations
    recommendations JSONB,    -- Actionable advice
    comparison_notes TEXT,    -- vs previous period
    
    -- Generation metadata
    llm_provider VARCHAR(50), -- Which provider generated this
    llm_model VARCHAR(100),
    generated_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(ledger_id, report_type, period_start)
);

-- Individual insights (transaction-level and general)
CREATE TABLE insights (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    transaction_id UUID REFERENCES transactions(id) ON DELETE CASCADE, -- NULL for general insights
    report_id UUID REFERENCES reports(id) ON DELETE CASCADE,           -- Link to source report
    
    insight_type VARCHAR(30) NOT NULL, -- 'transaction', 'spending_alert', 'trend', 'recommendation', 'anomaly'
    content TEXT NOT NULL,
    importance INTEGER DEFAULT 5 CHECK (importance >= 1 AND importance <= 10), -- 1-10 scale
    is_key BOOLEAN DEFAULT FALSE,      -- Surface to dashboard
    is_dismissed BOOLEAN DEFAULT FALSE,
    
    -- Structured data for filtering/display
    metadata JSONB,  -- Flexible: amounts, categories, merchants involved
    
    -- Generation metadata
    llm_provider VARCHAR(50),
    llm_model VARCHAR(100),
    
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for efficient queries
CREATE INDEX idx_insights_ledger_key ON insights(ledger_id, is_key) WHERE NOT is_dismissed;
CREATE INDEX idx_insights_ledger_importance ON insights(ledger_id, importance DESC) WHERE NOT is_dismissed;
CREATE INDEX idx_insights_transaction ON insights(transaction_id) WHERE transaction_id IS NOT NULL;
CREATE INDEX idx_insights_report ON insights(report_id) WHERE report_id IS NOT NULL;
CREATE INDEX idx_insights_type ON insights(ledger_id, insight_type);
CREATE INDEX idx_insights_created ON insights(ledger_id, created_at DESC);

CREATE INDEX idx_reports_ledger_period ON reports(ledger_id, report_type, period_start DESC);
CREATE INDEX idx_reports_ledger_type ON reports(ledger_id, report_type);

-- +goose Down
DROP TABLE IF EXISTS insights;
DROP TABLE IF EXISTS reports;
