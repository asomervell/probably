-- +goose Up
-- +goose StatementBegin

-- Step 1: Create a "person" entity for each existing user
-- Use a unique identifier to avoid conflicts
INSERT INTO entities (id, type, subtype, name, slug, user_verified, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'person',
    'individual',
    COALESCE(u.email, 'User ' || u.id::text),
    'user-' || u.id::text,
    true,
    u.created_at,
    u.updated_at
FROM users u
WHERE NOT EXISTS (
    SELECT 1 FROM entities e 
    JOIN user_entity_permissions uep ON e.id = uep.entity_id
    WHERE uep.user_id = u.id AND e.type = 'person' AND e.subtype = 'individual'
);

-- Step 2: Link existing ledgers to user's person entity via entity_ledgers
INSERT INTO entity_ledgers (id, entity_id, ledger_id, role, created_at)
SELECT 
    gen_random_uuid(),
    e.id,
    l.id,
    'owner',
    l.created_at
FROM ledgers l
JOIN users u ON l.user_id = u.id
JOIN entities e ON e.type = 'person' 
    AND e.subtype = 'individual'
    AND e.name = COALESCE(u.email, 'User ' || u.id::text)
WHERE NOT EXISTS (
    SELECT 1 FROM entity_ledgers el WHERE el.ledger_id = l.id
);

-- Step 3: Grant "owner" permission to each user for their person entity
INSERT INTO user_entity_permissions (id, user_id, entity_id, permission_level, granted_by, granted_at, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    u.id,
    e.id,
    'owner',
    u.id,
    NOW(),
    u.created_at,
    u.updated_at
FROM users u
JOIN entities e ON e.type = 'person' 
    AND e.subtype = 'individual'
    AND e.name = COALESCE(u.email, 'User ' || u.id::text)
WHERE NOT EXISTS (
    SELECT 1 FROM user_entity_permissions uep 
    WHERE uep.user_id = u.id AND uep.entity_id = e.id
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove permissions and entity-ledger links
DELETE FROM user_entity_permissions;
DELETE FROM entity_ledgers;

-- Note: We don't delete the person entities created above, as they might be referenced elsewhere
-- If needed, they can be cleaned up separately

-- +goose StatementEnd
