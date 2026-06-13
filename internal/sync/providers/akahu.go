package providers

import (
	"context"
	"fmt"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// AkahuCredentials implements the Credentials interface for Akahu
type AkahuCredentials struct {
	AccessTokenValue string
	UserID           string // Akahu user ID (optional)
}

func (c *AkahuCredentials) Provider() ProviderName {
	return ProviderAkahu
}

func (c *AkahuCredentials) AccessToken() string {
	return c.AccessTokenValue
}

func (c *AkahuCredentials) IsValid() bool {
	return c.AccessTokenValue != ""
}

// AkahuProvider implements the Provider interface for Akahu
type AkahuProvider struct {
	akahuImpl ProviderImpl
	cfg       *config.Config
}

// NewAkahuProvider creates a new Akahu provider.
// akahuImpl is created in the sync package to avoid import cycles.
func NewAkahuProvider(cfg *config.Config, akahuImpl ProviderImpl) (*AkahuProvider, error) {
	if akahuImpl == nil {
		return nil, fmt.Errorf("akahuImpl cannot be nil")
	}

	return &AkahuProvider{
		akahuImpl: akahuImpl,
		cfg:       cfg,
	}, nil
}

// Name returns the provider name
func (p *AkahuProvider) Name() ProviderName {
	return ProviderAkahu
}

// SyncAccounts syncs accounts from Akahu
func (p *AkahuProvider) SyncAccounts(ctx context.Context, ledgerID uuid.UUID, credentials Credentials) ([]*models.Account, error) {
	akahuCreds, ok := credentials.(*AkahuCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type for Akahu provider")
	}

	accounts, err := p.akahuImpl.SyncAccountsWithToken(ctx, ledgerID, akahuCreds.AccessTokenValue)
	if err != nil {
		return nil, err
	}

	for _, acc := range accounts {
		if acc.Provider == "" {
			acc.Provider = "akahu"
		}
	}

	return accounts, nil
}

// SyncTransactions syncs transactions for an account
func (p *AkahuProvider) SyncTransactions(ctx context.Context, account *models.Account) (int, error) {
	return syncTransactionsImpl(ctx, account, p.akahuImpl)
}


