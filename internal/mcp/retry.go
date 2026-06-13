package mcp

import (
	"context"
	"errors"
	"strings"
	"time"
)

// maxToolRetries bounds how many extra attempts a transient tool failure gets.
// Total attempts = maxToolRetries + 1 (so 2 retries = up to 3 attempts).
const maxToolRetries = 2

// retryWithBackoff calls fn up to maxRetries+1 times. When fn returns retry=true,
// it waits with exponential back-off (1s, 2s, 4s, …) before the next attempt.
// A context cancellation during the back-off sleep is returned immediately, so
// the retry budget never exceeds the caller's per-request deadline.
//
// This mirrors the shape of internal/sync/retry.go deliberately rather than
// importing it: that helper is unexported, and keeping a sibling copy avoids
// coupling the MCP server to the bank-sync package.
func retryWithBackoff(ctx context.Context, maxRetries int, fn func() (retry bool, err error)) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(1<<uint(attempt-1)) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		retry, err := fn()
		if err == nil {
			return nil
		}
		if retry {
			lastErr = err
			continue
		}
		return err
	}
	return lastErr
}

// transientErrorTokens are lowercase substrings that mark a tool error as a
// transient downstream condition (network blips, momentary DB unavailability,
// or HTTP 5xx/429 surfaced as text). The set is deliberately conservative:
// deterministic failures (auth, validation, 4xx other than 429, "unknown
// column", "invalid arguments") must NOT match so they are returned on the
// first attempt.
var transientErrorTokens = []string{
	"connection refused",
	"connection reset",
	"connection closed",
	"no such host",
	"i/o timeout",
	"timeout",
	"temporarily unavailable",
	"service unavailable",
	"too many requests",
	"status 500",
	"status 502",
	"status 503",
	"status 504",
	"status 429",
	"status: 500",
	"status: 502",
	"status: 503",
	"status: 504",
	"status: 429",
}

// isTransientToolError reports whether err from a tool execution is transient
// and therefore safe to retry. It is conservative by design — only genuinely
// transient failures qualify:
//
//   - context.DeadlineExceeded (a slow downstream that may recover);
//   - context.Canceled, but only when the parent request ctx is NOT itself done
//     (a cancellation propagated from a sub-operation, not the client/timeout
//     giving up — retrying past a dead parent ctx is pointless);
//   - network/DB connection hiccups and HTTP 5xx/429 patterns matched on the
//     error text via transientErrorTokens.
//
// All read-only financial tools are idempotent, so retrying them is safe.
func isTransientToolError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if errors.Is(err, context.Canceled) {
		// If the parent request context is already done, the deadline/cancel is
		// terminal — do not burn a retry. Only treat as transient when the
		// cancellation came from a recoverable sub-operation.
		return ctx.Err() == nil
	}

	msg := strings.ToLower(err.Error())
	for _, token := range transientErrorTokens {
		if strings.Contains(msg, token) {
			return true
		}
	}

	return false
}
