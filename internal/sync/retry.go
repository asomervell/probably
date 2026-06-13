package sync

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// readHTTPErrorBody reads the body of an error HTTP response and returns it.
// Logs a warning on read failure (partial reads are still returned).
func readHTTPErrorBody(resp *http.Response, provider string) []byte {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("failed to read error response body", "provider", provider, "err", err, "status", resp.Status)
	}
	return body
}

// isTransientHTTPStatus reports whether a raw HTTP status code represents a
// transient provider-side condition that should be retried with back-off.
func isTransientHTTPStatus(code int) bool {
	return code >= 500 || code == 429
}

// IsProviderTransientError reports whether err is a transient error from any
// supported bank sync provider (Teller, Akahu, or Plaid). Callers should log
// a warning and allow the next sync cycle to retry rather than capturing to
// the error tracker.
func IsProviderTransientError(err error) bool {
	if IsTellerTransientError(err) || IsAkahuTransientError(err) {
		return true
	}
	// For Plaid, check structured error types only. IsPlaidTransientError(err, nil)
	// is unsafe here because its nil-httpResp path returns true for any unrecognised
	// error, which would misclassify Teller/Akahu errors as transient.
	_, _, errType := plaidExtractError(err)
	return errType == "API_ERROR" || errType == "RATE_LIMIT_EXCEEDED"
}

// retryWithBackoff calls fn up to maxRetries+1 times. When fn returns retry=true,
// it waits with exponential back-off (1s, 2s, 4s, …) before the next attempt.
// A context cancellation during the back-off sleep is returned immediately.
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
