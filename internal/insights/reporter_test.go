package insights

import (
	"testing"

	"github.com/asomervell/probably/internal/config"
)

func TestBuildDefaultProviderChainOrdering(t *testing.T) {
	t.Run("claude is the primary provider when configured", func(t *testing.T) {
		cfg := &config.Config{
			AnthropicAPIKey: "sk-ant-test",
			ClaudeModel:     "claude-sonnet-4-6",
			XAIAPIKey:       "xai-test",
			GrokModel:       "grok-3",
		}
		chain := buildDefaultProviderChain(cfg)
		names := chain.ProviderNames()
		if len(names) == 0 {
			t.Fatal("expected at least one configured provider")
		}
		if names[0] != "claude" {
			t.Errorf("first provider = %q, want claude (primary)", names[0])
		}
	})

	t.Run("empty anthropic key filters claude out", func(t *testing.T) {
		cfg := &config.Config{
			XAIAPIKey:   "xai-test",
			GrokModel:   "grok-3",
			ClaudeModel: "claude-sonnet-4-6",
		}
		chain := buildDefaultProviderChain(cfg)
		for _, n := range chain.ProviderNames() {
			if n == "claude" {
				t.Errorf("claude present in chain despite empty key")
			}
		}
		if !chain.IsConfigured() {
			t.Error("chain should still be configured via grok")
		}
	})
}
