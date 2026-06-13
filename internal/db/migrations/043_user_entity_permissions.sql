-- +goose Up
-- +goose StatementBegin

-- Create user_entity_permissions table
CREATE TABLE user_entity_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    permission_level VARCHAR(20) NOT NULL CHECK (permission_level IN ('owner', 'edit', 'view')),
    granted_by UUID REFERENCES users(id),
    granted_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, entity_id)
);

CREATE INDEX idx_user_entity_permissions_user_id ON user_entity_permissions(user_id);
CREATE INDEX idx_user_entity_permissions_entity_id ON user_entity_permissions(entity_id);
CREATE INDEX idx_user_entity_permissions_level ON user_entity_permissions(permission_level);

-- Create entity_ledgers table (entities own ledgers)
CREATE TABLE entity_ledgers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    role VARCHAR(50) DEFAULT 'owner',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(entity_id, ledger_id)
);

CREATE INDEX idx_entity_ledgers_entity_id ON entity_ledgers(entity_id);
CREATE INDEX idx_entity_ledgers_ledger_id ON entity_ledgers(ledger_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS entity_ledgers;
DROP TABLE IF EXISTS user_entity_permissions;

-- +goose StatementEnd
