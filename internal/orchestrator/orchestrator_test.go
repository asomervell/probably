package orchestrator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/asomervell/probably/internal/config"
)

func TestNewOrchestratorAnthropicWiring(t *testing.T) {
	cfg := &config.Config{
		LLMDefaultModel: "anthropic/claude-3-5-sonnet-20241022",
		AnthropicAPIKey: "sk-ant-test",
	}

	orch, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}
	if orch == nil {
		t.Fatal("NewOrchestrator returned nil")
	}
	if got := orch.endpoints[ProviderAnthropic]; got != "https://api.anthropic.com/v1" {
		t.Errorf("endpoints[anthropic] = %q, want https://api.anthropic.com/v1", got)
	}
	if got := orch.getAPIKey(ProviderAnthropic); got != "sk-ant-test" {
		t.Errorf("getAPIKey(anthropic) = %q, want sk-ant-test", got)
	}
}

func TestCallOpenAIAPIAnthropicRequestShape(t *testing.T) {
	var (
		gotAuth   string
		gotPath   string
		gotMethod string
		gotModel  string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method

		body, _ := io.ReadAll(r.Body)
		var parsed struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &parsed)
		gotModel = parsed.Model

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{}"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		LLMDefaultModel: "anthropic/claude-3-5-sonnet-20241022",
		AnthropicAPIKey: "sk-ant-test",
	}
	orch, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}
	// Point the Anthropic endpoint at the test server (test seam).
	orch.endpoints[ProviderAnthropic] = srv.URL

	spec := &ModelSpec{Provider: ProviderAnthropic, Model: "claude-3-5-sonnet-20241022"}
	messages := []LLMMessage{{Role: "user", Content: "hello"}}

	resp, err := orch.callOpenAIAPI(context.Background(), spec, messages, false)
	if err != nil {
		t.Fatalf("callOpenAIAPI error: %v", err)
	}
	if resp == nil || len(resp.Choices) == 0 {
		t.Fatal("expected a non-empty response")
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer sk-ant-test" {
		t.Errorf("Authorization = %q, want Bearer sk-ant-test", gotAuth)
	}
	if gotModel != "claude-3-5-sonnet-20241022" {
		t.Errorf("model = %q, want claude-3-5-sonnet-20241022", gotModel)
	}
}
