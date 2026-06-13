package config

import "testing"

func TestLoadAnthropicFields(t *testing.T) {
	t.Run("defaults when unset", func(t *testing.T) {
		cfg := Load()
		if cfg.AnthropicAPIKey != "" {
			t.Errorf("AnthropicAPIKey = %q, want empty", cfg.AnthropicAPIKey)
		}
		if cfg.ClaudeModel != "claude-sonnet-4-6" {
			t.Errorf("ClaudeModel = %q, want claude-sonnet-4-6", cfg.ClaudeModel)
		}
		if cfg.DemoMode {
			t.Errorf("DemoMode = true, want false")
		}
	})

	t.Run("ANTHROPIC_API_KEY is trimmed", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "  test-key  ")
		cfg := Load()
		if cfg.AnthropicAPIKey != "test-key" {
			t.Errorf("AnthropicAPIKey = %q, want test-key", cfg.AnthropicAPIKey)
		}
	})

	t.Run("CLAUDE_MODEL override", func(t *testing.T) {
		t.Setenv("CLAUDE_MODEL", "claude-opus-4")
		cfg := Load()
		if cfg.ClaudeModel != "claude-opus-4" {
			t.Errorf("ClaudeModel = %q, want claude-opus-4", cfg.ClaudeModel)
		}
	})

	t.Run("DEMO_MODE bool parsing", func(t *testing.T) {
		for _, v := range []string{"true", "1", "yes"} {
			t.Setenv("DEMO_MODE", v)
			cfg := Load()
			if !cfg.DemoMode {
				t.Errorf("DEMO_MODE=%q: DemoMode = false, want true", v)
			}
		}
	})
}

func TestLoadDefaultModel(t *testing.T) {
	t.Run("defaults to Anthropic Claude when unset", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "test-key")
		cfg := Load()
		if cfg.LLMDefaultModel != "anthropic/claude-sonnet-4-6" {
			t.Errorf("LLMDefaultModel = %q, want anthropic/claude-sonnet-4-6", cfg.LLMDefaultModel)
		}
	})

	t.Run("default uses CLAUDE_MODEL override", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "test-key")
		t.Setenv("CLAUDE_MODEL", "claude-opus-4-8")
		cfg := Load()
		if cfg.LLMDefaultModel != "anthropic/claude-opus-4-8" {
			t.Errorf("LLMDefaultModel = %q, want anthropic/claude-opus-4-8", cfg.LLMDefaultModel)
		}
	})

	t.Run("explicit LLM_DEFAULT_MODEL wins over Anthropic default", func(t *testing.T) {
		t.Setenv("LLM_DEFAULT_MODEL", "google/gemini-2.5-flash")
		cfg := Load()
		if cfg.LLMDefaultModel != "google/gemini-2.5-flash" {
			t.Errorf("LLMDefaultModel = %q, want google/gemini-2.5-flash (explicit wins)", cfg.LLMDefaultModel)
		}
	})

	t.Run("explicit non-Anthropic provider is preserved", func(t *testing.T) {
		t.Setenv("LLM_DEFAULT_MODEL", "xai/grok-3")
		cfg := Load()
		if cfg.LLMDefaultModel != "xai/grok-3" {
			t.Errorf("LLMDefaultModel = %q, want xai/grok-3", cfg.LLMDefaultModel)
		}
	})
}
