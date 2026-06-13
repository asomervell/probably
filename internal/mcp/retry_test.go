package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsTransientToolError(t *testing.T) {
	bgCtx := context.Background()

	// A ctx that is already cancelled, used to prove the context.Canceled branch
	// is treated as terminal (not transient) when the parent ctx is done.
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{name: "nil error", ctx: bgCtx, err: nil, want: false},
		{name: "deadline exceeded", ctx: bgCtx, err: context.DeadlineExceeded, want: true},
		{
			name: "wrapped deadline exceeded",
			ctx:  bgCtx,
			err:  fmt.Errorf("calling chat api: %w", context.DeadlineExceeded),
			want: true,
		},
		{
			name: "canceled with live parent is transient",
			ctx:  bgCtx,
			err:  context.Canceled,
			want: true,
		},
		{
			name: "canceled with done parent is terminal",
			ctx:  cancelledCtx,
			err:  context.Canceled,
			want: false,
		},
		{name: "connection refused", ctx: bgCtx, err: errors.New("dial tcp: connection refused"), want: true},
		{name: "connection reset", ctx: bgCtx, err: errors.New("read: connection reset by peer"), want: true},
		{name: "http 503", ctx: bgCtx, err: errors.New("upstream returned status 503"), want: true},
		{name: "http 429", ctx: bgCtx, err: errors.New("rate limited: status 429"), want: true},
		{name: "i/o timeout", ctx: bgCtx, err: errors.New("net/http: i/o timeout"), want: true},
		// Deterministic / non-transient: must NOT retry.
		{name: "validation error", ctx: bgCtx, err: errors.New("invalid arguments: missing field"), want: false},
		{name: "unknown column", ctx: bgCtx, err: errors.New("ERROR: column \"foo\" does not exist (unknown column)"), want: false},
		{name: "auth error", ctx: bgCtx, err: errors.New("status 401 unauthorized"), want: false},
		{name: "not found 404", ctx: bgCtx, err: errors.New("status 404 not found"), want: false},
		{name: "generic error", ctx: bgCtx, err: errors.New("something went wrong"), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTransientToolError(tc.ctx, tc.err); got != tc.want {
				t.Fatalf("isTransientToolError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestRetryWithBackoff_SucceedsAfterTransient(t *testing.T) {
	calls := 0
	transient := errors.New("status 503")

	err := retryWithBackoff(context.Background(), 2, func() (bool, error) {
		calls++
		if calls < 2 {
			return true, transient // retry once
		}
		return false, nil // success
	})

	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestRetryWithBackoff_NonTransientReturnsImmediately(t *testing.T) {
	calls := 0
	deterministic := errors.New("invalid arguments")

	err := retryWithBackoff(context.Background(), 2, func() (bool, error) {
		calls++
		return false, deterministic
	})

	if !errors.Is(err, deterministic) {
		t.Fatalf("expected deterministic error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want exactly 1 (no retry for non-transient)", calls)
	}
}

func TestRetryWithBackoff_CtxCancelledDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	transient := errors.New("status 503")

	// Cancel the ctx shortly after the first attempt fails, while the helper is
	// sleeping in its 1s backoff. The helper must abort and return ctx.Err().
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := retryWithBackoff(ctx, 2, func() (bool, error) {
		calls++
		return true, transient
	})
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (cancelled during first backoff)", calls)
	}
	if elapsed >= time.Second {
		t.Fatalf("helper slept the full backoff (%v); it should abort on ctx cancel", elapsed)
	}
}

// TestProcessToolCall_RetriesTransientThenSucceeds drives the full
// processToolCall retry wrapper: the injected tool fails transiently twice then
// succeeds, and must be invoked exactly 3 times with the success result surfaced.
// This test incurs the real 1s+2s backoff, so it is gated out of -short runs.
func TestProcessToolCall_RetriesTransientThenSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry-backoff timing test in short mode")
	}

	s := newTestServer(t)
	s.requestTimeout = 30 * time.Second

	var calls int32
	s.execTool = func(context.Context, *UserContext, string, json.RawMessage) (map[string]interface{}, error) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			return nil, errors.New("upstream status 503")
		}
		return map[string]interface{}{"ok": true, "attempt": n}, nil
	}

	ctx := withUserContext(context.Background())
	result, rpcErr := s.processToolCall(ctx, "search_transactions", json.RawMessage(`{}`))

	if rpcErr != nil {
		t.Fatalf("expected eventual success, got JSON-RPC error %+v", rpcErr)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("execTool calls = %d, want 3", got)
	}
	if result["ok"] != true {
		t.Fatalf("expected success result, got %+v", result)
	}
}

func TestProcessToolCall_NonTransientNotRetried(t *testing.T) {
	s := newTestServer(t)
	s.requestTimeout = 30 * time.Second

	var calls int32
	s.execTool = func(context.Context, *UserContext, string, json.RawMessage) (map[string]interface{}, error) {
		atomic.AddInt32(&calls, 1)
		return nil, errors.New("unknown column \"foo\"")
	}

	ctx := withUserContext(context.Background())
	_, rpcErr := s.processToolCall(ctx, "search_transactions", json.RawMessage(`{}`))

	if rpcErr == nil {
		t.Fatalf("expected a JSON-RPC error for a deterministic failure")
	}
	if rpcErr.Code != -32603 {
		t.Fatalf("error code = %d, want -32603", rpcErr.Code)
	}
	if rpcErr.Message != "tool execution failed" {
		t.Fatalf("error message = %q, want 'tool execution failed'", rpcErr.Message)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("execTool calls = %d, want exactly 1 (non-transient must not retry)", got)
	}
}

func TestProcessToolCall_RetryStopsOnCancelledCtx(t *testing.T) {
	s := newTestServer(t)
	s.requestTimeout = 30 * time.Second

	ctx, cancel := context.WithCancel(withUserContext(context.Background()))

	var calls int32
	s.execTool = func(context.Context, *UserContext, string, json.RawMessage) (map[string]interface{}, error) {
		atomic.AddInt32(&calls, 1)
		// Fail transiently; cancel the request ctx so the backoff aborts before
		// a second attempt.
		cancel()
		return nil, errors.New("status 503")
	}

	start := time.Now()
	_, rpcErr := s.processToolCall(ctx, "search_transactions", json.RawMessage(`{}`))
	elapsed := time.Since(start)

	if rpcErr == nil {
		t.Fatalf("expected an error after ctx cancellation")
	}
	if got := atomic.LoadInt32(&calls); got > 2 {
		t.Fatalf("execTool calls = %d, want <= 2 (ctx cancel must stop retries)", got)
	}
	if elapsed >= time.Second {
		t.Fatalf("retry slept the full backoff (%v); ctx cancel should abort it", elapsed)
	}
}
