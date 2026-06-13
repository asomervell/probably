package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Token represents an OAuth access token
type Token struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	ClientID     string
	Scopes       []string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	CreatedAt    time.Time
	APIKey       string
}

// TokenStore manages OAuth tokens in the database
type TokenStore struct {
	pool    *pgxpool.Pool
	apiKeys *models.APIKeyStore
}

// NewTokenStore creates a new OAuth token store
func NewTokenStore(pool *pgxpool.Pool, apiKeys *models.APIKeyStore) *TokenStore {
	return &TokenStore{
		pool:    pool,
		apiKeys: apiKeys,
	}
}

// CreateToken creates a new OAuth access token
func (s *TokenStore) CreateToken(ctx context.Context, userID uuid.UUID, clientID string, scopes []string, expiresIn int) (*Token, error) {
	now := time.Now().UTC()
	token := &Token{
		ID:        uuid.New(),
		UserID:    userID,
		ClientID:  clientID,
		Scopes:    scopes,
		ExpiresAt: now.Add(time.Duration(expiresIn) * time.Second),
		CreatedAt: now,
	}

	accessTokenBytes := make([]byte, 64)
	if _, err := rand.Read(accessTokenBytes); err != nil {
		return nil, err
	}
	token.AccessToken = "prob_oauth_" + base64.RawURLEncoding.EncodeToString(accessTokenBytes)

	refreshTokenBytes := make([]byte, 64)
	if _, err := rand.Read(refreshTokenBytes); err != nil {
		return nil, err
	}
	token.RefreshToken = "prob_refresh_" + base64.RawURLEncoding.EncodeToString(refreshTokenBytes)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO oauth_tokens (id, user_id, client_id, scopes, access_token, refresh_token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, token.ID, token.UserID, token.ClientID, scopes, token.AccessToken, token.RefreshToken, token.ExpiresAt, token.CreatedAt)
	if err != nil {
		return nil, err
	}

	apiKey, err := s.ensureAPIKeyMapping(ctx, token)
	if err != nil {
		return nil, err
	}
	token.APIKey = apiKey

	return token, nil
}

// ValidateToken validates an access token and returns the token info
func (s *TokenStore) ValidateToken(ctx context.Context, accessToken string) (*Token, error) {
	var token Token
	var scopes []string

	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, client_id, scopes, refresh_token, expires_at, created_at
		FROM oauth_tokens
		WHERE access_token = $1 AND expires_at > (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')
	`, accessToken).Scan(&token.ID, &token.UserID, &token.ClientID, &scopes, &token.RefreshToken, &token.ExpiresAt, &token.CreatedAt)
	if err != nil {
		return nil, err
	}

	token.AccessToken = accessToken
	token.Scopes = scopes

	apiKey, err := s.ensureAPIKeyMapping(ctx, &token)
	if err != nil {
		return nil, err
	}
	token.APIKey = apiKey

	return &token, nil
}

// DeleteToken deletes an access token
func (s *TokenStore) DeleteToken(ctx context.Context, accessToken string) error {
	if err := s.deleteAPIKeyMapping(ctx, accessToken); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM oauth_tokens WHERE access_token = $1`, accessToken)
	return err
}

// DeleteUserTokens deletes all tokens for a user
func (s *TokenStore) DeleteUserTokens(ctx context.Context, userID uuid.UUID) error {
	if err := s.deleteAPIKeysForUser(ctx, userID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM oauth_tokens WHERE user_id = $1`, userID)
	return err
}

func (s *TokenStore) ensureAPIKeyMapping(ctx context.Context, token *Token) (string, error) {
	if s.apiKeys == nil {
		return "", fmt.Errorf("api key store not configured")
	}

	var plaintext string
	err := s.pool.QueryRow(ctx, `
		SELECT api_key_plaintext
		FROM oauth_token_api_keys
		WHERE token_id = $1
	`, token.ID).Scan(&plaintext)
	if err == nil {
		return plaintext, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}

	name := fmt.Sprintf("ChatGPT OAuth %s", token.ClientID)
	plaintext, apiKey, err := models.GenerateAPIKey(token.UserID, name)
	if err != nil {
		return "", err
	}
	if err := s.apiKeys.Create(ctx, apiKey); err != nil {
		return "", err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO oauth_token_api_keys (token_id, api_key_id, api_key_plaintext)
		VALUES ($1, $2, $3)
		ON CONFLICT (token_id) DO UPDATE SET api_key_id = EXCLUDED.api_key_id, api_key_plaintext = EXCLUDED.api_key_plaintext
	`, token.ID, apiKey.ID, plaintext)
	if err != nil {
		return "", err
	}

	return plaintext, nil
}

func (s *TokenStore) deleteAPIKeyMapping(ctx context.Context, accessToken string) error {
	if s.apiKeys == nil {
		return nil
	}

	var apiKeyID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		DELETE FROM oauth_token_api_keys
		WHERE token_id = (SELECT id FROM oauth_tokens WHERE access_token = $1)
		RETURNING api_key_id
	`, accessToken).Scan(&apiKeyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	return s.apiKeys.Delete(ctx, apiKeyID)
}

func (s *TokenStore) deleteAPIKeysForUser(ctx context.Context, userID uuid.UUID) error {
	if s.apiKeys == nil {
		return nil
	}

	rows, err := s.pool.Query(ctx, `
		DELETE FROM oauth_token_api_keys
		WHERE token_id IN (SELECT id FROM oauth_tokens WHERE user_id = $1)
		RETURNING api_key_id
	`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var apiKeyID uuid.UUID
		if err := rows.Scan(&apiKeyID); err != nil {
			return err
		}
		if err := s.apiKeys.Delete(ctx, apiKeyID); err != nil {
			return err
		}
	}

	return rows.Err()
}
