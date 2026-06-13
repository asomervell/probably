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

func TestSupportsVisionAnthropicDefault(t *testing.T) {
	cfg := &config.Config{
		LLMDefaultModel: "anthropic/claude-sonnet-4-6",
		AnthropicAPIKey: "sk-ant-test",
	}
	orch, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}
	if !orch.SupportsVision() {
		t.Error("SupportsVision() = false, want true for Anthropic default")
	}
	if m := orch.models[RoleVision]; m == nil || m.Provider != ProviderAnthropic {
		t.Errorf("vision role = %+v, want Anthropic provider", m)
	}
}

func TestCallVisionAnthropicRequestShape(t *testing.T) {
	var (
		gotPath    string
		gotAPIKey  string
		gotVersion string
		gotModel   string
		blockTypes []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")

		body, _ := io.ReadAll(r.Body)
		var parsed struct {
			Model    string `json:"model"`
			Messages []struct {
				Content []struct {
					Type string `json:"type"`
				} `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &parsed)
		gotModel = parsed.Model
		if len(parsed.Messages) > 0 {
			for _, b := range parsed.Messages[0].Content {
				blockTypes = append(blockTypes, b.Type)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"{\"transactions\":[]}"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		LLMDefaultModel: "anthropic/claude-sonnet-4-6",
		AnthropicAPIKey: "sk-ant-test",
	}
	orch, err := NewOrchestrator(cfg)
	if err != nil {
		t.Fatalf("NewOrchestrator error: %v", err)
	}
	orch.endpoints[ProviderAnthropic] = srv.URL // test seam

	t.Run("pdf is sent as a document block", func(t *testing.T) {
		blockTypes = nil
		resp, err := orch.CallVision(context.Background(), &VisionRequest{
			Prompt:   "extract",
			Document: []byte("%PDF-1.4 fake"),
			MimeType: "application/pdf",
		})
		if err != nil {
			t.Fatalf("CallVision error: %v", err)
		}
		if gotPath != "/messages" {
			t.Errorf("path = %q, want /messages", gotPath)
		}
		if gotAPIKey != "sk-ant-test" {
			t.Errorf("x-api-key = %q, want sk-ant-test", gotAPIKey)
		}
		if gotVersion == "" {
			t.Error("anthropic-version header missing")
		}
		if gotModel != "claude-sonnet-4-6" {
			t.Errorf("model = %q, want claude-sonnet-4-6", gotModel)
		}
		if len(blockTypes) != 2 || blockTypes[0] != "document" || blockTypes[1] != "text" {
			t.Errorf("content blocks = %v, want [document text]", blockTypes)
		}
		if resp.Content != `{"transactions":[]}` {
			t.Errorf("Content = %q", resp.Content)
		}
		if resp.TokensUsed != 15 {
			t.Errorf("TokensUsed = %d, want 15", resp.TokensUsed)
		}
	})

	t.Run("image is sent as an image block", func(t *testing.T) {
		blockTypes = nil
		_, err := orch.CallVision(context.Background(), &VisionRequest{
			Prompt:   "extract",
			Document: []byte("\x89PNG fake"),
			MimeType: "image/png",
		})
		if err != nil {
			t.Fatalf("CallVision error: %v", err)
		}
		if len(blockTypes) != 2 || blockTypes[0] != "image" || blockTypes[1] != "text" {
			t.Errorf("content blocks = %v, want [image text]", blockTypes)
		}
	})
}
