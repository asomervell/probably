package providers

import (
	"context"
	"fmt"
	"log/slog"
)

// ProviderChain manages a fallback chain of LLM providers
type ProviderChain struct {
	providers []InsightProvider
}

// NewProviderChain creates a new provider chain with the given providers in priority order
func NewProviderChain(providers []InsightProvider) *ProviderChain {
	// Filter to only configured providers
	var configured []InsightProvider
	for _, p := range providers {
		if p.IsConfigured() {
			configured = append(configured, p)
			slog.Info("insights provider configured", "provider", p.Name())
		}
	}

	if len(configured) == 0 {
		slog.Warn("no insights providers configured")
	}

	return &ProviderChain{
		providers: configured,
	}
}

// IsConfigured returns true if at least one provider is available
func (c *ProviderChain) IsConfigured() bool {
	return len(c.providers) > 0
}

// ProviderNames returns the names of the configured providers in priority order.
func (c *ProviderChain) ProviderNames() []string {
	names := make([]string, len(c.providers))
	for i, p := range c.providers {
		names[i] = p.Name()
	}
	return names
}

// GenerateReport tries each provider in order until one succeeds
func (c *ProviderChain) GenerateReport(ctx context.Context, req ReportRequest) (*ReportResponse, string, string, error) {
	if len(c.providers) == 0 {
		return nil, "", "", fmt.Errorf("no insight providers configured")
	}

	var lastErr error
	for _, provider := range c.providers {
		slog.DebugContext(ctx, "attempting report generation", "provider", provider.Name())

		resp, err := provider.GenerateReport(ctx, req)
		if err == nil {
			slog.DebugContext(ctx, "report generated", "provider", provider.Name())
			return resp, provider.Name(), getModelName(provider), nil
		}

		lastErr = err
		slog.WarnContext(ctx, "insights provider failed, trying next", "provider", provider.Name(), "err", err)
	}

	return nil, "", "", fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// AnalyzeTransaction tries each provider in order until one succeeds
func (c *ProviderChain) AnalyzeTransaction(ctx context.Context, req TransactionRequest) (*TransactionInsight, string, string, error) {
	if len(c.providers) == 0 {
		return nil, "", "", fmt.Errorf("no insight providers configured")
	}

	var lastErr error
	for _, provider := range c.providers {
		slog.DebugContext(ctx, "attempting transaction analysis", "provider", provider.Name())

		insight, err := provider.AnalyzeTransaction(ctx, req)
		if err == nil {
			slog.DebugContext(ctx, "transaction analyzed", "provider", provider.Name())
			return insight, provider.Name(), getModelName(provider), nil
		}

		lastErr = err
		slog.WarnContext(ctx, "insights provider failed, trying next", "provider", provider.Name(), "err", err)
	}

	return nil, "", "", fmt.Errorf("all providers failed, last error: %w", lastErr)
}

// getModelName extracts model name from provider if available
func getModelName(provider InsightProvider) string {
	type modelNamer interface {
		ModelName() string
	}
	if mn, ok := provider.(modelNamer); ok {
		return mn.ModelName()
	}
	return ""
}
