package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// newTestServer builds a *Server wired only with the stateless collaborators
// that the JSON-RPC dispatch path needs (tool registry, validation, audit, and
// a zero-value context handler whose GetContext just reads the request ctx key).
// It deliberately avoids any DB/OAuth dependency: execTool and checkAccess are
// injected so envelope/timeout/retry behaviour is unit-testable in isolation.
// Callers override execTool / checkAccess / requestTimeout as needed.
func newTestServer(t *testing.T) *Server {
	t.Helper()

	tools, err := NewToolRegistry()
	if err != nil {
		t.Fatalf("NewToolRegistry: %v", err)
	}

	s := &Server{
		tools:          tools,
		context:        &ContextHandler{},
		validation:     NewValidationHandler(),
		audit:          NewAuditLogger(),
		requestTimeout: defaultRequestTimeout,
		// Default: no tool is actually executed and access is always granted.
		// Tests that exercise tool calls override these.
		execTool: func(context.Context, *UserContext, string, json.RawMessage) (map[string]interface{}, error) {
			return map[string]interface{}{"ok": true}, nil
		},
		checkAccess: func(context.Context, uuid.UUID) (bool, error) {
			return true, nil
		},
	}
	return s
}

// withUserContext returns a context carrying a fully-scoped UserContext so
// processToolCall passes its auth/scope gates without the OAuth middleware.
func withUserContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, userContextKey, &UserContext{
		UserID:   uuid.New(),
		LedgerID: uuid.New(),
		APIKey:   "test-key",
		Scopes: []string{
			"read:transactions",
			"read:accounts",
			"read:financial",
			"read:patterns",
		},
	})
}

// postMCPRoot drives handleMCPRoot with a raw JSON body and returns the decoded
// JSON-RPC response. ctx (if non-nil) is attached to the request so tests can
// inject a user context. envelope-failure cases pass ctx=nil because dispatch is
// never reached.
func postMCPRoot(t *testing.T, s *Server, body string, ctx context.Context) (*httptest.ResponseRecorder, jsonRPCResponse) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	rec := httptest.NewRecorder()
	s.handleMCPRoot(rec, req)

	var resp jsonRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%q)", err, rec.Body.String())
	}
	return rec, resp
}

func TestValidateRPCEnvelope(t *testing.T) {
	tests := []struct {
		name     string
		req      jsonRPCRequest
		wantNil  bool
		wantCode int
	}{
		{
			name:    "valid no params",
			req:     jsonRPCRequest{JSONRPC: "2.0", Method: "ping"},
			wantNil: true,
		},
		{
			name:    "valid with params",
			req:     jsonRPCRequest{JSONRPC: "2.0", Method: "tools/call", Params: json.RawMessage(`{"name":"x"}`)},
			wantNil: true,
		},
		{
			name:     "missing jsonrpc",
			req:      jsonRPCRequest{Method: "ping"},
			wantCode: -32600,
		},
		{
			name:     "wrong jsonrpc version",
			req:      jsonRPCRequest{JSONRPC: "1.0", Method: "ping"},
			wantCode: -32600,
		},
		{
			name:     "empty method",
			req:      jsonRPCRequest{JSONRPC: "2.0", Method: ""},
			wantCode: -32600,
		},
		{
			name:     "malformed params",
			req:      jsonRPCRequest{JSONRPC: "2.0", Method: "tools/call", Params: json.RawMessage(`{invalid`)},
			wantCode: -32700,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateRPCEnvelope(tc.req)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil error, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected error code %d, got nil", tc.wantCode)
			}
			if got.Code != tc.wantCode {
				t.Fatalf("error code = %d, want %d", got.Code, tc.wantCode)
			}
		})
	}
}

