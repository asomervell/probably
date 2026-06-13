package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestHandleMCPRoot_ToolExceedingTimeoutReturns32000(t *testing.T) {
	s := newTestServer(t)
	s.requestTimeout = 50 * time.Millisecond
	// A tool that blocks until the bounded ctx is cancelled, simulating a hung
	// downstream. It must not pin the goroutine: the timeout cancels ctx.
	s.execTool = func(ctx context.Context, _ *UserContext, _ string, _ json.RawMessage) (map[string]interface{}, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	body := `{"jsonrpc":"2.0","id":99,"method":"tools/call","params":{"name":"search_transactions","arguments":{}}}`

	start := time.Now()
	rec, resp := postMCPRoot(t, s, body, withUserContext(context.Background()))
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200", rec.Code)
	}
	if resp.Error == nil {
		t.Fatalf("expected a timeout JSON-RPC error, got result %+v", resp.Result)
	}
	if resp.Error.Code != -32000 {
		t.Fatalf("error code = %d, want -32000", resp.Error.Code)
	}
	if resp.Error.Message != "request timeout" {
		t.Fatalf("error message = %q, want 'request timeout'", resp.Error.Message)
	}
	if got := toJSONNumberInt(t, resp.ID); got != 99 {
		t.Fatalf("response id = %v, want 99", resp.ID)
	}
	if resp.Result != nil {
		t.Fatalf("result must be nil on timeout, got %+v", resp.Result)
	}
	// The handler should return shortly after the deadline, not hang.
	if elapsed > 2*time.Second {
		t.Fatalf("handler took %v, expected it to return promptly after the 50ms deadline", elapsed)
	}
}

func TestHandleMCPRoot_FastToolFinishesWithinBudget(t *testing.T) {
	s := newTestServer(t)
	s.requestTimeout = 5 * time.Second
	s.execTool = func(context.Context, *UserContext, string, json.RawMessage) (map[string]interface{}, error) {
		return map[string]interface{}{"ok": true, "fast": true}, nil
	}

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_transactions","arguments":{}}}`

	_, resp := postMCPRoot(t, s, body, withUserContext(context.Background()))

	if resp.Error != nil {
		t.Fatalf("unexpected error for a fast tool: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("result shape = %T, want map", resp.Result)
	}
	if result["fast"] != true {
		t.Fatalf("expected fast tool result to round-trip, got %+v", result)
	}
}

func TestRequestTimeoutFromEnv(t *testing.T) {
	tests := []struct {
		name string
		set  bool
		val  string
		want time.Duration
	}{
		{name: "unset uses default", set: false, want: defaultRequestTimeout},
		{name: "valid override", set: true, val: "1s", want: time.Second},
		{name: "valid sub-second override", set: true, val: "250ms", want: 250 * time.Millisecond},
		{name: "empty uses default", set: true, val: "", want: defaultRequestTimeout},
		{name: "garbage uses default", set: true, val: "not-a-duration", want: defaultRequestTimeout},
		{name: "non-positive uses default", set: true, val: "0s", want: defaultRequestTimeout},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("MCP_REQUEST_TIMEOUT", tc.val)
			} else {
				// Ensure the var is not present for the unset case.
				t.Setenv("MCP_REQUEST_TIMEOUT", "")
				_ = os.Unsetenv("MCP_REQUEST_TIMEOUT")
			}
			if got := requestTimeoutFromEnv(); got != tc.want {
				t.Fatalf("requestTimeoutFromEnv() = %v, want %v", got, tc.want)
			}
		})
	}
}
