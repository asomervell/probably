-- +goose Up
CREATE TABLE IF NOT EXISTS oauth_token_api_keys (
    token_id UUID PRIMARY KEY REFERENCES oauth_tokens(id) ON DELETE CASCADE,
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    api_key_plaintext TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_token_api_keys_api_key_id
    ON oauth_token_api_keys(api_key_id);

-- +goose Down
DROP INDEX IF EXISTS idx_oauth_token_api_keys_api_key_id;
DROP TABLE IF EXISTS oauth_token_api_keys;
