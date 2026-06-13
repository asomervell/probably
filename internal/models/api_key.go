package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// APIKeyPrefix is prepended to all generated API keys
	APIKeyPrefix = "prob_"
	// APIKeyLength is the number of random bytes in the key (32 bytes = 64 hex chars)
	APIKeyLength = 32
)

type APIKey struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"` // First 8 chars for identification
	KeyHash    string     `json:"-"`          // SHA-256 hash, never exposed
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// GenerateAPIKey creates a new random API key and returns both the plaintext key
// (to show to user once) and the APIKey struct with the hash (for storage)
func GenerateAPIKey(userID uuid.UUID, name string) (plaintext string, key *APIKey, err error) {
	// Generate random bytes
	randomBytes := make([]byte, APIKeyLength)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, err
	}

	// Create the plaintext key: prefix + hex encoded random bytes
	plaintext = APIKeyPrefix + hex.EncodeToString(randomBytes)

	// Hash the key for storage
	keyHash := HashAPIKey(plaintext)

	// Create the key prefix (first 8 chars after the prefix for identification)
	keyPrefix := plaintext[:len(APIKeyPrefix)+8]

	key = &APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
		CreatedAt: time.Now(),
	}

	return plaintext, key, nil
}

// HashAPIKey computes the SHA-256 hash of a plaintext API key
func HashAPIKey(plaintext string) string {
	hash := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(hash[:])
}

type APIKeyStore struct {
	pool *pgxpool.Pool
}

func NewAPIKeyStore(pool *pgxpool.Pool) *APIKeyStore {
	return &APIKeyStore{pool: pool}
}

func (s *APIKeyStore) Create(ctx context.Context, key *APIKey) error {
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO api_keys (id, user_id, name, key_prefix, key_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, key.ID, key.UserID, key.Name, key.KeyPrefix, key.KeyHash, key.CreatedAt)

	return err
}

func (s *APIKeyStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM api_keys WHERE id = $1`, id)
	return err
}

func (s *APIKeyStore) GetByID(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	var k APIKey
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, name, key_prefix, key_hash, last_used_at, created_at
		FROM api_keys WHERE id = $1
	`, id).Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.LastUsedAt, &k.CreatedAt)

	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (s *APIKeyStore) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*APIKey, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, name, key_prefix, key_hash, last_used_at, created_at
		FROM api_keys WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.LastUsedAt, &k.CreatedAt); err != nil {
			return nil, err
		}
		keys = append(keys, &k)
	}

	return keys, rows.Err()
}

// GetByKeyHash looks up an API key by its hash (used for authentication)
func (s *APIKeyStore) GetByKeyHash(ctx context.Context, keyHash string) (*APIKey, error) {
	var k APIKey
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, name, key_prefix, key_hash, last_used_at, created_at
		FROM api_keys WHERE key_hash = $1
	`, keyHash).Scan(&k.ID, &k.UserID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.LastUsedAt, &k.CreatedAt)

	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ValidateAndGetUser validates a plaintext API key and returns the associated user ID
// Also updates last_used_at timestamp
func (s *APIKeyStore) ValidateAndGetUser(ctx context.Context, plaintext string) (*APIKey, error) {
	keyHash := HashAPIKey(plaintext)

	key, err := s.GetByKeyHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}

	// Update last used timestamp
	if _, err = s.pool.Exec(ctx, `
		UPDATE api_keys SET last_used_at = $1 WHERE id = $2
	`, time.Now(), key.ID); err != nil {
		slog.WarnContext(ctx, "failed to update api key last_used_at", "key_id", key.ID, "err", err)
	}

	return key, nil
}