// dispatchCalled wires a Server whose dispatchable methods are observable: it
// returns a server plus a pointer flag set true only if a real method runs.
// Envelope rejections must leave the flag false.
func serverThatRecordsDispatch(t *testing.T, dispatched *bool) *Server {
	t.Helper()
	s := newTestServer(t)
	// "ping" is a no-side-effect dispatch target; we detect dispatch by swapping
	// execTool, but ping doesn't call it. Instead we observe via a custom method
	// path: any reach into dispatchJSONRPC for "tools/call" hits execTool.
	s.execTool = func(context.Context, *UserContext, string, json.RawMessage) (map[string]interface{}, error) {
		*dispatched = true
		return map[string]interface{}{"ok": true}, nil
	}
	return s
}

func TestHandleMCPRoot_EnvelopeRejectedBeforeDispatch(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
	}{
		{
			name:     "missing jsonrpc field",
			body:     `{"id":1,"method":"tools/call","params":{"name":"search_transactions","arguments":{}}}`,
			wantCode: -32600,
		},
		{
			name:     "wrong jsonrpc version",
			body:     `{"jsonrpc":"1.0","id":2,"method":"tools/call","params":{"name":"search_transactions","arguments":{}}}`,
			wantCode: -32600,
		},
		{
			name:     "empty method",
			body:     `{"jsonrpc":"2.0","id":3,"method":""}`,
			wantCode: -32600,
		},
		{
			name:     "malformed params",
			body:     `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{invalid}`,
			wantCode: -32700,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dispatched := false
			s := serverThatRecordsDispatch(t, &dispatched)

			rec, resp := postMCPRoot(t, s, tc.body, withUserContext(context.Background()))

			if rec.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d, want 200 (JSON-RPC errors ride on 200)", rec.Code)
			}
			if resp.Error == nil {
				t.Fatalf("expected JSON-RPC error, got result %+v", resp.Result)
			}
			if resp.Error.Code != tc.wantCode {
				t.Fatalf("error code = %d, want %d", resp.Error.Code, tc.wantCode)
			}
			if dispatched {
				t.Fatalf("dispatchJSONRPC ran for a malformed envelope; it must be rejected first")
			}
		})
	}
}

func TestHandleMCPRoot_EmptyMethodMentionsMethodRequiredAndPreservesID(t *testing.T) {
	s := newTestServer(t)
	body := `{"jsonrpc":"2.0","id":"abc-123","method":""}`

	_, resp := postMCPRoot(t, s, body, withUserContext(context.Background()))

	if resp.Error == nil || resp.Error.Code != -32600 {
		t.Fatalf("want -32600 error, got %+v", resp.Error)
	}
	data, ok := resp.Error.Data.(string)
	if !ok || !strings.Contains(data, "method is required") {
		t.Fatalf("error data = %v, want mention of 'method is required'", resp.Error.Data)
	}
	if resp.ID != "abc-123" {
		t.Fatalf("response id = %v, want abc-123", resp.ID)
	}
}

func TestHandleMCPRoot_HappyPathDispatches(t *testing.T) {
	dispatched := false
	s := serverThatRecordsDispatch(t, &dispatched)

	// A well-formed tools/call with valid args for search_transactions, which
	// requires no specific fields (query/limit are optional in its schema).
	body := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"search_transactions","arguments":{}}}`

	_, resp := postMCPRoot(t, s, body, withUserContext(context.Background()))

	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error on happy path: %+v", resp.Error)
	}
	if !dispatched {
		t.Fatalf("expected dispatch to reach execTool on a well-formed request")
	}
	if got := toJSONNumberInt(t, resp.ID); got != 7 {
		t.Fatalf("response id = %v, want 7", resp.ID)
	}
}

// toJSONNumberInt extracts an int from a JSON-decoded numeric id (float64).
func toJSONNumberInt(t *testing.T, v interface{}) int {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("id is not numeric: %T", v)
	}
	return int(f)
}

// sanity: ensure the default request timeout constant is sane so dependent
// tests that rely on it (timeout_test.go) have a stable baseline.
func TestDefaultRequestTimeoutIsPositive(t *testing.T) {
	if defaultRequestTimeout <= 0 || defaultRequestTimeout > time.Minute {
		t.Fatalf("defaultRequestTimeout = %v, want a sane positive duration", defaultRequestTimeout)
	}
}
