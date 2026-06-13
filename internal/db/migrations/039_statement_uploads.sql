-- +goose Up
-- +goose StatementBegin
CREATE TABLE statement_uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    account_id UUID REFERENCES accounts(id) ON DELETE SET NULL,
    original_filename TEXT NOT NULL,
    gcs_path TEXT NOT NULL,
    file_size_bytes BIGINT NOT NULL,
    content_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    extracted_count INTEGER DEFAULT 0,
    created_count INTEGER DEFAULT 0,
    error_message TEXT,
    processed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_statement_uploads_ledger_id ON statement_uploads(ledger_id);
CREATE INDEX idx_statement_uploads_account_id ON statement_uploads(account_id);
CREATE INDEX idx_statement_uploads_status ON statement_uploads(status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_statement_uploads_status;
DROP INDEX IF EXISTS idx_statement_uploads_account_id;
DROP INDEX IF EXISTS idx_statement_uploads_ledger_id;
DROP TABLE IF EXISTS statement_uploads;
-- +goose StatementEnd
