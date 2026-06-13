package providers

import (
	"context"
	"fmt"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// TellerCredentials implements the Credentials interface for Teller
type TellerCredentials struct {
	AccessTokenValue string
}

func (c *TellerCredentials) Provider() ProviderName {
	return ProviderTeller
}

func (c *TellerCredentials) AccessToken() string {
	return c.AccessTokenValue
}

func (c *TellerCredentials) IsValid() bool {
	return c.AccessTokenValue != ""
}

// TellerProvider implements the Provider interface for Teller
type TellerProvider struct {
	tellerImpl ProviderImpl
	cfg        *config.Config
}

// NewTellerProvider creates a new Teller provider.
// tellerImpl is created in the sync package to avoid import cycles.
func NewTellerProvider(cfg *config.Config, tellerImpl ProviderImpl) (*TellerProvider, error) {
	if tellerImpl == nil {
		return nil, fmt.Errorf("tellerImpl cannot be nil")
	}

	return &TellerProvider{
		tellerImpl: tellerImpl,
		cfg:        cfg,
	}, nil
}

// Name returns the provider name
func (p *TellerProvider) Name() ProviderName {
	return ProviderTeller
}

// SyncAccounts syncs accounts from Teller
func (p *TellerProvider) SyncAccounts(ctx context.Context, ledgerID uuid.UUID, credentials Credentials) ([]*models.Account, error) {
	tellerCreds, ok := credentials.(*TellerCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type for Teller provider")
	}

	accounts, err := p.tellerImpl.SyncAccountsWithToken(ctx, ledgerID, tellerCreds.AccessTokenValue)
	if err != nil {
		return nil, err
	}

	// Update accounts to use provider-agnostic fields
	for _, acc := range accounts {
		// Map Teller fields to generic fields if not already set
		if acc.TellerAccountID != "" && acc.ExternalAccountID == "" {
			acc.ExternalAccountID = acc.TellerAccountID
		}
		if acc.TellerEnrollmentID != "" && acc.ConnectionID == "" {
			acc.ConnectionID = acc.TellerEnrollmentID
		}
		if acc.TellerAccessToken != "" && acc.AccessToken == "" {
			acc.AccessToken = acc.TellerAccessToken
		}
		if acc.TellerSubtype != "" && acc.AccountSubtype == "" {
			acc.AccountSubtype = acc.TellerSubtype
		}
		if acc.TellerStatus != "" && acc.AccountStatus == "" {
			acc.AccountStatus = acc.TellerStatus
		}
	}

	return accounts, nil
}

// syncTellerFields keeps generic and Teller-specific account fields in sync.
// Accounts loaded from DB already have both sets populated from the same column,
// but callers constructing accounts in memory may only set one side.
func syncTellerFields(account *models.Account) {
	if account.AccessToken == "" {
		account.AccessToken = account.TellerAccessToken
	}
	if account.TellerAccessToken == "" {
		account.TellerAccessToken = account.AccessToken
	}
	if account.ExternalAccountID == "" {
		account.ExternalAccountID = account.TellerAccountID
	}
	if account.TellerAccountID == "" {
		account.TellerAccountID = account.ExternalAccountID
	}
}

// SyncTransactions syncs transactions for an account
func (p *TellerProvider) SyncTransactions(ctx context.Context, account *models.Account) (int, error) {
	syncTellerFields(account)
	return syncTransactionsImpl(ctx, account, p.tellerImpl)
}


