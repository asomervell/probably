package mcp

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OAuthClient represents a dynamically registered OAuth client
type OAuthClient struct {
	ID           string
	ClientID     string
	ClientSecret string // Optional for public clients
	RedirectURIs []string
	Scopes       []string
	CreatedAt    time.Time
}

// OAuthClientStore manages OAuth clients (Dynamic Client Registration)
type OAuthClientStore struct {
	pool *pgxpool.Pool
}

// NewOAuthClientStore creates a new OAuth client store
func NewOAuthClientStore(pool *pgxpool.Pool) *OAuthClientStore {
	return &OAuthClientStore{pool: pool}
}

// RegisterClient registers a new OAuth client (Dynamic Client Registration)
func (s *OAuthClientStore) RegisterClient(ctx context.Context, redirectURIs []string, scopes []string) (*OAuthClient, error) {
	client := &OAuthClient{
		ID:           uuid.New().String(),
		ClientID:     uuid.New().String(),
		ClientSecret: "", // Public clients don't need secrets (PKCE)
		RedirectURIs: redirectURIs,
		Scopes:       scopes,
		CreatedAt:    time.Now(),
	}

	// Store in database
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oauth_clients (id, client_id, redirect_uris, scopes, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, client.ID, client.ClientID, redirectURIs, scopes, client.CreatedAt)

	if err != nil {
		return nil, err
	}

	return client, nil
}

// GetClient retrieves a client by client_id
func (s *OAuthClientStore) GetClient(ctx context.Context, clientID string) (*OAuthClient, error) {
	var client OAuthClient
	var redirectURIs []string
	var scopes []string

	err := s.pool.QueryRow(ctx, `
		SELECT id, client_id, redirect_uris, scopes, created_at
		FROM oauth_clients
		WHERE client_id = $1
	`, clientID).Scan(&client.ID, &client.ClientID, &redirectURIs, &scopes, &client.CreatedAt)

	if err != nil {
		return nil, err
	}

	client.RedirectURIs = redirectURIs
	client.Scopes = scopes

	return &client, nil
}

// ValidateRedirectURI validates that a redirect URI is registered for the client
func (s *OAuthClientStore) ValidateRedirectURI(ctx context.Context, clientID, redirectURI string) bool {
	client, err := s.GetClient(ctx, clientID)
	if err != nil {
		return false
	}

	for _, uri := range client.RedirectURIs {
		if uri == redirectURI {
			return true
		}
	}

	return false
}
