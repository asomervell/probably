-- +goose Up
-- Chat threads (supports future sub-threading via parent_thread_id)
CREATE TABLE chat_threads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_thread_id UUID REFERENCES chat_threads(id) ON DELETE CASCADE,
    title TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Chat messages
CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    thread_id UUID NOT NULL REFERENCES chat_threads(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    sql_query TEXT,
    results_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chat_threads_ledger ON chat_threads(ledger_id);
CREATE INDEX idx_chat_threads_user ON chat_threads(user_id);
CREATE INDEX idx_chat_threads_parent ON chat_threads(parent_thread_id);
CREATE INDEX idx_chat_threads_updated ON chat_threads(updated_at DESC);
CREATE INDEX idx_chat_messages_thread ON chat_messages(thread_id);

-- +goose Down
DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chat_threads;
