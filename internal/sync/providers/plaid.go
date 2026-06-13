package providers

import (
	"context"
	"fmt"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// PlaidCredentials implements the Credentials interface for Plaid
type PlaidCredentials struct {
	AccessTokenValue string
	ItemID           string // Plaid item ID (connection ID)
}

func (c *PlaidCredentials) Provider() ProviderName {
	return ProviderPlaid
}

func (c *PlaidCredentials) AccessToken() string {
	return c.AccessTokenValue
}

func (c *PlaidCredentials) IsValid() bool {
	return c.AccessTokenValue != "" && c.ItemID != ""
}

// PlaidProvider implements the Provider interface for Plaid
type PlaidProvider struct {
	plaidImpl ProviderImpl
	cfg       *config.Config
}

// NewPlaidProvider creates a new Plaid provider.
// plaidImpl is created in the sync package to avoid import cycles.
func NewPlaidProvider(cfg *config.Config, plaidImpl ProviderImpl) (*PlaidProvider, error) {
	if plaidImpl == nil {
		return nil, fmt.Errorf("plaidImpl cannot be nil")
	}

	return &PlaidProvider{
		plaidImpl: plaidImpl,
		cfg:       cfg,
	}, nil
}

// Name returns the provider name
func (p *PlaidProvider) Name() ProviderName {
	return ProviderPlaid
}

// SyncAccounts syncs accounts from Plaid
func (p *PlaidProvider) SyncAccounts(ctx context.Context, ledgerID uuid.UUID, credentials Credentials) ([]*models.Account, error) {
	plaidCreds, ok := credentials.(*PlaidCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type for Plaid provider")
	}

	accounts, err := p.plaidImpl.SyncAccountsWithToken(ctx, ledgerID, plaidCreds.AccessTokenValue)
	if err != nil {
		return nil, err
	}

	// Ensure all accounts have the provider set and use consistent field mapping
	for _, acc := range accounts {
		if acc.Provider == "" {
			acc.Provider = "plaid"
		}
		// Ensure ConnectionID is set from ItemID if not already set
		if acc.ConnectionID == "" && plaidCreds.ItemID != "" {
			acc.ConnectionID = plaidCreds.ItemID
		}
	}

	return accounts, nil
}

// SyncTransactions syncs transactions for an account
func (p *PlaidProvider) SyncTransactions(ctx context.Context, account *models.Account) (int, error) {
	return syncTransactionsImpl(ctx, account, p.plaidImpl)
}


