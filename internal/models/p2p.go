package models

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HouseholdMember represents a person in the user's household/family
// whose transfers should be treated as household transfers (not expenses)
type HouseholdMember struct {
	ID           uuid.UUID `json:"id"`
	LedgerID     uuid.UUID `json:"ledger_id"`
	NamePattern  string    `json:"name_pattern"` // Pattern to match counterparty names
	Relationship string    `json:"relationship"` // 'spouse', 'self', 'family', 'partner', etc.
	Notes        string    `json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// HouseholdMemberStore handles database operations for household members
type HouseholdMemberStore struct {
	pool *pgxpool.Pool
}

// NewHouseholdMemberStore creates a new HouseholdMemberStore
func NewHouseholdMemberStore(pool *pgxpool.Pool) *HouseholdMemberStore {
	return &HouseholdMemberStore{pool: pool}
}

// GetByLedgerID returns all household members for a ledger
func (s *HouseholdMemberStore) GetByLedgerID(ctx context.Context, ledgerID uuid.UUID) ([]*HouseholdMember, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ledger_id, name_pattern, relationship, notes, created_at, updated_at
		FROM household_members WHERE ledger_id = $1
		ORDER BY name_pattern
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*HouseholdMember
	for rows.Next() {
		var m HouseholdMember
		var relationship, notes sql.NullString

		if err := rows.Scan(&m.ID, &m.LedgerID, &m.NamePattern, &relationship, &notes,
			&m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}

		m.Relationship = relationship.String
		m.Notes = notes.String
		members = append(members, &m)
	}

	return members, rows.Err()
}
