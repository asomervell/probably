-- +goose Up
-- +goose StatementBegin

-- OAuth clients table (for Dynamic Client Registration)
CREATE TABLE IF NOT EXISTS oauth_clients (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    client_id VARCHAR(255) NOT NULL UNIQUE,
    client_secret VARCHAR(255), -- Optional for public clients with PKCE
    redirect_uris TEXT[] NOT NULL,
    scopes TEXT[],
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_clients_client_id ON oauth_clients(client_id);

-- OAuth tokens table (access tokens and refresh tokens)
CREATE TABLE IF NOT EXISTS oauth_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL,
    scopes TEXT[] NOT NULL,
    access_token VARCHAR(512) NOT NULL UNIQUE,
    refresh_token VARCHAR(512) UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_tokens_access_token ON oauth_tokens(access_token);
CREATE INDEX idx_oauth_tokens_user_id ON oauth_tokens(user_id);
CREATE INDEX idx_oauth_tokens_expires_at ON oauth_tokens(expires_at);

-- OAuth authorization codes table (for PKCE state management)
-- Note: In production, consider using Redis for short-lived state
CREATE TABLE IF NOT EXISTS oauth_authorization_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(255) NOT NULL UNIQUE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL,
    redirect_uri VARCHAR(512) NOT NULL,
    code_challenge VARCHAR(255) NOT NULL,
    code_challenge_method VARCHAR(10) NOT NULL DEFAULT 'S256',
    scopes TEXT[] NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_oauth_auth_codes_code ON oauth_authorization_codes(code);
CREATE INDEX idx_oauth_auth_codes_expires_at ON oauth_authorization_codes(expires_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS oauth_authorization_codes;
DROP TABLE IF EXISTS oauth_tokens;
DROP TABLE IF EXISTS oauth_clients;

-- +goose StatementEnd
