package providers

import (
	"fmt"

	"github.com/asomervell/probably/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Registry manages provider instances
type Registry struct {
	providers map[ProviderName]Provider
	cfg       *config.Config
	pool      *pgxpool.Pool
}

// RegisterProvider registers a provider instance
// This is used to register providers created outside the registry (e.g., Teller from sync package)
func (r *Registry) RegisterProvider(name ProviderName, provider Provider) {
	if r.providers == nil {
		r.providers = make(map[ProviderName]Provider)
	}
	r.providers[name] = provider
}

// NewRegistry creates a new provider registry
func NewRegistry(cfg *config.Config, pool *pgxpool.Pool) *Registry {
	return &Registry{
		providers: make(map[ProviderName]Provider),
		cfg:       cfg,
		pool:      pool,
	}
}

// GetProvider returns a provider instance for the given provider name
// Providers are created lazily on first access
func (r *Registry) GetProvider(name ProviderName) (Provider, error) {
	// Check if provider already exists
	if provider, ok := r.providers[name]; ok {
		return provider, nil
	}

	// All providers should be registered by sync.NewSyncService
	switch name {
	case ProviderTeller:
		// Teller provider should be registered by sync.NewSyncService
		// If it's not registered, return error
		return nil, fmt.Errorf("Teller provider not registered - call sync.NewSyncService first")

	case ProviderPlaid:
		// Plaid provider should be registered by sync.NewSyncService
		// If it's not registered, return error
		return nil, fmt.Errorf("Plaid provider not registered - call sync.NewSyncService first")

	case ProviderAkahu:
		// Akahu provider should be registered by sync.NewSyncService
		// If it's not registered, return error
		return nil, fmt.Errorf("Akahu provider not registered - call sync.NewSyncService first")

	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

// GetProviderForAccount returns the provider for a given account
func (r *Registry) GetProviderForAccount(providerName string) (Provider, error) {
	name := ProviderName(providerName)
	if name == "" {
		// Default to Teller for backward compatibility
		name = ProviderTeller
	}
	return r.GetProvider(name)
}

// GetAllProviders returns all registered providers
func (r *Registry) GetAllProviders() map[ProviderName]Provider {
	return r.providers
}
