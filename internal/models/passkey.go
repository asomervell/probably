package models

import (
	"context"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Passkey represents a WebAuthn credential stored for a user
type Passkey struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	CredentialID    []byte     `json:"-"`
	PublicKey       []byte     `json:"-"`
	AttestationType string     `json:"attestation_type"`
	Transport       []string   `json:"transport"`
	AAGUID          []byte     `json:"-"`
	SignCount       uint32     `json:"sign_count"`
	Name            string     `json:"name"`
	LastUsedAt      *time.Time `json:"last_used_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ToWebAuthnCredential converts a Passkey to a webauthn.Credential
func (p *Passkey) ToWebAuthnCredential() webauthn.Credential {
	transports := make([]protocol.AuthenticatorTransport, len(p.Transport))
	for i, t := range p.Transport {
		transports[i] = protocol.AuthenticatorTransport(t)
	}

	return webauthn.Credential{
		ID:              p.CredentialID,
		PublicKey:       p.PublicKey,
		AttestationType: p.AttestationType,
		Transport:       transports,
		Authenticator: webauthn.Authenticator{
			AAGUID:    p.AAGUID,
			SignCount: p.SignCount,
		},
	}
}

// PasskeyStore handles passkey persistence
type PasskeyStore struct {
	pool *pgxpool.Pool
}

// NewPasskeyStore creates a new PasskeyStore
func NewPasskeyStore(pool *pgxpool.Pool) *PasskeyStore {
	return &PasskeyStore{pool: pool}
}

// Create creates a new passkey
func (s *PasskeyStore) Create(ctx context.Context, passkey *Passkey) error {
	if passkey.ID == uuid.Nil {
		passkey.ID = uuid.New()
	}
	passkey.CreatedAt = time.Now()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO passkeys (id, user_id, credential_id, public_key, attestation_type, transport, aaguid, sign_count, name, last_used_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, passkey.ID, passkey.UserID, passkey.CredentialID, passkey.PublicKey, passkey.AttestationType, passkey.Transport, passkey.AAGUID, passkey.SignCount, passkey.Name, passkey.LastUsedAt, passkey.CreatedAt)

	return err
}

// GetByID retrieves a passkey by ID
func (s *PasskeyStore) GetByID(ctx context.Context, id uuid.UUID) (*Passkey, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, credential_id, public_key, attestation_type, transport, aaguid, sign_count, name, last_used_at, created_at
		FROM passkeys WHERE id = $1
	`, id)
	return s.scanPasskey(row)
}

// GetByCredentialID retrieves a passkey by credential ID
func (s *PasskeyStore) GetByCredentialID(ctx context.Context, credentialID []byte) (*Passkey, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, credential_id, public_key, attestation_type, transport, aaguid, sign_count, name, last_used_at, created_at
		FROM passkeys WHERE credential_id = $1
	`, credentialID)
	return s.scanPasskey(row)
}

// GetByUserID retrieves all passkeys for a user
func (s *PasskeyStore) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*Passkey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, credential_id, public_key, attestation_type, transport, aaguid, sign_count, name, last_used_at, created_at
		FROM passkeys WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var passkeys []*Passkey
	for rows.Next() {
		p, err := s.scanPasskey(rows)
		if err != nil {
			return nil, err
		}
		passkeys = append(passkeys, p)
	}

	return passkeys, rows.Err()
}

// UpdateSignCount updates the sign count and last used timestamp for a passkey
func (s *PasskeyStore) UpdateSignCount(ctx context.Context, id uuid.UUID, signCount uint32) error {
	now := time.Now()
	_, err := s.pool.Exec(ctx, `
		UPDATE passkeys SET sign_count = $2, last_used_at = $3 WHERE id = $1
	`, id, signCount, now)
	return err
}

// Delete deletes a passkey
func (s *PasskeyStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM passkeys WHERE id = $1`, id)
	return err
}

// CountByUserID returns the number of passkeys for a user
func (s *PasskeyStore) CountByUserID(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM passkeys WHERE user_id = $1`, userID).Scan(&count)
	return count, err
}

// scanPasskey scans a passkey from a database row
func (s *PasskeyStore) scanPasskey(row pgx.Row) (*Passkey, error) {
	var p Passkey
	err := row.Scan(
		&p.ID,
		&p.UserID,
		&p.CredentialID,
		&p.PublicKey,
		&p.AttestationType,
		&p.Transport,
		&p.AAGUID,
		&p.SignCount,
		&p.Name,
		&p.LastUsedAt,
		&p.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
