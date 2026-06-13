package providers

import "testing"

func TestNewClaudeProvider(t *testing.T) {
	t.Run("empty key is not configured", func(t *testing.T) {
		p := NewClaudeProvider("", "")
		if p.IsConfigured() {
			t.Error("IsConfigured() = true, want false for empty key")
		}
		if p.Name() != "claude" {
			t.Errorf("Name() = %q, want claude", p.Name())
		}
	})

	t.Run("configured with key", func(t *testing.T) {
		p := NewClaudeProvider("sk-ant-xxx", "claude-3-5-sonnet-20241022")
		if !p.IsConfigured() {
			t.Error("IsConfigured() = false, want true")
		}
		if p.Name() != "claude" {
			t.Errorf("Name() = %q, want claude", p.Name())
		}
		if p.ModelName() != "claude-3-5-sonnet-20241022" {
			t.Errorf("ModelName() = %q, want claude-3-5-sonnet-20241022", p.ModelName())
		}
	})

	t.Run("empty model uses default", func(t *testing.T) {
		p := NewClaudeProvider("sk-ant-xxx", "")
		if p.ModelName() != "claude-3-5-sonnet-20241022" {
			t.Errorf("ModelName() = %q, want default claude-3-5-sonnet-20241022", p.ModelName())
		}
	})
}
