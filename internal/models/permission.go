package models

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PermissionLevel represents the level of access
type PermissionLevel string

const (
	PermissionLevelOwner PermissionLevel = "owner"
	PermissionLevelEdit  PermissionLevel = "edit"
	PermissionLevelView  PermissionLevel = "view"
)

// UserEntityPermission represents a user's permission on an entity
type UserEntityPermission struct {
	ID              uuid.UUID       `json:"id"`
	UserID          uuid.UUID       `json:"user_id"`
	EntityID        uuid.UUID       `json:"entity_id"`
	PermissionLevel PermissionLevel `json:"permission_level"`
	GrantedBy       *uuid.UUID      `json:"granted_by,omitempty"`
	GrantedAt       time.Time       `json:"granted_at"`
	ExpiresAt       *time.Time      `json:"expires_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// EntityLedger represents the relationship between an entity and a ledger
type EntityLedger struct {
	ID        uuid.UUID `json:"id"`
	EntityID  uuid.UUID `json:"entity_id"`
	LedgerID  uuid.UUID `json:"ledger_id"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// PermissionStore handles database operations for permissions
type PermissionStore struct {
	pool *pgxpool.Pool
}

// NewPermissionStore creates a new PermissionStore
func NewPermissionStore(pool *pgxpool.Pool) *PermissionStore {
	return &PermissionStore{pool: pool}
}

// CreateUserEntityPermission creates a new user-entity permission
func (s *PermissionStore) CreateUserEntityPermission(ctx context.Context, perm *UserEntityPermission) error {
	if perm.ID == uuid.Nil {
		perm.ID = uuid.New()
	}
	now := time.Now()
	perm.CreatedAt = now
	perm.UpdatedAt = now
	if perm.GrantedAt.IsZero() {
		perm.GrantedAt = now
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_entity_permissions (id, user_id, entity_id, permission_level, granted_by, granted_at, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, perm.ID, perm.UserID, perm.EntityID, perm.PermissionLevel, perm.GrantedBy, perm.GrantedAt, perm.ExpiresAt, perm.CreatedAt, perm.UpdatedAt)

	return err
}

// GetUserEntityPermission gets a user's permission on an entity
func (s *PermissionStore) GetUserEntityPermission(ctx context.Context, userID, entityID uuid.UUID) (*UserEntityPermission, error) {
	var perm UserEntityPermission
	var grantedBy sql.NullString
	var expiresAt sql.NullTime

	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, entity_id, permission_level, granted_by, granted_at, expires_at, created_at, updated_at
		FROM user_entity_permissions
		WHERE user_id = $1 AND entity_id = $2
		AND (expires_at IS NULL OR expires_at > NOW())
	`, userID, entityID).Scan(
		&perm.ID, &perm.UserID, &perm.EntityID, &perm.PermissionLevel,
		&grantedBy, &perm.GrantedAt, &expiresAt, &perm.CreatedAt, &perm.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if grantedBy.Valid {
		id, _ := uuid.Parse(grantedBy.String)
		perm.GrantedBy = &id
	}
	if expiresAt.Valid {
		perm.ExpiresAt = &expiresAt.Time
	}

	return &perm, nil
}

// GetUserEntityPermissions gets all permissions for a user
func (s *PermissionStore) GetUserEntityPermissions(ctx context.Context, userID uuid.UUID) ([]*UserEntityPermission, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, entity_id, permission_level, granted_by, granted_at, expires_at, created_at, updated_at
		FROM user_entity_permissions
		WHERE user_id = $1
		AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []*UserEntityPermission
	for rows.Next() {
		var perm UserEntityPermission
		var grantedBy sql.NullString
		var expiresAt sql.NullTime

		if err := rows.Scan(
			&perm.ID, &perm.UserID, &perm.EntityID, &perm.PermissionLevel,
			&grantedBy, &perm.GrantedAt, &expiresAt, &perm.CreatedAt, &perm.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if grantedBy.Valid {
			id, _ := uuid.Parse(grantedBy.String)
			perm.GrantedBy = &id
		}
		if expiresAt.Valid {
			perm.ExpiresAt = &expiresAt.Time
		}

		perms = append(perms, &perm)
	}

	return perms, rows.Err()
}

// DeleteUserEntityPermission removes a user's permission on an entity
func (s *PermissionStore) DeleteUserEntityPermission(ctx context.Context, userID, entityID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM user_entity_permissions
		WHERE user_id = $1 AND entity_id = $2
	`, userID, entityID)
	return err
}

// CreateEntityLedger creates a new entity-ledger relationship
func (s *PermissionStore) CreateEntityLedger(ctx context.Context, el *EntityLedger) error {
	if el.ID == uuid.Nil {
		el.ID = uuid.New()
	}
	el.CreatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO entity_ledgers (id, entity_id, ledger_id, role, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, el.ID, el.EntityID, el.LedgerID, el.Role, el.CreatedAt)

	return err
}

// GetEntityLedgers gets all ledgers for an entity
func (s *PermissionStore) GetEntityLedgers(ctx context.Context, entityID uuid.UUID) ([]*EntityLedger, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, entity_id, ledger_id, role, created_at
		FROM entity_ledgers
		WHERE entity_id = $1
		ORDER BY created_at ASC
	`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var els []*EntityLedger
	for rows.Next() {
		var el EntityLedger
		if err := rows.Scan(&el.ID, &el.EntityID, &el.LedgerID, &el.Role, &el.CreatedAt); err != nil {
			return nil, err
		}
		els = append(els, &el)
	}

	return els, rows.Err()
}

// GetLedgerEntities gets all entities for a ledger
func (s *PermissionStore) GetLedgerEntities(ctx context.Context, ledgerID uuid.UUID) ([]*EntityLedger, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, entity_id, ledger_id, role, created_at
		FROM entity_ledgers
		WHERE ledger_id = $1
		ORDER BY created_at ASC
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var els []*EntityLedger
	for rows.Next() {
		var el EntityLedger
		if err := rows.Scan(&el.ID, &el.EntityID, &el.LedgerID, &el.Role, &el.CreatedAt); err != nil {
			return nil, err
		}
		els = append(els, &el)
	}

	return els, rows.Err()
}

// DeleteEntityLedger removes an entity-ledger relationship
func (s *PermissionStore) DeleteEntityLedger(ctx context.Context, entityID, ledgerID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM entity_ledgers
		WHERE entity_id = $1 AND ledger_id = $2
	`, entityID, ledgerID)
	return err
}
