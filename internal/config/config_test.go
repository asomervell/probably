package config

import "testing"

func TestLoadAnthropicFields(t *testing.T) {
	t.Run("defaults when unset", func(t *testing.T) {
		cfg := Load()
		if cfg.AnthropicAPIKey != "" {
			t.Errorf("AnthropicAPIKey = %q, want empty", cfg.AnthropicAPIKey)
		}
		if cfg.ClaudeModel != "claude-3-5-sonnet-20241022" {
			t.Errorf("ClaudeModel = %q, want claude-3-5-sonnet-20241022", cfg.ClaudeModel)
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

func TestLoadDemoModeDefaultModel(t *testing.T) {
	t.Run("demo mode sets default model when unset", func(t *testing.T) {
		t.Setenv("DEMO_MODE", "true")
		t.Setenv("ANTHROPIC_API_KEY", "test-key")
		cfg := Load()
		if cfg.LLMDefaultModel != "anthropic/claude-3-5-sonnet-20241022" {
			t.Errorf("LLMDefaultModel = %q, want anthropic/claude-3-5-sonnet-20241022", cfg.LLMDefaultModel)
		}
	})

	t.Run("demo mode uses CLAUDE_MODEL for default", func(t *testing.T) {
		t.Setenv("DEMO_MODE", "true")
		t.Setenv("ANTHROPIC_API_KEY", "test-key")
		t.Setenv("CLAUDE_MODEL", "claude-opus-4")
		cfg := Load()
		if cfg.LLMDefaultModel != "anthropic/claude-opus-4" {
			t.Errorf("LLMDefaultModel = %q, want anthropic/claude-opus-4", cfg.LLMDefaultModel)
		}
	})

	t.Run("explicit user value wins over demo default", func(t *testing.T) {
		t.Setenv("DEMO_MODE", "true")
		t.Setenv("ANTHROPIC_API_KEY", "test-key")
		t.Setenv("LLM_DEFAULT_MODEL", "google/gemini-2.5-flash")
		cfg := Load()
		if cfg.LLMDefaultModel != "google/gemini-2.5-flash" {
			t.Errorf("LLMDefaultModel = %q, want google/gemini-2.5-flash (explicit wins)", cfg.LLMDefaultModel)
		}
	})

	t.Run("no demo mode leaves default model unchanged", func(t *testing.T) {
		t.Setenv("LLM_DEFAULT_MODEL", "xai/grok-3")
		cfg := Load()
		if cfg.LLMDefaultModel != "xai/grok-3" {
			t.Errorf("LLMDefaultModel = %q, want xai/grok-3", cfg.LLMDefaultModel)
		}
	})

	t.Run("no demo mode, no default model stays empty", func(t *testing.T) {
		cfg := Load()
		if cfg.LLMDefaultModel != "" {
			t.Errorf("LLMDefaultModel = %q, want empty", cfg.LLMDefaultModel)
		}
	})
}
