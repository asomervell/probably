-- +goose Up
-- Migration: Add API keys table for programmatic access
-- API keys are stored as SHA-256 hashes for security

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(16) NOT NULL, -- First chars of key for identification (e.g., "prob_abc12345")
    key_hash VARCHAR(64) NOT NULL,   -- SHA-256 hash of the full key
    last_used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);
CREATE UNIQUE INDEX idx_api_keys_key_hash ON api_keys(key_hash);

-- +goose Down
DROP TABLE IF EXISTS api_keys;

