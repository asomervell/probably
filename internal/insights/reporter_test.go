package insights

import (
	"testing"

	"github.com/asomervell/probably/internal/config"
)

func TestBuildDefaultProviderChainOrdering(t *testing.T) {
	t.Run("demo mode puts claude first", func(t *testing.T) {
		cfg := &config.Config{
			DemoMode:        true,
			AnthropicAPIKey: "sk-ant-test",
			ClaudeModel:     "claude-3-5-sonnet-20241022",
			XAIAPIKey:       "xai-test",
			GrokModel:       "grok-3",
		}
		chain := buildDefaultProviderChain(cfg)
		names := chain.ProviderNames()
		if len(names) == 0 {
			t.Fatal("expected at least one configured provider")
		}
		if names[0] != "claude" {
			t.Errorf("first provider = %q, want claude (demo mode)", names[0])
		}
	})

	t.Run("non-demo mode keeps claude as fallback", func(t *testing.T) {
		cfg := &config.Config{
			DemoMode:        false,
			AnthropicAPIKey: "sk-ant-test",
			ClaudeModel:     "claude-3-5-sonnet-20241022",
			XAIAPIKey:       "xai-test",
			GrokModel:       "grok-3",
		}
		chain := buildDefaultProviderChain(cfg)
		names := chain.ProviderNames()
		if len(names) == 0 {
			t.Fatal("expected at least one configured provider")
		}
		if names[0] == "claude" {
			t.Errorf("first provider = claude, want a non-claude provider first (non-demo mode)")
		}
		if names[len(names)-1] != "claude" {
			t.Errorf("last provider = %q, want claude (fallback)", names[len(names)-1])
		}
	})

	t.Run("empty anthropic key filters claude out", func(t *testing.T) {
		cfg := &config.Config{
			DemoMode:    true,
			XAIAPIKey:   "xai-test",
			GrokModel:   "grok-3",
			ClaudeModel: "claude-3-5-sonnet-20241022",
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
