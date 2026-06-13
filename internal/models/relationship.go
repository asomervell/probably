package models

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Relationship represents a user's relationship to a person, entity, or thing
type Relationship struct {
	ID               uuid.UUID  `json:"id"`
	LedgerID         uuid.UUID  `json:"ledger_id"`
	Name             string     `json:"name"`                // Display name (e.g., "Sarah", "My Tesla", "Acme Corp")
	Category         string     `json:"category"`            // person, work, asset
	RelationshipType string     `json:"relationship_type"`   // partner, employer, vehicle, etc.
	EntityID         *uuid.UUID `json:"entity_id,omitempty"` // Optional link to an entity
	Notes            string     `json:"notes,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// RelationshipStore handles database operations for relationships
type RelationshipStore struct {
	pool *pgxpool.Pool
}

// NewRelationshipStore creates a new RelationshipStore
func NewRelationshipStore(pool *pgxpool.Pool) *RelationshipStore {
	return &RelationshipStore{pool: pool}
}

// Create creates a new relationship
func (s *RelationshipStore) Create(ctx context.Context, rel *Relationship) error {
	if rel.ID == uuid.Nil {
		rel.ID = uuid.New()
	}
	rel.CreatedAt = time.Now()
	rel.UpdatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO relationships (id, ledger_id, name, category, relationship_type, entity_id, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, rel.ID, rel.LedgerID, rel.Name, rel.Category, rel.RelationshipType, rel.EntityID, rel.Notes, rel.CreatedAt, rel.UpdatedAt)

	return err
}

// GetByID retrieves a relationship by ID
func (s *RelationshipStore) GetByID(ctx context.Context, id uuid.UUID) (*Relationship, error) {
	rel := &Relationship{}
	err := s.pool.QueryRow(ctx, `
		SELECT id, ledger_id, name, category, relationship_type, entity_id, notes, created_at, updated_at
		FROM relationships WHERE id = $1
	`, id).Scan(&rel.ID, &rel.LedgerID, &rel.Name, &rel.Category, &rel.RelationshipType, &rel.EntityID, &rel.Notes, &rel.CreatedAt, &rel.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return rel, nil
}

// GetByLedgerID retrieves all relationships for a ledger
func (s *RelationshipStore) GetByLedgerID(ctx context.Context, ledgerID uuid.UUID) ([]*Relationship, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, name, category, relationship_type, entity_id, notes, created_at, updated_at
		FROM relationships WHERE ledger_id = $1
		ORDER BY category, name
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relationships []*Relationship
	for rows.Next() {
		rel := &Relationship{}
		if err := rows.Scan(&rel.ID, &rel.LedgerID, &rel.Name, &rel.Category, &rel.RelationshipType, &rel.EntityID, &rel.Notes, &rel.CreatedAt, &rel.UpdatedAt); err != nil {
			return nil, err
		}
		relationships = append(relationships, rel)
	}

	return relationships, rows.Err()
}

// Update updates a relationship
func (s *RelationshipStore) Update(ctx context.Context, rel *Relationship) error {
	rel.UpdatedAt = time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE relationships SET name = $2, category = $3, relationship_type = $4, entity_id = $5, notes = $6, updated_at = $7
		WHERE id = $1
	`, rel.ID, rel.Name, rel.Category, rel.RelationshipType, rel.EntityID, rel.Notes, rel.UpdatedAt)
	return err
}

// Delete deletes a relationship
func (s *RelationshipStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM relationships WHERE id = $1`, id)
	return err
}
