package processing

// NonRetryableError marks errors that must not be retried (e.g., permission denied).
// Callers check via IsNonRetryableError.
type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string { return e.Err.Error() }
func (e *NonRetryableError) Unwrap() error { return e.Err }

// retryable is satisfied by errors that carry an explicit retryability flag
// (e.g., *orchestrator.OrchestratorError). Checking via this interface avoids
// an import cycle between the processing and orchestrator packages.
type retryable interface {
	IsRetryable() bool
}

// IsNonRetryableError reports whether err is definitely not retryable.
// It recognises both *NonRetryableError and any error whose IsRetryable()
// method returns false (e.g., *orchestrator.OrchestratorError).
func IsNonRetryableError(err error) bool {
	if _, ok := err.(*NonRetryableError); ok {
		return true
	}
	if r, ok := err.(retryable); ok {
		return !r.IsRetryable()
	}
	return false
}
