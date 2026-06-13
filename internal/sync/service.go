package sync

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/asomervell/probably/internal/config"
	"github.com/asomervell/probably/internal/models"
	"github.com/asomervell/probably/internal/sync/providers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncService provides a generic interface for syncing accounts and transactions
// from any supported provider
type SyncService struct {
	registry *providers.Registry
}

// NewSyncService creates a new generic sync service
func NewSyncService(cfg *config.Config, pool *pgxpool.Pool) *SyncService {
	registry := providers.NewRegistry(cfg, pool)

	// Register Teller provider
	if tellerClient, err := NewTellerClient(cfg); err != nil {
		slog.Error("failed to init Teller provider — sync disabled for Teller accounts", "err", err)
	} else if tellerProvider, err := providers.NewTellerProvider(cfg, NewTellerSyncService(pool, tellerClient, cfg)); err != nil {
		slog.Error("failed to create Teller provider", "err", err)
	} else {
		registry.RegisterProvider(providers.ProviderTeller, tellerProvider)
	}

	// Register Akahu provider
	if cfg.AkahuAppID != "" && cfg.AkahuAppSecret != "" {
		if akahuClient, err := NewAkahuClient(cfg); err != nil {
			slog.Error("failed to init Akahu provider — sync disabled for Akahu accounts", "err", err)
		} else if akahuProvider, err := providers.NewAkahuProvider(cfg, NewAkahuSyncService(pool, akahuClient, cfg)); err != nil {
			slog.Error("failed to create Akahu provider", "err", err)
		} else {
			registry.RegisterProvider(providers.ProviderAkahu, akahuProvider)
		}
	}

	// Register Plaid provider
	if cfg.PlaidClientID != "" && cfg.PlaidSecret() != "" {
		if plaidClient, err := NewPlaidClient(cfg); err != nil {
			slog.Error("failed to init Plaid provider — sync disabled for Plaid accounts", "err", err)
		} else if plaidProvider, err := providers.NewPlaidProvider(cfg, NewPlaidSyncService(pool, plaidClient, cfg)); err != nil {
			slog.Error("failed to create Plaid provider", "err", err)
		} else {
			registry.RegisterProvider(providers.ProviderPlaid, plaidProvider)
		}
	}

	return &SyncService{
		registry: registry,
	}
}

// getContraAccounts returns or creates the expense and income accounts for a ledger.
func getContraAccounts(ctx context.Context, store *models.AccountStore, ledgerID uuid.UUID) (*models.Account, *models.Account, error) {
	expense, err := store.GetOrCreateExpenseAccount(ctx, ledgerID)
	if err != nil {
		return nil, nil, err
	}
	income, err := store.GetOrCreateIncomeAccount(ctx, ledgerID)
	if err != nil {
		return nil, nil, err
	}
	return expense, income, nil
}

// SyncTransactions syncs transactions for an account using its provider
func (s *SyncService) SyncTransactions(ctx context.Context, account *models.Account) (int, error) {
	// Determine provider from account
	providerName := providers.ProviderName(account.Provider)
	if providerName == "" {
		// Default to Teller for backward compatibility
		providerName = providers.ProviderTeller
	}

	provider, err := s.registry.GetProvider(providerName)
	if err != nil {
		return 0, fmt.Errorf("failed to get provider %s: %w", providerName, err)
	}

	count, err := provider.SyncTransactions(ctx, account)
	if err != nil {
		return 0, fmt.Errorf("failed to sync transactions: %w", err)
	}

	return count, nil
}

