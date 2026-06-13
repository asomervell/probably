package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asomervell/probably/internal/config"
	"github.com/go-chi/chi/v5"
)

// newClientConfigServer builds a *Server with only the collaborators
// handleClientConfig touches: a tool registry and an AuthHandler carrying the
// config that drives URL composition. No DB or OAuth wiring, so a route mounted
// via RegisterRoutes is reachable without an Authorization header.
func newClientConfigServer(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	tools, err := NewToolRegistry()
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	return &Server{
		tools: tools,
		auth:  &AuthHandler{cfg: cfg},
	}
}

func decodeClientConfig(t *testing.T, body map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	v, ok := body[key].(map[string]interface{})
	if !ok {
		t.Fatalf("config[%q] is not an object: %v", key, body[key])
	}
	return v
}

func TestHandleClientConfig_ShapeAndContentType(t *testing.T) {
	s := newClientConfigServer(t, &config.Config{BaseURL: "https://probably.example"})

	req := httptest.NewRequest(http.MethodGet, "/.well-known/mcp-client-config", nil)
	rec := httptest.NewRecorder()
	s.handleClientConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	// server.url must end in /mcp.
	server := decodeClientConfig(t, body, "server")
	serverURL, _ := server["url"].(string)
	if !strings.HasSuffix(serverURL, "/mcp") {
		t.Fatalf("server.url = %q, want suffix /mcp", serverURL)
	}
	if name, _ := server["name"].(string); name != "probably-mcp" {
		t.Fatalf("server.name = %q, want probably-mcp", name)
	}

	// claude_desktop.mcpServers.probably.url must end in /mcp.
	desktop := decodeClientConfig(t, body, "claude_desktop")
	mcpServers := decodeClientConfig(t, desktop, "mcpServers")
	probably := decodeClientConfig(t, mcpServers, "probably")
	if u, _ := probably["url"].(string); !strings.HasSuffix(u, "/mcp") {
		t.Fatalf("claude_desktop.mcpServers.probably.url = %q, want suffix /mcp", u)
	}

	// claude_code_cli must be the literal add command.
	cli, _ := body["claude_code_cli"].(string)
	if want := "claude mcp add probably https://probably.example/mcp"; cli != want {
		t.Fatalf("claude_code_cli = %q, want %q", cli, want)
	}

	// auth.discovery_url must point at the OAuth metadata endpoint.
	auth := decodeClientConfig(t, body, "auth")
	if disc, _ := auth["discovery_url"].(string); !strings.HasSuffix(disc, "/.well-known/oauth-authorization-server") {
		t.Fatalf("auth.discovery_url = %q, want OAuth metadata suffix", disc)
	}
	if typ, _ := auth["type"].(string); typ != "oauth2" {
		t.Fatalf("auth.type = %q, want oauth2", typ)
	}

	// tools length must equal the registry's tool count.
	tools, ok := body["tools"].([]interface{})
	if !ok {
		t.Fatalf("tools is not an array: %v", body["tools"])
	}
	registry, err := NewToolRegistry()
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}
	if len(tools) != len(registry.GetAllTools()) {
		t.Fatalf("tools length = %d, want %d", len(tools), len(registry.GetAllTools()))
	}
	// Each tool entry carries name + description.
	for i, raw := range tools {
		tool, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("tools[%d] is not an object: %v", i, raw)
		}
		if n, _ := tool["name"].(string); n == "" {
			t.Fatalf("tools[%d].name is empty", i)
		}
		if d, _ := tool["description"].(string); d == "" {
			t.Fatalf("tools[%d].description is empty", i)
		}
	}
}

func TestHandleClientConfig_BaseURLComposition(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantURL string
	}{
		{
			name:    "MCPBaseURL wins when set",
			cfg:     &config.Config{BaseURL: "https://app.example", MCPBaseURL: "https://mcp.example"},
			wantURL: "https://mcp.example/mcp",
		},
		{
			name:    "falls back to BaseURL when MCPBaseURL unset",
			cfg:     &config.Config{BaseURL: "https://app.example"},
			wantURL: "https://app.example/mcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newClientConfigServer(t, tt.cfg)

			req := httptest.NewRequest(http.MethodGet, "/.well-known/mcp-client-config", nil)
			rec := httptest.NewRecorder()
			s.handleClientConfig(rec, req)

			var body map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}

			server := decodeClientConfig(t, body, "server")
			if got, _ := server["url"].(string); got != tt.wantURL {
				t.Fatalf("server.url = %q, want %q", got, tt.wantURL)
			}
			auth := decodeClientConfig(t, body, "auth")
			wantDisc := strings.TrimSuffix(tt.wantURL, "/mcp") + "/.well-known/oauth-authorization-server"
			if got, _ := auth["discovery_url"].(string); got != wantDisc {
				t.Fatalf("auth.discovery_url = %q, want %q", got, wantDisc)
			}
		})
	}
}

// TestHandleClientConfig_NoAuthRequired confirms the route is registered outside
// any auth middleware group, like the OAuth metadata endpoints.
func TestHandleClientConfig_NoAuthRequired(t *testing.T) {
	s := newClientConfigServer(t, &config.Config{BaseURL: "https://probably.example"})

	r := chi.NewRouter()
	s.RegisterRoutes(r, nil)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	for _, path := range []string{
		"/.well-known/mcp-client-config",
		"/mcp/.well-known/mcp-client-config",
	} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		func() {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s status = %d, want 200 (unauthenticated)", path, resp.StatusCode)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("GET %s Content-Type = %q, want application/json", path, ct)
			}
		}()
	}
}
