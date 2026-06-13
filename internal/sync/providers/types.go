package providers

import (
	"context"
	"errors"

	"github.com/asomervell/probably/internal/models"
	"github.com/google/uuid"
)

// ProviderName represents a bank connection provider
type ProviderName string

const (
	ProviderTeller ProviderName = "teller"
	ProviderPlaid  ProviderName = "plaid"
	ProviderAkahu  ProviderName = "akahu"
)

// Credentials represents provider-specific authentication credentials
// Each provider will have different credential structures
type Credentials interface {
	Provider() ProviderName
	AccessToken() string
	IsValid() bool
}

// ProviderImpl is the shared interface that sync-package implementations must satisfy.
// Defined here (not per-provider) to avoid repeating identical method sets.
type ProviderImpl interface {
	SyncAccountsWithToken(ctx context.Context, ledgerID uuid.UUID, accessToken string) ([]*models.Account, error)
	SyncTransactionsForAccount(ctx context.Context, account *models.Account) (int, error)
}

// Provider is the interface that all bank connection providers must implement
type Provider interface {
	// Name returns the provider name
	Name() ProviderName

	// SyncAccounts syncs accounts from the provider for a given connection
	SyncAccounts(ctx context.Context, ledgerID uuid.UUID, credentials Credentials) ([]*models.Account, error)

	// SyncTransactions syncs transactions for a specific account
	SyncTransactions(ctx context.Context, account *models.Account) (int, error)
}

// ConnectionDisconnectedError is an interface for errors that indicate
// a connection needs re-authentication
type ConnectionDisconnectedError interface {
	error
	IsConnectionDisconnected() bool
	Provider() ProviderName
}

// noCredentialsError is returned when an account has no access token.
// It implements ConnectionDisconnectedError so the sync worker marks the
// account as disconnected rather than counting it as an application error.
type noCredentialsError struct {
	providerName ProviderName
}

func (e *noCredentialsError) Error() string                { return "no access token for account" }
func (e *noCredentialsError) IsConnectionDisconnected() bool { return true }
func (e *noCredentialsError) Provider() ProviderName        { return e.providerName }

// syncTransactionsImpl is shared by all provider SyncTransactions methods.
// It guards on an empty access token, then delegates to the impl.
// Callers that need pre-processing (e.g. Teller field sync) do so before calling this.
func syncTransactionsImpl(ctx context.Context, account *models.Account, impl ProviderImpl) (int, error) {
	if account.AccessToken == "" {
		return 0, &noCredentialsError{providerName: ProviderName(account.Provider)}
	}
	return impl.SyncTransactionsForAccount(ctx, account)
}

// IsConnectionDisconnectedError checks if an error indicates a connection needs re-authentication.
// Uses errors.As to handle wrapped errors from service/provider layers.
func IsConnectionDisconnectedError(err error) bool {
	if err == nil {
		return false
	}
	var connErr ConnectionDisconnectedError
	if errors.As(err, &connErr) {
		return connErr.IsConnectionDisconnected()
	}
	return false
}

