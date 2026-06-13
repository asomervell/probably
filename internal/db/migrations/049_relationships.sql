-- +goose Up
-- Relationships table for My Life feature
-- Stores user relationships to people, work entities, and assets

CREATE TABLE IF NOT EXISTS relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ledger_id UUID NOT NULL REFERENCES ledgers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(50) NOT NULL, -- person, work, asset
    relationship_type VARCHAR(50) NOT NULL, -- partner, employer, vehicle, etc.
    entity_id UUID REFERENCES entities(id) ON DELETE SET NULL, -- Optional link to entity
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_relationships_ledger_id ON relationships(ledger_id);
CREATE INDEX idx_relationships_category ON relationships(ledger_id, category);
CREATE INDEX idx_relationships_entity_id ON relationships(entity_id) WHERE entity_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS relationships;
